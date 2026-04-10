# flake.nix
#
# Nix Flake for clickhouse-shard-health.
#
# ═══════════════════════════════════════════════════════════════════════════════
# FLAKE OUTPUTS OVERVIEW
# ═══════════════════════════════════════════════════════════════════════════════
#
# This flake provides:
#   - packages.*    : Binary and OCI container
#   - checks.*      : CI checks (linting, testing, security scanning)
#   - devShells.*   : Development environment with all tools
#
# ═══════════════════════════════════════════════════════════════════════════════
# CI CHECKS (nix flake check)
# ═══════════════════════════════════════════════════════════════════════════════
#
# Run all checks:
#   nix flake check
#
# Run individual checks:
#   nix build .#checks.x86_64-linux.<check-name>
#
# Available checks:
#
#   -- Go Analysis ──────────────────────────────────────────────────────────────
#   go-vet                       Go built-in static analysis
#   go-sec                       Security scanner (gosec)
#
#   -- Linting Tiers ────────────────────────────────────────────────────────────
#   golangci-lint-quick          Tier 0 (~30s): gofmt, govet, errcheck, ineffassign, unused
#   golangci-lint                Tier 1 (~2min): Tier 0 + staticcheck, gosec, gocritic, revive, noctx, durationcheck
#   golangci-lint-comprehensive  Tier 2 (~10min): Tier 1 + exhaustive, prealloc, gocyclo, funlen, goconst, dupl, misspell
#
#   -- Testing ──────────────────────────────────────────────────────────────────
#   go-test                      Unit and integration tests
#
#   -- Nix Validation ───────────────────────────────────────────────────────────
#   nix-fmt                      Nix file formatting check
#   flake-valid                  Flake schema validation
#
# ═══════════════════════════════════════════════════════════════════════════════
# PACKAGES
# ═══════════════════════════════════════════════════════════════════════════════
#
# Binary variants (3 sizes × native + cross targets):
#
#   nix build                                         Default (stripped+UPX, smallest)
#   nix build .#clickhouse-shard-health-stripped       Stripped only (no UPX)
#   nix build .#clickhouse-shard-health-debug          Unstripped (full debug symbols)
#
# Container variants:
#   nix build .#container                              Default (stripped+UPX)
#   nix build .#container-stripped                     Stripped only
#   nix build .#container-debug                        Unstripped
#
# Cross-compilation (x86_64-linux only, same 3 variants per target):
#   nix build .#cross-aarch64-linux                    ARM64 (stripped+UPX)
#   nix build .#cross-aarch64-linux-stripped            ARM64 (stripped)
#   nix build .#cross-aarch64-linux-debug               ARM64 (debug)
#   nix build .#container-aarch64-linux                ARM64 container (stripped+UPX)
#   nix build .#cross-riscv64-linux                    RISC-V 64 (stripped+UPX)
#   ... (same pattern for all targets)
#
# Container usage:
#   nix build .#container
#   docker load < ./result
#   docker run --rm -p 9363:9363 clickhouse-shard-health:<version>
#
# ═══════════════════════════════════════════════════════════════════════════════
# DEVELOPMENT
# ═══════════════════════════════════════════════════════════════════════════════
#
#   nix develop                  Enter dev shell with all tools
#   nix flake check              Run all CI checks
#
{
  description = "Lightweight sidecar that monitors ClickHouse cluster health and exposes Prometheus metrics";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs { inherit system; };
        lib = pkgs.lib;

        version = self.shortRev or self.dirtyShortRev or "dev";
        src = lib.cleanSource ./.;

        # Single source of truth for Go module dependency hash
        vendorHash = "sha256-VsfdO6CyFzhjafuOuXYgS6YJaRrZiU+2JVk35u2ZrKg=";

        # Single source of truth for Go version
        goPackage = pkgs.go_1_26;

        # ─── Binary & container variants (debug, stripped, upx) ─────────
        native = import ./nix/variants.nix {
          inherit
            pkgs
            lib
            src
            version
            vendorHash
            ;
          pkgsNative = pkgs;
        };

        # ─── Cross-compilation (x86_64-linux → foreign architectures) ──
        crossTargetDefs = {
          aarch64-linux = {
            crossSystem = "aarch64-unknown-linux-gnu";
            arch = "aarch64";
            qemuBin = "qemu-aarch64";
          };
          riscv64-linux = {
            crossSystem = "riscv64-unknown-linux-gnu";
            arch = "riscv64";
            qemuBin = "qemu-riscv64";
          };
        };

        crossTargets = lib.optionalAttrs (system == "x86_64-linux") (
          builtins.mapAttrs (
            name: def:
            import ./nix/variants.nix {
              pkgs = import nixpkgs {
                localSystem = system;
                crossSystem.config = def.crossSystem;
              };
              pkgsNative = pkgs;
              inherit
                lib
                src
                version
                vendorHash
                ;
            }
          ) crossTargetDefs
        );

        crossPackages = lib.concatMapAttrs (name: target: {
          "cross-${name}" = target.package;
          "cross-${name}-stripped" = target.package-stripped;
          "cross-${name}-debug" = target.package-debug;
          "container-${name}" = target.container;
          "container-${name}-stripped" = target.container-stripped;
          "container-${name}-debug" = target.container-debug;
        }) crossTargets;

        crossChecks = lib.concatMapAttrs (
          name: target:
          let
            def = crossTargetDefs.${name};
          in
          {
            # Smoke test uses stripped variant — UPX binaries don't run under QEMU
            "cross-smoke-${name}" = import ./nix/tests/cross-smoke.nix {
              inherit pkgs;
              package = target.package-stripped;
              inherit (def) arch qemuBin;
            };
          }
        ) crossTargets;

      in
      {
        # ─── Packages ────────────────────────────────────────────────────
        packages = {
          default = native.package;
          clickhouse-shard-health = native.package;
          clickhouse-shard-health-stripped = native.package-stripped;
          clickhouse-shard-health-debug = native.package-debug;
          container = native.container;
          container-stripped = native.container-stripped;
          container-debug = native.container-debug;
        }
        // crossPackages;

        # ─── Development Shell ───────────────────────────────────────────
        devShells.default = import ./nix/shell.nix { inherit pkgs goPackage; };

        # ─── Checks ─────────────────────────────────────────────────────
        checks =
          import ./nix/checks.nix {
            inherit
              pkgs
              lib
              vendorHash
              goPackage
              ;
            src = ./.;
          }
          // crossChecks;

        # ─── Formatter ───────────────────────────────────────────────────
        formatter = pkgs.nixfmt-rfc-style;
      }
    );
}

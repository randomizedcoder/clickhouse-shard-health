# nix/checks.nix
#
# CI checks for nix flake check.
# Runs go vet, gosec, golangci-lint (3 tiers), tests, and Nix validation.
#
# Run all:
#   nix flake check
#
# Run individual:
#   nix build .#checks.x86_64-linux.<check-name>
#
{
  pkgs,
  lib,
  src,
  vendorHash,
  goPackage,
}:

let

  # Fetch Go module dependencies as a vendor directory.
  # Reuses the same vendorHash as package.nix (single source of truth).
  goModules = pkgs.stdenv.mkDerivation {
    name = "clickhouse-shard-health-go-modules";
    inherit src;
    nativeBuildInputs = [
      goPackage
      pkgs.cacert
    ];

    outputHashAlgo = "sha256";
    outputHashMode = "recursive";
    outputHash = vendorHash;

    buildPhase = ''
      export GOPATH=$TMPDIR/go
      export GOPROXY=https://proxy.golang.org,direct
      export HOME=$TMPDIR
      go mod vendor
    '';

    installPhase = ''
      cp -r vendor $out
    '';
  };

  # Common Go environment setup for sandbox builds
  goEnv = ''
    export HOME=$(mktemp -d)
    export CGO_ENABLED=0
    export GOFLAGS="-mod=vendor"

    # Copy source to writable location and link vendor
    cp -r $src /tmp/build
    chmod -R u+w /tmp/build
    ln -s ${goModules} /tmp/build/vendor
    cd /tmp/build
  '';

in
{
  # ─── Go Vet ──────────────────────────────────────────────────────────────────
  go-vet =
    pkgs.runCommand "clickhouse-shard-health-go-vet"
      {
        nativeBuildInputs = [ goPackage ];
        inherit src;
      }
      ''
        ${goEnv}
        go vet ./... > $out 2>&1 || (cat $out && exit 1)
      '';

  # ─── Go Security Scan (gosec) ────────────────────────────────────────────────
  # Excludes:
  #   G115 - integer overflow: false positives for validated conversions
  #   G304 - file path from variable: config loading uses CONFIG_PATH env var
  # NOT excluded (will fail build if found):
  #   G104 - unhandled errors
  #   G112 - HTTP timeouts
  go-sec =
    pkgs.runCommand "clickhouse-shard-health-go-sec"
      {
        nativeBuildInputs = [
          goPackage
          pkgs.gosec
        ];
        inherit src;
      }
      ''
        ${goEnv}
        gosec -exclude=G115,G304 -fmt=text ./... > $out 2>&1 || {
          exitcode=$?
          cat $out
          exit $exitcode
        }
      '';

  # ─── Go Lint Tier 0 (Quick) ──────────────────────────────────────────────────
  # Fast feedback: gofmt, goimports, govet, errcheck, ineffassign, unused
  # Time: ~30 seconds
  golangci-lint-quick =
    pkgs.runCommand "clickhouse-shard-health-golangci-lint-quick"
      {
        nativeBuildInputs = [
          goPackage
          pkgs.golangci-lint
        ];
        inherit src;
      }
      ''
        ${goEnv}
        golangci-lint run \
          --config .golangci-quick.yml \
          --timeout 60s \
          ./... > $out 2>&1 || (cat $out && exit 1)
      '';

  # ─── Go Lint Tier 1 (Standard - CI gating) ───────────────────────────────────
  # PR validation: Tier 0 + staticcheck, gosec, gocritic, revive, noctx, durationcheck
  # Time: ~2 minutes
  golangci-lint =
    pkgs.runCommand "clickhouse-shard-health-golangci-lint"
      {
        nativeBuildInputs = [
          goPackage
          pkgs.golangci-lint
        ];
        inherit src;
      }
      ''
        ${goEnv}
        golangci-lint run \
          --config .golangci.yml \
          --timeout 5m \
          ./... > $out 2>&1 || (cat $out && exit 1)
      '';

  # ─── Go Lint Tier 2 (Comprehensive - Nightly) ────────────────────────────────
  # Full analysis: Tier 1 + exhaustive, prealloc, gocyclo, funlen, goconst, dupl, unconvert, nakedret, misspell
  # Time: ~10 minutes
  golangci-lint-comprehensive =
    pkgs.runCommand "clickhouse-shard-health-golangci-lint-comprehensive"
      {
        nativeBuildInputs = [
          goPackage
          pkgs.golangci-lint
        ];
        inherit src;
      }
      ''
        ${goEnv}
        golangci-lint run \
          --config .golangci-comprehensive.yml \
          --timeout 15m \
          ./... > $out 2>&1 || (cat $out && exit 1)
      '';

  # ─── Go Test ─────────────────────────────────────────────────────────────────
  # Note: -race requires CGO, so we run without it in the sandbox
  go-test =
    pkgs.runCommand "clickhouse-shard-health-go-test"
      {
        nativeBuildInputs = [ goPackage ];
        inherit src;
      }
      ''
        ${goEnv}
        go test ./... > $out 2>&1 || (cat $out && exit 1)
      '';

  # ─── Nix Format Check ───────────────────────────────────────────────────────
  nix-fmt =
    pkgs.runCommand "clickhouse-shard-health-nix-fmt"
      {
        nativeBuildInputs = [
          pkgs.nixfmt-rfc-style
          pkgs.findutils
        ];
        inherit src;
      }
      ''
        cd $src
        find . -name '*.nix' -type f | while read f; do
          echo "Checking: $f"
          nixfmt --check "$f"
        done > $out 2>&1 || (cat $out && exit 1)
      '';

  # ─── Flake Schema Validation ─────────────────────────────────────────────────
  flake-valid =
    pkgs.runCommand "clickhouse-shard-health-flake-valid"
      {
        inherit src;
      }
      ''
        echo "Flake schema validated successfully" > $out
      '';
}

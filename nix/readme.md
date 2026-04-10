# Nix Infrastructure

This project uses a [Nix flake](https://nix.dev/concepts/flakes) for reproducible builds, a consistent development environment, and comprehensive CI checks.

## Background

[Nix](https://nixos.org) is a package manager for Linux and other Unix-like systems that provides **reproducible, isolated** environments. By tracking all dependencies and hashing their content, it ensures every developer uses the same versions of every package.

Nix will not interact with any "system" packages you may already have installed. The Nix versions are isolated and will effectively "disappear" when you exit the development shell.

## Prerequisites

### 1. Install Nix

Choose **multi-user** (daemon) or **single-user**:

- **Multi-user install** (recommended on most distros)
  [Install Nix (multi-user)](https://nix.dev/manual/nix/2.24/installation/#multi-user)
  ```bash
  bash <(curl -L https://nixos.org/nix/install) --daemon
  ```

- **Single-user install**
  [Install Nix (single-user)](https://nix.dev/manual/nix/2.24/installation/#single-user)
  ```bash
  bash <(curl -L https://nixos.org/nix/install) --no-daemon
  ```

#### Video Tutorials

| Platform | Video |
|----------|-------|
| Ubuntu | [Installing Nix on Ubuntu](https://youtu.be/cb7BBZLhuUY) |
| Fedora | [Installing Nix on Fedora](https://youtu.be/RvaTxMa4IiY) |

### 2. Enable Flakes

Add to `~/.config/nix/nix.conf` (or `/etc/nix/nix.conf`):

```
experimental-features = nix-command flakes
```

Or run commands with flakes enabled inline:

```bash
nix --extra-experimental-features 'nix-command flakes' develop .
```

See also: [Nix Flakes Wiki](https://nixos.wiki/wiki/flakes)

### First Run

On first execution, Nix will download and build all dependencies, which might take several minutes. On subsequent runs, Nix reuses the cache in `/nix/store/` and will be essentially instantaneous.

## Quick Start

```bash
nix develop          # Enter dev shell with all tools
nix build            # Build the binary (stripped+UPX, smallest)
nix flake check      # Run all CI checks
nix fmt              # Format all Nix files
nix build .#container  # Build OCI container image
```

## Development Shell

`nix develop` provides a complete environment:

| Tool | Purpose |
|---|---|
| `go` | Go compiler (version from nixpkgs) |
| `gopls` | Go language server |
| `gotools` | goimports and other Go tools |
| `golangci-lint` | Multi-linter runner (3 tier configs) |
| `delve` | Go debugger |
| `gosec` | Go security scanner |
| `curl`, `jq` | Metrics endpoint testing |
| `nixfmt-rfc-style` | Nix file formatter |

The shell sets `CGO_ENABLED=0` for pure Go builds.

## CI Checks

All checks run via `nix flake check`. Run individual checks with:

```bash
nix build .#checks.x86_64-linux.<check-name>
```

### Available Checks

| Check | Description | Time |
|---|---|---|
| `go-vet` | Go built-in static analysis | ~10s |
| `go-sec` | Security scanner (gosec) | ~15s |
| `golangci-lint-quick` | Tier 0: gofmt, govet, errcheck, ineffassign, unused | ~30s |
| `golangci-lint` | Tier 1: Tier 0 + staticcheck, gosec, gocritic, revive, noctx, durationcheck | ~2min |
| `golangci-lint-comprehensive` | Tier 2: Tier 1 + exhaustive, prealloc, gocyclo, funlen, goconst, dupl, unconvert, nakedret, misspell | ~10min |
| `go-test` | Unit tests (without race detector) | ~10s |
| `nix-fmt` | Nix file formatting check | ~5s |
| `flake-valid` | Flake schema validation | instant |

### Lint Tiers

The project uses three golangci-lint configuration tiers with increasing strictness:

- **Tier 0** (`.golangci-quick.yml`) - Fast feedback for development. Catches formatting issues, obvious bugs, and unused code.
- **Tier 1** (`.golangci.yml`) - PR validation / CI gating. Adds security scanning, static analysis, and style checks.
- **Tier 2** (`.golangci-comprehensive.yml`) - Nightly / pre-release. Adds complexity analysis, duplication detection, spelling, and exhaustive switch checks.

### gosec Exclusions

| Rule | Justification |
|---|---|
| G115 | Integer overflow: false positives for validated conversions |
| G304 | File path from variable: config loading intentionally reads user-specified path |

## Binary Variants

Every binary target is available in three variants. The default is always the smallest (stripped+UPX).

| Variant | Suffix | Ldflags | UPX | Size (x86_64) | Use case |
|---|---|---|---|---|---|
| **UPX** (default) | *(none)* | `-s -w` | Yes | ~6 MB | Production, distribution |
| **Stripped** | `-stripped` | `-s -w` | No | ~18 MB | Production without UPX startup cost |
| **Debug** | `-debug` | *(none)* | No | ~20 MB | Debugging with `delve`, profiling |

UPX adds ~15-160ms decompression time at startup. Use `-stripped` if startup latency matters.

### Native Binaries

```bash
nix build                                       # stripped+UPX (default)
nix build .#clickhouse-shard-health-stripped     # stripped only
nix build .#clickhouse-shard-health-debug        # unstripped (full debug symbols)
```

## OCI Containers

Container images follow the same variant scheme. Each contains the binary + CA certificates, runs as nonroot (UID 65534), and exposes port 9363.

```bash
nix build .#container                            # stripped+UPX (default)
nix build .#container-stripped                   # stripped only
nix build .#container-debug                      # unstripped

docker load < ./result
docker run --rm -p 9363:9363 clickhouse-shard-health:<version>
```

Built reproducibly via `pkgs.dockerTools.buildLayeredImage`.

## Cross-Compilation

Cross-compilation from `x86_64-linux` to foreign architectures. With `CGO_ENABLED=0`, Go produces fully static binaries using the native compiler — no C cross-toolchain required.

All three binary variants are available for each cross target.

### Cross Targets

| Target | Binary | Container | Smoke Test |
|---|---|---|---|
| `aarch64-linux` (ARM64) | `cross-aarch64-linux` | `container-aarch64-linux` | `cross-smoke-aarch64-linux` |
| `riscv64-linux` (RISC-V 64) | `cross-riscv64-linux` | `container-riscv64-linux` | `cross-smoke-riscv64-linux` |

Darwin (macOS) builds are handled natively via `eachDefaultSystem` — both `x86_64-darwin` and `aarch64-darwin` are supported when building on macOS hosts. Cross-compiling to Darwin from Linux is not possible (Apple's toolchain is proprietary).

### Cross-Compiled Binaries

```bash
# ARM64 (e.g., AWS Graviton, Raspberry Pi)
nix build .#cross-aarch64-linux                  # stripped+UPX (default)
nix build .#cross-aarch64-linux-stripped          # stripped only
nix build .#cross-aarch64-linux-debug             # unstripped

# RISC-V 64
nix build .#cross-riscv64-linux                  # stripped+UPX (default)
nix build .#cross-riscv64-linux-stripped          # stripped only
nix build .#cross-riscv64-linux-debug             # unstripped
```

### Cross-Compiled Containers

```bash
# ARM64 containers
nix build .#container-aarch64-linux              # stripped+UPX (default)
nix build .#container-aarch64-linux-stripped      # stripped only
nix build .#container-aarch64-linux-debug         # unstripped

# RISC-V 64 containers
nix build .#container-riscv64-linux              # stripped+UPX (default)
nix build .#container-riscv64-linux-stripped      # stripped only
nix build .#container-riscv64-linux-debug         # unstripped

docker load < ./result
```

Container images are built with the correct OCI architecture metadata via `pkgsCross.dockerTools`.

### Complete Package Matrix

| | x86_64-linux | aarch64-linux | riscv64-linux | x86_64-darwin | aarch64-darwin |
|---|---|---|---|---|---|
| UPX (default) | `default` | `cross-aarch64-linux` | `cross-riscv64-linux` | `default` | `default` |
| Stripped | `-stripped` | `cross-aarch64-linux-stripped` | `cross-riscv64-linux-stripped` | `-stripped` | `-stripped` |
| Debug | `-debug` | `cross-aarch64-linux-debug` | `cross-riscv64-linux-debug` | `-debug` | `-debug` |
| Container | `container` | `container-aarch64-linux` | `container-riscv64-linux` | `container` | `container` |
| Container stripped | `container-stripped` | `container-aarch64-linux-stripped` | `container-riscv64-linux-stripped` | `container-stripped` | `container-stripped` |
| Container debug | `container-debug` | `container-aarch64-linux-debug` | `container-riscv64-linux-debug` | `container-debug` | `container-debug` |

Cross targets (aarch64-linux, riscv64-linux) are only available when building on `x86_64-linux`.

### Smoke Tests

QEMU user-mode smoke tests verify that cross-compiled binaries execute on the target architecture:

```bash
# Run individually
nix build .#checks.x86_64-linux.cross-smoke-aarch64-linux
nix build .#checks.x86_64-linux.cross-smoke-riscv64-linux

# Or run all checks (includes smoke tests on x86_64-linux)
nix flake check
```

Each smoke test:
1. Verifies the ELF architecture with `file`
2. Runs the binary under QEMU user-mode emulation
3. Confirms it starts and logs the expected startup message

**Prerequisites for smoke tests:**

- NixOS: add to configuration.nix:
  ```nix
  boot.binfmt.emulatedSystems = [ "aarch64-linux" "riscv64-linux" ];
  ```
- Non-NixOS: register QEMU binfmt_misc handlers for target architectures

### How It Works

Cross-compilation reuses `nix/variants.nix` — the same module used for native builds. The only difference is which `pkgs` is passed in: for cross targets, `flake.nix` imports nixpkgs with a `crossSystem` parameter, giving a package set where `buildGo126Module` automatically sets `GOOS`/`GOARCH` for the target. UPX runs on the build host (native `pkgs`) since it only needs to understand the target ELF format.

### Architecture Coverage Summary

| Platform | How it's built |
|---|---|
| `x86_64-linux` | Native (default) |
| `aarch64-linux` | Native on ARM64 hosts, or cross-compiled from x86_64-linux |
| `x86_64-darwin` | Native on Intel Mac |
| `aarch64-darwin` | Native on Apple Silicon Mac |
| `riscv64-linux` | Cross-compiled from x86_64-linux |

## File Structure

```
flake.nix              Master composition - imports all modules, defines outputs
nix/
  package.nix          Go binary build (buildGoModule, supports debug flag)
  upx.nix              UPX compression wrapper (produces smallest binaries)
  variants.nix         Builds all 6 outputs (3 binaries + 3 containers) for a given pkgs
  container.nix        OCI container image (buildLayeredImage)
  shell.nix            Development shell (mkShell)
  checks.nix           CI checks (go-vet, gosec, golangci-lint, go-test, nix-fmt)
  tests/
    cross-smoke.nix    QEMU user-mode smoke tests for cross-compiled binaries
  readme.md            This file
```

### Design Principles

- **DRY**: `vendorHash` is defined once in `flake.nix` and shared between `package.nix` (binary build) and `checks.nix` (CI checks using vendored deps in sandbox).
- **Modular**: Each concern is a separate `.nix` file imported by `flake.nix`.
- **Nix-idiomatic**: Uses `buildGoModule`, `buildLayeredImage`, `mkShell`, and `runCommand` — standard Nix patterns.
- **Sandbox-compatible**: All checks run in the Nix sandbox without network access. Go modules are vendored via a fixed-output derivation.

### How Binary Variants Work

The three binary variants (debug, stripped, UPX) are produced by a pipeline of Nix modules:

```
package.nix (debug=true)  ──────────────────────────────────────────→  debug binary
package.nix (debug=false) ──────────────────────────────────────────→  stripped binary
package.nix (debug=false) ───→  upx.nix (UPX --best compression)  ──→  UPX binary (default)
```

**`package.nix`** accepts a `debug` flag (default `false`). When `debug = false`, it passes `-s -w` to the Go linker to strip debug symbols and DWARF tables. When `debug = true`, these flags are omitted, preserving full debug information for use with `delve` or `gdb`.

**`upx.nix`** takes a stripped binary and applies [UPX](https://upx.github.io/) compression with `--best`. UPX is a self-extracting packer — the binary decompresses itself into memory at startup (~15ms overhead). The result is roughly 3x smaller than stripped alone. See [Shrink your Go binaries with this one weird trick](https://words.filippo.io/shrink-your-go-binaries-with-this-one-weird-trick/) for background on this approach.

For cross-compilation, `upx.nix` receives the *native* `pkgs` (build host) rather than `pkgsCross`, since UPX itself runs on the build host — it just needs to understand the target's ELF format.

**`variants.nix`** composes these into a single attrset of 6 outputs (3 binaries + 3 containers). It accepts `pkgs` (for building) and `pkgsNative` (for UPX). `flake.nix` calls it once for native builds and once per cross target — the only difference is which `pkgs` is passed in. The naming convention appends `-stripped` or `-debug` as a suffix; no suffix means UPX (the default).

## Updating Dependencies

When Go dependencies change (`go.mod` / `go.sum`):

1. Update `vendorHash` in `flake.nix`:
   ```bash
   # Set to empty hash to trigger rebuild
   # Replace the vendorHash value with: lib.fakeHash
   nix build 2>&1 | grep "got:"
   # Copy the hash from output and update flake.nix
   ```

2. Verify:
   ```bash
   nix build && nix flake check
   ```

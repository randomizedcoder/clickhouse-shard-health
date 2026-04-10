# nix/shell.nix
#
# Development shell for clickhouse-shard-health.
# Provides Go toolchain, linting, debugging, and utility tools.
#
# Usage:
#   nix develop
#
{ pkgs, goPackage }:

pkgs.mkShell {
  name = "clickhouse-shard-health-dev";

  packages = [
    # Go toolchain
    goPackage
    pkgs.gopls
    pkgs.gotools
    pkgs.golangci-lint
    pkgs.delve

    # Security scanning
    pkgs.gosec

    # Utilities (metrics endpoint testing)
    pkgs.curl
    pkgs.jq

    # Nix formatting
    pkgs.nixfmt-rfc-style
  ];

  shellHook = ''
    echo ""
    echo "clickhouse-shard-health Development Shell"
    echo "=========================================="
    echo "  Go:   $(go version | cut -d' ' -f3)"
    echo "  CGO:  disabled (pure Go)"
    echo ""
    echo "Build Commands:"
    echo "  make build     Build binary to bin/"
    echo "  make test      Run tests with race detector"
    echo "  make lint      Run golangci-lint"
    echo "  make fmt       Format code"
    echo ""
    echo "Nix Commands:"
    echo "  nix flake check              Run all CI checks"
    echo "  nix build                    Build binary"
    echo "  nix build .#container        Build OCI container"
    echo ""
  '';

  CGO_ENABLED = "0";
}

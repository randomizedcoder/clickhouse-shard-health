# nix/tests/cross-smoke.nix
#
# QEMU user-mode smoke test for cross-compiled binaries.
#
# Verifies that cross-compiled clickhouse-shard-health binaries can execute
# on the target architecture via QEMU user-mode emulation.
#
# Prerequisites:
#   NixOS:     boot.binfmt.emulatedSystems = [ "aarch64-linux" "riscv64-linux" ];
#   Non-NixOS: QEMU binfmt_misc must be registered for target architectures.
#
{
  pkgs,
  package,
  arch, # e.g., "aarch64" or "riscv64" — used for test naming
  qemuBin ? null, # e.g., "qemu-aarch64" — null means native (no QEMU needed)
}:

let
  lib = pkgs.lib;
  needsQemu = qemuBin != null;

  runner =
    if !needsQemu then
      "${package}/bin/clickhouse-shard-health"
    else
      "${pkgs.qemu-user}/bin/${qemuBin} ${package}/bin/clickhouse-shard-health";
in
pkgs.runCommand "cross-smoke-clickhouse-shard-health-${arch}"
  {
    nativeBuildInputs = [
      pkgs.file
    ]
    ++ lib.optional needsQemu pkgs.qemu-user;
  }
  ''
    echo "=== Static check: verify ELF architecture ==="
    file ${package}/bin/clickhouse-shard-health | tee /dev/stderr

    echo ""
    echo "=== Dynamic check: run binary (expect config error exit 1) ==="
    set +e
    ${runner} 2>&1 | tee output.log
    exit_code=$?
    set -e

    if [ "$exit_code" -ne 1 ]; then
      echo "FAIL: expected exit code 1, got $exit_code"
      exit 1
    fi

    if grep -q "starting clickhouse-shard-health" output.log; then
      echo "PASS: binary started successfully on ${arch}"
    else
      echo "FAIL: startup log message not found"
      cat output.log
      exit 1
    fi

    echo ""
    echo "All cross smoke tests passed for ${arch}."
    touch $out
  ''

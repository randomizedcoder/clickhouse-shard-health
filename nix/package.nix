# nix/package.nix
#
# Go binary build for clickhouse-shard-health.
#
# Usage:
#   nix build            # Build the binary
#   ./result/bin/clickhouse-shard-health
#
{
  pkgs,
  lib,
  src,
  version,
  vendorHash,
  debug ? false,
}:

pkgs.buildGo126Module {
  pname = "clickhouse-shard-health";
  inherit version src vendorHash;

  subPackages = [ "cmd" ];

  ldflags = [
    "-X main.version=${version}"
  ]
  ++ lib.optionals (!debug) [
    "-s"
    "-w"
  ];

  env.CGO_ENABLED = "0";

  # buildGoModule names the binary after the subPackage directory
  postInstall = ''
    mv $out/bin/cmd $out/bin/clickhouse-shard-health
  '';

  meta = {
    description = "Lightweight sidecar that monitors ClickHouse cluster health and exposes Prometheus metrics";
    homepage = "https://github.com/mmtretiak/clickhouse-shard-health";
    license = lib.licenses.mit;
    mainProgram = "clickhouse-shard-health";
  };
}

# nix/container.nix
#
# OCI container image for clickhouse-shard-health.
#
# Usage:
#   nix build .#container
#   docker load < ./result
#   docker run --rm -p 9363:9363 clickhouse-shard-health:<version>
#
{
  pkgs,
  package,
  version,
}:

pkgs.dockerTools.buildLayeredImage {
  name = "clickhouse-shard-health";
  tag = version;

  contents = [
    package
    pkgs.cacert
  ];

  config = {
    Entrypoint = [ "/bin/clickhouse-shard-health" ];

    Env = [
      "SSL_CERT_FILE=${pkgs.cacert}/etc/ssl/certs/ca-bundle.crt"
    ];

    ExposedPorts = {
      "9363/tcp" = { };
    };

    User = "65534:65534";

    Labels = {
      "org.opencontainers.image.title" = "clickhouse-shard-health";
      "org.opencontainers.image.description" =
        "ClickHouse shard health monitoring with Prometheus metrics";
      "org.opencontainers.image.source" = "https://github.com/mmtretiak/clickhouse-shard-health";
    };
  };
}

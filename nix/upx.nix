# nix/upx.nix
#
# UPX compression for Go binaries.
#
# Applies UPX (Ultimate Packer for eXecutables) to produce the smallest
# possible binary.  Typically achieves ~7x reduction on stripped Go binaries.
#
# Trade-off: ~15-160ms additional startup time for decompression.
#
# The pkgs parameter must be the *native* package set (build host), since
# UPX itself runs on the build host even when compressing cross-compiled
# binaries.
#
{
  pkgs,
  package,
}:

pkgs.runCommand "${package.pname}-${package.version}"
  {
    nativeBuildInputs = [ pkgs.upx ];
    meta = package.meta // {
      description = "${package.meta.description} (UPX compressed)";
    };
  }
  ''
    mkdir -p $out/bin
    cp ${package}/bin/clickhouse-shard-health $out/bin/
    chmod +w $out/bin/clickhouse-shard-health
    upx --best $out/bin/clickhouse-shard-health
  ''

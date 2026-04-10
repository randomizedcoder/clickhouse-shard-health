# nix/variants.nix
#
# Build all binary and container variants for a given package set.
#
# Returns 6 outputs: 3 binaries (debug, stripped, upx) and 3 matching
# containers.  Used for both native and cross-compiled builds — the only
# difference is which pkgs is passed in.
#
# For cross-compilation, pkgsNative must be the build-host package set
# (UPX runs on the build host, not the target).
#
{
  pkgs,
  pkgsNative,
  lib,
  src,
  version,
  vendorHash,
}:

let
  debug = import ./package.nix {
    inherit
      pkgs
      lib
      src
      version
      vendorHash
      ;
    debug = true;
  };

  stripped = import ./package.nix {
    inherit
      pkgs
      lib
      src
      version
      vendorHash
      ;
  };

  upx = import ./upx.nix {
    pkgs = pkgsNative;
    package = stripped;
  };

  containerFor = package: import ./container.nix { inherit pkgs package version; };
in
{
  package = upx;
  package-stripped = stripped;
  package-debug = debug;
  container = containerFor upx;
  container-stripped = containerFor stripped;
  container-debug = containerFor debug;
}

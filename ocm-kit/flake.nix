{
  description = "ocm-kit development environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";

    # Provides pinned Go toolchain versions as go_<major>_<minor> packages.
    go-overlay = {
      url = "github:purpleclay/go-overlay";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.flake-utils.follows = "flake-utils";
    };
  };

  outputs = { nixpkgs, flake-utils, go-overlay, ... }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        # Go toolchain version, as a plain "X.Y.Z" string so CI can read it
        # (grep `goVersion` in this file). The dev shell pins the matching line.
        goVersion = "1.27.1";

        pkgs = import nixpkgs {
          inherit system;
          overlays = [ go-overlay.overlays.default ];
        };

        goAttr = "go_" + builtins.concatStringsSep "_"
          (pkgs.lib.take 2 (pkgs.lib.splitString "." goVersion));
        go = pkgs.${goAttr};
      in
      {
        devShells.default = pkgs.mkShell {
          name = "ocm-kit";
          # golangci-lint and osv-scanner are intentionally not provided here;
          # they are installed from tools.lock via the Makefile so their versions
          # stay pinned in one place (tools.lock).
          packages = [
            go
            pkgs.git
            pkgs.gnumake
            pkgs.gh
            pkgs.shellcheck
          ];
        };
      });
}

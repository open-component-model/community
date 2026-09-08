{
  description = "ocm-kit development environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";

    # Provides exact Go toolchains as `pkgs.go-bin.versions."X.Y.Z"`.
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
        goVersion = "1.26.4";

        pkgs = import nixpkgs {
          inherit system;
          overlays = [ go-overlay.overlays.default ];
        };

        go = pkgs.go-bin.versions.${goVersion};
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

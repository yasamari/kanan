{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    treefmt-nix = {
      url = "github:numtide/treefmt-nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      treefmt-nix,
      ...
    }:
    let
      inherit (nixpkgs) lib;
      eachSystem =
        f: lib.genAttrs nixpkgs.lib.systems.flakeExposed (system: f nixpkgs.legacyPackages.${system});

      treefmtEval = eachSystem (pkgs: treefmt-nix.lib.evalModule pkgs ./treefmt.nix);
    in
    {
      packages = eachSystem (pkgs: rec {
        default = kanan;

        kanan = pkgs.buildGoModule {
          pname = "kanan";
          version = builtins.substring 0 8 (self.lastModifiedDate or "19700101");
          src = self.outPath;
          vendorHash = "sha256-0O/gZNT/qGL39H4VcF/GfOXo9aXwYvt7FkrBxF9Yc9E=";
          subPackages = [ "cmd/kanan" ];
          meta = with pkgs.lib; {
            mainProgram = "kanan";
          };
        };
      });

      devShells = eachSystem (pkgs: {
        default = pkgs.mkShell {
          packages = [
            pkgs.go
            pkgs.gopls
          ];
        };
      });

      formatter = eachSystem (pkgs: treefmtEval.${pkgs.system}.config.build.wrapper);

      checks = eachSystem (pkgs: {
        treefmt = treefmtEval.${pkgs.system}.config.build.check self;
      });
    };
}

{
  description = "Nix language parser and Nix-compatible file hasher in Go";

  inputs = {
    flake-parts.url = "github:hercules-ci/flake-parts";
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
  };

  outputs = inputs: inputs.flake-parts.lib.mkFlake { inherit inputs; } {
    systems = [ "x86_64-linux" "aarch64-linux" ];
    perSystem = { pkgs, ... }: {
      packages = rec {
        go-nix = with pkgs; buildGoModule {
          name = "go-nix";

          src = lib.cleanSource ./.;

          nativeBuildInputs = [
            gotools
            ragel
          ];

          preBuild = ''
            go generate ./...
          '';

          vendorHash = "sha256-dE3YS68DliQVjYLhRCuNRtRzk5ODU4P3vbySN1wZYQg=";

          proxyVendor = true;

          ldflags = [ "-s" "-w" ];
        };
        default = go-nix;
      };
      devShells.default = with pkgs; mkShell {
        packages = [
          go
          gotools
          ragel
          gore # test go feature
          graphviz # go tool pprof
        ];
      };
    };
  };
}

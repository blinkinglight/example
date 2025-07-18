{
  description = "templ";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.05";
    nixpkgs-unstable.url = "github:NixOS/nixpkgs/nixos-unstable";
    gitignore = {
      url = "github:hercules-ci/gitignore.nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    version = {
      url = "github:a-h/version/0.0.8";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    xc = {
      url = "github:joerdav/xc";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = { self, nixpkgs, nixpkgs-unstable, gitignore, version, xc }:
    let
      allSystems = [
        "x86_64-linux" # 64-bit Intel/AMD Linux
        "aarch64-linux" # 64-bit ARM Linux
        "x86_64-darwin" # 64-bit Intel macOS
        "aarch64-darwin" # 64-bit ARM macOS
      ];
      forAllSystems = f: nixpkgs.lib.genAttrs allSystems (system: f {
        inherit system;
        pkgs = import nixpkgs { inherit system; };
        pkgs-unstable = import nixpkgs-unstable { inherit system; };
      });
    in
    {
      packages = forAllSystems ({ system, pkgs, ... }:
        rec {
          default = templ;

          templ = pkgs.buildGo124Module {
            name = "templ";
            subPackages = [ "cmd/templ" ];
            src = gitignore.lib.gitignoreSource ./.;
            vendorHash = "sha256-oObzlisjvS9LeMYh3DzP+l7rgqBo9bQcbNjKCUJ8rcY=";
            env = {
              CGO_ENABLED = 0;
            };
            flags = [
              "-trimpath"
            ];
            ldflags = [
              "-s"
              "-w"
              "-extldflags -static"
            ];
          };
        });

      # `nix develop` provides a shell containing development tools.
      devShell = forAllSystems ({ system, pkgs, pkgs-unstable, ... }:
        pkgs.mkShell {
          buildInputs = [
            pkgs.golangci-lint
            pkgs.cosign # Used to sign container images.
            pkgs.esbuild # Used to package JS examples.
            pkgs.go
            pkgs-unstable.gopls
            pkgs.goreleaser
            pkgs.gotestsum
            pkgs.ko # Used to build Docker images.
            pkgs.nodejs # Used to build templ-docs.
            version.packages.${system}.default # Used to apply version numbers to the repo.
            xc.packages.${system}.xc
          ];
        });

      # This flake outputs an overlay that can be used to add templ and
      # templ-docs to nixpkgs as per https://templ.guide/quick-start/installation/#nix
      #
      # Example usage:
      #
      # nixpkgs.overlays = [
      #   inputs.templ.overlays.default
      # ];
      overlays.default = final: prev: {
        templ = self.packages.${final.stdenv.system}.templ;
        templ-docs = self.packages.${final.stdenv.system}.templ-docs;
      };
    };
}


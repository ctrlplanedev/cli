{
  description = "Ctrlplane CLI (ctrlc)";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};

        version =
          if (self ? rev)
          then self.shortRev
          else "dev";
      in
      {
        packages = {
          ctrlc = pkgs.buildGoModule {
            pname = "ctrlc";
            inherit version;
            src = ./.;

            vendorHash = "sha256-n/yfb1cCeQmfECyKsjsdsEYJM/rTip3d5mzMHXOHEDY=";

            subPackages = [ "cmd/ctrlc" ];

            env.CGO_ENABLED = "0";

            tags = [ "containers_image_openpgp" ];

            ldflags = [
              "-s" "-w"
              "-X github.com/ctrlplanedev/cli/cmd/ctrlc/root/version.Version=${version}"
              "-X github.com/ctrlplanedev/cli/cmd/ctrlc/root/version.GitCommit=${self.rev or "dirty"}"
              "-X github.com/ctrlplanedev/cli/cmd/ctrlc/root/version.BuildDate=1970-01-01T00:00:00Z"
            ];

            meta = with pkgs.lib; {
              description = "CLI for Ctrlplane - manage deployments, resources, and jobs";
              homepage = "https://github.com/ctrlplanedev/cli";
              mainProgram = "ctrlc";
            };
          };

          default = self.packages.${system}.ctrlc;
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go_1_24
            golangci-lint
            goreleaser
          ];
        };
      });
}

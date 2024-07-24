{
  inputs.nixpkgs.url = "https://flakehub.com/f/NixOS/nixpkgs/0.1.*.tar.gz";
  inputs.knixpkgs.url = "https://github.com/Karitham/knixpkgs/archive/master.tar.gz";
  inputs.knixpkgs.inputs.nixpkgs.follows = "nixpkgs";
  outputs = {
    self,
    nixpkgs,
    knixpkgs,
  }: let
    goVersion = 22; # Change this to update the whole stack
    overlays = [
      (final: prev: {go = prev."go_1_${toString goVersion}";})
    ];
    supportedSystems = ["x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin"];
    forEachSupportedSystem = f:
      nixpkgs.lib.genAttrs supportedSystems (system:
        f {
          pkgs = import nixpkgs {inherit overlays system;};
        });
  in {
    devShells = forEachSupportedSystem ({pkgs}: let
      code-generator = pkgs.buildGoModule rec {
        pname = "code-generator";
        version = "0.30.3";
        src = pkgs.fetchFromGitHub {
          owner = "kubernetes";
          repo = pname;
          rev = "v${version}";
          hash = "sha256-GC8L/s+pMx07BgM3XbnJqNaKnprUM3BCHbn0WvRCEME=";
        };
        ldflags = ["-s" "-w"];
        vendorHash = "sha256-kN8qFinFMQ739cxko8uR/AaGjYsqAT9iBLsDY149wwI=";
      };
    in {
      default = pkgs.mkShell {
        packages = with pkgs; [
          go
          gotools
          gofumpt
          kubectl
          kubernetes-helm
          minikube
          kind
          code-generator
          helm-ls
          knixpkgs.packages.${system}.helm-readme-generator
        ];
      };
    });
  };
}

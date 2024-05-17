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
      (final: prev: {
        kubernetes-code-generator = prev.kubernetes-code-generator.overrideAttrs (oldAttrs: {
        version = "0.30.1";
      });})
    ];
    supportedSystems = ["x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin"];
    forEachSupportedSystem = f:
      nixpkgs.lib.genAttrs supportedSystems (system:
        f {
          pkgs = import nixpkgs {inherit overlays system;};
        });
  in {
    devShells = forEachSupportedSystem ({pkgs}: {
      default = pkgs.mkShell {
        packages = with pkgs; [
          go
          gotools
          gofumpt
          kubectl
          kubernetes-helm
          minikube
          kind
          kubernetes-code-generator
          knixpkgs.packages.${system}.helm-readme-generator
        ];
      };
    });
  };
}

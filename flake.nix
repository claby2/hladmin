{
  description = "hladmin - homelab administration tool";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let pkgs = nixpkgs.legacyPackages.${system};
      in {
        packages.default = pkgs.rustPlatform.buildRustPackage {
          pname = "hladmin";
          version = "0.1.0";

          src = ./.;

          cargoHash = "sha256-6p12Qh74JYst+9mdHCU52LupN6yIEElyNDg4hCqeovk=";

          meta = with pkgs.lib; {
            description = "Homelab administration tool";
            homepage = "https://github.com/claby2/hladmin";
            maintainers = [ ];
          };
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [ cargo rustc rustfmt clippy rust-analyzer ];
        };
      });
}

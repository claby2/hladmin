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
        packages.default = pkgs.buildGoModule {
          pname = "hladmin";
          version = "0.1.0";

          src = ./.;

          vendorHash = "sha256-K66gswCaU2GRdTF6V/Tyysg43o1ZJs+FEcOwvoK5YGk=";

          meta = with pkgs.lib; {
            description = "Homelab administration tool";
            homepage = "https://github.com/claby2/hladmin";
            maintainers = [ ];
          };
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [ go gopls gotools go-tools ];
        };
      });
}


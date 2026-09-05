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

          cargoHash = "sha256-n5YI3N/UTouuZQDYjQE4evWZWhz79LxPtBtY95FEVO4=";

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

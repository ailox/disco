{
  description = "A simple Go package";

  # Nixpkgs / NixOS version to use.
  inputs.nixpkgs.url = "nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let

      # to work with older version of flakes
      lastModifiedDate = self.lastModifiedDate or self.lastModified or "19700101";

      # Generate a user-friendly version number.
      #version = builtins.substring 0 8 lastModifiedDate;
      version = self.rev or "dirty";

      # System types to support.
      supportedSystems = [ "x86_64-linux" "x86_64-darwin" "aarch64-linux" "aarch64-darwin" ];

      # Helper function to generate an attrset '{ x86_64-linux = f "x86_64-linux"; ... }'.
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;

      # Nixpkgs instantiated for supported system types.
      nixpkgsFor = forAllSystems (system: import nixpkgs { inherit system; });

    in
    {
      devShells = forAllSystems (system:
      let
        pkgs = nixpkgsFor.${system};
      in {
        default = pkgs.mkShell {
          buildInputs = with pkgs; [
            nodejs_20  # LTS
            yarn
            brotli

            go
            go-outline
            go-tools
            gocode-gomod
            godef
            golint
            gopkgs
            gopls
            gotools

            kubectl

            postgresql
            pspg
            bashInteractive
          ];
          shellHook = ''
          export RANDOM_DIR=$(mktemp --tmpdir -d database.XXXXX)
          export PGHOST=''${RANDOM_DIR}/postgres_data
          export PGDATA=''${PGHOST}/data
          export PGDATABASE=postgres
          export DATABASE_URL=postgresql:///postgres?host=''${PGHOST}
          LOG_PATH=''${PGHOST}/LOG

	        test -d "''${PGHOST}" || {
	        	mkdir -p "''${PGHOST}";
	        	echo 'Initializing postgresql database...';
	        	initdb -E UTF-8 --auth=trust --pgdata=''${PGDATA} >/dev/null;
	        }
	        pg_ctl start -l ''${LOG_PATH} -o \
	        	"-c listen_addresses= -c unix_socket_directories=''${PGHOST}"

          finish()
          {
            pg_ctl stop
            rm -rf "''${PGHOST}"
            rm -rf "''${RANDOM_DIR}"
          }
          trap finish EXIT
          '';
        };
      });

    };
}

{
  description = "Flake BudaBit CORS PROXY";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    nixvim.url = "github:Pleb5/neovim-flake/master";
  };

    outputs = { self, nixpkgs, flake-utils, nixvim, ... }:

        flake-utils.lib.eachDefaultSystem (system:
            let
                pkgs = nixpkgs.legacyPackages.${system};
                nvim = nixvim.packages.${system}.nvim;
            in {
                devShell = pkgs.mkShell {
                    buildInputs = with pkgs; [ 
                        # Editor
                        nvim
                        ripgrep
                        
                        # Go toolchain
                        go                    # Go compiler and runtime
                        gotools              # Additional Go tools (goimports, godoc, etc.)
                        golangci-lint        # Comprehensive Go linter
                        
                        # Development tools
                        gnumake             # Build automation (optional but common)
                        delve               # Go debugger
                    ];
                    
                    shellHook = ''
                        # Set Go environment variables
                        export CARGOPATH=$HOME/.cargo
                        export GOPATH=$PWD/.go
                        export GOCACHE=$PWD/.go/cache
                        export GOMODCACHE=$PWD/.go/mod
                        
                        # Create Go directories if they don't exist
                        mkdir -p $GOPATH/bin $GOCACHE $GOMODCACHE
                        
                        # Add Go binaries to PATH
                        export PATH=$CARGOPATH/bin:$GOPATH/bin:$PATH
                        
                        echo "🐹 Go development environment loaded!"
                        echo "Go version: $(go version)"
                        echo "GOPATH: $GOPATH"
                    '';
                };
            }
        );    
}

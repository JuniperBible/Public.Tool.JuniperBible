{ pkgs ? import <nixpkgs> {} }:

pkgs.mkShell {
  buildInputs = with pkgs; [
    go
    sqlite
    sword
  ];

  shellHook = ''
    echo "Juniper - Bible Extraction CLI"
    echo "==============================="
    echo ""
    echo "Build:"
    echo "  go build -o juniper ./cmd/juniper"
    echo ""
    echo "Test:"
    echo "  go test ./..."
    echo ""
    echo "Extract Bibles:"
    echo "  ./juniper extract -o ../../data/ -v"
    echo ""
    echo "SWORD tools:"
    echo "  diatheke -b KJV -k Gen 1:1"
    echo ""
  '';
}

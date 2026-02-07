build:
    go build -o out -trimpath -ldflags="-s -w" ./...
    GOOS=js GOARCH=wasm go build -o web/malaina.wasm ./web
    cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" web

web:
    cd web
    python -m http.server
    cd ..
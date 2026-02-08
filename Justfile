build:
    go build -o out -trimpath -ldflags="-s -w" ./...
    GOOS=js GOARCH=wasm go build -o docs/malaina.wasm ./docs
    cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" docs

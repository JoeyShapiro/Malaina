build:
    go build -o out ./... -trimpath -ldflags="-s -w"
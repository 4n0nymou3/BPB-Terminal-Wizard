#!/bin/bash

set -e

mkdir -p bin

echo "Building for Linux (amd64)..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/BPB-Terminal-Wizard-linux-amd64 src/main.go

echo "Building for Linux (arm64)..."
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o bin/BPB-Terminal-Wizard-linux-arm64 src/main.go

echo "Building for macOS (amd64)..."
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o bin/BPB-Terminal-Wizard-darwin-amd64 src/main.go

echo "Building for macOS (arm64)..."
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o bin/BPB-Terminal-Wizard-darwin-arm64 src/main.go

echo "Build completed successfully!"
ls -l bin/
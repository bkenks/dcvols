#!/usr/bin/env bash
set -euo pipefail

BINARY_NAME="dcvols"

PLATFORMS=(
  "linux amd64"
  "linux arm64"
)

OUTPUT_DIR=".builds"
mkdir -p "$OUTPUT_DIR"

for PLATFORM in "${PLATFORMS[@]}"; do
  OS=$(echo $PLATFORM | awk '{print $1}')
  ARCH=$(echo $PLATFORM | awk '{print $2}')

  ZIP_NAME="${OUTPUT_DIR}/${OS}_${ARCH}.zip"

  echo "Building $OS/$ARCH -> $ZIP_NAME"

  GOOS=$OS GOARCH=$ARCH go build -o "$BINARY_NAME"

  zip -j "$ZIP_NAME" "$BINARY_NAME"

  rm "$BINARY_NAME"
done

echo "All builds completed and zipped!"

#!/bin/bash

# Script to build adbctl for multiple platforms

# Name of your Go file (without extension)
APP_NAME="adbctl"

# Supported GOOS and GOARCH combinations
PLATFORMS=("windows/amd64" "windows/386" "darwin/amd64" "darwin/arm64" "linux/amd64" "linux/386" "linux/arm" "linux/arm64")

# Get the latest tag or commit hash
VERSION=$(git describe --tags --always --abbrev=0 2>/dev/null || git rev-parse --short HEAD)

# Remove the 'v' prefix if present
VERSION=${VERSION#v}

echo "Building version: $VERSION"

# Create a build directory
mkdir -p build

# Loop through platforms and build
for PLATFORM in "${PLATFORMS[@]}"
do
    # Split PLATFORM into OS and ARCH
    IFS='/' read -r -a array <<< "$PLATFORM"
    GOOS=${array[0]}
    GOARCH=${array[1]}
    
    # Set the output binary name
    if [ $GOOS = "windows" ]; then
        OUTPUT_NAME=$APP_NAME'_'$VERSION'_'$GOOS'_'$GOARCH'.exe'
    else
        OUTPUT_NAME=$APP_NAME'_'$VERSION'_'$GOOS'_'$GOARCH
    fi

    # Build
    echo "Building $OUTPUT_NAME..."
    GOOS=$GOOS GOARCH=$GOARCH go build -o build/$OUTPUT_NAME -ldflags="-X main.Version=$VERSION" .
    if [ $? -ne 0 ]; then
        echo 'An error has occurred! Aborting the script execution...'
        exit 1
    fi
done

echo "Build completed!"
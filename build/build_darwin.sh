#!/bin/bash

# Build script for macOS (Darwin)

set -e

cd "$(dirname "$0")/.."

echo ""
echo "========================================"
echo "  Building Sing-Box Launcher (macOS)"
echo "========================================"
echo ""

echo "=== Checking build tools ==="
# Check for Xcode or Command Line Tools
if ! command -v xcrun &> /dev/null; then
    echo "ERROR: xcrun not found. Please install Xcode or Command Line Tools:"
    echo "  xcode-select --install"
    exit 1
fi

# Get SDK path and version
SDK_PATH=$(xcrun --show-sdk-path 2>/dev/null || echo "")
if [ -z "$SDK_PATH" ]; then
    echo "ERROR: Cannot find macOS SDK. Please install Xcode or Command Line Tools:"
    echo "  xcode-select --install"
    exit 1
fi

SDK_VERSION=$(xcrun --show-sdk-version 2>/dev/null || echo "unknown")
echo "SDK Path: $SDK_PATH"
echo "SDK Version: $SDK_VERSION"

echo ""
echo "=== Tidying Go modules ==="
go mod tidy

echo ""
echo "=== Setting build environment ==="
export CGO_ENABLED=1
export GOOS=darwin

# Auto-detect architecture (arm64 for Apple Silicon, amd64 for Intel)
ARCH=$(uname -m)
if [ "$ARCH" = "arm64" ]; then
    export GOARCH=arm64
    echo "Architecture: arm64 (Apple Silicon)"
elif [ "$ARCH" = "x86_64" ]; then
    export GOARCH=amd64
    echo "Architecture: amd64 (Intel)"
else
    export GOARCH=amd64
    echo "Architecture: amd64 (default, detected: $ARCH)"
fi

# Check if full Xcode is required (Command Line Tools have incomplete SDK)
UTCORETYPES_H="$SDK_PATH/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Headers/UTCoreTypes.h"
CURRENT_DEVELOPER_DIR=$(xcode-select -p 2>/dev/null || echo "")

# Check if UTCoreTypes.h exists (only in full Xcode SDK)
if [ ! -f "$UTCORETYPES_H" ]; then
    echo ""
    echo "❌ ERROR: Full Xcode is required for building this project!"
    echo "   Command Line Tools have incomplete SDK headers (missing UTCoreTypes.h)."
    echo ""
    
    # Check if Xcode is installed but not selected
    if [ -d "/Applications/Xcode.app" ]; then
        if [[ "$CURRENT_DEVELOPER_DIR" != *"Xcode.app"* ]]; then
            echo "   💡 Full Xcode detected at /Applications/Xcode.app but not selected."
            echo "   Switch to Xcode with:"
            echo "   sudo xcode-select --switch /Applications/Xcode.app"
            echo ""
            echo "   Or accept the Xcode license first:"
            echo "   sudo xcodebuild -license accept"
            exit 1
        fi
    else
        echo "   Please install Xcode from the App Store:"
        echo "   https://apps.apple.com/app/xcode/id497799835"
        echo ""
        echo "   After installation, accept the license:"
        echo "   sudo xcodebuild -license accept"
        echo ""
        echo "   Then switch to Xcode:"
        echo "   sudo xcode-select --switch /Applications/Xcode.app"
        exit 1
    fi
fi

# Set SDK path for CGO compiler
export SDKROOT="$SDK_PATH"

# Determine output filename
BASE_NAME="singbox-launcher"
EXTENSION=""
OUTPUT_FILENAME="${BASE_NAME}${EXTENSION}"
COUNTER=0

while [ -f "$OUTPUT_FILENAME" ]; do
    COUNTER=$((COUNTER + 1))
    OUTPUT_FILENAME="${BASE_NAME}-${COUNTER}${EXTENSION}"
done

echo "Using output file: $OUTPUT_FILENAME"

echo ""
echo "=== Getting version from git tag ==="
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "0.4.1")
echo "Version: $VERSION"

echo ""
echo "=== Starting Build ==="
go build -buildvcs=false -ldflags="-s -w -X singbox-launcher/internal/constants.AppVersion=$VERSION" -o "$OUTPUT_FILENAME"

if [ $? -eq 0 ]; then
    echo ""
    echo "=== Creating .app bundle ==="
    
    # Create .app bundle structure
    APP_NAME="${BASE_NAME}.app"
    APP_CONTENTS="$APP_NAME/Contents"
    APP_MACOS="$APP_CONTENTS/MacOS"
    APP_RESOURCES="$APP_CONTENTS/Resources"
    
    # Remove old bundle if exists
    rm -rf "$APP_NAME"
    
    # Create directory structure
    mkdir -p "$APP_MACOS"
    mkdir -p "$APP_RESOURCES"
    
    # Move binary to MacOS directory
    mv "$OUTPUT_FILENAME" "$APP_MACOS/$BASE_NAME"
    chmod +x "$APP_MACOS/$BASE_NAME"
    
    # Handle application icon
    ICON_FILE="assets/app.icns"
    ICON_NAME="app"
    HAS_ICON=false
    
    if [ -f "$ICON_FILE" ]; then
        echo "=== Copying application icon ==="
        cp "$ICON_FILE" "$APP_RESOURCES/${ICON_NAME}.icns"
        HAS_ICON=true
        echo "Icon copied: $ICON_FILE -> $APP_RESOURCES/${ICON_NAME}.icns"
    else
        echo "=== Icon not found: $ICON_FILE (skipping icon setup) ==="
    fi
    
    # Create Info.plist with optional icon
    {
        echo '<?xml version="1.0" encoding="UTF-8"?>'
        echo '<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">'
        echo '<plist version="1.0">'
        echo '<dict>'
        echo '    <key>CFBundleExecutable</key>'
        echo "    <string>$BASE_NAME</string>"
        echo '    <key>CFBundleIdentifier</key>'
        echo '    <string>com.singbox.launcher</string>'
        echo '    <key>CFBundleName</key>'
        echo '    <string>Sing-Box Launcher</string>'
        echo '    <key>CFBundlePackageType</key>'
        echo '    <string>APPL</string>'
        if [ "$HAS_ICON" = true ]; then
            echo '    <key>CFBundleIconFile</key>'
            echo "    <string>$ICON_NAME</string>"
        fi
        echo '    <key>CFBundleShortVersionString</key>'
        echo "    <string>$VERSION</string>"
        echo '    <key>CFBundleVersion</key>'
        echo "    <string>$VERSION</string>"
        echo '    <key>LSMinimumSystemVersion</key>'
        echo '    <string>10.15</string>'
        echo '    <key>NSHighResolutionCapable</key>'
        echo '    <true/>'
        echo '    <key>LSUIElement</key>'
        echo '    <false/>'
        echo '</dict>'
        echo '</plist>'
    } > "$APP_CONTENTS/Info.plist"
    
    echo "Created .app bundle: $APP_NAME"
    echo ""
    echo "========================================"
    echo "  Build completed successfully!"
    echo "  Output: $APP_NAME"
    echo "  Run with: open $APP_NAME"
    echo "========================================"
else
    echo ""
    echo "!!! Build failed !!!"
    exit 1
fi


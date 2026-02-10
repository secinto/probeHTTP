#!/bin/bash

# Build script for probeHTTP with static linking
# This script creates statically linked executables
# Usage: ./build-static.sh [options]
#   Platform options:
#     -all: Build for all platforms (Linux and macOS)
#     (default): Build only for the detected host OS
#
#   Install options:
#     --install, -i: Install binary to GOPATH/bin after building
#   Test options:
#     -T, --skip-tests: Skip running tests before build
#
# Examples:
#   ./build-static.sh                # Build for host OS
#   ./build-static.sh -all           # Build for all platforms
#   ./build-static.sh --install      # Build and install to GOPATH/bin
#   ./build-static.sh -all --install # Build all platforms and install native binary
#   ./build-static.sh -T             # Build without running tests

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
BUILD_DIR="./bin"
BINARY_NAME="probeHTTP"
MAIN_PACKAGE="./cmd/probehttp"

# Parse command line arguments
BUILD_ALL=false
INSTALL=false
SKIP_TESTS=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -all)
            BUILD_ALL=true
            shift
            ;;
        --install|-i)
            INSTALL=true
            shift
            ;;
        -T|--skip-tests)
            SKIP_TESTS=true
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [options]"
            echo ""
            echo "Platform options:"
            echo "  -all              Build for all platforms (Linux and macOS)"
            echo "  (default)         Build only for the detected host OS"
            echo ""
            echo "Install options:"
            echo "  --install, -i     Install binary to GOPATH/bin after building"
            echo ""
            echo "Test options:"
            echo "  -T, --skip-tests  Skip running tests before build"
            echo ""
            echo "Examples:"
            echo "  $0                    # Build for host OS"
            echo "  $0 -all               # Build for all platforms"
            echo "  $0 --install          # Build and install to GOPATH/bin"
            echo "  $0 -all --install     # Build all platforms and install native binary"
            echo "  $0 -T                 # Build without running tests"
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            echo "Use -h or --help for usage information"
            exit 1
            ;;
    esac
done

# Detect host OS and architecture
HOST_OS=$(uname -s | tr '[:upper:]' '[:lower:]')
if [[ "$HOST_OS" == "darwin" ]]; then
    HOST_OS="darwin"
elif [[ "$HOST_OS" == "linux" ]]; then
    HOST_OS="linux"
fi

HOST_ARCH=$(uname -m)
if [[ "$HOST_ARCH" == "arm64" || "$HOST_ARCH" == "aarch64" ]]; then
    HOST_ARCH="arm64"
else
    HOST_ARCH="amd64"
fi

echo -e "${BLUE}Detected host: ${HOST_OS}/${HOST_ARCH}${NC}"
if [[ "$BUILD_ALL" == true ]]; then
    echo -e "${BLUE}Building for all platforms${NC}"
else
    echo -e "${BLUE}Building for host OS only${NC}"
fi
echo ""

# Build information
BASE_VERSION="1.0.0"
COMMIT_HASH=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')

# Construct full version string with dirty flag
if [ "$COMMIT_HASH" != "unknown" ]; then
    if git diff-index --quiet HEAD -- 2>/dev/null; then
        VERSION="${BASE_VERSION}"
    else
        VERSION="${BASE_VERSION}-dirty"
    fi
else
    VERSION="${BASE_VERSION}"
fi

echo -e "${GREEN}Building probeHTTP (static)${NC}"
echo "Version: ${VERSION}"
echo "Commit: ${COMMIT_HASH}"
echo "Build Time: ${BUILD_TIME}"
echo ""

# Run tests before building
if [[ "$SKIP_TESTS" == false ]]; then
    echo -e "${BLUE}Running tests...${NC}"
    if go test ./... ; then
        echo -e "${GREEN}All tests passed${NC}"
    else
        echo -e "${RED}Tests failed! Aborting build.${NC}"
        exit 1
    fi
    echo ""
fi

# Create build directory
mkdir -p "${BUILD_DIR}"

# Function to build for a specific platform
build_for_platform() {
    local target_os=$1
    local target_arch=$2
    local output_suffix=$3

    echo -e "${YELLOW}Building ${BINARY_NAME} for ${target_os}/${target_arch}...${NC}"
    
    # Build flags for static linking
    export CGO_ENABLED=0
    export GOOS=${target_os}
    export GOARCH=${target_arch}
    
    # Linker flags - inject version info into pkg/version
    local LDFLAGS="-s -w"
    LDFLAGS="${LDFLAGS} -X probeHTTP/pkg/version.Version=${VERSION}"
    LDFLAGS="${LDFLAGS} -X probeHTTP/pkg/version.GitCommit=${COMMIT_HASH}"
    LDFLAGS="${LDFLAGS} -X probeHTTP/pkg/version.BuildDate=${BUILD_TIME}"
    LDFLAGS="${LDFLAGS} -extldflags '-static'"
    
    local output_path="${BUILD_DIR}/${BINARY_NAME}${output_suffix}"

    # Build the binary
    go build \
        -v \
        -a \
        -installsuffix cgo \
        -ldflags "${LDFLAGS}" \
        -o "${output_path}" \
        ${MAIN_PACKAGE}
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ Build successful for ${target_os}/${target_arch}${NC}"
        echo "Binary location: ${output_path}"
        ls -lh "${output_path}"
        
        # Verify static linking only for native builds
        if [[ "${target_os}" == "${HOST_OS}" && "${target_arch}" == "${HOST_ARCH}" ]]; then
            echo -e "${YELLOW}Verifying static linking...${NC}"
            
            if [[ "${target_os}" == "darwin" ]]; then
                if otool -L "${output_path}" | grep -q "dylib"; then
                    echo -e "${YELLOW}⚠ Warning: Binary has dynamic library dependencies:${NC}"
                    otool -L "${output_path}"
                else
                    echo -e "${GREEN}✓ No dynamic library dependencies found${NC}"
                fi
            elif [[ "${target_os}" == "linux" ]]; then
                if LC_ALL=C ldd "${output_path}" 2>&1 | grep -q "not a dynamic executable"; then
                    echo -e "${GREEN}✓ Statically linked executable${NC}"
                else
                    echo -e "${YELLOW}⚠ Warning: Binary has dynamic dependencies:${NC}"
                    LC_ALL=C ldd "${output_path}"
                fi
            fi
        fi
        
        echo ""
        return 0
    else
        echo -e "${RED}✗ Build failed for ${target_os}/${target_arch}${NC}"
        return 1
    fi
}

# Track which binary to install (native build)
NATIVE_BINARY=""

# Build based on flag
if [[ "$BUILD_ALL" == true ]]; then
    # Build for all platforms
    echo -e "${BLUE}=== Building for Linux amd64 ===${NC}"
    build_for_platform "linux" "amd64" "-linux-amd64"
    if [[ "$HOST_OS" == "linux" && "$HOST_ARCH" == "amd64" ]]; then
        NATIVE_BINARY="${BUILD_DIR}/${BINARY_NAME}-linux-amd64"
    fi

    echo -e "${BLUE}=== Building for macOS Intel ===${NC}"
    build_for_platform "darwin" "amd64" "-darwin-amd64"
    if [[ "$HOST_OS" == "darwin" && "$HOST_ARCH" == "amd64" ]]; then
        NATIVE_BINARY="${BUILD_DIR}/${BINARY_NAME}-darwin-amd64"
    fi

    echo -e "${BLUE}=== Building for macOS ARM64 ===${NC}"
    build_for_platform "darwin" "arm64" "-darwin-arm64"
    if [[ "$HOST_OS" == "darwin" && "$HOST_ARCH" == "arm64" ]]; then
        NATIVE_BINARY="${BUILD_DIR}/${BINARY_NAME}-darwin-arm64"
    fi
else
    # Build only for host OS
    if [[ "$HOST_OS" == "darwin" ]]; then
        if [[ "$HOST_ARCH" == "arm64" ]]; then
            echo -e "${BLUE}=== Building for macOS ARM64 ===${NC}"
            build_for_platform "darwin" "arm64" "-darwin-arm64"
            NATIVE_BINARY="${BUILD_DIR}/${BINARY_NAME}-darwin-arm64"
        else
            echo -e "${BLUE}=== Building for macOS Intel ===${NC}"
            build_for_platform "darwin" "amd64" "-darwin-amd64"
            NATIVE_BINARY="${BUILD_DIR}/${BINARY_NAME}-darwin-amd64"
        fi
    elif [[ "$HOST_OS" == "linux" ]]; then
        echo -e "${BLUE}=== Building for Linux amd64 ===${NC}"
        build_for_platform "linux" "amd64" "-linux-amd64"
        NATIVE_BINARY="${BUILD_DIR}/${BINARY_NAME}-linux-amd64"
    else
        echo -e "${RED}Unsupported host OS: ${HOST_OS}${NC}"
        exit 1
    fi
fi

# Install to GOPATH/bin if requested
if [[ "$INSTALL" == true ]]; then
    echo ""
    echo -e "${BLUE}=== Installing to GOPATH/bin ===${NC}"
    
    # Determine GOPATH
    GOBIN="${GOPATH:-$HOME/go}/bin"
    
    if [[ -z "$NATIVE_BINARY" || ! -f "$NATIVE_BINARY" ]]; then
        echo -e "${RED}✗ No native binary found to install${NC}"
        exit 1
    fi
    
    # Create GOPATH/bin if it doesn't exist
    mkdir -p "$GOBIN"
    
    # Copy binary to GOPATH/bin
    cp "$NATIVE_BINARY" "$GOBIN/$BINARY_NAME"
    chmod +x "$GOBIN/$BINARY_NAME"
    
    echo -e "${GREEN}✓ Installed ${BINARY_NAME} to ${GOBIN}/${BINARY_NAME}${NC}"
    ls -lh "$GOBIN/$BINARY_NAME"
    
    # Verify installation
    if command -v "$BINARY_NAME" &> /dev/null; then
        echo -e "${GREEN}✓ ${BINARY_NAME} is now available in PATH${NC}"
        "$BINARY_NAME" --version 2>/dev/null || true
    else
        echo -e "${YELLOW}⚠ Note: ${GOBIN} may not be in your PATH${NC}"
        echo "Add it with: export PATH=\$PATH:${GOBIN}"
    fi
fi

echo ""
echo -e "${GREEN}════════════════════════════════════════${NC}"
echo -e "${GREEN}Build complete!${NC}"
echo -e "${GREEN}════════════════════════════════════════${NC}"
echo ""
echo "Built binaries:"
ls -lh "${BUILD_DIR}/"

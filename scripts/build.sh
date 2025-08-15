#!/bin/bash

# TraceAce Build Script
# Builds binaries for multiple platforms and creates packages

set -e

VERSION=${VERSION:-"1.0.0"}
BUILD_DIR=${BUILD_DIR:-"build"}
DIST_DIR=${DIST_DIR:-"dist"}

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Clean previous builds
clean() {
    print_status "Cleaning previous builds..."
    rm -rf "$BUILD_DIR" "$DIST_DIR"
    mkdir -p "$BUILD_DIR" "$DIST_DIR"
}

# Run tests
run_tests() {
    print_status "Running tests..."
    go test ./...
    go test -race ./...
    print_success "All tests passed"
}

# Run linters
run_linters() {
    print_status "Running linters..."
    
    # Check if golangci-lint is installed
    if command -v golangci-lint &> /dev/null; then
        golangci-lint run
        print_success "Linting completed"
    else
        print_warning "golangci-lint not found, skipping linting"
    fi
    
    # Format check
    if [ -n "$(gofmt -l .)" ]; then
        print_error "Code is not formatted. Run 'go fmt ./...'"
        exit 1
    fi
    
    # Vet check
    go vet ./...
    print_success "Vet check completed"
}

# Build for a specific platform
build_platform() {
    local os=$1
    local arch=$2
    local ext=$3
    
    local output_name="traceace-${os}-${arch}${ext}"
    local output_path="${BUILD_DIR}/${output_name}"
    
    print_status "Building for ${os}/${arch}..."
    
    GOOS=${os} GOARCH=${arch} CGO_ENABLED=0 go build \
        -ldflags "-X main.version=${VERSION} -X main.buildTime=$(date -u '+%Y-%m-%d_%H:%M:%S') -s -w" \
        -o "${output_path}" \
        .
    
    print_success "Built ${output_name}"
    
    # Calculate SHA256
    if command -v sha256sum &> /dev/null; then
        sha256sum "${output_path}" > "${output_path}.sha256"
    elif command -v shasum &> /dev/null; then
        shasum -a 256 "${output_path}" > "${output_path}.sha256"
    fi
}

# Build all platforms
build_all() {
    print_status "Building for all platforms..."
    
    # Linux
    build_platform "linux" "amd64" ""
    build_platform "linux" "arm64" ""
    build_platform "linux" "386" ""
    
    # macOS
    build_platform "darwin" "amd64" ""
    build_platform "darwin" "arm64" ""
    
    # Windows
    build_platform "windows" "amd64" ".exe"
    build_platform "windows" "386" ".exe"
    
    print_success "All platforms built successfully"
}

# Create packages
create_packages() {
    print_status "Creating packages..."
    
    # Create tarball for each binary
    for binary in "$BUILD_DIR"/*; do
        if [[ -f "$binary" ]] && [[ ! "$binary" =~ \.sha256$ ]]; then
            local basename=$(basename "$binary")
            local package_name="${basename}.tar.gz"
            local package_path="${DIST_DIR}/${package_name}"
            
            # Create temporary directory
            local temp_dir=$(mktemp -d)
            local package_dir="${temp_dir}/traceace-${VERSION}"
            
            mkdir -p "$package_dir"
            
            # Copy binary
            cp "$binary" "${package_dir}/traceace"
            if [[ "$basename" == *".exe" ]]; then
                mv "${package_dir}/traceace" "${package_dir}/traceace.exe"
            fi
            
            # Copy documentation
            cp README.md "${package_dir}/"
            cp docs/traceace.1 "${package_dir}/"
            
            # Create LICENSE file if it doesn't exist
            if [[ ! -f LICENSE ]]; then
                cat > "${package_dir}/LICENSE" << 'EOF'
MIT License

Copyright (c) 2024 TraceAce contributors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
EOF
            else
                cp LICENSE "${package_dir}/"
            fi
            
            # Create tarball
            tar -czf "$package_path" -C "$temp_dir" "traceace-${VERSION}"
            
            # Calculate SHA256 for package
            if command -v sha256sum &> /dev/null; then
                sha256sum "$package_path" > "${package_path}.sha256"
            elif command -v shasum &> /dev/null; then
                shasum -a 256 "$package_path" > "${package_path}.sha256"
            fi
            
            # Cleanup
            rm -rf "$temp_dir"
            
            print_success "Created package: ${package_name}"
        fi
    done
}

# Create DEB package
create_deb() {
    if ! command -v fpm &> /dev/null; then
        print_warning "fpm not found, skipping DEB package creation"
        return
    fi
    
    print_status "Creating DEB package..."
    
    local deb_output="${DIST_DIR}/traceace_${VERSION}_amd64.deb"
    
    fpm -s dir -t deb \
        --name traceace \
        --version "${VERSION}" \
        --architecture amd64 \
        --description "TraceAce - Blazing fast terminal log analyzer" \
        --url "https://github.com/loganalyzer/traceace" \
        --maintainer "TraceAce Team <support@example.com>" \
        --license MIT \
        --package "${deb_output}" \
        "${BUILD_DIR}/traceace-linux-amd64=/usr/bin/traceace" \
        "docs/traceace.1=/usr/share/man/man1/traceace.1"
    
    print_success "Created DEB package: $(basename "$deb_output")"
}

# Create RPM package
create_rpm() {
    if ! command -v fpm &> /dev/null; then
        print_warning "fpm not found, skipping RPM package creation"
        return
    fi
    
    print_status "Creating RPM package..."
    
    local rpm_output="${DIST_DIR}/traceace-${VERSION}-1.x86_64.rpm"
    
    fpm -s dir -t rpm \
        --name traceace \
        --version "${VERSION}" \
        --architecture x86_64 \
        --description "TraceAce - Blazing fast terminal log analyzer" \
        --url "https://github.com/loganalyzer/traceace" \
        --maintainer "TraceAce Team <support@example.com>" \
        --license MIT \
        --package "${rpm_output}" \
        "${BUILD_DIR}/traceace-linux-amd64=/usr/bin/traceace" \
        "docs/traceace.1=/usr/share/man/man1/traceace.1"
    
    print_success "Created RPM package: $(basename "$rpm_output")"
}

# Create Homebrew formula
create_homebrew_formula() {
    print_status "Creating Homebrew formula..."
    
    local formula_file="${DIST_DIR}/traceace.rb"
    local tarball_url="https://github.com/loganalyzer/traceace/releases/download/v${VERSION}/traceace-darwin-amd64.tar.gz"
    local tarball_path="${DIST_DIR}/traceace-darwin-amd64.tar.gz"
    
    # Calculate SHA256 for macOS tarball
    local sha256=""
    if [[ -f "$tarball_path" ]]; then
        if command -v sha256sum &> /dev/null; then
            sha256=$(sha256sum "$tarball_path" | cut -d' ' -f1)
        elif command -v shasum &> /dev/null; then
            sha256=$(shasum -a 256 "$tarball_path" | cut -d' ' -f1)
        fi
    fi
    
    cat > "$formula_file" << EOF
class Traceace < Formula
  desc "TraceAce - Blazing fast terminal log analyzer"
  homepage "https://github.com/loganalyzer/traceace"
  url "${tarball_url}"
  sha256 "${sha256}"
  license "MIT"
  version "${VERSION}"

  depends_on "go" => :build

  def install
    bin.install "traceace"
    man1.install "traceace.1"
  end

  test do
    system "#{bin}/traceace", "version"
  end
end
EOF
    
    print_success "Created Homebrew formula: $(basename "$formula_file")"
}

# Generate release notes
generate_release_notes() {
    print_status "Generating release notes..."
    
    local release_notes="${DIST_DIR}/RELEASE_NOTES.md"
    
    cat > "$release_notes" << EOF
# TraceAce v${VERSION}

## Features

- Real-time log tailing with automatic file rotation detection
- Two-pane view showing all logs and filtered results simultaneously
- Interactive search and filtering with regex and field-based queries
- Syntax highlighting for timestamps, log levels, IPs, status codes, UUIDs, URLs
- Structured log support with JSON/YAML parsing and collapsible views
- Multi-file monitoring with tab support and merged streams
- Bookmarking system for important log entries
- Export functionality supporting text, JSON, CSV, and HTML formats

## Installation

### Linux
\`\`\`bash
# Download and extract
wget https://github.com/loganalyzer/traceace/releases/download/v${VERSION}/traceace-linux-amd64.tar.gz
tar -xzf traceace-linux-amd64.tar.gz
sudo cp traceace-${VERSION}/traceace /usr/local/bin/
\`\`\`

### macOS
\`\`\`bash
# Using Homebrew (recommended)
brew install traceace

# Or download directly
curl -LO https://github.com/loganalyzer/traceace/releases/download/v${VERSION}/traceace-darwin-amd64.tar.gz
tar -xzf traceace-darwin-amd64.tar.gz
cp traceace-${VERSION}/traceace /usr/local/bin/
\`\`\`

### Windows
Download \`traceace-windows-amd64.tar.gz\` from the releases page and extract.

## Usage

\`\`\`bash
# Basic usage
traceace /var/log/app.log

# Multiple files
traceace /var/log/app.log /var/log/sys.log

# With theme
traceace --theme=light /var/log/app.log
\`\`\`

## Checksums

All release binaries include SHA256 checksums for verification.

EOF
    
    print_success "Generated release notes: $(basename "$release_notes")"
}

# Show usage
usage() {
    echo "Usage: $0 [OPTIONS] [COMMAND]"
    echo ""
    echo "Commands:"
    echo "  clean       Clean build directories"
    echo "  test        Run tests"
    echo "  lint        Run linters"
    echo "  build       Build all platforms"
    echo "  package     Create packages"
    echo "  deb         Create DEB package"
    echo "  rpm         Create RPM package"
    echo "  homebrew    Create Homebrew formula"
    echo "  release     Generate release notes"
    echo "  all         Run all steps (default)"
    echo ""
    echo "Options:"
    echo "  -v VERSION  Set version (default: 1.0.0)"
    echo "  -h          Show this help"
}

# Main execution
main() {
    local command=${1:-"all"}
    
    case "$command" in
        clean)
            clean
            ;;
        test)
            run_tests
            ;;
        lint)
            run_linters
            ;;
        build)
            build_all
            ;;
        package)
            create_packages
            ;;
        deb)
            create_deb
            ;;
        rpm)
            create_rpm
            ;;
        homebrew)
            create_homebrew_formula
            ;;
        release)
            generate_release_notes
            ;;
        all)
            clean
            run_tests
            run_linters
            build_all
            create_packages
            create_deb
            create_rpm
            create_homebrew_formula
            generate_release_notes
            ;;
        -h|--help)
            usage
            ;;
        *)
            print_error "Unknown command: $command"
            usage
            exit 1
            ;;
    esac
}

# Parse command line arguments
while getopts "v:h" opt; do
    case $opt in
        v)
            VERSION="$OPTARG"
            ;;
        h)
            usage
            exit 0
            ;;
        \?)
            echo "Invalid option: -$OPTARG" >&2
            usage
            exit 1
            ;;
    esac
done

shift $((OPTIND-1))

# Run main function
main "$@"

print_success "Build completed successfully!"
print_status "Binaries available in: $BUILD_DIR"
print_status "Packages available in: $DIST_DIR"

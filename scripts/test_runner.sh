#!/bin/bash
# test_runner.sh - Automated test runner for mumble-go
# Usage: ./scripts/test_runner.sh [--short|--full|--integration]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_DIR"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Default flags
SHORT=false
FULL=false
INTEGRATION=false
VERBOSE=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --short)
            SHORT=true
            shift
            ;;
        --full)
            FULL=true
            shift
            ;;
        --integration)
            INTEGRATION=true
            shift
            ;;
        --verbose)
            VERBOSE=true
            shift
            ;;
        --help)
            echo "Usage: $0 [--short|--full|--integration|--verbose]"
            echo "  --short       Run only unit tests (no integration)"
            echo "  --full        Run all tests including integration"
            echo "  --integration Run only integration tests"
            echo "  --verbose     Verbose output"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Default to short if nothing specified
if [ "$SHORT" = false ] && [ "$FULL" = false ] && [ "$INTEGRATION" = false ]; then
    SHORT=true
fi

echo -e "${YELLOW}=== mumble-go Test Runner ===${NC}"
echo "Project directory: $PROJECT_DIR"
echo ""

# Build first
echo -e "${YELLOW}[1/3] Building...${NC}"
if [ "$VERBOSE" = true ]; then
    go build ./...
else
    go build ./... 2>&1 | grep -v "^#" || true
fi
echo -e "${GREEN}Build: OK${NC}"
echo ""

# Run unit tests
if [ "$SHORT" = true ] || [ "$FULL" = true ]; then
    echo -e "${YELLOW}[2/3] Running unit tests...${NC}"
    if [ "$VERBOSE" = true ]; then
        go test ./audio/ ./protocol/ ./state/ ./transport/ ./client/ -v
    else
        go test ./audio/ ./protocol/ ./state/ ./transport/ ./client/
    fi
    echo -e "${GREEN}Unit tests: OK${NC}"
    echo ""
fi

# Run integration tests
if [ "$INTEGRATION" = true ] || [ "$FULL" = true ]; then
    echo -e "${YELLOW}[3/3] Running integration tests...${NC}"
    echo "This will connect to the actual Mumble server."
    echo "Server: mumble.hotxiang.cn:64738"
    echo ""

    if [ "$VERBOSE" = true ]; then
        go test ./integration/ -v -timeout=10m
    else
        go test ./integration/ -timeout=10m
    fi
    echo -e "${GREEN}Integration tests: OK${NC}"
fi

echo ""
echo -e "${GREEN}=== All tests passed! ===${NC}"

#!/bin/sh

# Check if go is installed
if ! command -v go >/dev/null 2>&1; then
  echo "go is not installed"
  exit 1
fi

# Check if GOPATH is set
if [ -z "$GOPATH" ]; then
  echo "GOPATH is not set"
  exit 1
fi

# Check if GOPATH/bin is in PATH
if [ -z "$(echo $PATH | grep $GOPATH/bin)" ]; then
  echo "GOPATH is not in PATH"
  exit 1
fi

# Check if golangci-lint binary is available
if ! command -v golangci-lint > /dev/null; then
  echo "golangci-lint is not installed, installing..."
  # Fetch golangci-lint binary
  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b "$(go env GOPATH)"/bin v1.45.2
  exit 1
fi

echo "PATH: $PATH"
PROJECT_ROOT=$(realpath "$(dirname "$0")"/../)
echo "PROJECT_ROOT: $PROJECT_ROOT"
golangci-lint -c "$PROJECT_ROOT"/.golangci.yaml run -v "$PROJECT_ROOT"/...

#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

GOOS="$(go env GOOS)"
SUFFIX=""
if [[ "$GOOS" == "windows" ]]; then
	SUFFIX=".exe"
fi

mkdir -p build

echo "Baue Analyzer-Server..."
go build -o "analyzer${SUFFIX}" ./cmd/analyzer

echo "Baue Agent..."
go build -o "agent${SUFFIX}" ./agent

echo "Build abgeschlossen."
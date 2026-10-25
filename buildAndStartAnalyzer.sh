#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

SUFFIX=""
if [[ "$(go env GOOS)" == "windows" ]]; then
	SUFFIX=".exe"
fi

echo "Baue Analyzer-Server..."
go build -o "analyzer${SUFFIX}" ./cmd/analyzer

echo "Starte Analyzer-Server..."
exec "./analyzer${SUFFIX}"

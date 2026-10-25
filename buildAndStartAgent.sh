#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

SUFFIX=""
if [[ "$(go env GOOS)" == "windows" ]]; then
	SUFFIX=".exe"
fi

echo "Baue Agent..."
go build -o "agent${SUFFIX}" ./agent

echo "Starte Agent..."
exec "./agent${SUFFIX}"

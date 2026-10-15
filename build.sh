#!/usr/bin/env bash
set -euo pipefail
go test ./...
go build -o siriusxm-sidecar ./server

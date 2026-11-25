#!/bin/bash
cd "$(dirname "$0")"

if ! command -v python3.14 >/dev/null 2>&1; then
  echo "Python 3.14 is required. Install it from python.org/downloads/macos/."
  read -r -p "Press Return to close."
  exit 1
fi

if ! python3.14 -c 'import requests' >/dev/null 2>&1; then
  echo "Install the Python requests package with: python3.14 -m pip install requests"
  read -r -p "Press Return to close."
  exit 1
fi

if ! command -v ffmpeg >/dev/null 2>&1; then
  echo "FFmpeg is required and must be available on PATH."
  read -r -p "Press Return to close."
  exit 1
fi

case "$(uname -m)" in
  arm64) executable="siriusxm-sidecar-darwin-arm64" ;;
  x86_64) executable="siriusxm-sidecar-darwin-amd64" ;;
  *) echo "Unsupported Mac processor: $(uname -m)"; read -r -p "Press Return to close."; exit 1 ;;
esac

read -r -p "SiriusXM username: " username
read -r -s -p "SiriusXM password: " password
echo
exec env SXM_PYTHON=python3.14 "./$executable" "$username" "$password"

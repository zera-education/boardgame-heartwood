#!/usr/bin/env bash
# Install the story transcription worker on the Mac mini (idempotent):
#   1. build hw-transcribe into ~/.local/bin;
#   2. make the worker's token, once, in ~/.config/heartwood/transcribe-token (mode 600). The server knows only its
#      SHA-256 (transcribeTokenHash in recording.go): a new token needs that hash changed and a deploy;
#   3. check the server accepts the token (so run this after the server with recording.go is deployed);
#   4. add the lanes job heartwood/stories/transcribe (every 5 minutes, survives reboots: lanes' launchd tick) and
#      switch the heartwood domain live.
# Usage: ops/transcribe/install.sh [server]   (default https://heartwood.zera.edu.my)
set -euo pipefail
cd "$(dirname "$0")/../.."
export PATH="$HOME/.local/bin:/opt/homebrew/bin:/usr/local/bin:$PATH"
SERVER="${1:-${HW_SERVER:-https://heartwood.zera.edu.my}}"
LANES="$HOME/lanes"
TOKEN="$HOME/.config/heartwood/transcribe-token"

mkdir -p "$HOME/.local/bin"
go build -o "$HOME/.local/bin/hw-transcribe" ./ops/transcribe
echo "built ~/.local/bin/hw-transcribe"

if [ ! -s "$TOKEN" ]; then
  mkdir -p "$(dirname "$TOKEN")" && chmod 700 "$(dirname "$TOKEN")"
  (umask 077; openssl rand -hex 32 > "$TOKEN")
  echo "made a new token: set transcribeTokenHash in recording.go to"
  printf %s "$(tr -d '\n' < "$TOKEN")" | shasum -a 256 | cut -d' ' -f1
  echo "then deploy and run this again."
  exit 1
fi
chmod 600 "$TOKEN"

code=$(curl -s -o /dev/null -w '%{http_code}' -A heartwood-transcribe/1 -H "Authorization: Bearer $(tr -d '\n' < "$TOKEN")" "$SERVER/api/transcribe/queue?limit=1")
if [ "$code" != 200 ]; then
  echo "$SERVER/api/transcribe/queue answered $code (200 needed): deploy the server with recording.go first, or check the token's hash."
  exit 1
fi
echo "$SERVER accepts the token"

steward run transcribe health >/dev/null || { echo "Steward's Whisper isn't ready: see ~/steward/tools/transcribe/README.md, Setup"; exit 1; }

export LANES_DB="$LANES/data/lanes.db"
LANES_SEED="$PWD/ops/transcribe/lanes-seed.json" "$LANES/bin/lanes" seed
"$LANES/bin/lanes" mode heartwood live
"$LANES/bin/lanes" ls heartwood

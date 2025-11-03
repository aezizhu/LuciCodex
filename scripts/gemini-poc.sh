#!/bin/sh
set -eu

# Author: aezizhu

# Minimal PoC for calling Gemini from a router with curl and jq.
# Requires: GEMINI_API_KEY and jq installed.

if [ -z "${GEMINI_API_KEY:-}" ]; then
  echo "GEMINI_API_KEY is required" >&2
  exit 1
fi

MODEL=${MODEL:-gemini-1.5-flash}
ENDPOINT=${ENDPOINT:-https://generativelanguage.googleapis.com/v1beta}
PROMPT=${1:-"Say hello as JSON"}

BODY=$(cat <<EOF
{
  "contents": [{ "parts": [{ "text": "${PROMPT}" }] }],
  "generationConfig": {"response_mime_type":"application/json"}
}
EOF
)

curl -sS -X POST \
  -H 'Content-Type: application/json' \
  -d "$BODY" \
  "$ENDPOINT/models/$MODEL:generateContent?key=$GEMINI_API_KEY" | jq -r '.candidates[0].content.parts[0].text'



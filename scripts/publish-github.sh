#!/bin/sh
set -eu

# Publish this repo to GitHub using HTTPS and a Personal Access Token.
# Required env:
#   GITHUB_USER    (e.g., aezizhu)
#   GITHUB_REPO    (e.g., LuciCodex)
#   GITHUB_TOKEN   (repo scope)

if [ -z "${GITHUB_USER:-}" ] || [ -z "${GITHUB_REPO:-}" ] || [ -z "${GITHUB_TOKEN:-}" ]; then
  echo "Set GITHUB_USER, GITHUB_REPO, and GITHUB_TOKEN (repo scope)." >&2
  exit 1
fi

# Create repo if it does not exist
curl -s -H "Authorization: Bearer $GITHUB_TOKEN" \
  -H "Accept: application/vnd.github+json" \
  https://api.github.com/user/repos \
  -d '{"name":"'"$GITHUB_REPO"'","description":"Natural language CLI and LuCI UI for OpenWrt; providers: Gemini, OpenAI, Anthropic.","private":false}' >/dev/null || true

git remote remove origin 2>/dev/null || true
git remote add origin https://github.com/$GITHUB_USER/$GITHUB_REPO.git
git branch -M main
git push -u https://$GITHUB_USER:$GITHUB_TOKEN@github.com/$GITHUB_USER/$GITHUB_REPO.git main

# About + topics (SEO)
curl -s -X PATCH \
  -H "Authorization: Bearer $GITHUB_TOKEN" \
  -H "Accept: application/vnd.github+json" \
  https://api.github.com/repos/$GITHUB_USER/$GITHUB_REPO \
  -d '{"homepage":"https://openwrt.org/","description":"Secure natural-language CLI for OpenWrt (LuCI UI, policy engine, Gemini/OpenAI/Anthropic).","has_wiki":true}' >/dev/null || true

curl -s -X PUT \
  -H "Authorization: Bearer $GITHUB_TOKEN" \
  -H "Accept: application/vnd.github.mercy-preview+json" \
  https://api.github.com/repos/$GITHUB_USER/$GITHUB_REPO/topics \
  -d '{"names":["openwrt","luci","router","gemini","anthropic","openai","cli","security","policy-engine","uci","ubus","fw4","opkg","networking","automation","devops","embedded-linux"]}' >/dev/null || true

echo "Pushed to https://github.com/$GITHUB_USER/$GITHUB_REPO"



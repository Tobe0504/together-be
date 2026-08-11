#!/usr/bin/env bash
# Shared helper: set KEY=VALUE in the root .env, preserving every other key.
#
# Exists because dev-up.sh and tunnel-up.sh each own different keys in the
# same file (LAN_IP vs TUNNEL_FRONTEND_ORIGIN), and the LiveKit Cloud
# credentials live there too — an earlier version rewrote .env wholesale and
# silently destroyed whatever the other script (or you) had put there.
env_upsert() {
  local key="$1" value="$2" file="${3:-.env}"
  touch "$file"
  local tmp
  tmp="$(mktemp)"
  grep -v "^${key}=" "$file" > "$tmp" 2>/dev/null || true
  printf '%s=%s\n' "$key" "$value" >> "$tmp"
  mv "$tmp" "$file"
}

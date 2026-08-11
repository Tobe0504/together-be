#!/usr/bin/env bash
# Starts LiveKit + the backend for LAN/phone testing, without needing to
# manually chase this machine's LAN IP through .env and livekit.yaml every
# time the network reassigns it (which, on this network, has been often).
# Run this instead of `docker compose up -d` directly.
set -euo pipefail
cd "$(dirname "$0")/.."

detect_lan_ip() {
  for iface in en0 en1 en2; do
    ip="$(ipconfig getifaddr "$iface" 2>/dev/null || true)"
    if [ -n "$ip" ]; then
      echo "$ip"
      return 0
    fi
  done
  # Fallback: first private-range IPv4 on any interface (covers Ethernet,
  # non-standard interface names, etc.)
  ifconfig 2>/dev/null | awk '/inet /{print $2}' | grep -v '^127\.' | \
    grep -E '^(10\.|172\.(1[6-9]|2[0-9]|3[0-1])\.|192\.168\.)' | head -1
}

LAN_IP="$(detect_lan_ip)"
if [ -z "$LAN_IP" ]; then
  echo "Couldn't detect a LAN IP (no active Wi-Fi/Ethernet with a private address?). Aborting." >&2
  exit 1
fi

echo "Detected LAN IP: $LAN_IP"

# .env — read by `docker compose` automatically. Only LAN_IP is ours; any
# LiveKit Cloud credentials or tunnel origin in there must survive.
# shellcheck source=scripts/env-upsert.sh
. "$(dirname "$0")/env-upsert.sh"
env_upsert LAN_IP "$LAN_IP"

# livekit.yaml's rtc.node_ip — plain YAML, no env substitution, so this is
# the one spot that still needs a literal rewrite rather than a variable.
sed -i '' -E "s/node_ip: [0-9.]+/node_ip: $LAN_IP/" livekit.yaml

echo "Updated .env and livekit.yaml. Starting containers..."
docker compose up -d --build

echo ""
echo "Ready. On your phone (same Wi-Fi), open:"
echo "  http://$LAN_IP:3001"
echo ""
echo "If the frontend isn't running yet:"
echo "  cd frontend && npm run dev"

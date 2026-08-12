#!/usr/bin/env bash

set -uo pipefail
cd "$(dirname "$0")"

BASE="${TOGETHER_TEST_BASE:-http://127.0.0.1:8080}"
if ! curl -sf -m 5 "$BASE/healthz" >/dev/null; then
  echo "Backend not reachable at $BASE — start it with ./scripts/dev-up.sh" >&2
  exit 1
fi

fail=0
echo "### REST API ###"
python3 api.py || fail=1
echo ""
echo "### Realtime (WebSocket) ###"
node realtime.mjs || fail=1
echo ""
echo "### Role changes take effect immediately ###"
python3 roles.py || fail=1
echo ""
echo "### Local-mode WebRTC signaling relay ###"
node local-mesh.mjs || fail=1
echo ""
echo "### Local room capacity ###"
python3 local-capacity.py || fail=1
echo ""
echo "### One presenter at a time ###"
python3 single-presenter.py || fail=1

echo ""
if [ "$fail" -eq 0 ]; then
  echo "All suites completed. Check for any FAIL lines above."
else
  echo "A suite exited non-zero." >&2
fi
exit "$fail"

#!/usr/bin/env python3
"""Local rooms cap participant count; online rooms don't."""
import json
import urllib.request
import urllib.error

BASE = "http://127.0.0.1:8080"
LIMIT = 12
results = []


def call(method, path, token=None, body=None):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(BASE + path, data=data, method=method)
    if data:
        req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(req, timeout=10) as r:
            return r.status, json.loads(r.read().decode() or "null")
    except urllib.error.HTTPError as e:
        body = e.read().decode()
        try:
            return e.code, json.loads(body)
        except Exception:
            return e.code, body


def ok(cond, name, detail=""):
    results.append((bool(cond), name, detail))


def fill(mode):
    """Create a room of `mode` and join until rejected (or well past the cap)."""
    status, room = call("POST", "/api/rooms",
                        body={"name": f"Capacity {mode}", "mode": mode,
                              "displayName": "Owner"})
    rid = room["room"]["id"]
    joined, last_status, last_body = 1, None, None
    for i in range(LIMIT + 3):
        last_status, last_body = call("POST", f"/api/rooms/{rid}/join",
                                      body={"displayName": f"Guest{i}"})
        if last_status == 200:
            joined += 1
        else:
            break
    return rid, joined, last_status, last_body


print("=" * 70)

rid, joined, status, body = fill("local")
ok(status == 409, "local room rejects the over-limit joiner with 409",
   f"got {status}")
ok(joined == LIMIT, f"local room admitted exactly {LIMIT}", f"got {joined}")
msg = body.get("error", "") if isinstance(body, dict) else str(body)
ok("full" in msg.lower(), "rejection explains the room is full", msg[:90])
ok("directly to every other device" in msg or "limited to" in msg,
   "message explains WHY local rooms are capped", msg[:90])
call("POST", f"/api/rooms/{rid}/end")

rid2, joined2, status2, _ = fill("online")
ok(status2 == 200 and joined2 > LIMIT,
   "online room is NOT capped (SFU fans out, no per-viewer cost)",
   f"joined={joined2} status={status2}")
call("POST", f"/api/rooms/{rid2}/end")

print("=" * 70)
for good, name, detail in results:
    print(f"[{'PASS' if good else 'FAIL'}] {name}" + (f"  <- {detail}" if not good else ""))
print("=" * 70)
passed = sum(1 for g, *_ in results if g)
print(f"{passed}/{len(results)} passed")
raise SystemExit(0 if passed == len(results) else 1)

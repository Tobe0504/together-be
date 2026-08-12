#!/usr/bin/env python3
"""Only one live screen per room, including under a simultaneous race."""
import json
import urllib.request
import urllib.error
from concurrent.futures import ThreadPoolExecutor

BASE = "http://127.0.0.1:8080"
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
        b = e.read().decode()
        try:
            return e.code, json.loads(b)
        except Exception:
            return e.code, b


def ok(cond, name, detail=""):
    results.append((bool(cond), name, detail))


# Owner plus a guest promoted to PRESENTER, so both are *allowed* to present
# and only the exclusivity rule can separate them.
_, room = call("POST", "/api/rooms",
               body={"name": "Presenter lock", "mode": "online", "displayName": "Owner"})
RID, OWNER_T = room["room"]["id"], room["token"]
OWNER_PID = room["participant"]["id"]

_, guest = call("POST", f"/api/rooms/{RID}/join", body={"displayName": "Guest"})
GUEST_T, GUEST_PID = guest["token"], guest["participant"]["id"]
call("PATCH", f"/api/rooms/{RID}/participants/{GUEST_PID}/role",
     token=OWNER_T, body={"role": "PRESENTER"})

# 1. First to start wins.
s1, _ = call("POST", f"/api/rooms/{RID}/screen/start", token=OWNER_T)
ok(s1 == 200, "first presenter can start", f"got {s1}")

# 2. Second is refused, with a reason worth reading.
s2, b2 = call("POST", f"/api/rooms/{RID}/screen/start", token=GUEST_T)
msg = b2.get("error", "") if isinstance(b2, dict) else str(b2)
ok(s2 == 409, "second presenter is refused while the first is live", f"got {s2}")
ok("presenting" in msg.lower(), "refusal explains why", msg[:80])

# 3. The room reports who holds the stage, so other clients can disable
#    their own button and name them.
_, r = call("GET", f"/api/rooms/{RID}", token=GUEST_T)
ok(r.get("primaryPresenterId") == OWNER_PID,
   "room reports the current presenter", str(r.get("primaryPresenterId")))

# 4. Stopping frees it for someone else.
call("POST", f"/api/rooms/{RID}/screen/stop", token=OWNER_T)
s4, _ = call("POST", f"/api/rooms/{RID}/screen/start", token=GUEST_T)
ok(s4 == 200, "the slot is released on stop", f"got {s4}")

# 5. Leaving mid-share frees it too — otherwise a closed tab locks the
#    stage for the life of the room.
call("POST", f"/api/rooms/{RID}/leave", token=GUEST_T)
s5, _ = call("POST", f"/api/rooms/{RID}/screen/start", token=OWNER_T)
ok(s5 == 200, "leaving mid-share releases the stage", f"got {s5}")
call("POST", f"/api/rooms/{RID}/screen/stop", token=OWNER_T)

# 6. The race: several clients pressing Present at the same instant. A
#    read-then-write check in Go would let more than one through here.
_, room2 = call("POST", "/api/rooms",
                body={"name": "Race", "mode": "online", "displayName": "Owner"})
RID2, OWNER2_T = room2["room"]["id"], room2["token"]
tokens = [OWNER2_T]
for i in range(4):
    _, g = call("POST", f"/api/rooms/{RID2}/join", body={"displayName": f"P{i}"})
    call("PATCH", f"/api/rooms/{RID2}/participants/{g['participant']['id']}/role",
         token=OWNER2_T, body={"role": "PRESENTER"})
    tokens.append(g["token"])

with ThreadPoolExecutor(max_workers=len(tokens)) as ex:
    codes = list(ex.map(
        lambda t: call("POST", f"/api/rooms/{RID2}/screen/start", token=t)[0], tokens))
winners = sum(1 for c in codes if c == 200)
ok(winners == 1, "exactly one wins a simultaneous race",
   f"{winners} of {len(codes)} succeeded: {codes}")
call("POST", f"/api/rooms/{RID2}/end", token=OWNER2_T)
call("POST", f"/api/rooms/{RID}/end", token=OWNER_T)

print("=" * 70)
for good, name, detail in results:
    print(f"[{'PASS' if good else 'FAIL'}] {name}" + (f"  <- {detail}" if not good else ""))
print("=" * 70)
passed = sum(1 for g, *_ in results if g)
print(f"{passed}/{len(results)} passed")
raise SystemExit(0 if passed == len(results) else 1)

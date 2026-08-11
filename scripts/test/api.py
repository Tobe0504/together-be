#!/usr/bin/env python3
"""End-to-end exercise of every Together API flow, to find what's broken."""
import json
import urllib.request
import urllib.error

import os
BASE = os.environ.get("TOGETHER_TEST_BASE", "http://127.0.0.1:8080")
results = []


def call(method, path, token=None, body=None, expect=None, label=None):
    url = BASE + path
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(url, data=data, method=method)
    if data:
        req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(req, timeout=10) as r:
            status, payload = r.status, r.read().decode()
    except urllib.error.HTTPError as e:
        status, payload = e.code, e.read().decode()
    except Exception as e:
        status, payload = 0, str(e)

    try:
        parsed = json.loads(payload) if payload else None
    except Exception:
        parsed = payload

    name = label or f"{method} {path}"
    ok = (status == expect) if expect else (200 <= status < 300)
    results.append((ok, name, status, parsed))
    return parsed, status


print("=" * 70)

room, _ = call("POST", "/api/rooms", body={
    "name": "QA room", "mode": "online", "displayName": "Owner"},
    expect=201, label="create room")
RID = room["room"]["id"]
OWNER_T = room["token"]
CODE = room["room"]["joinCode"]

call("GET", f"/api/rooms/by-code/{CODE}", label="lookup by join code")

guest, _ = call("POST", f"/api/rooms/{RID}/join",
                body={"displayName": "Guest"}, label="guest join")
GUEST_T = guest["token"]
GUEST_PID = guest["participant"]["id"]

call("GET", f"/api/rooms/{RID}", token=OWNER_T, label="get room")
plist, _ = call("GET", f"/api/rooms/{RID}/participants", token=OWNER_T,
                label="list participants")
print(f"  participants: {[(p['displayName'], p['role']) for p in (plist or [])]}")

call("PATCH", f"/api/rooms/{RID}/participants/{GUEST_PID}/role",
     token=OWNER_T, body={"role": "PRESENTER"}, label="promote guest to PRESENTER")
plist, _ = call("GET", f"/api/rooms/{RID}/participants", token=OWNER_T,
                label="verify role after promote")
gp = next((p for p in (plist or []) if p["id"] == GUEST_PID), None)
print(f"  guest role now: {gp and gp['role']}  (expected PRESENTER)")
results.append((gp and gp["role"] == "PRESENTER", "role actually changed to PRESENTER",
                "-", gp and gp["role"]))

call("POST", f"/api/rooms/{RID}/presenter/transfer", token=OWNER_T,
     body={"toParticipantId": GUEST_PID}, label="transfer presenter to guest")
plist, _ = call("GET", f"/api/rooms/{RID}/participants", token=OWNER_T,
                label="verify after transfer")
gp = next((p for p in (plist or []) if p["id"] == GUEST_PID), None)
print(f"  guest isPrimary: {gp and gp.get('isPrimary')}  (expected True)")
results.append((bool(gp and gp.get("isPrimary")), "transfer set isPrimary on target",
                "-", gp and gp.get("isPrimary")))

call("PATCH", f"/api/rooms/{RID}/participants/{GUEST_PID}/role",
     token=OWNER_T, body={"role": "PARTICIPANT"}, label="demote guest to PARTICIPANT")
call("POST", f"/api/rooms/{RID}/actions", token=GUEST_T,
     body={"actionType": "REQUEST_STAGE"}, label="guest requests stage")
actions, _ = call("GET", f"/api/rooms/{RID}/actions", token=OWNER_T,
                  label="owner lists pending actions")
print(f"  pending actions: {actions}")
if actions:
    AID = actions[0]["id"]
    call("PATCH", f"/api/rooms/{RID}/actions/{AID}", token=OWNER_T,
         body={"accept": True}, label="owner accepts stage request")
else:
    results.append((False, "pending action was listed", "-", "none returned"))

call("POST", f"/api/rooms/{RID}/screen/start", token=OWNER_T, label="screen start")
call("POST", f"/api/rooms/{RID}/screen/stop", token=OWNER_T, label="screen stop")

call("POST", f"/api/rooms/{RID}/pause", token=OWNER_T, label="pause room")
call("POST", f"/api/rooms/{RID}/resume", token=OWNER_T, label="resume room")

call("POST", f"/api/rooms/{RID}/invites", token=OWNER_T, label="create invite")

call("POST", "/api/livekit/token", token=OWNER_T, body={"roomId": RID},
     label="livekit token")

call("PATCH", f"/api/rooms/{RID}", token=OWNER_T,
     body={"name": "Renamed", "accessProtected": False}, label="rename room")

call("DELETE", f"/api/rooms/{RID}/participants/{GUEST_PID}", token=GUEST_T,
     expect=403, label="NEGATIVE guest cannot kick (expect 403)")

call("DELETE", f"/api/rooms/{RID}/participants/{GUEST_PID}", token=OWNER_T,
     label="owner kicks guest")

call("POST", f"/api/rooms/{RID}/end", token=OWNER_T, label="end room")

call("POST", f"/api/rooms/{RID}/join", body={"displayName": "Late"},
     expect=410, label="NEGATIVE join ended room (expect 410)")

print("=" * 70)
passed = sum(1 for ok, *_ in results if ok)
for ok, name, status, payload in results:
    mark = "PASS" if ok else "FAIL"
    extra = ""
    if not ok:
        extra = f"  <- status={status} body={json.dumps(payload)[:160] if not isinstance(payload, str) else payload[:160]}"
    print(f"[{mark}] {name}{extra}")
print("=" * 70)
print(f"{passed}/{len(results)} passed")

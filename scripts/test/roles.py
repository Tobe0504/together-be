#!/usr/bin/env python3
"""Does a promoted participant actually gain their new powers?"""
import json, urllib.request, urllib.error, base64

import os
BASE = os.environ.get("TOGETHER_TEST_BASE", "http://127.0.0.1:8080")


def call(method, path, token=None, body=None):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(BASE + path, data=data, method=method)
    if data:
        req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(req, timeout=10) as r:
            return json.loads(r.read().decode() or "null"), r.status
    except urllib.error.HTTPError as e:
        try:
            return json.loads(e.read().decode() or "null"), e.code
        except Exception:
            return None, e.code


room, _ = call("POST", "/api/rooms", body={
    "name": "Role check", "mode": "online", "displayName": "Owner"})
RID, OWNER_T = room["room"]["id"], room["token"]

guest, _ = call("POST", f"/api/rooms/{RID}/join", body={"displayName": "Guest"})
GUEST_T, GPID = guest["token"], guest["participant"]["id"]


def role_in(tok):
    p = tok.split(".")[1]
    p += "=" * (-len(p) % 4)
    return json.loads(base64.urlsafe_b64decode(p)).get("role")


print(f"guest role in DB at join : {guest['participant']['role']}")
print(f"guest role inside token  : {role_in(GUEST_T)}")

_, s = call("POST", f"/api/rooms/{RID}/screen/start", token=GUEST_T)
print(f"\nbefore promotion, guest screen/start -> {s} (403 expected)")

_, s = call("PATCH", f"/api/rooms/{RID}/participants/{GPID}/role",
            token=OWNER_T, body={"role": "PRESENTER"})
print(f"owner promotes guest to PRESENTER -> {s}")

plist, _ = call("GET", f"/api/rooms/{RID}/participants", token=OWNER_T)
db_role = next(p["role"] for p in plist if p["id"] == GPID)
print(f"guest role in DB now      : {db_role}")
print(f"guest role in their token : {role_in(GUEST_T)}  <-- unchanged, token was minted at join")

_, s = call("POST", f"/api/rooms/{RID}/screen/start", token=GUEST_T)
print(f"\nAFTER promotion, guest screen/start -> {s}")
print("   200 = works.  403 = BUG: promotion doesn't take effect for the promoted user.")

_, s2 = call("POST", f"/api/rooms/{RID}/presenter/transfer", token=GUEST_T,
             body={"toParticipantId": GPID})
print(f"AFTER promotion, guest presenter/transfer -> {s2} (200 expected for PRESENTER)")

call("POST", f"/api/rooms/{RID}/end", token=OWNER_T)

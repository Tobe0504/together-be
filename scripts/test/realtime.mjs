const BASE = process.env.TOGETHER_TEST_BASE || "http://127.0.0.1:8080";
const WS = BASE.replace(/^http/, "ws");
const results = [];
const ok = (cond, name, detail = "") => results.push([!!cond, name, detail]);

async function api(method, path, { token, body } = {}) {
  const res = await fetch(BASE + path, {
    method,
    headers: {
      ...(body ? { "Content-Type": "application/json" } : {}),
      ...(token ? { Authorization: "Bearer " + token } : {}),
    },
    body: body ? JSON.stringify(body) : undefined,
  });
  const text = await res.text();
  let json = null;
  try {
    json = text ? JSON.parse(text) : null;
  } catch {
    json = text;
  }
  return { status: res.status, json };
}

function open(roomId, token) {
  const sock = new WebSocket(
    `${WS}/ws/rooms/${roomId}?token=${encodeURIComponent(token)}`,
  );
  sock.received = [];
  sock.addEventListener("message", (e) => {
    try {
      sock.received.push(JSON.parse(e.data));
    } catch {}
  });
  return new Promise((resolve, reject) => {
    sock.addEventListener("open", () => resolve(sock));
    sock.addEventListener("error", () => reject(new Error("ws error")));
    setTimeout(() => reject(new Error("ws open timeout")), 5000);
  });
}

const wait = (ms) => new Promise((r) => setTimeout(r, ms));
const sawType = (sock, type) => sock.received.some((e) => e.type === type);

const room = await api("POST", "/api/rooms", {
  body: { name: "WS test", mode: "online", displayName: "Owner" },
});
const RID = room.json.room.id;
const OWNER_T = room.json.token;

const ownerSock = await open(RID, OWNER_T);
ok(true, "owner websocket connects");

const guest = await api("POST", `/api/rooms/${RID}/join`, {
  body: { displayName: "Guest" },
});
const GUEST_T = guest.json.token;
const GPID = guest.json.participant.id;
const guestSock = await open(RID, GUEST_T);
await wait(400);
ok(sawType(ownerSock, "PARTICIPANT_JOINED"), "owner sees PARTICIPANT_JOINED");

ownerSock.send(
  JSON.stringify({
    type: "ANNOTATION_CREATED",
    roomId: RID,
    payload: {
      id: "a1",
      type: "pen",
      points: [
        [0.1, 0.1],
        [0.2, 0.2],
      ],
      color: "#000",
    },
  }),
);
await wait(400);
ok(
  sawType(guestSock, "ANNOTATION_CREATED"),
  "guest receives owner's annotation",
);

const ownerBefore = ownerSock.received.filter(
  (e) => e.type === "ANNOTATION_CREATED",
).length;
guestSock.send(
  JSON.stringify({
    type: "ANNOTATION_CREATED",
    roomId: RID,
    payload: { id: "b1", type: "pen", points: [[0.3, 0.3]], color: "#000" },
  }),
);
await wait(400);
const ownerAfter = ownerSock.received.filter(
  (e) => e.type === "ANNOTATION_CREATED",
).length;
ok(
  ownerAfter === ownerBefore,
  "PARTICIPANT's annotation is rejected (permission enforced)",
  `before=${ownerBefore} after=${ownerAfter}`,
);

guestSock.send(
  JSON.stringify({
    type: "REACTION_SENT",
    roomId: RID,
    payload: { emoji: "👏" },
  }),
);
await wait(400);
ok(sawType(ownerSock, "REACTION_SENT"), "PARTICIPANT's reaction is allowed");

await api("PATCH", `/api/rooms/${RID}/participants/${GPID}/role`, {
  token: OWNER_T,
  body: { role: "PRESENTER" },
});
await wait(500);
ok(
  sawType(guestSock, "PARTICIPANT_UPDATED") ||
    sawType(ownerSock, "PARTICIPANT_UPDATED"),
  "PARTICIPANT_UPDATED broadcast on promotion",
);

const before2 = ownerSock.received.filter(
  (e) => e.type === "ANNOTATION_CREATED",
).length;
guestSock.send(
  JSON.stringify({
    type: "ANNOTATION_CREATED",
    roomId: RID,
    payload: { id: "c1", type: "pen", points: [[0.5, 0.5]], color: "#000" },
  }),
);
await wait(500);
const after2 = ownerSock.received.filter(
  (e) => e.type === "ANNOTATION_CREATED",
).length;
ok(
  after2 > before2,
  "promoted PRESENTER can annotate on the SAME open socket",
  `before=${before2} after=${after2}`,
);

await api("POST", `/api/rooms/${RID}/end`, { token: OWNER_T });
await wait(400);
ok(sawType(guestSock, "ROOM_ENDED"), "guest sees ROOM_ENDED");

ownerSock.close();
guestSock.close();

console.log("=".repeat(66));
for (const [pass, name, detail] of results) {
  console.log(
    `[${pass ? "PASS" : "FAIL"}] ${name}${!pass && detail ? "  <- " + detail : ""}`,
  );
}
console.log("=".repeat(66));
console.log(`${results.filter(([p]) => p).length}/${results.length} passed`);

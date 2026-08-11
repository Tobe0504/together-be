const BASE = process.env.TOGETHER_TEST_BASE || "http://127.0.0.1:8080";
const WS = BASE.replace(/^http/, "ws");

async function api(method, path, { token, body } = {}) {
  const res = await fetch(BASE + path, {
    method,
    headers: {
      ...(body ? { "Content-Type": "application/json" } : {}),
      ...(token ? { Authorization: "Bearer " + token } : {}),
    },
    body: body ? JSON.stringify(body) : undefined,
  });
  const t = await res.text();
  return { status: res.status, json: t ? JSON.parse(t) : null };
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
    setTimeout(() => reject(new Error("timeout")), 5000);
  });
}
const wait = (ms) => new Promise((r) => setTimeout(r, ms));

const results = [];
const ok = (c, n, d = "") => results.push([!!c, n, d]);

const room = await api("POST", "/api/rooms", {
  body: { name: "Mesh test", mode: "local", displayName: "Host" },
});
const RID = room.json.room.id;
const HOST_T = room.json.token;
const HOST_PID = room.json.participant.id;
ok(
  room.json.room.mode === "local",
  "room created in local mode",
  room.json.room.mode,
);

const hostSock = await open(RID, HOST_T);
await wait(300);

ok(
  hostSock.received.some((e) => e.type === "PEERS_ANNOUNCE"),
  "host receives PEERS_ANNOUNCE on connect",
);

const guest = await api("POST", `/api/rooms/${RID}/join`, {
  body: { displayName: "Guest" },
});
const GUEST_T = guest.json.token;
const GUEST_PID = guest.json.participant.id;
const guestSock = await open(RID, GUEST_T);
await wait(400);

const announce = guestSock.received.find((e) => e.type === "PEERS_ANNOUNCE");
ok(
  announce && Array.isArray(announce.data) && announce.data.includes(HOST_PID),
  "guest's PEERS_ANNOUNCE lists the host to connect to",
  JSON.stringify(announce?.data),
);

guestSock.send(
  JSON.stringify({
    type: "WEBRTC_OFFER",
    roomId: RID,
    to: HOST_PID,
    data: { type: "offer", sdp: "INITIAL_OFFER_SDP" },
  }),
);
await wait(300);
const hostGotOffer = hostSock.received.filter((e) => e.type === "WEBRTC_OFFER");
ok(
  hostGotOffer.length === 1,
  "host receives the guest's initial offer",
  `count=${hostGotOffer.length}`,
);
ok(
  hostGotOffer[0]?.from === GUEST_PID,
  "offer carries the sender's participant id",
  hostGotOffer[0]?.from,
);

hostSock.send(
  JSON.stringify({
    type: "WEBRTC_ANSWER",
    roomId: RID,
    to: GUEST_PID,
    data: { type: "answer", sdp: "INITIAL_ANSWER_SDP" },
  }),
);
await wait(300);
ok(
  guestSock.received.some((e) => e.type === "WEBRTC_ANSWER"),
  "guest receives the host's answer",
);

hostSock.send(
  JSON.stringify({
    type: "WEBRTC_ICE_CANDIDATE",
    roomId: RID,
    to: GUEST_PID,
    data: {
      candidate: "candidate:1 1 udp 2130706431 192.168.0.5 54321 typ host",
    },
  }),
);
await wait(250);
ok(
  guestSock.received.some((e) => e.type === "WEBRTC_ICE_CANDIDATE"),
  "ICE candidates relay between peers",
);

const guestOffersBefore = guestSock.received.filter(
  (e) => e.type === "WEBRTC_OFFER",
).length;
hostSock.send(
  JSON.stringify({
    type: "WEBRTC_OFFER",
    roomId: RID,
    to: GUEST_PID,
    data: { type: "offer", sdp: "RENEGOTIATION_OFFER_WITH_VIDEO_TRACK" },
  }),
);
await wait(350);
const guestOffersAfter = guestSock.received.filter(
  (e) => e.type === "WEBRTC_OFFER",
).length;
ok(
  guestOffersAfter === guestOffersBefore + 1,
  "a renegotiation offer (adding the screen track) reaches the other peer",
  `before=${guestOffersBefore} after=${guestOffersAfter}`,
);

const third = await api("POST", `/api/rooms/${RID}/join`, {
  body: { displayName: "Third" },
});
const thirdSock = await open(RID, third.json.token);
await wait(300);
const thirdBefore = thirdSock.received.filter(
  (e) => e.type === "WEBRTC_OFFER",
).length;
hostSock.send(
  JSON.stringify({
    type: "WEBRTC_OFFER",
    roomId: RID,
    to: GUEST_PID,
    data: { type: "offer", sdp: "PRIVATE_TO_GUEST" },
  }),
);
await wait(300);
const thirdAfter = thirdSock.received.filter(
  (e) => e.type === "WEBRTC_OFFER",
).length;
ok(
  thirdAfter === thirdBefore,
  "signaling is point-to-point, not broadcast",
  `third saw ${thirdAfter - thirdBefore} extra`,
);

await api("POST", `/api/rooms/${RID}/end`, { token: HOST_T });
hostSock.close();
guestSock.close();
thirdSock.close();

console.log("=".repeat(70));
for (const [p, n, d] of results)
  console.log(`[${p ? "PASS" : "FAIL"}] ${n}${!p && d ? "  <- " + d : ""}`);
console.log("=".repeat(70));
console.log(`${results.filter(([p]) => p).length}/${results.length} passed`);

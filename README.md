# Together

Turn a group of nearby devices into a temporary shared screen.

Create a room → share the code → present. No account required to join, no
meeting scheduling, no permanent record — a room is a temporary thing that
exists for as long as you need it and then it's gone.

This repo is a working MVP: a Next.js frontend, a Go backend, a self-hosted
LiveKit SFU for online rooms, and a genuine LAN-only transport for rooms
that need to work with zero internet.

## Quick start

You'll need [Node 20+](https://nodejs.org/) and [Docker](https://www.docker.com/).
(Go itself is only needed if you want to build/run the backend outside
Docker — see "Running the backend without Docker" below; the normal path
doesn't need it installed.)

```bash
# 1. Start LiveKit + the backend
./scripts/dev-up.sh

# 2. Start the frontend, in another terminal
cd together-fe
npm install
npm run dev
# -> http://localhost:3000
```

Open two browser tabs at `http://localhost:3000` — create a room in one,
join it with the code in the other. Everything (rooms, roles, presence,
screen share, annotations, reactions) is real and live between the tabs;
nothing here is mocked.

### Testing from a phone or another device on your Wi-Fi

This is the actual point of the product, and the part that broke
repeatedly during development, for two independent reasons — both now
fixed structurally rather than by hand each time:

**1. The backend runs in Docker, not as a bare `go run` binary.** On macOS,
a freshly-compiled Go binary has no track record with the OS's per-app
"Local Network" privacy permission, and can end up silently unreachable
from other devices even with the Application Firewall off — while Docker
Desktop's own network helper is already trusted (it's how the `livekit`
container has always been reachable). `scripts/dev-up.sh` builds and runs
the backend via `docker compose` for exactly this reason. If you'd rather
not use Docker for the backend, see "Running the backend without Docker."

**2. `scripts/dev-up.sh` re-detects and re-syncs your LAN IP every time you
run it**, instead of it going stale. Run it again any time a phone/device
can't connect — most of the time that's simply because the machine's IP
changed (DHCP renewal, switching Wi-Fi networks), and re-running is the
fix. It updates `.env` (which `docker compose` reads automatically) and
`livekit.yaml`'s `rtc.node_ip` (LiveKit's config has no env-var
substitution, so this one file still needs a literal rewrite), then
restarts the containers. It prints the URL to open on your phone when done.

The frontend needs no manual IP configuration at all: `lib/api/client.ts`
derives the API base from whatever host actually served the page
(`window.location.hostname`), and `next.config.ts` populates
`allowedDevOrigins` from every private-range IP on the machine
(`os.networkInterfaces()`) at dev-server startup — without that second
part, Next.js silently blocks HMR/dev-resource requests from a LAN origin,
and the page loads but never becomes interactive (easy to mistake for "the
buttons are broken"). Set `NEXT_PUBLIC_API_BASE` / `NEXT_DEV_ALLOWED_ORIGIN`
only if the backend is genuinely hosted somewhere other than "same machine
as the frontend."

### If LAN testing still won't connect: use a tunnel instead

`scripts/dev-up.sh` fixes IP staleness, but not everything is fixable from
this side — a router isolating your phone from your Mac (common on
mesh/guest Wi-Fi) will keep failing no matter how correct the IP is. If
`dev-up.sh` isn't working, stop debugging the network and run:

```bash
./scripts/tunnel-up.sh
```

This exposes the frontend and backend via Cloudflare quick tunnels (no
account needed) — public HTTPS URLs that work from *any* network,
including cellular data, not just the same Wi-Fi. It works structurally
differently from everything above: it's an **outbound** connection from
your Mac, so none of macOS's Local Network permission, router client
isolation, or LAN IP churn can affect it. It rewrites `frontend/.env.local`
and the backend's CORS allowlist to match the new tunnel URLs each time
(they're random per run — re-run the script after restarting tunnels,
don't hand-edit the URLs anywhere), then tells you the URL to open on your
phone.

**Known limitation**: LiveKit screen-share media (UDP) isn't tunneled by
this script — room creation, joining, roles, annotations, and reactions
all work over the tunnel, but actual screen-share video may not arrive.
That needs LiveKit's TCP-fallback path tunneled too, which is a further
step from here, not yet automated.

### What works on a phone, and what doesn't

Phones are first-class *participants* — joining, watching, reacting,
raising a hand, requesting the stage, and copying the room code all work.

**Presenting from a phone does not work, and can't.** iOS Safari and
Android Chrome don't implement `getDisplayMedia` at all; there is no
screen to capture. The Present control is shown disabled with the reason
next to it rather than failing when tapped (spec §28). Present from a
laptop or desktop.

Two things additionally require a **secure context** (HTTPS, or
localhost), which plain `http://<lan-ip>:3000` is not:

- **Screen sharing** — `navigator.mediaDevices` is undefined entirely
  outside a secure context, so even a laptop can't present over a bare LAN
  IP. Use the tunnel (`./scripts/tunnel-up.sh`) or present from the host
  machine at `localhost`.
- **Copying** — `navigator.clipboard` is likewise unavailable.
  `lib/clipboard.ts` falls back to a `document.execCommand("copy")` path so
  copy still works over plain http, since that's the normal way a phone
  reaches a local room.

## Testing

With the backend running (`./scripts/dev-up.sh`):

```bash
./scripts/test/run.sh
```

Three suites, all against the real server over HTTP/WebSocket — no mocks:

- `scripts/test/api.py` — every REST flow: room lifecycle, join by code,
  roles, presenter transfer, stage requests, invites, LiveKit tokens, plus
  negative cases (a participant can't kick; an ended room can't be joined).
- `scripts/test/realtime.mjs` — the WebSocket layer: join/leave events,
  annotation fan-out, and that a PARTICIPANT's annotation is *rejected*
  while their reaction is allowed.
- `scripts/test/roles.py` — that a role change takes effect immediately for
  the person whose role changed.

Point them at any deployment with `TOGETHER_TEST_BASE=https://... ./scripts/test/run.sh`.

That last suite exists because of a bug worth knowing about if you touch
auth: a participant's role is baked into their session JWT when they join,
so a promotion changes the database but not the token the browser is
holding. It has to be re-read from the database on every request
(`internal/auth/middleware.go`), pushed onto their live WebSocket
(`syncRoomRoles` in `internal/api/server.go`), *and* pushed to LiveKit as a
permission update (`internal/livekit`), because publish rights are likewise
baked into the LiveKit token at connect time. Miss any one of the three and
"make presenter" silently half-works.

### Using LiveKit Cloud (needed for screen share on other devices)

The bundled self-hosted `livekit` container works for same-machine and
same-LAN viewers, but its media is UDP sent straight to this machine's LAN
address. That means it cannot reach a device that can't route to this
machine directly — and over an HTTPS tunnel it fails even earlier, since a
`ws://` LiveKit URL is blocked outright as mixed content on an HTTPS page.
Neither is fixable by configuration; HTTP tunnels don't carry WebRTC media.

For screen share that works from any network, point the backend at
[LiveKit Cloud](https://cloud.livekit.io) (free developer tier). Create a
project, then add its three values to the root `.env`:

```
LIVEKIT_URL=wss://your-project.livekit.cloud
LIVEKIT_API_KEY=APIxxxxxxxxxxxx
LIVEKIT_API_SECRET=your-api-secret
```

Then `docker compose up -d backend` to pick them up. `wss://` means no
mixed-content block, and media relays through LiveKit's TURN servers, so
it no longer matters what this machine is reachable at. `.env` is
gitignored — keep the secret out of version control.

With this set, the local `livekit` container is unused; stop it with
`docker compose stop livekit` if you want it out of the way. Note that
LOCAL-mode rooms don't use LiveKit at all (they're a direct WebRTC mesh
over the LAN), so the offline path still works with no cloud dependency.

### Running the backend without Docker

```bash
cd together-be
go run ./cmd/server --bind=0.0.0.0
```

Same-machine-only (`http://localhost:3000`) testing works fine this way.
For phone/LAN testing this way, you're on your own for macOS's Local
Network permission prompt (System Settings → Privacy & Security → Local
Network) — it's what `dev-up.sh`'s Docker path exists to avoid.

If your machine has multiple active network interfaces (VPN, virtual
adapters, a real Wi-Fi card), confirm which IP your phone would actually
reach with `ipconfig getifaddr en0` (swap `en0` for your Wi-Fi interface —
`networksetup -listallhardwareports` lists them) or just check what
`npm run dev` prints under `Network:`; that's the one Next itself will use.

**One value can't self-detect**: `livekit.yaml`'s `rtc.node_ip` is read by
the LiveKit *container*, not app code, so it stays a manual field — update
it to your current LAN IP and `docker compose restart livekit` whenever
your network changes. It only matters if a second physical device needs to
actually publish/subscribe media through an **online** room (viewing/
presenting), not for loading the page or testing a **local**-mode room.

Restart `go run` and `npm run dev` after changing any bind/env settings —
none of it is picked up live.

## Architecture

```
Together/
  frontend/        Next.js (App Router) + TypeScript + Tailwind + shadcn/ui
  server/          Go backend (REST + WebSocket)
  docker-compose.yml   self-hosted LiveKit (open-source livekit-server)
  livekit.yaml         LiveKit dev config
```

**Media never touches the Go backend.** For online rooms, the presenter
publishes one screen track to a self-hosted LiveKit SFU; LiveKit forwards it
to every viewer. The Go server's only involvement with media is minting a
scoped LiveKit access token per participant (`internal/livekit/token.go`).
Everything else — room lifecycle, auth, permissions, invites, presenter
handoff, annotations, reactions — is the Go backend's job, delivered over a
REST API plus one WebSocket per room for realtime events.

```
Presenter → WebRTC → LiveKit SFU → Participants     (online rooms)
Presenter → WebRTC (direct, mesh) → Participants     (local rooms)
                    ↕
         Go backend (REST + WS): auth, rooms, roles,
         permissions, invites, presenter handoff,
         annotations/reactions relay
```

### Transport abstraction

The frontend never talks to LiveKit or raw WebRTC directly from UI code —
everything goes through a `RoomTransport` interface
(`frontend/lib/room/transport.ts`):

```ts
interface RoomTransport {
  connect(roomId: string, token: string): Promise<void>;
  disconnect(): Promise<void>;
  publishScreenShare(stream: MediaStream): Promise<void>;
  unpublishScreenShare(): Promise<void>;
  onRemoteTrack(cb): () => void;
  onTrackEnded(cb): () => void;
  sendEvent(event): void;
  onEvent(cb): () => void;
}
```

Two implementations:

- **`OnlineTransport`** (`lib/transport/online/`) — `livekit-client` for
  media, plus the app's own WebSocket for room/annotation/reaction data.
- **`LocalTransport`** (`lib/transport/local/`) — the same WebSocket, reused
  both for data *and* as a signaling relay for a direct browser-to-browser
  WebRTC mesh (`lib/webrtc/mesh.ts`). No SFU, no cloud dependency.

`hooks/useRoomTransport.ts` is the one place that picks which
implementation to use, based on the room's own `mode` — every other hook
and component is written against the interface only.

### Local mode, end to end

This is the part worth spelling out honestly, because "works offline" is
easy to fake and we didn't want to do that.

1. Host picks **Local** when creating a room. The Go server (started with
   `--mode=local`, which binds it to `0.0.0.0` instead of `127.0.0.1`)
   detects its own LAN IP (`internal/discovery/lan.go`) and stores it on
   the room (`host_lan_addr`).
2. The invite QR code / link **literally encodes that LAN IP:port** —
   `http://192.168.1.42:8080/join?code=ABC123`. This is the actual
   mechanism, not a detail we hide: a browser cannot discover a peer on the
   network with zero configuration, so the code *is* the discovery step.
   The UI says so ("must be on the same Wi-Fi").
3. For a guest's phone to load the app at all without internet, the Go
   server also serves the frontend itself as static files
   (`TOGETHER_STATIC_DIR`, see below) — there's no separate Node process to
   reach.
4. Once joined, `LocalTransport` opens one WebSocket to that same LAN
   server. Screen sharing negotiates a direct `RTCPeerConnection` per
   participant, with SDP/ICE relayed through that WebSocket
   (`internal/ws/` on the backend, `lib/webrtc/mesh.ts` on the frontend).
   Annotations and reactions use the exact same event protocol as online
   rooms, just carried over this socket instead of a second one.

Running it for real:

```bash
cd together-fe
npm run build:local        # static export → frontend/out

cd ../server
go run ./cmd/server --mode=local --static=../frontend/out
# -> prints the LAN address it's bound to; open that from a second device
#    on the same Wi-Fi (or scan the room's QR code)
```

One disclosed exception: local rooms use a public STUN server by default
for ICE candidate gathering (most same-subnet LANs don't even need it, but
it's there as a robustness fallback). Pass `iceServers: []` in
`lib/webrtc/mesh.ts`'s `DEFAULT_ICE_SERVERS` to verify a strictly-offline
demo with that removed.

### Data model

SQLite (`server/internal/db/migrations/0001_init.sql`), plain
`database/sql` with hand-written queries — no ORM. `users`, `rooms`,
`room_participants`, `room_invites`, `room_sessions`, `room_permissions`,
`room_actions`. IDs and timestamps are TEXT (UUID/RFC3339) so a future
Postgres migration is a type swap, not a rewrite.

### Security model

- Room IDs are 20-character cryptographically random strings — the real
  access boundary. The 6-character join code is convenience only, never
  security (spec's own framing).
- Every room action is authorized server-side against a role→action matrix
  (`internal/rooms/permissions.go`), consulted identically by REST handlers
  and the WebSocket hub — the client's claimed role is never trusted.
- Room-scoped JWTs are separate from account JWTs: identity ("who you are")
  never gates room access ("what you can do in this room") by itself. A
  kick or room-end revokes the specific session server-side
  (`room_sessions.token_jti`).

### Realtime events

One event envelope, one WebSocket per room, used identically for both
transports: `PARTICIPANT_JOINED/LEFT/UPDATED`, `PRESENTER_ASSIGNED
/TRANSFERRED/REQUESTED`, `SCREEN_STARTED/STOPPED`, `ANNOTATION_CREATED
/UPDATED/DELETED`, `REACTION_SENT`, `ROOM_PAUSED/RESUMED/ENDED`,
`ACTION_CREATED/RESOLVED`, plus `WEBRTC_OFFER/ANSWER/ICE_CANDIDATE` for
local-mode signaling relay. See `server/internal/ws/events.go`.

### Annotations

Synchronized vector data, never baked into the video stream. A stroke is
broadcast whole on completion (not streamed point-by-point — an intentional
MVP simplification), normalized to 0–1 relative to the actual video content
box (not the container — the letterboxing math is in
`lib/annotations/geometry.ts`), and rendered as SVG on a layer above the
`<video>` element. They aren't persisted — a room is temporary, and so are
its annotations.

## Deploying

Free-tier stack: **Vercel** (frontend) + **Render** (backend) + **Turso**
(database) + **LiveKit Cloud** (media). All four have free plans that fit
this app.

### 1. Database — Turso

Render's free disk is ephemeral, so a local SQLite file would be erased on
every restart. Turso is SQLite-compatible, so nothing in the schema or the
queries changes — only the connection string.

```bash
brew install tursodatabase/tap/turso
turso auth signup
turso db create together
turso db show together --url        # libsql://together-<org>.turso.io
turso db tokens create together     # the auth token
```

Combine them: `libsql://together-<org>.turso.io?authToken=<token>`. That
single string is `DATABASE_URL`. Migrations run automatically on boot, so
there is no separate migration step.

### 2. Backend — Render

Push this repo to GitHub, then in Render: **New + → Blueprint**, point it at
the repo. `render.yaml` describes the service; Render will prompt for the
secrets it declares (`DATABASE_URL`, `JWT_SECRET`, `LIVEKIT_*`,
`CORS_ORIGIN`). Generate the JWT secret with `openssl rand -base64 48`.

Set `CORS_ORIGIN` to your Vercel URL once you have it (step 3) — the two
steps reference each other, so expect to come back and fill this in.

**Know what the free plan does**: the service sleeps after ~15 minutes of
inactivity, and a sleeping service drops every WebSocket, which ends every
live room. The first request afterwards takes ~30s to wake it. Your *data*
survives (that's Turso's job), but a room in progress does not. Fine for
demos; upgrade to a paid instance before anyone depends on it.

### 3. Frontend — Vercel

Import the `together-fe` repo at [vercel.com/new](https://vercel.com/new).
Next.js is detected automatically. Set one environment variable:

```
NEXT_PUBLIC_API_BASE=https://together-backend.onrender.com
```

Redeploy after changing it — `NEXT_PUBLIC_*` values are baked in at build
time, not read at runtime.

### 4. Media — LiveKit Cloud

Create a project at [cloud.livekit.io](https://cloud.livekit.io) and put its
`wss://` URL, API key and secret into Render's environment. Without this,
online rooms fall back to the self-hosted container from
`docker-compose.yml`, which is not reachable from a deployed frontend.

### Local rooms on a deployed server

Invite links and QR codes point at the **page's own origin** (your Vercel
URL) unless the server is explicitly told it sits on the guests' network.

That's what `TOGETHER_MODE=local` means. Leave it unset in production.
With it set, the server also needs `TOGETHER_LAN_ADDR=<host-ip>:<port>`
when running in a container — a container can only see its own `172.x`
address and has no way to discover the host's. Getting this wrong is how
QR codes end up encoding an unreachable internal IP.

`docker compose` sets both for you (see `docker-compose.yml`), because
that path is by definition running on your own machine.

Note the honesty limit: a local room whose signaling goes through a
deployed server still needs internet to *join* — only the media stays
peer-to-peer on the LAN. Zero-internet rooms require running the server
on the local network yourself.

### What deploying does not fix

**Local-mode rooms still require both devices on the same network.** They
use a direct browser-to-browser WebRTC mesh; hosting the signaling server
in the cloud doesn't create a network path between two devices that can't
reach each other. Deployment only helps online rooms.

## Environment variables

See `server/.env.example` and `frontend/.env.example`. Every backend
variable has a working local default (`internal/config/config.go`) — you
don't need a `.env` file to run this locally.

## What's genuinely real vs. what's deliberately deferred

Real: room creation/join/lifecycle, role-based permissions (server
re-validated on every action), live participant sync, online screen share
through a real self-hosted LiveKit SFU, presenter handoff and
request-to-present approval, annotations, reactions, local-mode LAN
discovery + WebRTC mesh + static-asset serving, optional accounts with room
history.

Deferred, and disclosed rather than faked:

- **File sharing** — in the product's action vocabulary, has a button, the
  backend endpoint exists and returns `501 Not Implemented` on purpose. No
  storage or transfer pipeline was built.
- **Large-room tuning, TURN server** — no load testing, no TURN relay for
  NAT-hostile networks. LiveKit's defaults are used as-is.
- **Multi-presenter layout** — the data model and permissions genuinely
  support multiple simultaneous `PRESENTER`s (`room_participants.role`
  isn't unique, `is_primary` tracks the spotlighted one), and a host can
  promote a second presenter from the People panel. The viewer UI still
  only renders one active video at a time, though — no side-by-side tile
  layout.
- **Zero-config LAN discovery** — not possible from a browser; the
  QR/code encoding the host's IP:port is the real, disclosed mechanism.
- **Annotation history** — not persisted; a late joiner doesn't see past
  strokes.
- **Postgres** — schema is portable (TEXT ids/timestamps) but the cutover
  itself wasn't done.

## Design system

Monochrome, calm, generous whitespace, no gradients or glassmorphism —
Geist for UI text, a restrained italic serif (Fraunces) for the one marketing
headline on the landing page. Built on shadcn/ui (Base UI primitives) with
a small set of composed components under `frontend/components/room`,
`components/presentation`, `components/participants`, `components/invite`,
and `components/annotations`.

## Project structure

```
frontend/
  app/                landing, create/join flows, room/[roomId], auth, account
  components/
    ui/                shadcn/ui primitives
    room/               RoomHeader, RoomCode, RoomQRCode, RoomProvider, dialogs
    presentation/       PresentationStage, Presenter/ViewerControls, reactions
    participants/       ParticipantList, ParticipantsPanel
    invite/             RoomInvite, InvitePanel
    annotations/        AnnotationToolbar, AnnotationsOverlay
  lib/
    api/                typed REST client functions
    auth/               room-scoped + account token storage
    room/               RoomTransport interface, WS socket wrapper, join links
    transport/online/    LiveKit-backed implementation
    transport/local/     LAN WebRTC-mesh implementation
    webrtc/              the mesh manager itself
    annotations/         types + letterboxing geometry
  hooks/                useRoom, useParticipants, usePresenter, usePresentation,
                         useAnnotations, useReactions, useRoomTransport, ...

server/
  cmd/server/           entrypoint
  internal/
    config/              env/flag loading
    db/                  SQLite store + embedded migrations
    models/              plain data structs
    auth/                JWT mint/verify, bcrypt, middleware
    rooms/                business logic + the permission matrix
    livekit/              LiveKit token minting
    ws/                   per-room WebSocket hub
    discovery/             LAN IP detection for local-mode invites
    api/                  HTTP handlers + router
```

## Development mode

`?demo=true` support and multi-tab testing: since guest join requires no
account, the simplest way to simulate multiple participants is just
opening multiple browser tabs/profiles and joining the same room code —
no separate demo flag was needed to make that work, and no media is ever
faked in doing so.

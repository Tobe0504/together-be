# Multi-stage build. modernc.org/sqlite is pure Go (no CGO), so this stays a
# plain, fully static binary — no gcc/musl-dev toolchain needed either stage.
FROM golang:1-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/together-server ./cmd/server

FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/together-server .
EXPOSE 8080
# Always 0.0.0.0 inside the container — Docker's own network stack is what
# gets exposed to the LAN via the port mapping in docker-compose.yml, so
# there's no "loopback-only" footgun the way there was running the binary
# directly on the host.
ENTRYPOINT ["./together-server", "--bind=0.0.0.0"]

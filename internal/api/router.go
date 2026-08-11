package api

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/tobenna/together/server/internal/auth"
)

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.RequestID, chimw.RealIP, chimw.Recoverer)
	r.Use(chimw.Logger)
	r.Use(cors.Handler(cors.Options{
		AllowOriginFunc:  s.isAllowedOrigin,
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/api", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", s.handleRegister)
			r.Post("/login", s.handleLogin)
			r.With(auth.RequireAccountAuth(s.Signer)).Get("/me", s.handleMe)
		})

		r.Route("/rooms", func(r chi.Router) {
			r.With(auth.OptionalAccountAuth(s.Signer)).Post("/", s.handleCreateRoom)
			r.Get("/by-code/{code}", s.handleGetRoomByCode)
			r.With(auth.OptionalAccountAuth(s.Signer)).Post("/{roomId}/join", s.handleJoinRoom)

			r.Group(func(r chi.Router) {
				r.Use(auth.RequireRoomAuth(s.Signer, s.Store))
				r.Use(requireMatchingRoom)

				r.Get("/{roomId}", s.handleGetRoom)
				r.Patch("/{roomId}", s.handlePatchRoom)
				r.Post("/{roomId}/end", s.handleEndRoom)
				r.Post("/{roomId}/pause", s.handlePauseRoom)
				r.Post("/{roomId}/resume", s.handleResumeRoom)
				r.Post("/{roomId}/screen/start", s.handleScreenStarted)
				r.Post("/{roomId}/screen/stop", s.handleScreenStopped)

				r.Get("/{roomId}/participants", s.handleListParticipants)
				r.Post("/{roomId}/leave", s.handleLeaveRoom)
				r.Delete("/{roomId}/participants/{participantId}", s.handleKickParticipant)
				r.Patch("/{roomId}/participants/{participantId}/role", s.handlePatchParticipantRole)

				r.Post("/{roomId}/presenter/transfer", s.handleTransferPresenter)
				r.Post("/{roomId}/actions", s.handleRequestAction)
				r.Get("/{roomId}/actions", s.handleListPendingActions)
				r.Patch("/{roomId}/actions/{actionId}", s.handleResolveAction)

				r.Post("/{roomId}/invites", s.handleCreateInvite)
				r.Post("/{roomId}/files", s.handleShareFile)
			})
		})

		r.With(auth.RequireRoomAuth(s.Signer, s.Store)).Post("/livekit/token", s.handleLiveKitToken)
	})

	r.With(auth.RequireRoomAuth(s.Signer, s.Store)).Get("/ws/rooms/{roomId}", s.handleRoomWebSocket)

	if s.Cfg.StaticDir != "" {
		if info, err := os.Stat(s.Cfg.StaticDir); err == nil && info.IsDir() {
			fileServer := http.FileServer(http.Dir(s.Cfg.StaticDir))
			serveFile := func(relPath string) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					http.ServeFile(w, r, filepath.Join(s.Cfg.StaticDir, relPath))
				}
			}

			r.Get("/", serveFile("index.html"))
			r.Get("/join", serveFile("join/index.html"))
			r.Get("/room/{roomId}", serveFile("room/_/index.html"))

			r.NotFound(func(w http.ResponseWriter, r *http.Request) {
				fileServer.ServeHTTP(w, r)
			})
		}
	}

	return r
}

func (s *Server) isAllowedOrigin(_ *http.Request, origin string) bool {
	if slices.Contains(s.Cfg.CORSOrigins, origin) {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := u.Hostname()
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}
	return strings.HasPrefix(host, "192.168.") ||
		strings.HasPrefix(host, "10.") ||
		isPrivate172(host)
}

func isPrivate172(host string) bool {
	if !strings.HasPrefix(host, "172.") {
		return false
	}
	parts := strings.SplitN(host, ".", 3)
	if len(parts) < 2 {
		return false
	}
	second := parts[1]
	return len(second) >= 2 && second >= "16" && second <= "31"
}

func requireMatchingRoom(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := auth.RoomClaimsFromContext(r.Context())
		if claims == nil || claims.RoomID != chiRoomID(r) {
			writeError(w, http.StatusForbidden, "You don't have permission to do that.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

package livekit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	auth "github.com/livekit/protocol/auth"
	"github.com/tobenna/together/server/internal/models"
)

var ErrParticipantNotConnected = errors.New("participant not connected to livekit")

type TokenIssuer struct {
	apiKey    string
	apiSecret string
	url       string
}

func NewTokenIssuer(apiKey, apiSecret, url string) *TokenIssuer {
	return &TokenIssuer{apiKey: apiKey, apiSecret: apiSecret, url: url}
}

func (t *TokenIssuer) URL() string { return t.url }

func CanPublish(role models.ParticipantRole) bool {
	return role == models.RoleOwner || role == models.RoleHost || role == models.RolePresenter
}

func (t *TokenIssuer) Mint(roomID, participantID, displayName string, role models.ParticipantRole) (string, error) {
	canPublish := CanPublish(role)
	canPublishData := true
	canSubscribe := true

	grant := &auth.VideoGrant{
		RoomJoin:       true,
		Room:           roomID,
		CanPublish:     &canPublish,
		CanSubscribe:   &canSubscribe,
		CanPublishData: &canPublishData,
	}

	at := auth.NewAccessToken(t.apiKey, t.apiSecret).
		SetVideoGrant(grant).
		SetIdentity(participantID).
		SetName(displayName).
		SetValidFor(24 * time.Hour)

	return at.ToJWT()
}

func (t *TokenIssuer) httpBase() string {
	if rest, ok := strings.CutPrefix(t.url, "wss://"); ok {
		return "https://" + rest
	}
	if rest, ok := strings.CutPrefix(t.url, "ws://"); ok {
		return "http://" + rest
	}
	return t.url
}


func (t *TokenIssuer) UpdatePermissions(ctx context.Context, roomID, participantID string, role models.ParticipantRole) error {
	adminToken, err := auth.NewAccessToken(t.apiKey, t.apiSecret).
		SetVideoGrant(&auth.VideoGrant{RoomAdmin: true, Room: roomID}).
		SetValidFor(time.Minute).
		ToJWT()
	if err != nil {
		return fmt.Errorf("mint admin token: %w", err)
	}

	canPublish := CanPublish(role)
	payload, err := json.Marshal(map[string]any{
		"room":     roomID,
		"identity": participantID,
		"permission": map[string]any{
			"canSubscribe":   true,
			"canPublish":     canPublish,
			"canPublishData": true,
		},
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		t.httpBase()+"/twirp/livekit.RoomService/UpdateParticipant",
		bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return ErrParticipantNotConnected
	}
	if resp.StatusCode != http.StatusOK {
		var body bytes.Buffer
		_, _ = body.ReadFrom(resp.Body)
		return fmt.Errorf("livekit update participant: %s: %s", resp.Status, strings.TrimSpace(body.String()))
	}
	return nil
}

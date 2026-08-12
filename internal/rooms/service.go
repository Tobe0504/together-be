package rooms

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/tobenna/together/server/internal/auth"
	"github.com/tobenna/together/server/internal/db"
	"github.com/tobenna/together/server/internal/models"
)

var (
	ErrForbidden         = errors.New("forbidden")
	ErrNotFound          = db.ErrNotFound
	ErrRoomEnded         = errors.New("room has ended")
	ErrRoomExpired       = errors.New("room has expired")
	ErrBadAccessCode     = errors.New("incorrect access code")
	ErrCodeCollision     = errors.New("could not allocate a unique code")
	ErrRoomFull          = errors.New("room is full")
	ErrAlreadyPresenting = errors.New("someone else is already presenting")
)

const (
	roomTTL              = 24 * time.Hour
	sessionTTL           = 24 * time.Hour
	maxCodeTry           = 8
	maxLocalParticipants = 12
)

func MaxParticipantsFor(mode models.RoomMode) int {
	if mode == models.RoomModeLocal {
		return maxLocalParticipants
	}
	return 0
}

func withLimits(room *models.Room) *models.Room {
	if room != nil {
		room.MaxParticipants = MaxParticipantsFor(room.Mode)
	}
	return room
}

type Service struct {
	Store  *db.Store
	Signer *auth.Signer
}

func NewService(store *db.Store, signer *auth.Signer) *Service {
	return &Service{Store: store, Signer: signer}
}

type JoinResult struct {
	Room        *models.Room
	Participant *models.RoomParticipant
	Token       string
}

type CreateRoomInput struct {
	Name            string
	Mode            models.RoomMode
	AccessProtected bool
	PIN             string
	OwnerUserID     *string
	DisplayName     string
}

func (s *Service) CreateRoom(ctx context.Context, in CreateRoomInput) (*JoinResult, error) {
	now := time.Now().UTC()

	var pinHash *string
	if in.AccessProtected && in.PIN != "" {
		h, err := auth.HashPIN(in.PIN)
		if err != nil {
			return nil, err
		}
		pinHash = &h
	}

	var room *models.Room
	for i := 0; i < maxCodeTry; i++ {
		id, err := NewRoomID()
		if err != nil {
			return nil, err
		}
		code, err := NewJoinCode()
		if err != nil {
			return nil, err
		}
		r := &models.Room{
			ID: id, Name: in.Name, Mode: in.Mode, OwnerID: in.OwnerUserID,
			Status: models.RoomStatusWaiting, JoinCode: code,
			AccessProtected: in.AccessProtected, PinHash: pinHash,
			ExpiresAt: now.Add(roomTTL), CreatedAt: now, UpdatedAt: now,
		}
		if err := s.Store.CreateRoom(ctx, r); err != nil {
			if db.IsUniqueViolation(err) {
				continue
			}

			return nil, err
		}
		room = r
		break
	}
	if room == nil {
		return nil, ErrCodeCollision
	}

	participant := &models.RoomParticipant{
		ID: uuid.NewString(), RoomID: room.ID, UserID: in.OwnerUserID,
		DisplayName: in.DisplayName, Role: models.RoleOwner,
		Status: models.ParticipantConnected, JoinedAt: now, LastSeenAt: now,
	}
	if err := s.Store.CreateParticipant(ctx, participant); err != nil {
		return nil, err
	}

	token, err := s.mintSession(ctx, room, participant)
	if err != nil {
		return nil, err
	}
	return &JoinResult{Room: withLimits(room), Participant: participant, Token: token}, nil
}

func (s *Service) LookupByCode(ctx context.Context, code string) (*models.Room, error) {
	room, err := s.Store.GetRoomByJoinCode(ctx, code)
	if err != nil {
		return nil, err
	}
	return withLimits(room), nil
}

func (s *Service) GetRoom(ctx context.Context, roomID string) (*models.Room, error) {
	room, err := s.Store.GetRoomByID(ctx, roomID)
	if err != nil {
		return nil, err
	}
	return withLimits(room), nil
}

type JoinRoomInput struct {
	RoomID      string
	DisplayName string
	PIN         string
	UserID      *string
}

func (s *Service) JoinRoom(ctx context.Context, in JoinRoomInput) (*JoinResult, error) {
	room, err := s.Store.GetRoomByID(ctx, in.RoomID)
	if err != nil {
		return nil, err
	}
	if room.Status == models.RoomStatusEnded {
		return nil, ErrRoomEnded
	}
	if time.Now().UTC().After(room.ExpiresAt) {
		return nil, ErrRoomExpired
	}
	if room.AccessProtected {
		if room.PinHash == nil || !auth.VerifyPIN(*room.PinHash, in.PIN) {
			return nil, ErrBadAccessCode
		}
	}
	if room.Mode == models.RoomModeLocal {
		existing, err := s.Store.ListParticipants(ctx, room.ID)
		if err != nil {
			return nil, err
		}
		connected := 0
		for _, p := range existing {
			if p.Status == models.ParticipantConnected {
				connected++
			}
		}
		if connected >= maxLocalParticipants {
			return nil, ErrRoomFull
		}
	}

	now := time.Now().UTC()
	participant := &models.RoomParticipant{
		ID: uuid.NewString(), RoomID: room.ID, UserID: in.UserID,
		DisplayName: in.DisplayName, Role: DefaultRoleForNewParticipant(),
		Status: models.ParticipantConnected, JoinedAt: now, LastSeenAt: now,
	}
	if err := s.Store.CreateParticipant(ctx, participant); err != nil {
		return nil, err
	}

	token, err := s.mintSession(ctx, room, participant)
	if err != nil {
		return nil, err
	}
	return &JoinResult{Room: withLimits(room), Participant: participant, Token: token}, nil
}

func (s *Service) mintSession(ctx context.Context, room *models.Room, p *models.RoomParticipant) (string, error) {
	now := time.Now().UTC()
	jti := uuid.NewString()
	expiresAt := now.Add(sessionTTL)
	if room.ExpiresAt.Before(expiresAt) {
		expiresAt = room.ExpiresAt
	}

	if err := s.Store.CreateSession(ctx, &models.RoomSession{
		ID: uuid.NewString(), RoomID: room.ID, ParticipantID: p.ID,
		TokenJTI: jti, IssuedAt: now, ExpiresAt: expiresAt,
	}); err != nil {
		return "", err
	}

	return s.Signer.MintRoomToken(p.ID, room.ID, p.Role, p.UserID, jti, expiresAt)
}

func (s *Service) LeaveRoom(ctx context.Context, participantID string) error {
	// Free the stage first: someone who closes their tab mid-share would
	// otherwise hold the presenter slot forever, permanently blocking
	// everyone else in a room that looks idle.
	if err := s.Store.ReleasePresenterForParticipant(ctx, participantID); err != nil {
		return err
	}
	return s.Store.UpdateParticipantStatus(ctx, participantID, models.ParticipantDisconnected)
}

// ListParticipants returns only people currently in the room.
//
// Filtered here rather than left to the client because this is what "who is
// in this room" means to every caller. Disconnected and kicked rows are
// kept in the database as history, but a client rendering them produces
// ghosts — someone who refreshed appearing twice, or a person who left
// hours ago still listed. That's belt-and-braces alongside the disconnect
// handler actually working: if a close is ever missed (a killed process, a
// dropped network with no close frame), a stale row still can't surface as
// a phantom participant.
func (s *Service) ListParticipants(ctx context.Context, roomID string) ([]models.RoomParticipant, error) {
	all, err := s.Store.ListParticipants(ctx, roomID)
	if err != nil {
		return nil, err
	}
	live := make([]models.RoomParticipant, 0, len(all))
	for _, p := range all {
		if p.Status == models.ParticipantConnected {
			live = append(live, p)
		}
	}
	return live, nil
}

func (s *Service) EndRoom(ctx context.Context, roomID string, actorRole models.ParticipantRole) error {
	if !Can(actorRole, ActionEndRoom) {
		return ErrForbidden
	}
	if err := s.Store.UpdateRoomStatus(ctx, roomID, models.RoomStatusEnded); err != nil {
		return err
	}
	return s.Store.RevokeSessionsForRoom(ctx, roomID)
}

func (s *Service) SetPaused(ctx context.Context, roomID string, actorRole models.ParticipantRole, paused bool) error {
	if !Can(actorRole, ActionPauseResume) {
		return ErrForbidden
	}
	status := models.RoomStatusWaiting
	if paused {
		status = models.RoomStatusPaused
	}
	return s.Store.UpdateRoomStatus(ctx, roomID, status)
}

// StartPresenting claims the room's single presenter slot.
//
// One live screen at a time is a deliberate product constraint, not a
// technical one: the stage shows a single source, so a second simultaneous
// share would simply replace the first in everyone's view with no
// indication of why. Enforced here rather than in the UI because the UI
// can only ever be a hint — two clients can race, and a client can lie.
func (s *Service) StartPresenting(ctx context.Context, roomID, participantID string, actorRole models.ParticipantRole) error {
	if !Can(actorRole, ActionStartPresent) {
		return ErrForbidden
	}
	claimed, err := s.Store.ClaimPresenter(ctx, roomID, participantID)
	if err != nil {
		return err
	}
	if !claimed {
		return ErrAlreadyPresenting
	}
	return s.Store.UpdateRoomStatus(ctx, roomID, models.RoomStatusPresenting)
}

// StopPresenting releases the slot. No permission check: anyone able to
// claim it can give it up, and a participant who has lost presenter rights
// mid-share still needs to be able to stop.
func (s *Service) StopPresenting(ctx context.Context, roomID, participantID string) error {
	if err := s.Store.ReleasePresenter(ctx, roomID, participantID); err != nil {
		return err
	}
	return s.Store.UpdateRoomStatus(ctx, roomID, models.RoomStatusWaiting)
}

func (s *Service) KickParticipant(ctx context.Context, actorRole models.ParticipantRole, targetParticipantID string) error {
	if !Can(actorRole, ActionKick) {
		return ErrForbidden
	}
	if err := s.Store.ReleasePresenterForParticipant(ctx, targetParticipantID); err != nil {
		return err
	}
	if err := s.Store.UpdateParticipantStatus(ctx, targetParticipantID, models.ParticipantKicked); err != nil {
		return err
	}
	return s.Store.RevokeSessionsForParticipant(ctx, targetParticipantID)
}

func (s *Service) ChangeRole(ctx context.Context, actorRole models.ParticipantRole, targetParticipantID string, newRole models.ParticipantRole) error {
	if !Can(actorRole, ActionPromoteDemote) {
		return ErrForbidden
	}
	return s.Store.UpdateParticipantRole(ctx, targetParticipantID, newRole)
}

func (s *Service) TransferPresenter(ctx context.Context, roomID string, actorRole models.ParticipantRole, fromParticipantID, toParticipantID string) error {
	if !Can(actorRole, ActionTransferPresent) {
		return ErrForbidden
	}
	target, err := s.Store.GetParticipantByID(ctx, toParticipantID)
	if err != nil {
		return err
	}
	if target.Role != models.RoleOwner && target.Role != models.RoleHost {
		if err := s.Store.UpdateParticipantRole(ctx, toParticipantID, models.RolePresenter); err != nil {
			return err
		}
	}
	if fromParticipantID != "" && fromParticipantID != toParticipantID {
		if from, err := s.Store.GetParticipantByID(ctx, fromParticipantID); err == nil {
			if from.Role == models.RolePresenter {
				_ = s.Store.UpdateParticipantRole(ctx, fromParticipantID, models.RoleParticipant)
			}
		}
	}
	if err := s.Store.ClearOtherPrimaries(ctx, roomID, toParticipantID); err != nil {
		return err
	}
	return s.Store.SetParticipantPrimary(ctx, roomID, toParticipantID, true)
}

func (s *Service) RequestAction(ctx context.Context, roomID, participantID string, actionType models.RoomActionType) (*models.RoomAction, error) {
	a := &models.RoomAction{
		ID: uuid.NewString(), RoomID: roomID, ParticipantID: participantID,
		ActionType: actionType, Status: models.ActionPending, CreatedAt: time.Now().UTC(),
	}
	if err := s.Store.CreateAction(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

func (s *Service) ResolveAction(ctx context.Context, actorRole models.ParticipantRole, actionID string, accept bool) (*models.RoomAction, error) {
	if !Can(actorRole, ActionResolveAction) {
		return nil, ErrForbidden
	}
	status := models.ActionDeclined
	if accept {
		status = models.ActionAccepted
	}
	if err := s.Store.ResolveAction(ctx, actionID, status); err != nil {
		return nil, err
	}
	return s.Store.GetActionByID(ctx, actionID)
}

func (s *Service) CreateInvite(ctx context.Context, roomID string, actorRole models.ParticipantRole, createdBy *string) (*models.RoomInvite, error) {
	if !Can(actorRole, ActionInvite) {
		return nil, ErrForbidden
	}
	room, err := s.Store.GetRoomByID(ctx, roomID)
	if err != nil {
		return nil, err
	}
	inv := &models.RoomInvite{
		ID: uuid.NewString(), RoomID: roomID, CreatedBy: createdBy,
		JoinCode: room.JoinCode, ExpiresAt: room.ExpiresAt, CreatedAt: time.Now().UTC(),
	}
	if err := s.Store.CreateInvite(ctx, inv); err != nil {
		return nil, err
	}
	return inv, nil
}

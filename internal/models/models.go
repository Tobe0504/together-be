package models

import "time"

type RoomMode string

const (
	RoomModeOnline RoomMode = "online"
	RoomModeLocal  RoomMode = "local"
)

type RoomStatus string

const (
	RoomStatusWaiting    RoomStatus = "WAITING"
	RoomStatusPresenting RoomStatus = "PRESENTING"
	RoomStatusPaused     RoomStatus = "PAUSED"
	RoomStatusEnded      RoomStatus = "ENDED"
)

type ParticipantRole string

const (
	RoleOwner       ParticipantRole = "OWNER"
	RoleHost        ParticipantRole = "HOST"
	RolePresenter   ParticipantRole = "PRESENTER"
	RoleParticipant ParticipantRole = "PARTICIPANT"
	RoleViewer      ParticipantRole = "VIEWER"
)

type ParticipantStatus string

const (
	ParticipantConnected    ParticipantStatus = "CONNECTED"
	ParticipantDisconnected ParticipantStatus = "DISCONNECTED"
	ParticipantKicked       ParticipantStatus = "KICKED"
)

type RoomActionType string

const (
	ActionRaiseHand        RoomActionType = "RAISE_HAND"
	ActionRequestStage     RoomActionType = "REQUEST_STAGE"
	ActionRequestPresenter RoomActionType = "REQUEST_PRESENTER"
)

type RoomActionStatus string

const (
	ActionPending   RoomActionStatus = "PENDING"
	ActionAccepted  RoomActionStatus = "ACCEPTED"
	ActionDeclined  RoomActionStatus = "DECLINED"
	ActionCancelled RoomActionStatus = "CANCELLED"
)

type User struct {
	ID           string    `json:"id"`
	Email        *string   `json:"email,omitempty"`
	PasswordHash *string   `json:"-"`
	DisplayName  string    `json:"displayName"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type Room struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	Mode             RoomMode   `json:"mode"`
	OwnerID          *string    `json:"ownerId,omitempty"`
	Status           RoomStatus `json:"status"`
	JoinCode         string     `json:"joinCode"`
	AccessProtected  bool       `json:"accessProtected"`
	PinHash          *string    `json:"-"`
	HostLANAddr      *string    `json:"hostLanAddr,omitempty"`
	PrimaryPresentID *string    `json:"primaryPresenterId,omitempty"`
	ExpiresAt        time.Time  `json:"expiresAt"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	MaxParticipants int `json:"maxParticipants"`
}

type RoomParticipant struct {
	ID          string            `json:"id"`
	RoomID      string            `json:"roomId"`
	UserID      *string           `json:"userId,omitempty"`
	DisplayName string            `json:"displayName"`
	Role        ParticipantRole   `json:"role"`
	Status      ParticipantStatus `json:"status"`
	IsPrimary   bool              `json:"isPrimary"`
	JoinedAt    time.Time         `json:"joinedAt"`
	LastSeenAt  time.Time         `json:"lastSeenAt"`
}

type RoomInvite struct {
	ID        string    `json:"id"`
	RoomID    string    `json:"roomId"`
	CreatedBy *string   `json:"createdBy,omitempty"`
	JoinCode  string    `json:"joinCode"`
	ExpiresAt time.Time `json:"expiresAt"`
	Revoked   bool      `json:"revoked"`
	CreatedAt time.Time `json:"createdAt"`
}

type RoomSession struct {
	ID            string    `json:"id"`
	RoomID        string    `json:"roomId"`
	ParticipantID string    `json:"participantId"`
	TokenJTI      string    `json:"-"`
	IssuedAt      time.Time `json:"issuedAt"`
	ExpiresAt     time.Time `json:"expiresAt"`
	Revoked       bool      `json:"-"`
}

type RoomAction struct {
	ID            string           `json:"id"`
	RoomID        string           `json:"roomId"`
	ParticipantID string           `json:"participantId"`
	ActionType    RoomActionType   `json:"actionType"`
	Status        RoomActionStatus `json:"status"`
	CreatedAt     time.Time        `json:"createdAt"`
	ResolvedAt    *time.Time       `json:"resolvedAt,omitempty"`
}

package ws

import (
	"encoding/json"
	"time"
)

type EventType string

const (
	EventRoomCreated        EventType = "ROOM_CREATED"
	EventParticipantJoined  EventType = "PARTICIPANT_JOINED"
	EventParticipantLeft    EventType = "PARTICIPANT_LEFT"
	EventParticipantUpdated EventType = "PARTICIPANT_UPDATED"
	EventPresenterAssigned  EventType = "PRESENTER_ASSIGNED"
	EventPresenterTransfer  EventType = "PRESENTER_TRANSFERRED"
	EventPresenterRequested EventType = "PRESENTER_REQUESTED"
	EventScreenStarted      EventType = "SCREEN_STARTED"
	EventScreenStopped      EventType = "SCREEN_STOPPED"
	EventAnnotationCreated  EventType = "ANNOTATION_CREATED"
	EventAnnotationUpdated  EventType = "ANNOTATION_UPDATED"
	EventAnnotationDeleted  EventType = "ANNOTATION_DELETED"
	EventReactionSent       EventType = "REACTION_SENT"
	EventRoomPaused         EventType = "ROOM_PAUSED"
	EventRoomResumed        EventType = "ROOM_RESUMED"
	EventRoomEnded          EventType = "ROOM_ENDED"
	EventActionCreated      EventType = "ACTION_CREATED"
	EventActionResolved     EventType = "ACTION_RESOLVED"
	EventWebRTCOffer   EventType = "WEBRTC_OFFER"
	EventWebRTCAnswer  EventType = "WEBRTC_ANSWER"
	EventWebRTCICE     EventType = "WEBRTC_ICE_CANDIDATE"
	EventPeersAnnounce EventType = "PEERS_ANNOUNCE" 
)

type Event struct {
	Type   EventType       `json:"type"`
	RoomID string          `json:"roomId"`
	From   string          `json:"from,omitempty"` 
	To     string          `json:"to,omitempty"`   
	Ts     string          `json:"ts"`
	Data   json.RawMessage `json:"data,omitempty"`
}

func NewEvent(t EventType, roomID string, data any) Event {
	b, _ := json.Marshal(data)
	return Event{Type: t, RoomID: roomID, Ts: time.Now().UTC().Format(time.RFC3339Nano), Data: b}
}

func marshalEvent(e Event) ([]byte, error) {
	return json.Marshal(e)
}

func unmarshalEvent(raw []byte) (Event, error) {
	var e Event
	err := json.Unmarshal(raw, &e)
	return e, err
}

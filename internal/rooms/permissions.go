package rooms

import "github.com/tobenna/together/server/internal/models"
type Action string

const (
	ActionEndRoom         Action = "END_ROOM"
	ActionPauseResume     Action = "PAUSE_RESUME"
	ActionManageAccess    Action = "MANAGE_ACCESS"
	ActionPromoteDemote   Action = "PROMOTE_DEMOTE"
	ActionKick            Action = "KICK"
	ActionAssignPresenter Action = "ASSIGN_PRESENTER"
	ActionInvite          Action = "INVITE"
	ActionResolveAction   Action = "RESOLVE_ACTION"
	ActionStartPresent    Action = "START_PRESENT"
	ActionStopPresent     Action = "STOP_PRESENT"
	ActionAnnotate        Action = "ANNOTATE"
	ActionTransferPresent Action = "TRANSFER_PRESENTER"
	ActionRequestPresent  Action = "REQUEST_PRESENTER"
	ActionRequestStage    Action = "REQUEST_STAGE"
	ActionRaiseHand       Action = "RAISE_HAND"
	ActionReact           Action = "REACT"
)

var matrix = map[models.ParticipantRole]map[Action]bool{
	models.RoleOwner: {
		ActionEndRoom: true, ActionPauseResume: true, ActionManageAccess: true,
		ActionPromoteDemote: true, ActionKick: true, ActionAssignPresenter: true,
		ActionInvite: true, ActionResolveAction: true, ActionStartPresent: true,
		ActionStopPresent: true, ActionAnnotate: true, ActionTransferPresent: true,
		ActionReact: true, ActionRaiseHand: true,
	},
	models.RoleHost: {
		ActionPauseResume: true, ActionPromoteDemote: true, ActionKick: true,
		ActionAssignPresenter: true, ActionInvite: true, ActionResolveAction: true,
		ActionStartPresent: true, ActionStopPresent: true, ActionAnnotate: true,
		ActionTransferPresent: true, ActionReact: true, ActionRaiseHand: true,
	},
	models.RolePresenter: {
		ActionStartPresent: true, ActionStopPresent: true, ActionAnnotate: true,
		ActionTransferPresent: true, ActionReact: true, ActionRaiseHand: true,
		ActionRequestPresent: true,
	},
	models.RoleParticipant: {
		ActionReact: true, ActionRaiseHand: true, ActionRequestStage: true,
		ActionRequestPresent: true,
	},
	models.RoleViewer: {
		ActionReact: true, ActionRaiseHand: true, ActionRequestStage: true,
	},
}

func Can(role models.ParticipantRole, action Action) bool {
	return matrix[role][action]
}

func DefaultRoleForNewParticipant() models.ParticipantRole {
	return models.RoleParticipant
}

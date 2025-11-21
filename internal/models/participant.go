package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ParticipantRole string

const (
	ParticipantRoleHost      ParticipantRole = "host"
	ParticipantRoleModerator ParticipantRole = "moderator"
	ParticipantRoleGuest     ParticipantRole = "guest"
)

type Participant struct {
	ID         uuid.UUID       `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	MeetingID  uuid.UUID       `gorm:"type:uuid;not null;index" json:"meeting_id"`
	UserID     uuid.UUID       `gorm:"type:uuid;not null;index" json:"user_id"`
	Role       ParticipantRole `gorm:"type:varchar(20);default:'guest'" json:"role"`
	JoinedAt   time.Time       `gorm:"not null" json:"joined_at"`
	LeftAt     *time.Time      `json:"left_at"`
	IsMuted    bool            `gorm:"default:false" json:"is_muted"`
	IsVideoOn  bool            `gorm:"default:true" json:"is_video_on"`
	IsSharing  bool            `gorm:"default:false" json:"is_sharing"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
	DeletedAt  gorm.DeletedAt  `gorm:"index" json:"-"`

	// Relationships
	Meeting Meeting `gorm:"foreignKey:MeetingID" json:"meeting,omitempty"`
	User    User    `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// BeforeCreate hook to generate UUID and set joined time
func (p *Participant) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	if p.JoinedAt.IsZero() {
		p.JoinedAt = time.Now()
	}
	return nil
}

// TableName specifies the table name for Participant model
func (Participant) TableName() string {
	return "participants"
}

// ParticipantResponse represents the participant data sent in API responses
type ParticipantResponse struct {
	ID        uuid.UUID       `json:"id"`
	MeetingID uuid.UUID       `json:"meeting_id"`
	User      UserResponse    `json:"user"`
	Role      ParticipantRole `json:"role"`
	JoinedAt  time.Time       `json:"joined_at"`
	LeftAt    *time.Time      `json:"left_at"`
	IsMuted   bool            `json:"is_muted"`
	IsVideoOn bool            `json:"is_video_on"`
	IsSharing bool            `json:"is_sharing"`
}

// ToResponse converts Participant model to ParticipantResponse
func (p *Participant) ToResponse() ParticipantResponse {
	return ParticipantResponse{
		ID:        p.ID,
		MeetingID: p.MeetingID,
		User:      p.User.ToResponse(),
		Role:      p.Role,
		JoinedAt:  p.JoinedAt,
		LeftAt:    p.LeftAt,
		IsMuted:   p.IsMuted,
		IsVideoOn: p.IsVideoOn,
		IsSharing: p.IsSharing,
	}
}

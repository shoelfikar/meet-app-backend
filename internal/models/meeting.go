package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MeetingStatus string

const (
	MeetingStatusScheduled MeetingStatus = "scheduled"
	MeetingStatusActive    MeetingStatus = "active"
	MeetingStatusEnded     MeetingStatus = "ended"
)

type Meeting struct {
	ID           uuid.UUID       `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	Code         string          `gorm:"uniqueIndex;not null;size:36" json:"code"`
	Title        string          `gorm:"not null" json:"title"`
	Description  string          `gorm:"type:text" json:"description"`
	HostID       uuid.UUID       `gorm:"type:uuid;not null" json:"host_id"`
	Status       MeetingStatus   `gorm:"type:varchar(20);default:'scheduled'" json:"status"`
	ScheduledAt  *time.Time      `json:"scheduled_at"`
	StartedAt    *time.Time      `json:"started_at"`
	EndedAt      *time.Time      `json:"ended_at"`
	MaxUsers     int             `gorm:"default:50" json:"max_users"`
	IsRecording  bool            `gorm:"default:false" json:"is_recording"`
	RecordingURL string          `gorm:"type:text" json:"recording_url"`
	Settings     MeetingSettings `gorm:"type:jsonb;serializer:json" json:"settings"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	DeletedAt    gorm.DeletedAt  `gorm:"index" json:"-"`

	// Relationships
	Host         User          `gorm:"foreignKey:HostID" json:"host,omitempty"`
	Participants []Participant `gorm:"foreignKey:MeetingID" json:"participants,omitempty"`
	Messages     []Message     `gorm:"foreignKey:MeetingID" json:"messages,omitempty"`
}

type MeetingSettings struct {
	AllowChat          bool `json:"allow_chat"`
	AllowScreenShare   bool `json:"allow_screen_share"`
	MuteOnJoin         bool `json:"mute_on_join"`
	VideoOnJoin        bool `json:"video_on_join"`
	WaitingRoomEnabled bool `json:"waiting_room_enabled"`
	RecordingEnabled   bool `json:"recording_enabled"`
}

// BeforeCreate hook to generate UUID and meeting code
func (m *Meeting) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}

	// Generate unique meeting code using UUID
	if m.Code == "" {
		m.Code = generateMeetingCode()
	}

	// Set default settings if all fields are zero (not provided)
	if m.Settings == (MeetingSettings{}) {
		m.Settings = MeetingSettings{
			AllowChat:          true,
			AllowScreenShare:   true,
			MuteOnJoin:         false,
			VideoOnJoin:        true,
			WaitingRoomEnabled: false,
			RecordingEnabled:   false,
		}
	}

	return nil
}

// TableName specifies the table name for Meeting model
func (Meeting) TableName() string {
	return "meetings"
}

// generateMeetingCode generates a unique meeting code using UUID
func generateMeetingCode() string {
	// Generate a new UUID and return as string
	return uuid.New().String()
}

// MeetingResponse represents the meeting data sent in API responses
type MeetingResponse struct {
	ID           uuid.UUID       `json:"id"`
	Code         string          `json:"code"`
	Title        string          `json:"title"`
	Description  string          `json:"description"`
	HostID       uuid.UUID       `json:"host_id"`
	Host         UserResponse    `json:"host"`
	Status       MeetingStatus   `json:"status"`
	ScheduledAt  *time.Time      `json:"scheduled_at"`
	StartedAt    *time.Time      `json:"started_at"`
	EndedAt      *time.Time      `json:"ended_at"`
	MaxUsers     int             `json:"max_users"`
	IsRecording  bool            `json:"is_recording"`
	RecordingURL string          `json:"recording_url,omitempty"`
	Settings     MeetingSettings `json:"settings"`
	CreatedAt    time.Time       `json:"created_at"`
}

// ToResponse converts Meeting model to MeetingResponse
func (m *Meeting) ToResponse() MeetingResponse {
	return MeetingResponse{
		ID:           m.ID,
		Code:         m.Code,
		Title:        m.Title,
		Description:  m.Description,
		HostID:       m.HostID,
		Host:         m.Host.ToResponse(),
		Status:       m.Status,
		ScheduledAt:  m.ScheduledAt,
		StartedAt:    m.StartedAt,
		EndedAt:      m.EndedAt,
		MaxUsers:     m.MaxUsers,
		IsRecording:  m.IsRecording,
		RecordingURL: m.RecordingURL,
		Settings:     m.Settings,
		CreatedAt:    m.CreatedAt,
	}
}

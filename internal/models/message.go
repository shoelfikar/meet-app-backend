package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MessageType string

const (
	MessageTypeText   MessageType = "text"
	MessageTypeSystem MessageType = "system"
	MessageTypeFile   MessageType = "file"
)

type Message struct {
	ID        uuid.UUID      `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	MeetingID uuid.UUID      `gorm:"type:uuid;not null;index" json:"meeting_id"`
	UserID    uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	Type      MessageType    `gorm:"type:varchar(20);default:'text'" json:"type"`
	Content   string         `gorm:"type:text;not null" json:"content"`
	FileURL   string         `gorm:"type:text" json:"file_url,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Meeting Meeting `gorm:"foreignKey:MeetingID" json:"meeting,omitempty"`
	User    User    `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// BeforeCreate hook to generate UUID
func (m *Message) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name for Message model
func (Message) TableName() string {
	return "messages"
}

// MessageResponse represents the message data sent in API responses
type MessageResponse struct {
	ID        uuid.UUID    `json:"id"`
	MeetingID uuid.UUID    `json:"meeting_id"`
	User      UserResponse `json:"user"`
	Type      MessageType  `json:"type"`
	Content   string       `json:"content"`
	FileURL   string       `json:"file_url,omitempty"`
	CreatedAt time.Time    `json:"created_at"`
}

// ToResponse converts Message model to MessageResponse
func (m *Message) ToResponse() MessageResponse {
	return MessageResponse{
		ID:        m.ID,
		MeetingID: m.MeetingID,
		User:      m.User.ToResponse(),
		Type:      m.Type,
		Content:   m.Content,
		FileURL:   m.FileURL,
		CreatedAt: m.CreatedAt,
	}
}

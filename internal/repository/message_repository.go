package repository

import (
	"errors"

	"github.com/google/uuid"
	"github.com/meet-app/backend/internal/models"
	"gorm.io/gorm"
)

var (
	ErrMessageNotFound = errors.New("message not found")
)

type MessageRepository interface {
	Create(message *models.Message) error
	FindByID(id uuid.UUID) (*models.Message, error)
	FindByMeetingID(meetingID uuid.UUID, limit int) ([]models.Message, error)
	FindByMeetingIDPaginated(meetingID uuid.UUID, offset, limit int) ([]models.Message, error)
	Update(message *models.Message) error
	Delete(id uuid.UUID) error
	CountByMeetingID(meetingID uuid.UUID) (int64, error)
}

type messageRepository struct {
	db *gorm.DB
}

func NewMessageRepository(db *gorm.DB) MessageRepository {
	return &messageRepository{db: db}
}

func (r *messageRepository) Create(message *models.Message) error {
	return r.db.Create(message).Error
}

func (r *messageRepository) FindByID(id uuid.UUID) (*models.Message, error) {
	var message models.Message
	err := r.db.Preload("User").Preload("Meeting").
		Where("id = ?", id).First(&message).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMessageNotFound
		}
		return nil, err
	}
	return &message, nil
}

func (r *messageRepository) FindByMeetingID(meetingID uuid.UUID, limit int) ([]models.Message, error) {
	var messages []models.Message
	query := r.db.Preload("User").
		Where("meeting_id = ?", meetingID).
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&messages).Error
	return messages, err
}

func (r *messageRepository) FindByMeetingIDPaginated(meetingID uuid.UUID, offset, limit int) ([]models.Message, error) {
	var messages []models.Message
	err := r.db.Preload("User").
		Where("meeting_id = ?", meetingID).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&messages).Error
	return messages, err
}

func (r *messageRepository) Update(message *models.Message) error {
	return r.db.Save(message).Error
}

func (r *messageRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&models.Message{}, id).Error
}

func (r *messageRepository) CountByMeetingID(meetingID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&models.Message{}).
		Where("meeting_id = ?", meetingID).
		Count(&count).Error
	return count, err
}

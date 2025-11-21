package repository

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/meet-app/backend/internal/models"
	"gorm.io/gorm"
)

var (
	ErrParticipantNotFound      = errors.New("participant not found")
	ErrParticipantAlreadyExists = errors.New("participant already exists in meeting")
)

type ParticipantRepository interface {
	Create(participant *models.Participant) error
	FindByID(id uuid.UUID) (*models.Participant, error)
	FindByMeetingID(meetingID uuid.UUID) ([]models.Participant, error)
	FindByUserAndMeeting(userID, meetingID uuid.UUID) (*models.Participant, error)
	FindActiveMeetingParticipants(meetingID uuid.UUID) ([]models.Participant, error)
	Update(participant *models.Participant) error
	UpdateMediaStatus(id uuid.UUID, isMuted, isVideoOn, isSharing bool) error
	MarkAsLeft(id uuid.UUID) error
	Delete(id uuid.UUID) error
	CountActiveMeetingParticipants(meetingID uuid.UUID) (int64, error)
	IsUserInMeeting(userID, meetingID uuid.UUID) (bool, error)
}

type participantRepository struct {
	db *gorm.DB
}

func NewParticipantRepository(db *gorm.DB) ParticipantRepository {
	return &participantRepository{db: db}
}

func (r *participantRepository) Create(participant *models.Participant) error {
	// Check if participant already exists
	exists, err := r.IsUserInMeeting(participant.UserID, participant.MeetingID)
	if err != nil {
		return err
	}
	if exists {
		return ErrParticipantAlreadyExists
	}

	return r.db.Create(participant).Error
}

func (r *participantRepository) FindByID(id uuid.UUID) (*models.Participant, error) {
	var participant models.Participant
	err := r.db.Preload("User").Preload("Meeting").
		Where("id = ?", id).First(&participant).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrParticipantNotFound
		}
		return nil, err
	}
	return &participant, nil
}

func (r *participantRepository) FindByMeetingID(meetingID uuid.UUID) ([]models.Participant, error) {
	var participants []models.Participant
	err := r.db.Preload("User").
		Where("meeting_id = ?", meetingID).
		Order("joined_at ASC").
		Find(&participants).Error
	return participants, err
}

func (r *participantRepository) FindByUserAndMeeting(userID, meetingID uuid.UUID) (*models.Participant, error) {
	var participant models.Participant
	err := r.db.Preload("User").Preload("Meeting").
		Where("user_id = ? AND meeting_id = ?", userID, meetingID).
		First(&participant).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrParticipantNotFound
		}
		return nil, err
	}
	return &participant, nil
}

func (r *participantRepository) FindActiveMeetingParticipants(meetingID uuid.UUID) ([]models.Participant, error) {
	var participants []models.Participant
	err := r.db.Preload("User").
		Where("meeting_id = ? AND left_at IS NULL", meetingID).
		Order("joined_at ASC").
		Find(&participants).Error
	return participants, err
}

func (r *participantRepository) Update(participant *models.Participant) error {
	return r.db.Save(participant).Error
}

func (r *participantRepository) UpdateMediaStatus(id uuid.UUID, isMuted, isVideoOn, isSharing bool) error {
	return r.db.Model(&models.Participant{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"is_muted":    isMuted,
			"is_video_on": isVideoOn,
			"is_sharing":  isSharing,
		}).Error
}

func (r *participantRepository) MarkAsLeft(id uuid.UUID) error {
	now := time.Now()
	return r.db.Model(&models.Participant{}).
		Where("id = ?", id).
		Update("left_at", now).Error
}

func (r *participantRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&models.Participant{}, id).Error
}

func (r *participantRepository) CountActiveMeetingParticipants(meetingID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&models.Participant{}).
		Where("meeting_id = ? AND left_at IS NULL", meetingID).
		Count(&count).Error
	return count, err
}

func (r *participantRepository) IsUserInMeeting(userID, meetingID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.Model(&models.Participant{}).
		Where("user_id = ? AND meeting_id = ? AND left_at IS NULL", userID, meetingID).
		Count(&count).Error
	return count > 0, err
}

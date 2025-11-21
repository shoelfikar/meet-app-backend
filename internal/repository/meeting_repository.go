package repository

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/meet-app/backend/internal/models"
	"gorm.io/gorm"
)

var (
	ErrMeetingNotFound      = errors.New("meeting not found")
	ErrMeetingCodeExists    = errors.New("meeting code already exists")
)

type MeetingRepository interface {
	Create(meeting *models.Meeting) error
	FindByID(id uuid.UUID) (*models.Meeting, error)
	FindByCode(code string) (*models.Meeting, error)
	FindByHostID(hostID uuid.UUID) ([]models.Meeting, error)
	FindActiveMeetings() ([]models.Meeting, error)
	Update(meeting *models.Meeting) error
	Delete(id uuid.UUID) error
	UpdateStatus(id uuid.UUID, status models.MeetingStatus) error
	StartMeeting(id uuid.UUID) error
	EndMeeting(id uuid.UUID) error
	ExistsByCode(code string) (bool, error)
}

type meetingRepository struct {
	db *gorm.DB
}

func NewMeetingRepository(db *gorm.DB) MeetingRepository {
	return &meetingRepository{db: db}
}

func (r *meetingRepository) Create(meeting *models.Meeting) error {
	// Check if code exists
	exists, err := r.ExistsByCode(meeting.Code)
	if err != nil {
		return err
	}
	if exists {
		return ErrMeetingCodeExists
	}

	return r.db.Create(meeting).Error
}

func (r *meetingRepository) FindByID(id uuid.UUID) (*models.Meeting, error) {
	var meeting models.Meeting
	err := r.db.Preload("Host").Where("id = ?", id).First(&meeting).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMeetingNotFound
		}
		return nil, err
	}
	return &meeting, nil
}

func (r *meetingRepository) FindByCode(code string) (*models.Meeting, error) {
	var meeting models.Meeting
	err := r.db.Preload("Host").Where("code = ?", code).First(&meeting).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMeetingNotFound
		}
		return nil, err
	}
	return &meeting, nil
}

func (r *meetingRepository) FindByHostID(hostID uuid.UUID) ([]models.Meeting, error) {
	var meetings []models.Meeting
	err := r.db.Preload("Host").Where("host_id = ?", hostID).Order("created_at DESC").Find(&meetings).Error
	return meetings, err
}

func (r *meetingRepository) FindActiveMeetings() ([]models.Meeting, error) {
	var meetings []models.Meeting
	err := r.db.Preload("Host").
		Where("status = ?", models.MeetingStatusActive).
		Order("started_at DESC").
		Find(&meetings).Error
	return meetings, err
}

func (r *meetingRepository) Update(meeting *models.Meeting) error {
	return r.db.Save(meeting).Error
}

func (r *meetingRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&models.Meeting{}, id).Error
}

func (r *meetingRepository) UpdateStatus(id uuid.UUID, status models.MeetingStatus) error {
	return r.db.Model(&models.Meeting{}).
		Where("id = ?", id).
		Update("status", status).Error
}

func (r *meetingRepository) StartMeeting(id uuid.UUID) error {
	now := time.Now()
	return r.db.Model(&models.Meeting{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     models.MeetingStatusActive,
			"started_at": now,
		}).Error
}

func (r *meetingRepository) EndMeeting(id uuid.UUID) error {
	now := time.Now()
	return r.db.Model(&models.Meeting{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":   models.MeetingStatusEnded,
			"ended_at": now,
		}).Error
}

func (r *meetingRepository) ExistsByCode(code string) (bool, error) {
	var count int64
	err := r.db.Model(&models.Meeting{}).Where("code = ?", code).Count(&count).Error
	return count > 0, err
}

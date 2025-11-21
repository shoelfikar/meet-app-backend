package service

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/meet-app/backend/internal/models"
	"github.com/meet-app/backend/internal/repository"
)

var (
	ErrMeetingFull         = errors.New("meeting has reached maximum participants")
	ErrUnauthorizedAccess  = errors.New("unauthorized to perform this action")
	ErrAlreadyInMeeting    = errors.New("user is already in the meeting")
)

type MeetingService interface {
	CreateMeeting(hostID uuid.UUID, title, description string, settings models.MeetingSettings) (*models.Meeting, error)
	GetMeetingByCode(code string) (*models.Meeting, error)
	GetMeetingByID(id uuid.UUID) (*models.Meeting, error)
	GetUserMeetings(userID uuid.UUID) ([]models.Meeting, error)
	JoinMeeting(userID, meetingID uuid.UUID, role models.ParticipantRole) (*models.Participant, error)
	LeaveMeeting(userID, meetingID uuid.UUID) error
	StartMeeting(meetingID, userID uuid.UUID) error
	EndMeeting(meetingID, userID uuid.UUID) error
	UpdateMeetingSettings(meetingID, userID uuid.UUID, settings models.MeetingSettings) error
	GetMeetingParticipants(meetingID uuid.UUID) ([]models.Participant, error)
	UpdateParticipantMediaStatus(participantID uuid.UUID, isMuted, isVideoOn, isSharing bool) error
}

type meetingService struct {
	meetingRepo     repository.MeetingRepository
	participantRepo repository.ParticipantRepository
}

func NewMeetingService(
	meetingRepo repository.MeetingRepository,
	participantRepo repository.ParticipantRepository,
) MeetingService {
	return &meetingService{
		meetingRepo:     meetingRepo,
		participantRepo: participantRepo,
	}
}

func (s *meetingService) CreateMeeting(
	hostID uuid.UUID,
	title, description string,
	settings models.MeetingSettings,
) (*models.Meeting, error) {
	meeting := &models.Meeting{
		Title:       title,
		Description: description,
		HostID:      hostID,
		Status:      models.MeetingStatusScheduled,
		MaxUsers:    50,
		Settings:    settings,
	}

	if err := s.meetingRepo.Create(meeting); err != nil {
		return nil, err
	}

	// Automatically add host as first participant
	participant := &models.Participant{
		MeetingID: meeting.ID,
		UserID:    hostID,
		Role:      models.ParticipantRoleHost,
		JoinedAt:  time.Now(),
	}

	if err := s.participantRepo.Create(participant); err != nil {
		return nil, err
	}

	// Reload meeting with host info
	return s.meetingRepo.FindByID(meeting.ID)
}

func (s *meetingService) GetMeetingByCode(code string) (*models.Meeting, error) {
	return s.meetingRepo.FindByCode(code)
}

func (s *meetingService) GetMeetingByID(id uuid.UUID) (*models.Meeting, error) {
	return s.meetingRepo.FindByID(id)
}

func (s *meetingService) GetUserMeetings(userID uuid.UUID) ([]models.Meeting, error) {
	return s.meetingRepo.FindByHostID(userID)
}

func (s *meetingService) JoinMeeting(
	userID, meetingID uuid.UUID,
	role models.ParticipantRole,
) (*models.Participant, error) {
	// Check if meeting exists
	meeting, err := s.meetingRepo.FindByID(meetingID)
	if err != nil {
		return nil, err
	}

	// Check if user is already in meeting
	exists, err := s.participantRepo.IsUserInMeeting(userID, meetingID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrAlreadyInMeeting
	}

	// Check if meeting is full
	count, err := s.participantRepo.CountActiveMeetingParticipants(meetingID)
	if err != nil {
		return nil, err
	}
	if count >= int64(meeting.MaxUsers) {
		return nil, ErrMeetingFull
	}

	// Create participant
	participant := &models.Participant{
		MeetingID: meetingID,
		UserID:    userID,
		Role:      role,
		JoinedAt:  time.Now(),
		IsMuted:   meeting.Settings.MuteOnJoin,
		IsVideoOn: meeting.Settings.VideoOnJoin,
	}

	if err := s.participantRepo.Create(participant); err != nil {
		return nil, err
	}

	// If meeting is not active, activate it
	if meeting.Status == models.MeetingStatusScheduled {
		if err := s.meetingRepo.StartMeeting(meetingID); err != nil {
			return nil, err
		}
	}

	return s.participantRepo.FindByID(participant.ID)
}

func (s *meetingService) LeaveMeeting(userID, meetingID uuid.UUID) error {
	// Find participant
	participant, err := s.participantRepo.FindByUserAndMeeting(userID, meetingID)
	if err != nil {
		return err
	}

	// Mark as left
	return s.participantRepo.MarkAsLeft(participant.ID)
}

func (s *meetingService) StartMeeting(meetingID, userID uuid.UUID) error {
	// Verify user is host
	meeting, err := s.meetingRepo.FindByID(meetingID)
	if err != nil {
		return err
	}

	if meeting.HostID != userID {
		return ErrUnauthorizedAccess
	}

	return s.meetingRepo.StartMeeting(meetingID)
}

func (s *meetingService) EndMeeting(meetingID, userID uuid.UUID) error {
	// Verify user is host
	meeting, err := s.meetingRepo.FindByID(meetingID)
	if err != nil {
		return err
	}

	if meeting.HostID != userID {
		return ErrUnauthorizedAccess
	}

	return s.meetingRepo.EndMeeting(meetingID)
}

func (s *meetingService) UpdateMeetingSettings(
	meetingID, userID uuid.UUID,
	settings models.MeetingSettings,
) error {
	// Verify user is host
	meeting, err := s.meetingRepo.FindByID(meetingID)
	if err != nil {
		return err
	}

	if meeting.HostID != userID {
		return ErrUnauthorizedAccess
	}

	meeting.Settings = settings
	return s.meetingRepo.Update(meeting)
}

func (s *meetingService) GetMeetingParticipants(meetingID uuid.UUID) ([]models.Participant, error) {
	return s.participantRepo.FindActiveMeetingParticipants(meetingID)
}

func (s *meetingService) UpdateParticipantMediaStatus(
	participantID uuid.UUID,
	isMuted, isVideoOn, isSharing bool,
) error {
	return s.participantRepo.UpdateMediaStatus(participantID, isMuted, isVideoOn, isSharing)
}

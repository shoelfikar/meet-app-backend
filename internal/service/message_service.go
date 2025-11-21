package service

import (
	"github.com/google/uuid"
	"github.com/meet-app/backend/internal/models"
	"github.com/meet-app/backend/internal/repository"
)

type MessageService interface {
	SendMessage(userID, meetingID uuid.UUID, messageType models.MessageType, content string) (*models.Message, error)
	GetMeetingMessages(meetingID uuid.UUID, limit int) ([]models.Message, error)
	GetMeetingMessagesPaginated(meetingID uuid.UUID, offset, limit int) ([]models.Message, error)
	DeleteMessage(messageID, userID uuid.UUID) error
}

type messageService struct {
	messageRepo     repository.MessageRepository
	participantRepo repository.ParticipantRepository
}

func NewMessageService(
	messageRepo repository.MessageRepository,
	participantRepo repository.ParticipantRepository,
) MessageService {
	return &messageService{
		messageRepo:     messageRepo,
		participantRepo: participantRepo,
	}
}

func (s *messageService) SendMessage(
	userID, meetingID uuid.UUID,
	messageType models.MessageType,
	content string,
) (*models.Message, error) {
	// Verify user is in meeting
	isInMeeting, err := s.participantRepo.IsUserInMeeting(userID, meetingID)
	if err != nil {
		return nil, err
	}
	if !isInMeeting {
		return nil, ErrUnauthorizedAccess
	}

	message := &models.Message{
		MeetingID: meetingID,
		UserID:    userID,
		Type:      messageType,
		Content:   content,
	}

	if err := s.messageRepo.Create(message); err != nil {
		return nil, err
	}

	return s.messageRepo.FindByID(message.ID)
}

func (s *messageService) GetMeetingMessages(meetingID uuid.UUID, limit int) ([]models.Message, error) {
	return s.messageRepo.FindByMeetingID(meetingID, limit)
}

func (s *messageService) GetMeetingMessagesPaginated(
	meetingID uuid.UUID,
	offset, limit int,
) ([]models.Message, error) {
	return s.messageRepo.FindByMeetingIDPaginated(meetingID, offset, limit)
}

func (s *messageService) DeleteMessage(messageID, userID uuid.UUID) error {
	// Find message
	message, err := s.messageRepo.FindByID(messageID)
	if err != nil {
		return err
	}

	// Verify user owns the message
	if message.UserID != userID {
		return ErrUnauthorizedAccess
	}

	return s.messageRepo.Delete(messageID)
}

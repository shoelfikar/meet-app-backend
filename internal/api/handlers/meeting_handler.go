package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/meet-app/backend/internal/api/middleware"
	"github.com/meet-app/backend/internal/models"
	"github.com/meet-app/backend/internal/repository"
	"github.com/meet-app/backend/internal/service"
	"github.com/meet-app/backend/internal/sse"
)

type MeetingHandler struct {
	meetingService service.MeetingService
	messageService service.MessageService
}

func NewMeetingHandler(meetingService service.MeetingService, messageService service.MessageService) *MeetingHandler {
	return &MeetingHandler{
		meetingService: meetingService,
		messageService: messageService,
	}
}

type CreateMeetingRequest struct {
	Title       string                  `json:"title" binding:"required"`
	Description string                  `json:"description"`
	Settings    models.MeetingSettings `json:"settings"`
}

type JoinMeetingRequest struct {
	Code string `json:"code" binding:"required"`
}

type SendMessageRequest struct {
	Content string `json:"content" binding:"required"`
	Type    models.MessageType `json:"type"`
}

type UpdateMediaStatusRequest struct {
	IsMuted   bool `json:"is_muted"`
	IsVideoOn bool `json:"is_video_on"`
	IsSharing bool `json:"is_sharing"`
}

// CreateMeeting godoc
// @Summary Create a new meeting
// @Description Create a new meeting with title, description, and settings
// @Tags meetings
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateMeetingRequest true "Create meeting request"
// @Success 201 {object} models.MeetingResponse
// @Failure 400 {object} middleware.ErrorResponse
// @Failure 401 {object} middleware.ErrorResponse
// @Failure 500 {object} middleware.ErrorResponse
// @Router /meetings [post]
func (h *MeetingHandler) CreateMeeting(c *gin.Context) {
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		middleware.RespondWithError(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req CreateMeetingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	meeting, err := h.meetingService.CreateMeeting(userID, req.Title, req.Description, req.Settings)
	if err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to create meeting")
		return
	}

	c.JSON(http.StatusCreated, meeting.ToResponse())
}

// GetMeetingByCode godoc
// @Summary Get meeting by code
// @Description Get meeting details by meeting code
// @Tags meetings
// @Produce json
// @Param code path string true "Meeting code"
// @Success 200 {object} models.MeetingResponse
// @Failure 404 {object} middleware.ErrorResponse
// @Failure 500 {object} middleware.ErrorResponse
// @Router /meetings/{code} [get]
func (h *MeetingHandler) GetMeetingByCode(c *gin.Context) {
	code := c.Param("code")

	meeting, err := h.meetingService.GetMeetingByCode(code)
	if err != nil {
		if err == repository.ErrMeetingNotFound {
			middleware.RespondWithError(c, http.StatusNotFound, "Meeting not found")
			return
		}
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to get meeting")
		return
	}

	c.JSON(http.StatusOK, meeting.ToResponse())
}

// JoinMeeting godoc
// @Summary Join a meeting
// @Description Join an existing meeting by code
// @Tags meetings
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body JoinMeetingRequest true "Join meeting request"
// @Success 200 {object} models.ParticipantResponse
// @Failure 400 {object} middleware.ErrorResponse
// @Failure 401 {object} middleware.ErrorResponse
// @Failure 404 {object} middleware.ErrorResponse
// @Failure 500 {object} middleware.ErrorResponse
// @Router /meetings/join [post]
func (h *MeetingHandler) JoinMeeting(c *gin.Context) {
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		middleware.RespondWithError(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req JoinMeetingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	// Get meeting by code
	meeting, err := h.meetingService.GetMeetingByCode(req.Code)
	if err != nil {
		if err == repository.ErrMeetingNotFound {
			middleware.RespondWithError(c, http.StatusNotFound, "Meeting not found")
			return
		}
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to get meeting")
		return
	}

	// Join meeting as guest
	participant, err := h.meetingService.JoinMeeting(userID, meeting.ID, models.ParticipantRoleGuest)
	if err != nil {
		if err == service.ErrMeetingFull {
			middleware.RespondWithError(c, http.StatusConflict, "Meeting is full")
			return
		}
		if err == service.ErrAlreadyInMeeting {
			middleware.RespondWithError(c, http.StatusConflict, "Already in meeting")
			return
		}
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to join meeting")
		return
	}

	// Broadcast participant joined event to other participants
	hub := sse.GetHub()
	log.Printf("JoinMeeting: Broadcasting participant_joined for user %s to meeting %s", userID, meeting.ID)
	hub.BroadcastToMeetingExcept(meeting.ID, userID, sse.Event{
		Type: sse.EventParticipantJoined,
		Data: participant.ToResponse(),
	})

	c.JSON(http.StatusOK, participant.ToResponse())
}

// LeaveMeeting godoc
// @Summary Leave a meeting
// @Description Leave the current meeting
// @Tags meetings
// @Security BearerAuth
// @Param id path string true "Meeting ID"
// @Success 200 {object} map[string]string
// @Failure 401 {object} middleware.ErrorResponse
// @Failure 500 {object} middleware.ErrorResponse
// @Router /meetings/{id}/leave [post]
func (h *MeetingHandler) LeaveMeeting(c *gin.Context) {
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		middleware.RespondWithError(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	meetingIDStr := c.Param("id")
	meetingID, err := uuid.Parse(meetingIDStr)
	if err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Invalid meeting ID")
		return
	}

	if err := h.meetingService.LeaveMeeting(userID, meetingID); err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to leave meeting")
		return
	}

	// Broadcast participant left event to other participants
	hub := sse.GetHub()
	hub.BroadcastToMeeting(meetingID, sse.Event{
		Type: sse.EventParticipantLeft,
		Data: map[string]string{"user_id": userID.String()},
	})

	c.JSON(http.StatusOK, gin.H{"message": "Left meeting successfully"})
}

// GetMeetingParticipants godoc
// @Summary Get meeting participants
// @Description Get all active participants in a meeting
// @Tags meetings
// @Produce json
// @Security BearerAuth
// @Param id path string true "Meeting ID"
// @Success 200 {array} models.ParticipantResponse
// @Failure 400 {object} middleware.ErrorResponse
// @Failure 401 {object} middleware.ErrorResponse
// @Failure 500 {object} middleware.ErrorResponse
// @Router /meetings/{id}/participants [get]
func (h *MeetingHandler) GetMeetingParticipants(c *gin.Context) {
	meetingIDStr := c.Param("id")
	meetingID, err := uuid.Parse(meetingIDStr)
	if err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Invalid meeting ID")
		return
	}

	participants, err := h.meetingService.GetMeetingParticipants(meetingID)
	if err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to get participants")
		return
	}

	// Convert to response format
	participantResponses := make([]models.ParticipantResponse, len(participants))
	for i, p := range participants {
		participantResponses[i] = p.ToResponse()
	}

	c.JSON(http.StatusOK, participantResponses)
}

// SendMessage godoc
// @Summary Send a chat message
// @Description Send a chat message in a meeting
// @Tags meetings
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Meeting ID"
// @Param request body SendMessageRequest true "Send message request"
// @Success 201 {object} models.MessageResponse
// @Failure 400 {object} middleware.ErrorResponse
// @Failure 401 {object} middleware.ErrorResponse
// @Failure 500 {object} middleware.ErrorResponse
// @Router /meetings/{id}/messages [post]
func (h *MeetingHandler) SendMessage(c *gin.Context) {
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		middleware.RespondWithError(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	meetingIDStr := c.Param("id")
	meetingID, err := uuid.Parse(meetingIDStr)
	if err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Invalid meeting ID")
		return
	}

	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	// Default to text message if type not specified
	if req.Type == "" {
		req.Type = models.MessageTypeText
	}

	message, err := h.messageService.SendMessage(userID, meetingID, req.Type, req.Content)
	if err != nil {
		if err == service.ErrUnauthorizedAccess {
			middleware.RespondWithError(c, http.StatusUnauthorized, "Not in meeting")
			return
		}
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to send message")
		return
	}

	c.JSON(http.StatusCreated, message.ToResponse())
}

// GetMessages godoc
// @Summary Get meeting messages
// @Description Get chat messages from a meeting
// @Tags meetings
// @Produce json
// @Security BearerAuth
// @Param id path string true "Meeting ID"
// @Param limit query int false "Limit number of messages" default(50)
// @Success 200 {array} models.MessageResponse
// @Failure 400 {object} middleware.ErrorResponse
// @Failure 401 {object} middleware.ErrorResponse
// @Failure 500 {object} middleware.ErrorResponse
// @Router /meetings/{id}/messages [get]
func (h *MeetingHandler) GetMessages(c *gin.Context) {
	meetingIDStr := c.Param("id")
	meetingID, err := uuid.Parse(meetingIDStr)
	if err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Invalid meeting ID")
		return
	}

	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := uuid.Parse(limitStr); err == nil {
			limit = int(parsedLimit.ID())
		}
	}

	messages, err := h.messageService.GetMeetingMessages(meetingID, limit)
	if err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to get messages")
		return
	}

	// Convert to response format
	messageResponses := make([]models.MessageResponse, len(messages))
	for i, m := range messages {
		messageResponses[i] = m.ToResponse()
	}

	c.JSON(http.StatusOK, messageResponses)
}

// EndMeeting godoc
// @Summary End a meeting
// @Description End a meeting (host only)
// @Tags meetings
// @Security BearerAuth
// @Param id path string true "Meeting ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} middleware.ErrorResponse
// @Failure 401 {object} middleware.ErrorResponse
// @Failure 403 {object} middleware.ErrorResponse
// @Failure 500 {object} middleware.ErrorResponse
// @Router /meetings/{id}/end [post]
func (h *MeetingHandler) EndMeeting(c *gin.Context) {
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		middleware.RespondWithError(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	meetingIDStr := c.Param("id")
	meetingID, err := uuid.Parse(meetingIDStr)
	if err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Invalid meeting ID")
		return
	}

	if err := h.meetingService.EndMeeting(meetingID, userID); err != nil {
		if err == service.ErrUnauthorizedAccess {
			middleware.RespondWithError(c, http.StatusForbidden, "Only host can end meeting")
			return
		}
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to end meeting")
		return
	}

	// Broadcast meeting ended event to all participants
	hub := sse.GetHub()
	hub.BroadcastToMeeting(meetingID, sse.Event{
		Type: sse.EventMeetingEnded,
		Data: map[string]string{"meeting_id": meetingID.String()},
	})

	c.JSON(http.StatusOK, gin.H{"message": "Meeting ended successfully"})
}

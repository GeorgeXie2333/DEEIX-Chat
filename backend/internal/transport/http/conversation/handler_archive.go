package conversation

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

const conversationArchiveImportMaxBytes = 10 << 20

// ExportConversationArchive godoc
// @Summary 导出单条会话 JSON
// @Description 将当前用户的一条会话导出为 deeix-chat.conversation.v1 JSON 归档
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "会话 public_id"
// @Success 200 {object} ConversationArchiveResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /conversations/{id}/export [get]
func (h *Handler) ExportConversationArchive(c *gin.Context) {
	userID := middleware.MustUserID(c)
	publicID, err := stringParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	archive, err := h.service.ExportConversationArchive(c.Request.Context(), userID, publicID)
	if err != nil {
		if errors.Is(err, appconversation.ErrConversationNotFound) {
			response.Error(c, http.StatusNotFound, "conversation not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "export conversation failed")
		return
	}

	h.recordAudit(c, "export_conversation_archive",
		"conversation",
		publicID,
		map[string]interface{}{"schema": archive.Schema, "messageCount": len(archive.Messages)},
	)

	response.Success(c, archive)
}

// ImportConversationArchive godoc
// @Summary 导入单条会话 JSON
// @Description 导入 deeix-chat.conversation.v1 JSON 归档，并创建新的会话副本
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body ConversationArchiveRequest true "会话归档 JSON"
// @Success 200 {object} ConversationUpdateResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 413 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /conversations/import [post]
func (h *Handler) ImportConversationArchive(c *gin.Context) {
	userID := middleware.MustUserID(c)
	if c.Request.ContentLength > conversationArchiveImportMaxBytes {
		response.Error(c, http.StatusRequestEntityTooLarge, "conversation archive too large")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, conversationArchiveImportMaxBytes)

	var archive appconversation.ConversationArchive
	if err := c.ShouldBindJSON(&archive); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "request body too large") {
			response.Error(c, http.StatusRequestEntityTooLarge, "conversation archive too large")
			return
		}
		response.InvalidRequestBody(c, err)
		return
	}

	item, err := h.service.ImportConversationArchive(c.Request.Context(), userID, &archive)
	if err != nil {
		switch {
		case errors.Is(err, appconversation.ErrConversationArchiveTooLarge):
			response.Error(c, http.StatusRequestEntityTooLarge, "conversation archive too large")
			return
		case errors.Is(err, appconversation.ErrInvalidConversationArchive):
			response.Error(c, http.StatusBadRequest, "invalid conversation archive")
			return
		default:
			response.Error(c, http.StatusInternalServerError, "import conversation failed")
			return
		}
	}

	h.recordAudit(c, "import_conversation_archive",
		"conversation",
		strconv.FormatUint(uint64(item.ID), 10),
		map[string]interface{}{"schema": archive.Schema, "messageCount": item.MessageCount},
	)

	response.Success(c, toConversationResponse(item))
}

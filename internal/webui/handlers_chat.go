package webui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"fenrir/internal/db"
	"fenrir/internal/webui/modes"
	"fenrir/internal/webui/prompts"

	"github.com/go-chi/chi/v5"
)

// ChatRequest — запрос к чату
type ChatRequest struct {
	Message   string   `json:"message"`
	Mode      string   `json:"mode"`
	Model     string   `json:"model"`
	AgentMode string   `json:"agentMode"`
	ChatID    string   `json:"chatId,omitempty"`
	Files     []string `json:"files,omitempty"`
}

// ChatResponse — ответ чата
type ChatResponse struct {
	Type    string      `json:"type"`
	Content interface{} `json:"content"`
	Mode    string      `json:"mode"`
	Model   string      `json:"model"`
}

// handleChat — POST /api/chat
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Неверный запрос", err.Error())
		return
	}

	modelName := req.Model
	if modelName == "" {
		modelName = s.config.Models.Default
	}

	model, err := s.modelService.GetModel(modelName)
	if err != nil {
		writeError(w, http.StatusNotFound, "Модель не найдена", err.Error())
		return
	}

	user := GetUserFromContext(r.Context())
	username := "anonymous"
	if user != nil {
		username = user.Login
	}

	agentMode := req.AgentMode
	if agentMode == "" {
		agentMode = "fast"
	}

	// === Чтение прикреплённых файлов ===
	var filesContext string
	if len(req.Files) > 0 {
		var sb strings.Builder
		sb.WriteString("\n\n=== КОНТЕКСТ ИЗ ЗАГРУЖЕННЫХ ФАЙЛОВ ===\n")
		for _, f := range req.Files {
			content, err := s.readFileContent(f)
			if err != nil {
				sb.WriteString(fmt.Sprintf("\n--- Файл %s: ошибка чтения (%v) ---\n", f, err))
				continue
			}
			if len(content) > 100000 {
				content = content[:100000] + "\n...[обрезано]"
			}
			sb.WriteString(fmt.Sprintf("\n--- Файл: %s ---\n%s\n", f, content))
		}
		sb.WriteString("\n=== КОНЕЦ КОНТЕКСТА ===\n\n")
		filesContext = sb.String()
	}

	var prompt string
	var responseType string
	var parsedResponse interface{}
	var parseErr error

	start := time.Now()

	switch req.Mode {
	case "slides":
		prompt = prompts.Slides(req.Message+filesContext, agentMode, username)
		responseType = "slides"
		raw := model.Generate(prompt)
		parsedResponse, parseErr = modes.ParseSlides(raw)

	case "tables":
		prompt = prompts.Tables(req.Message+filesContext, agentMode, username)
		responseType = "table"
		raw := model.Generate(prompt)
		parsedResponse, parseErr = modes.ParseTable(raw)

	case "research":
		prompt = prompts.Research(req.Message+filesContext, agentMode, username)
		responseType = "research"
		raw := model.Generate(prompt)
		parsedResponse = modes.ParseResearch(raw)

	default:
		prompt = prompts.Chat(req.Message+filesContext, agentMode, username)
		responseType = "text"
		raw := model.Generate(prompt)
		parsedResponse = modes.ParseText(raw, nil)
	}

	duration := time.Since(start)

	if parseErr != nil {
		writeError(w, http.StatusInternalServerError, "Ошибка обработки ответа", parseErr.Error())
		return
	}

	// === Сохраняем в БД ===
	if req.ChatID != "" {
		msgs, _ := s.db.ListMessages(req.ChatID, 1)
		isFirst := len(msgs) == 0

		s.db.AddMessage(req.ChatID, "user", req.Message, modelName, req.Mode, 0, 0)

		if isFirst {
			s.db.UpdateChatTitle(req.ChatID, generateTitle(req.Message))
		}

		if textResp, ok := parsedResponse.(*modes.TextResponse); ok {
			s.db.AddMessage(req.ChatID, "assistant", textResp.Answer, modelName, req.Mode, 0, int(duration.Milliseconds()))
		}
	}

	writeJSON(w, http.StatusOK, ChatResponse{
		Type:    responseType,
		Content: parsedResponse,
		Mode:    req.Mode,
		Model:   modelName,
	})
}

// handleListChats — GET /api/chats
func (s *Server) handleListChats(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())

	var chats []db.Chat
	var err error

	if user != nil && user.Role == "admin" {
		chats, err = s.db.ListAllChats()
	} else if user != nil {
		chats, err = s.db.ListChatsByUser(user.UserID)
	} else {
		chats, err = s.db.ListAllChats()
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, "Ошибка получения чатов", err.Error())
		return
	}

	if chats == nil {
		chats = []db.Chat{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"chats": chats})
}

// handleCreateChat — POST /api/chats
func (s *Server) handleCreateChat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title string `json:"title"`
		Model string `json:"model"`
		Mode  string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Неверный запрос", err.Error())
		return
	}

	user := GetUserFromContext(r.Context())
	var userID int64 = 1
	if user != nil {
		userID = user.UserID
	}

	id := fmt.Sprintf("chat-%d-%d", userID, time.Now().UnixNano())

	if req.Title == "" {
		req.Title = "Новый чат"
	}
	if req.Model == "" {
		req.Model = s.config.Models.Default
	}
	if req.Mode == "" {
		req.Mode = "chat"
	}

	chat, err := s.db.CreateChat(id, userID, req.Title, req.Model, req.Mode)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Ошибка создания чата", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, chat)
}

// handleGetChat — GET /api/chats/{id}
func (s *Server) handleGetChat(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	chat, err := s.db.GetChat(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Чат не найден", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, chat)
}

// handleUpdateChat — PATCH /api/chats/{id}
func (s *Server) handleUpdateChat(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Неверный запрос", err.Error())
		return
	}

	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "Название не указано", "")
		return
	}

	if err := s.db.UpdateChatTitle(id, req.Title); err != nil {
		writeError(w, http.StatusInternalServerError, "Ошибка обновления", err.Error())
		return
	}

	chat, _ := s.db.GetChat(id)
	writeJSON(w, http.StatusOK, chat)
}

// handleDeleteChat — DELETE /api/chats/{id}
func (s *Server) handleDeleteChat(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := s.db.DeleteChat(id); err != nil {
		writeError(w, http.StatusInternalServerError, "Ошибка удаления", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// handleListMessages — GET /api/chats/{id}/messages
func (s *Server) handleListMessages(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	messages, err := s.db.ListMessages(id, 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Ошибка получения сообщений", err.Error())
		return
	}

	if messages == nil {
		messages = []db.Message{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"messages": messages})
}

// generateTitle — создаёт краткое название чата из первого сообщения
func generateTitle(text string) string {
	text = strings.TrimSpace(text)
	runes := []rune(text)
	if len(runes) > 60 {
		return string(runes[:60]) + "..."
	}
	if len(runes) == 0 {
		return "Новый чат"
	}
	return text
}
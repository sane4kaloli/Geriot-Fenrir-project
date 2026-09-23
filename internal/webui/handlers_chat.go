package webui

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"fenrir/internal/db"
	"fenrir/internal/webui/modes"

	"github.com/go-chi/chi/v5"
)
// ChatRequest вЂ” Р·Р°РїСЂРѕСЃ Рє С‡Р°С‚Сѓ
type ChatRequest struct {
	Message   string   `json:"message"`
	Mode      string   `json:"mode"`
	Model     string   `json:"model"`
	AgentMode string   `json:"agentMode"`
	ChatID    string   `json:"chatId,omitempty"`
	Files     []string `json:"files,omitempty"`
}

// ChatResponse вЂ” РѕС‚РІРµС‚ С‡Р°С‚Р°
type ChatResponse struct {
	Type    string      `json:"type"`
	Content interface{} `json:"content"`
	Mode    string      `json:"mode"`
	Model   string      `json:"model"`
}

// handleChat вЂ” POST /api/chat
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "РќРµРІРµСЂРЅС‹Р№ Р·Р°РїСЂРѕСЃ", err.Error())
		return
	}

	modelName := req.Model
	if modelName == "" {
		modelName = s.config.Models.Default
	}

	model, err := s.modelService.GetModel(modelName)
	if err != nil {
		writeError(w, http.StatusNotFound, "РњРѕРґРµР»СЊ РЅРµ РЅР°Р№РґРµРЅР°", err.Error())
		return
	}
	user := GetUserFromContext(r.Context())
	username := "anonymous"
	if user != nil {
		username = user.Login
	}

	// 🆕 Получаем агента из store
	agentID := req.AgentMode
	if agentID == "" {
		agentID = "default"
	}

	agentInfo, agentCfg, err := s.agentStore.GetAgent(agentID)
	if err != nil {
		agentInfo, agentCfg, _ = s.agentStore.GetAgent("default")
	}

	if agentCfg != nil && agentCfg.Model != "" {
		modelName = agentCfg.Model
		model, err = s.modelService.GetModel(modelName)
		if err != nil {
			writeError(w, http.StatusNotFound, "Модель агента не найдена", err.Error())
			return
		}
	}

	agentPrompt := "Ты — полезный ассистент."
	if agentCfg != nil && agentCfg.Prompt != "" {
		agentPrompt = agentCfg.Prompt
	}
	_ = agentInfo

	log.Printf("💬 Chat request: user=%s agent=%s model=%s", username, agentID, modelName)

	// === Р§С‚РµРЅРёРµ РїСЂРёРєСЂРµРїР»С‘РЅРЅС‹С… С„Р°Р№Р»РѕРІ ===
	var filesContext string
	if len(req.Files) > 0 {
		var sb strings.Builder
		sb.WriteString("\n\n=== РљРћРќРўР•РљРЎРў РР— Р—РђР“Р РЈР–Р•РќРќР«РҐ Р¤РђР™Р›РћР’ ===\n")
		for _, f := range req.Files {
			content, err := s.readFileContent(f)
			if err != nil {
				sb.WriteString(fmt.Sprintf("\n--- Р¤Р°Р№Р» %s: РѕС€РёР±РєР° С‡С‚РµРЅРёСЏ (%v) ---\n", f, err))
				continue
			}
			if len(content) > 100000 {
				content = content[:100000] + "\n...[РѕР±СЂРµР·Р°РЅРѕ]"
			}
			sb.WriteString(fmt.Sprintf("\n--- Р¤Р°Р№Р»: %s ---\n%s\n", f, content))
		}
		sb.WriteString("\n=== РљРћРќР•Р¦ РљРћРќРўР•РљРЎРўРђ ===\n\n")
		filesContext = sb.String()
	}

	var prompt string
	var responseType string
	var parsedResponse interface{}
	var parseErr error

	start := time.Now()

	switch req.Mode {
	case "slides":
		prompt = agentPrompt + "\n\n" + req.Message + filesContext
		responseType = "slides"
		raw := model.Generate(prompt)
		parsedResponse, parseErr = modes.ParseSlides(raw)

	case "tables":
		prompt = agentPrompt + "\n\n" + req.Message + filesContext
		responseType = "table"
		raw := model.Generate(prompt)
		parsedResponse, parseErr = modes.ParseTable(raw)

	case "research":
		prompt = agentPrompt + "\n\n" + req.Message + filesContext
		responseType = "research"
		raw := model.Generate(prompt)
		parsedResponse = modes.ParseResearch(raw)

	default:
		prompt = agentPrompt + "\n\nПользователь: " + req.Message + filesContext
		responseType = "text"
		raw := model.Generate(prompt)
		parsedResponse = modes.ParseText(raw, nil)
	}

	duration := time.Since(start)

	if parseErr != nil {
		writeError(w, http.StatusInternalServerError, "РћС€РёР±РєР° РѕР±СЂР°Р±РѕС‚РєРё РѕС‚РІРµС‚Р°", parseErr.Error())
		return
	}

	// === РЎРѕС…СЂР°РЅСЏРµРј РІ Р‘Р” ===
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

// handleListChats вЂ” GET /api/chats
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
		writeError(w, http.StatusInternalServerError, "РћС€РёР±РєР° РїРѕР»СѓС‡РµРЅРёСЏ С‡Р°С‚РѕРІ", err.Error())
		return
	}

	if chats == nil {
		chats = []db.Chat{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"chats": chats})
}

// handleCreateChat вЂ” POST /api/chats
func (s *Server) handleCreateChat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title string `json:"title"`
		Model string `json:"model"`
		Mode  string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "РќРµРІРµСЂРЅС‹Р№ Р·Р°РїСЂРѕСЃ", err.Error())
		return
	}

	user := GetUserFromContext(r.Context())
	var userID int64 = 1
	if user != nil {
		userID = user.UserID
	}

	id := fmt.Sprintf("chat-%d-%d", userID, time.Now().UnixNano())

	if req.Title == "" {
		req.Title = "РќРѕРІС‹Р№ С‡Р°С‚"
	}
	if req.Model == "" {
		req.Model = s.config.Models.Default
	}
	if req.Mode == "" {
		req.Mode = "chat"
	}

	chat, err := s.db.CreateChat(id, userID, req.Title, req.Model, req.Mode)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "РћС€РёР±РєР° СЃРѕР·РґР°РЅРёСЏ С‡Р°С‚Р°", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, chat)
}

// handleGetChat вЂ” GET /api/chats/{id}
func (s *Server) handleGetChat(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	chat, err := s.db.GetChat(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Р§Р°С‚ РЅРµ РЅР°Р№РґРµРЅ", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, chat)
}

// handleUpdateChat вЂ” PATCH /api/chats/{id}
func (s *Server) handleUpdateChat(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "РќРµРІРµСЂРЅС‹Р№ Р·Р°РїСЂРѕСЃ", err.Error())
		return
	}

	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "РќР°Р·РІР°РЅРёРµ РЅРµ СѓРєР°Р·Р°РЅРѕ", "")
		return
	}

	if err := s.db.UpdateChatTitle(id, req.Title); err != nil {
		writeError(w, http.StatusInternalServerError, "РћС€РёР±РєР° РѕР±РЅРѕРІР»РµРЅРёСЏ", err.Error())
		return
	}

	chat, _ := s.db.GetChat(id)
	writeJSON(w, http.StatusOK, chat)
}

// handleDeleteChat вЂ” DELETE /api/chats/{id}
func (s *Server) handleDeleteChat(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := s.db.DeleteChat(id); err != nil {
		writeError(w, http.StatusInternalServerError, "РћС€РёР±РєР° СѓРґР°Р»РµРЅРёСЏ", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// handleListMessages вЂ” GET /api/chats/{id}/messages
func (s *Server) handleListMessages(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	messages, err := s.db.ListMessages(id, 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "РћС€РёР±РєР° РїРѕР»СѓС‡РµРЅРёСЏ СЃРѕРѕР±С‰РµРЅРёР№", err.Error())
		return
	}

	if messages == nil {
		messages = []db.Message{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"messages": messages})
}

// generateTitle вЂ” СЃРѕР·РґР°С‘С‚ РєСЂР°С‚РєРѕРµ РЅР°Р·РІР°РЅРёРµ С‡Р°С‚Р° РёР· РїРµСЂРІРѕРіРѕ СЃРѕРѕР±С‰РµРЅРёСЏ
func generateTitle(text string) string {
	text = strings.TrimSpace(text)
	runes := []rune(text)
	if len(runes) > 60 {
		return string(runes[:60]) + "..."
	}
	if len(runes) == 0 {
		return "РќРѕРІС‹Р№ С‡Р°С‚"
	}
	return text
}

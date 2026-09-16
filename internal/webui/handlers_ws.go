package webui

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"fenrir/internal/webui/prompts"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // для локальной разработки
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// WSMessage — сообщение от клиента
type WSMessage struct {
	Type      string   `json:"type"`
	Message   string   `json:"message"`
	Mode      string   `json:"mode"`
	Model     string   `json:"model"`
	AgentMode string   `json:"agentMode"`
	ChatID    string   `json:"chatId"`
	Files     []string `json:"files,omitempty"`
}

// WSOutgoing — сообщение клиенту
type WSOutgoing struct {
	Type        string `json:"type"`
	Delta       string `json:"delta,omitempty"`
	FullContent string `json:"fullContent,omitempty"`
	Error       string `json:"error,omitempty"`
	ChatID      string `json:"chatId,omitempty"`
}

// handleWebSocket — GET /ws
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Проверка авторизации
	user := GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Требуется авторизация", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("❌ WebSocket upgrade: %v", err)
		return
	}
	defer conn.Close()

	// Мьютекс для безопасной записи в conn
	var writeMu sync.Mutex

	sendJSON := func(msg WSOutgoing) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.WriteJSON(msg)
	}

	log.Printf("🔗 WS подключён: %s (role: %s)", user.Login, user.Role)

	// Отправляем приветствие
	sendJSON(WSOutgoing{
		Type: "connected",
	})

	// Читаем сообщения
	for {
		_, rawMsg, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("⚠️ WS закрыт: %v", err)
			}
			break
		}

		var msg WSMessage
		if err := json.Unmarshal(rawMsg, &msg); err != nil {
			sendJSON(WSOutgoing{Type: "error", Error: "Неверный формат сообщения"})
			continue
		}

		switch msg.Type {
		case "ping":
			sendJSON(WSOutgoing{Type: "pong"})

		case "chat":
			// Обрабатываем чат в отдельной горутине (чтобы не блокировать чтение)
			go s.handleWSChat(sendJSON, msg, user)

		default:
			sendJSON(WSOutgoing{Type: "error", Error: "Неизвестный тип: " + msg.Type})
		}
	}

	log.Printf("🔌 WS отключён: %s", user.Login)
}

// handleWSChat — обработка чата через стриминг
func (s *Server) handleWSChat(send func(WSOutgoing) error, msg WSMessage, user *Session) {
	// Модель
	modelName := msg.Model
	if modelName == "" {
		modelName = s.config.Models.Default
	}

	model, err := s.modelService.GetModel(modelName)
	if err != nil {
		send(WSOutgoing{Type: "error", Error: "Модель не найдена: " + err.Error()})
		return
	}

	// Режим агента
	agentMode := msg.AgentMode
	if agentMode == "" {
		agentMode = "fast"
	}

	// Контекст из файлов
	var filesContext string
	if len(msg.Files) > 0 {
		var sb strings.Builder
		sb.WriteString("\n\n=== КОНТЕКСТ ИЗ ЗАГРУЖЕННЫХ ФАЙЛОВ ===\n")
		for _, f := range msg.Files {
			content, err := s.readFileContent(f)
			if err != nil {
				sb.WriteString("\n--- Файл " + f + ": ошибка чтения ---\n")
				continue
			}
			if len(content) > 100000 {
				content = content[:100000] + "\n...[обрезано]"
			}
			sb.WriteString("\n--- Файл: " + f + " ---\n" + content + "\n")
		}
		sb.WriteString("\n=== КОНЕЦ КОНТЕКСТА ===\n\n")
		filesContext = sb.String()
	}

	// Строим промпт
	var prompt string
	switch msg.Mode {
	case "slides":
		prompt = prompts.Slides(msg.Message+filesContext, agentMode, user.Login)
	case "tables":
		prompt = prompts.Tables(msg.Message+filesContext, agentMode, user.Login)
	case "research":
		prompt = prompts.Research(msg.Message+filesContext, agentMode, user.Login)
	default:
		prompt = prompts.Chat(msg.Message+filesContext, agentMode, user.Login)
	}

	// Стримим ответ
	start := time.Now()
	stream := model.StreamGenerate(prompt)

	var fullContent strings.Builder
	for chunk := range stream {
		fullContent.WriteString(chunk)
		if err := send(WSOutgoing{
			Type:  "chat.delta",
			Delta: chunk,
		}); err != nil {
			log.Printf("⚠️ Ошибка отправки в WS: %v", err)
			return
		}
	}

	// Завершаем
	duration := time.Since(start)
	finalText := fullContent.String()

	send(WSOutgoing{
		Type:        "chat.done",
		FullContent: finalText,
	})

	// Сохраняем в БД
	if msg.ChatID != "" {
		msgs, _ := s.db.ListMessages(msg.ChatID, 1)
		isFirst := len(msgs) == 0

		s.db.AddMessage(msg.ChatID, "user", msg.Message, modelName, msg.Mode, 0, 0)

		if isFirst {
			s.db.UpdateChatTitle(msg.ChatID, generateTitle(msg.Message))
		}

		s.db.AddMessage(msg.ChatID, "assistant", finalText, modelName, msg.Mode, 0, int(duration.Milliseconds()))
	}

	log.Printf("✅ Стриминг завершён: %d символов за %v", fullContent.Len(), duration)
}
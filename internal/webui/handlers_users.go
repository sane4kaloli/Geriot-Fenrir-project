package webui

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/go-chi/chi/v5"
)

// handleRegister — POST /api/register (открытая регистрация)
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Неверный запрос", err.Error())
		return
	}

	req.Login = strings.TrimSpace(req.Login)
	if len(req.Login) < 3 {
		writeError(w, http.StatusBadRequest, "Логин должен быть минимум 3 символа", "")
		return
	}
	if len(req.Password) < 4 {
		writeError(w, http.StatusBadRequest, "Пароль должен быть минимум 4 символа", "")
		return
	}

	// Проверяем, что логин не занят
	if _, err := s.db.GetUserByLogin(req.Login); err == nil {
		writeError(w, http.StatusConflict, "Логин уже занят", "Выберите другой логин")
		return
	}

	// Хешируем пароль
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Ошибка хеширования", err.Error())
		return
	}

	// Создаём пользователя с ролью user
	user, err := s.db.CreateUser(req.Login, string(hash), "user", req.Login)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Ошибка создания", err.Error())
		return
	}

	// Сразу авторизуем
	sessionID := generateSessionID()
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	if err := s.db.CreateSession(sessionID, user.ID, expiresAt); err != nil {
		writeError(w, http.StatusInternalServerError, "Ошибка создания сессии", err.Error())
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "fenrir_session",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   7 * 24 * 3600,
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":        true,
		"login":     user.Login,
		"role":      user.Role,
		"scopes":    getScopesForRole(user.Role),
		"expiresAt": expiresAt,
	})
}

// handleListUsers — GET /api/users (только admin)
func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.db.ListUsers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Ошибка получения", err.Error())
		return
	}
	type SafeUser struct {
		ID          int64     `json:"id"`
		Login       string    `json:"login"`
		Role        string    `json:"role"`
		DisplayName string    `json:"displayName"`
		CreatedAt   time.Time `json:"createdAt"`
	}
	var result []SafeUser
	for _, u := range users {
		result = append(result, SafeUser{
			ID: u.ID, Login: u.Login, Role: u.Role,
			DisplayName: u.DisplayName, CreatedAt: u.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"users": result})
}

// handleDeleteUser — DELETE /api/users/{id} (только admin)
func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	var id int64
	var ok bool
	for _, c := range idStr {
		if c < '0' || c > '9' {
			ok = false
			break
		}
		id = id*10 + int64(c-'0')
		ok = true
	}
	if !ok || id == 0 {
		writeError(w, http.StatusBadRequest, "Неверный ID", "")
		return
	}

	// Нельзя удалить себя
	currentUser := GetUserFromContext(r.Context())
	if currentUser != nil && currentUser.UserID == id {
		writeError(w, http.StatusBadRequest, "Нельзя удалить себя", "")
		return
	}

	// Нельзя удалить последнего admin'а
	target, err := s.db.GetUserByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Пользователь не найден", "")
		return
	}
	if target.Role == "admin" {
		users, _ := s.db.ListUsers()
		adminCount := 0
		for _, u := range users {
			if u.Role == "admin" {
				adminCount++
			}
		}
		if adminCount <= 1 {
			writeError(w, http.StatusBadRequest, "Нельзя удалить последнего администратора", "")
			return
		}
	}

	if err := s.db.DeleteUser(id); err != nil {
		writeError(w, http.StatusInternalServerError, "Ошибка удаления", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}
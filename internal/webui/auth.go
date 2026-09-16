package webui

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Session — сессия пользователя
type Session struct {
	ID        string
	UserID    int64
	Login     string
	Role      string
	Scopes    []string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type contextKey string

const userContextKey contextKey = "user"

func GetUserFromContext(ctx context.Context) *Session {
	if user, ok := ctx.Value(userContextKey).(*Session); ok {
		return user
	}
	return nil
}

func SetUserToContext(ctx context.Context, user *Session) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

func generateSessionID() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func getScopesForRole(role string) []string {
	scopes := map[string][]string{
		"admin": {
			"web.read", "web.write",
			"web.admin.skills", "web.admin.gateway",
			"web.admin.config", "web.admin.backup",
			"web.admin.logs", "web.admin.agents", "web.admin.users",
		},
		"user": {
			"web.read", "web.write",
		},
	}
	return scopes[role]
}

// AuthMiddleware — достаёт сессию из cookie, кладёт в контекст
func (s *Server) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("fenrir_session")
		if err == nil && cookie != nil {
			dbSession, err := s.db.GetSession(cookie.Value)
			if err == nil && dbSession != nil {
				user, err := s.db.GetUserByID(dbSession.UserID)
				if err == nil {
					ctx := SetUserToContext(r.Context(), &Session{
						ID:        dbSession.ID,
						UserID:    dbSession.UserID,
						Login:     user.Login,
						Role:      user.Role,
						Scopes:    getScopesForRole(user.Role),
						CreatedAt: dbSession.CreatedAt,
						ExpiresAt: dbSession.ExpiresAt,
					})
					r = r.WithContext(ctx)
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAuth — требует авторизации
func (s *Server) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetUserFromContext(r.Context())
		if user == nil {
			writeError(w, http.StatusUnauthorized, "Требуется авторизация", "Войдите в систему")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireRole — требует конкретной роли
func (s *Server) RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := GetUserFromContext(r.Context())
			if user == nil || user.Role != role {
				writeError(w, http.StatusForbidden, "Доступ запрещён", "Недостаточно прав")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireScope — требует конкретного scope
func (s *Server) RequireScope(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := GetUserFromContext(r.Context())
			if user == nil {
				writeError(w, http.StatusUnauthorized, "Требуется авторизация", "")
				return
			}
			for _, sc := range user.Scopes {
				if sc == scope {
					next.ServeHTTP(w, r)
					return
				}
			}
			writeError(w, http.StatusForbidden, "Доступ запрещён", "")
		})
	}
}

// handleLogin — POST /api/login
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Неверный запрос", err.Error())
		return
	}

	if req.Login == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "Укажите логин и пароль", "")
		return
	}

	user, err := s.db.GetUserByLogin(req.Login)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Неверный логин или пароль", "")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "Неверный логин или пароль", "")
		return
	}

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

// handleLogout — POST /api/logout
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("fenrir_session")
	if err == nil && cookie != nil {
		s.db.DeleteSession(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "fenrir_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// handleGetMe — GET /api/me
func (s *Server) handleGetMe(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"authenticated": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"authenticated": true,
		"login":         user.Login,
		"role":          user.Role,
		"scopes":        user.Scopes,
	})
}
package webui

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"fenrir/internal/agents"
	"fenrir/internal/config"
	"fenrir/internal/db"
	"fenrir/internal/webui/models"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

//go:embed static/*
var staticFS embed.FS

// Server вЂ” HTTP-СЃРµСЂРІРµСЂ Fenrir
type Server struct {
	config       *config.Config
	db           *db.DB
	modelService *models.Service
	agentStore   *agents.Store
}

// NewServer вЂ” СЃРѕР·РґР°С‘С‚ РЅРѕРІС‹Р№ СЃРµСЂРІРµСЂ
func NewServer(cfg *config.Config, database *db.DB) *Server {
	return &Server{
		config:       cfg,
		db:           database,
		modelService: models.NewService(cfg),
		agentStore:   agents.NewStore(cfg),
	}
}

// Router вЂ” РІРѕР·РІСЂР°С‰Р°РµС‚ РЅР°СЃС‚СЂРѕРµРЅРЅС‹Р№ chi-СЂРѕСѓС‚РµСЂ
func (s *Server) Router() *chi.Mux {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(180 * time.Second))

	// CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// РђСѓС‚РµРЅС‚РёС„РёРєР°С†РёСЏ
	r.Use(s.AuthMiddleware)

	// РЎС‚Р°С‚РёРєР°
	staticSub, _ := fs.Sub(staticFS, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))

	// === API ===
	r.Route("/api", func(r chi.Router) {
		// === РњРµС‚Р° / Auth ===
		r.Post("/register", s.handleRegister)
		r.Post("/login", s.handleLogin)
		r.Post("/logout", s.handleLogout)
		r.Get("/me", s.handleGetMe)
		r.Get("/health", s.handleHealth)
		r.Get("/models", s.handleListModels)

		// Провайдеры и агенты
		r.Get("/providers", s.handleListProviders)
		r.Get("/providers/{id}/agents", s.handleListProviderAgents)
		r.Get("/agents", s.handleListAgents)
		r.Get("/agents/{id}", s.handleGetAgent)

		// === Р¤Р°Р№Р»С‹ (upload / list / delete / raw) ===
		r.Route("/files", func(r chi.Router) {
			r.Use(s.RequireAuth)
			r.Post("/upload", s.handleUpload)
			r.Get("/uploads", s.handleListUploads)
			r.Delete("/uploads/{name}", s.handleDeleteUpload)
			r.Get("/raw", s.handleGetRawFile)
		})

		// === Workspace (РїСЂРѕСЃРјРѕС‚СЂ) ===
		r.Route("/workspace", func(r chi.Router) {
			r.Use(s.RequireAuth)
			r.Get("/", s.handleListWorkspace)
			r.Get("/file", s.handleWorkspaceFile)
			r.Get("/download", s.handleWorkspaceDownload)
		})

		// === Р§Р°С‚ ===
		r.Group(func(r chi.Router) {
			r.Use(s.RequireAuth)
			r.Post("/chat", s.handleChat)
		})

		// === Р§Р°С‚С‹ (РёСЃС‚РѕСЂРёСЏ) ===
		r.Route("/chats", func(r chi.Router) {
			r.Use(s.RequireAuth)
			r.Get("/", s.handleListChats)
			r.Post("/", s.handleCreateChat)
			r.Get("/{id}", s.handleGetChat)
			r.Patch("/{id}", s.handleUpdateChat)
			r.Delete("/{id}", s.handleDeleteChat)
			r.Get("/{id}/messages", s.handleListMessages)
		})

		// === РџРѕР»СЊР·РѕРІР°С‚РµР»Рё (С‚РѕР»СЊРєРѕ admin) ===
		r.Route("/users", func(r chi.Router) {
			r.Use(s.RequireAuth)
			r.Use(s.RequireRole("admin"))
			r.Get("/", s.handleListUsers)
			r.Delete("/{id}", s.handleDeleteUser)
		})

		// === РђРґРјРёРЅРєР° ===
		r.Route("/admin", func(r chi.Router) {
			r.Use(s.RequireAuth)
			r.Use(s.RequireRole("admin"))

			// РЎРєРёР»Р»С‹
			r.Get("/skills", s.handleListSkills)
			r.Post("/skills/{name}/enable", s.handleEnableSkill)
			r.Post("/skills/{name}/disable", s.handleDisableSkill)
			r.Post("/skills/reload", s.handleReloadSkills)

			// РќР°СЃС‚СЂРѕР№РєРё
			r.Get("/settings", s.handleListSettings)
			r.Post("/settings", s.handleSetSetting)

			// РђСѓРґРёС‚
			r.Get("/audit", s.handleListAudit)

			// Р›РѕРіРё
			r.Get("/logs/files", s.handleListLogs)
			r.Get("/logs", s.handleGetLog)
		})
	})

	// WebSocket
	r.Get("/ws", s.handleWebSocket)

	// SPA fallback
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api") || strings.HasPrefix(r.URL.Path, "/ws") {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		index, err := staticFS.ReadFile("static/index.html")
		if err != nil {
			http.Error(w, "index.html not found", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		w.Write(index)
	})

	log.Println("вњ… HTTP-СЂРѕСѓС‚РµСЂ РЅР°СЃС‚СЂРѕРµРЅ")
	return r
}

// === РҐРµР»РїРµСЂС‹ ===

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string, hint string) {
	writeJSON(w, status, map[string]interface{}{
		"code":    status,
		"message": message,
		"hint":    hint,
	})
}

// === РџСЂРѕСЃС‚С‹Рµ С…РµРЅРґР»РµСЂС‹ ===

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func (s *Server) handleListModels(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"models":  s.modelService.ListModels(),
		"default": s.config.Models.Default,
	})
}



package webui

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// handleListSkills — GET /api/admin/skills
func (s *Server) handleListSkills(w http.ResponseWriter, r *http.Request) {
	skills, err := s.db.ListSkills()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Ошибка получения скиллов", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"skills": skills})
}

// handleEnableSkill — POST /api/admin/skills/{name}/enable
func (s *Server) handleEnableSkill(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "Имя скилла не указано", "")
		return
	}

	if err := s.db.SetSkillEnabled(name, true); err != nil {
		writeError(w, http.StatusInternalServerError, "Ошибка обновления", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"skill":   name,
		"enabled": true,
	})
}

// handleDisableSkill — POST /api/admin/skills/{name}/disable
func (s *Server) handleDisableSkill(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "Имя скилла не указано", "")
		return
	}

	if err := s.db.SetSkillEnabled(name, false); err != nil {
		writeError(w, http.StatusInternalServerError, "Ошибка обновления", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"skill":   name,
		"enabled": false,
	})
}

// handleReloadSkills — POST /api/admin/skills/reload
func (s *Server) handleReloadSkills(w http.ResponseWriter, r *http.Request) {
	skills, _ := s.db.ListSkills()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":     true,
		"loaded": len(skills),
		"skills": skills,
	})
}

// handleListSettings — GET /api/admin/settings
func (s *Server) handleListSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.db.GetAllSettings()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Ошибка получения настроек", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"settings": settings})
}

// handleSetSetting — POST /api/admin/settings
func (s *Server) handleSetSetting(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Неверный запрос", err.Error())
		return
	}

	if err := s.db.SetSetting(req.Key, req.Value); err != nil {
		writeError(w, http.StatusInternalServerError, "Ошибка сохранения", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// handleListAudit — GET /api/admin/audit
func (s *Server) handleListAudit(w http.ResponseWriter, r *http.Request) {
	logs, err := s.db.ListAudit(100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Ошибка получения аудита", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"logs": logs})
}
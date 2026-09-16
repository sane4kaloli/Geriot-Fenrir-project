package webui

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

// WorkspaceItem — элемент дерева файлов
type WorkspaceItem struct {
	Name     string `json:"name"`
	Path     string `json:"path"`      // относительно workspace
	IsDir    bool   `json:"isDir"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
}

// handleListWorkspace — GET /api/workspace?path=...
func (s *Server) handleListWorkspace(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Требуется авторизация", "")
		return
	}

	relPath := r.URL.Query().Get("path")
	if relPath == "" {
		relPath = "."
	}

	// Безопасный резолв
	fullPath, err := s.resolveWorkspacePath(user, relPath)
	if err != nil {
		writeError(w, http.StatusForbidden, "Доступ запрещён", err.Error())
		return
	}

	// Проверка существования
	info, err := os.Stat(fullPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "Путь не найден", err.Error())
		return
	}
	if !info.IsDir() {
		writeError(w, http.StatusBadRequest, "Это не папка", "")
		return
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Ошибка чтения папки", err.Error())
		return
	}

	var items []WorkspaceItem
	for _, entry := range entries {
		entryInfo, _ := entry.Info()
		itemPath := filepath.Join(relPath, entry.Name())
		if relPath == "." {
			itemPath = entry.Name()
		}
		itemPath = filepath.ToSlash(itemPath)

		items = append(items, WorkspaceItem{
			Name:     entry.Name(),
			Path:     itemPath,
			IsDir:    entry.IsDir(),
			Size:     entryInfo.Size(),
			Modified: entryInfo.ModTime().Format("2006-01-02 15:04:05"),
		})
	}

	// Сортировка: сначала папки, потом файлы, внутри — по имени
	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return items[i].Name < items[j].Name
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"path":  relPath,
		"items": items,
	})
}

// handleWorkspaceFile — GET /api/workspace/file?path=...
func (s *Server) handleWorkspaceFile(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Требуется авторизация", "")
		return
	}

	relPath := r.URL.Query().Get("path")
	if relPath == "" {
		writeError(w, http.StatusBadRequest, "Путь не указан", "")
		return
	}

	fullPath, err := s.resolveWorkspacePath(user, relPath)
	if err != nil {
		writeError(w, http.StatusForbidden, "Доступ запрещён", err.Error())
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "Файл не найден", err.Error())
		return
	}
	if info.IsDir() {
		writeError(w, http.StatusBadRequest, "Это папка", "")
		return
	}

	// Лимит для просмотра — 1 МБ
	const maxViewSize = 1024 * 1024
	if info.Size() > maxViewSize {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"path":   relPath,
			"name":   filepath.Base(relPath),
			"size":   info.Size(),
			"binary": true,
			"text":   fmt.Sprintf("⚠️ Файл слишком большой для просмотра (%d байт). Скачайте его.", info.Size()),
		})
		return
	}

	// Проверяем, текстовый ли
	ext := strings.ToLower(filepath.Ext(relPath))
	isText := allowedExts[ext] || isTextExt(ext)

	if !isText {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"path":   relPath,
			"name":   filepath.Base(relPath),
			"size":   info.Size(),
			"binary": true,
			"text":   "⚠️ Бинарный файл. Скачайте для просмотра.",
		})
		return
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Ошибка чтения", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"path":     relPath,
		"name":     filepath.Base(relPath),
		"size":     info.Size(),
		"modified": info.ModTime().Format("2006-01-02 15:04:05"),
		"binary":   false,
		"text":     string(data),
	})
}

// handleWorkspaceDownload — GET /api/workspace/download?path=...
func (s *Server) handleWorkspaceDownload(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Требуется авторизация", "")
		return
	}

	relPath := r.URL.Query().Get("path")
	if relPath == "" {
		writeError(w, http.StatusBadRequest, "Путь не указан", "")
		return
	}

	fullPath, err := s.resolveWorkspacePath(user, relPath)
	if err != nil {
		writeError(w, http.StatusForbidden, "Доступ запрещён", err.Error())
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename=\""+filepath.Base(relPath)+"\"")
	http.ServeFile(w, r, fullPath)
}

// resolveWorkspacePath — безопасно резолвит путь внутри workspace
func (s *Server) resolveWorkspacePath(user *Session, relPath string) (string, error) {
	// Очистка пути
	clean := filepath.Clean(relPath)
	clean = filepath.ToSlash(clean)

	// Запрет выхода за workspace
	if strings.Contains(clean, "..") {
		return "", fmt.Errorf("выход за пределы workspace")
	}

	// Если user — ограничиваем его папкой uploads/{userId}/
	if user.Role != "admin" {
		prefix := fmt.Sprintf("uploads/%d", user.UserID)
		if clean == "." || clean == "" {
			clean = prefix
		} else if !strings.HasPrefix(clean, prefix) {
			return "", fmt.Errorf("доступ только к своим файлам")
		}
	}

	fullPath := filepath.Join(s.config.Agents.Workspace, clean)
	absWorkspace, _ := filepath.Abs(s.config.Agents.Workspace)
	absFile, _ := filepath.Abs(fullPath)

	if !strings.HasPrefix(absFile, absWorkspace) {
		return "", fmt.Errorf("путь вне workspace")
	}
	return fullPath, nil
}

// isTextExt — дополнительные текстовые расширения
func isTextExt(ext string) bool {
	textExts := map[string]bool{
		"": true,
		".gitignore": true, ".gitattributes": true, ".editorconfig": true,
		".env": true, ".env.local": true, ".dockerignore": true,
		".npmrc": true, ".nvmrc": true, ".babelrc": true, ".eslintrc": true,
		".prettierrc": true, ".htaccess": true, ".gitkeep": true,
	}
	return textExts[ext]
}

// Заглушка, чтобы strconv не был неиспользован
var _ = strconv.Itoa
var _ = chi.URLParam
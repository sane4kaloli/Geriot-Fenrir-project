package webui

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// Разрешённые расширения
var allowedExts = map[string]bool{
	".txt": true, ".text": true, ".md": true, ".markdown": true,
	".rst": true, ".adoc": true, ".asciidoc": true, ".org": true,
	".tex": true, ".bib": true, ".rtf": true, ".wiki": true, ".textile": true,
	".json": true, ".jsonl": true, ".ndjson": true, ".geojson": true,
	".csv": true, ".tsv": true, ".psv": true,
	".yaml": true, ".yml": true,
	".toml": true, ".ini": true, ".cfg": true, ".conf": true,
	".properties": true, ".env": true, ".envrc": true,
	".xml": true, ".html": true, ".htm": true, ".xhtml": true,
	".svg": true, ".css": true, ".scss": true, ".sass": true,
	".less": true, ".styl": true, ".stylus": true,
	".c": true, ".h": true, ".cpp": true, ".cxx": true, ".cc": true,
	".hpp": true, ".hh": true, ".hxx": true, ".ino": true,
	".cs": true, ".java": true, ".kt": true, ".kts": true, ".scala": true,
	".groovy": true, ".gradle": true, ".clj": true, ".cljs": true, ".cljc": true,
	".js": true, ".mjs": true, ".cjs": true, ".jsx": true,
	".ts": true, ".tsx": true, ".vue": true, ".svelte": true,
	".go": true, ".rs": true, ".swift": true, ".m": true, ".mm": true,
	".zig": true, ".nim": true, ".d": true, ".v": true, ".vala": true,
	".py": true, ".pyw": true, ".pyi": true, ".ipynb": true,
	".rb": true, ".erb": true, ".rake": true, ".gemspec": true,
	".php": true, ".phtml": true, ".phps": true,
	".pl": true, ".pm": true, ".t": true,
	".lua": true, ".r": true, ".rmd": true, ".rmarkdown": true,
	".jl": true, ".ex": true, ".exs": true, ".erl": true, ".hrl": true,
	".hs": true, ".lhs": true, ".ml": true, ".mli": true,
	".fs": true, ".fsx": true, ".fsi": true, ".vb": true,
	".dart": true, ".pas": true, ".pp": true, ".f": true,
	".f90": true, ".f95": true, ".for": true, ".asm": true, ".s": true,
	".sh": true, ".bash": true, ".zsh": true, ".fish": true,
	".csh": true, ".ksh": true, ".ps1": true, ".psm1": true, ".psd1": true,
	".bat": true, ".cmd": true, ".awk": true, ".sed": true,
	".sql": true, ".ddl": true, ".dml": true, ".plsql": true,
	".psql": true, ".mysql": true, ".sqlite": true,
	".log": true, ".out": true, ".err": true, ".trace": true, ".syslog": true,
	".dockerfile": true, ".containerfile": true,
	".makefile": true, ".mk": true, ".cmake": true,
	".tf": true, ".tfvars": true, ".hcl": true,
	".nix": true, ".cue": true, ".jsonnet": true,
	".editorconfig": true, ".gitignore": true, ".gitattributes": true,
	".gitmodules": true, ".dockerignore": true, ".npmrc": true,
	".nvmrc": true, ".babelrc": true, ".eslintrc": true,
	".prettierrc": true, ".stylelintrc": true, ".htaccess": true,
	".diff": true, ".patch": true,
	".srt": true, ".vtt": true, ".ass": true, ".ssa": true, ".sub": true,
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
	".webp": true, ".bmp": true, ".tiff": true, ".tif": true,
	".ico": true, ".heic": true, ".heif": true, ".avif": true,
}

const maxUploadSize = 10 * 1024 * 1024 // 10 МБ

const UploadsDir = "uploads"

// getUserUploadDir — папка пользователя
func (s *Server) getUserUploadDir(userID int64) string {
	return filepath.Join(s.config.Agents.Workspace, UploadsDir, strconv.FormatInt(userID, 10))
}

// handleUpload — POST /api/files/upload
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Требуется авторизация", "")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize+1024)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		writeError(w, http.StatusBadRequest, "Файл слишком большой (макс 10 МБ)", err.Error())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "Файл не получен", err.Error())
		return
	}
	defer file.Close()

	if header.Size > maxUploadSize {
		writeError(w, http.StatusBadRequest, "Файл слишком большой (макс 10 МБ)", "")
		return
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedExts[ext] {
		writeError(w, http.StatusBadRequest, "Недопустимый тип файла", "")
		return
	}

	safeName := sanitizeFilename(header.Filename)
	if safeName == "" {
		safeName = fmt.Sprintf("file-%d%s", time.Now().UnixNano(), ext)
	}

	userDir := s.getUserUploadDir(user.UserID)
	if err := os.MkdirAll(userDir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "Ошибка создания папки", err.Error())
		return
	}

	finalName := safeName
	fullPath := filepath.Join(userDir, finalName)
	if _, err := os.Stat(fullPath); err == nil {
		base := strings.TrimSuffix(safeName, ext)
		finalName = fmt.Sprintf("%s-%d%s", base, time.Now().Unix(), ext)
		fullPath = filepath.Join(userDir, finalName)
	}

	out, err := os.Create(fullPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Ошибка создания файла", err.Error())
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		writeError(w, http.StatusInternalServerError, "Ошибка сохранения", err.Error())
		return
	}

	relPath := filepath.Join(UploadsDir, strconv.FormatInt(user.UserID, 10), finalName)
	relPath = filepath.ToSlash(relPath)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":       true,
		"path":     relPath,
		"name":     finalName,
		"size":     header.Size,
		"uploaded": time.Now().Format(time.RFC3339),
	})
}

// handleListUploads — GET /api/files/uploads
func (s *Server) handleListUploads(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Требуется авторизация", "")
		return
	}

	var files []map[string]interface{}

	if user.Role == "admin" {
		uploadsPath := filepath.Join(s.config.Agents.Workspace, UploadsDir)
		entries, _ := os.ReadDir(uploadsPath)
		for _, userEntry := range entries {
			if !userEntry.IsDir() {
				continue
			}
			userID := userEntry.Name()
			userDir := filepath.Join(uploadsPath, userID)
			fileEntries, _ := os.ReadDir(userDir)
			for _, e := range fileEntries {
				if e.IsDir() {
					continue
				}
				info, _ := e.Info()
				files = append(files, map[string]interface{}{
					"name":    e.Name(),
					"size":    info.Size(),
					"modTime": info.ModTime(),
					"path":    UploadsDir + "/" + userID + "/" + e.Name(),
					"userId":  userID,
				})
			}
		}
	} else {
		userDir := s.getUserUploadDir(user.UserID)
		entries, err := os.ReadDir(userDir)
		if err != nil {
			if os.IsNotExist(err) {
				writeJSON(w, http.StatusOK, map[string]interface{}{"files": []interface{}{}})
				return
			}
			writeError(w, http.StatusInternalServerError, "Ошибка чтения", err.Error())
			return
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			info, _ := e.Info()
			files = append(files, map[string]interface{}{
				"name":    e.Name(),
				"size":    info.Size(),
				"modTime": info.ModTime(),
				"path":    UploadsDir + "/" + strconv.FormatInt(user.UserID, 10) + "/" + e.Name(),
			})
		}
	}

	if files == nil {
		files = []map[string]interface{}{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"files": files})
}

// handleDeleteUpload — DELETE /api/files/uploads/{name}
func (s *Server) handleDeleteUpload(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Требуется авторизация", "")
		return
	}

	name := chi.URLParam(r, "name")
	safeName := sanitizeFilename(name)
	if safeName == "" {
		writeError(w, http.StatusBadRequest, "Неверное имя файла", "")
		return
	}

	fullPath := filepath.Join(s.getUserUploadDir(user.UserID), safeName)

	absWorkspace, _ := filepath.Abs(s.config.Agents.Workspace)
	absFile, _ := filepath.Abs(fullPath)
	if !strings.HasPrefix(absFile, absWorkspace) {
		writeError(w, http.StatusBadRequest, "Некорректный путь", "")
		return
	}

	if err := os.Remove(fullPath); err != nil {
		writeError(w, http.StatusInternalServerError, "Ошибка удаления", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// handleGetRawFile — GET /api/files/raw?path=uploads/2/photo.jpg
func (s *Server) handleGetRawFile(w http.ResponseWriter, r *http.Request) {
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

	if user.Role != "admin" {
		expectedPrefix := fmt.Sprintf("uploads/%d/", user.UserID)
		if !strings.HasPrefix(relPath, expectedPrefix) {
			writeError(w, http.StatusForbidden, "Доступ запрещён", "")
			return
		}
	}

	fullPath := filepath.Join(s.config.Agents.Workspace, relPath)
	absWorkspace, _ := filepath.Abs(s.config.Agents.Workspace)
	absFile, _ := filepath.Abs(fullPath)
	if !strings.HasPrefix(absFile, absWorkspace) {
		writeError(w, http.StatusBadRequest, "Некорректный путь", "")
		return
	}

	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeFile(w, r, fullPath)
}

// sanitizeFilename — убирает опасные символы
func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' || r == ' ' ||
			(r >= 'а' && r <= 'я') || (r >= 'А' && r <= 'Я') {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// readFileContent — читает файл из workspace (для контекста чата)
func (s *Server) readFileContent(relPath string) (string, error) {
	fullPath := filepath.Join(s.config.Agents.Workspace, relPath)
	absWorkspace, _ := filepath.Abs(s.config.Agents.Workspace)
	absFile, _ := filepath.Abs(fullPath)

	if !strings.HasPrefix(absFile, absWorkspace) {
		return "", fmt.Errorf("путь вне workspace")
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		return "", err
	}
	if info.Size() > maxUploadSize {
		return "", fmt.Errorf("файл слишком большой")
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
package webui

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"net/http"
)

// logDir — папка с логами
func (s *Server) logDir() string {
	return filepath.Join(s.config.Agents.Workspace, "logs")
}

// handleListLogs — GET /api/admin/logs/files
func (s *Server) handleListLogs(w http.ResponseWriter, r *http.Request) {
	dir := s.logDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusOK, map[string]interface{}{"files": []interface{}{}})
			return
		}
		writeError(w, http.StatusInternalServerError, "Ошибка чтения папки логов", err.Error())
		return
	}

	type LogFile struct {
		Name     string `json:"name"`
		Size     int64  `json:"size"`
		Modified string `json:"modified"`
	}
	var files []LogFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		info, _ := e.Info()
		files = append(files, LogFile{
			Name:     e.Name(),
			Size:     info.Size(),
			Modified: info.ModTime().Format("2006-01-02 15:04:05"),
		})
	}

	// Сортируем по дате изменения (свежие — выше)
	sort.Slice(files, func(i, j int) bool {
		return files[i].Modified > files[j].Modified
	})

	if files == nil {
		files = []LogFile{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"files": files})
}

// handleGetLog — GET /api/admin/logs?file=fenrir.log&lines=200
func (s *Server) handleGetLog(w http.ResponseWriter, r *http.Request) {
	fileName := r.URL.Query().Get("file")
	if fileName == "" {
		fileName = "fenrir.log"
	}

	// Защита от path traversal
	fileName = filepath.Base(fileName)
	if !strings.HasSuffix(fileName, ".log") {
		writeError(w, http.StatusBadRequest, "Можно читать только .log файлы", "")
		return
	}

	linesParam := r.URL.Query().Get("lines")
	maxLines := 200
	if linesParam != "" {
		if n, err := strconv.Atoi(linesParam); err == nil && n > 0 && n <= 10000 {
			maxLines = n
		}
	}

	fullPath := filepath.Join(s.logDir(), fileName)
	absDir, _ := filepath.Abs(s.logDir())
	absFile, _ := filepath.Abs(fullPath)
	if !strings.HasPrefix(absFile, absDir) {
		writeError(w, http.StatusBadRequest, "Некорректный путь", "")
		return
	}

	file, err := os.Open(fullPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "Лог-файл не найден", err.Error())
		return
	}
	defer file.Close()

	// Читаем последние N строк
	var lines []string
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // до 1 МБ на строку
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	// Оставляем последние maxLines
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}

	info, _ := file.Stat()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"file":     fileName,
		"lines":    lines,
		"total":    len(lines),
		"size":     info.Size(),
		"modified": info.ModTime().Format("2006-01-02 15:04:05"),
	})
}
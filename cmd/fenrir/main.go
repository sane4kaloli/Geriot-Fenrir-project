package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"fenrir/internal/config"
	"fenrir/internal/db"
	"fenrir/internal/webui"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	// === 1. Настраиваем логирование в файл ===
	logsDir := filepath.Join("workspace", "logs")
	os.MkdirAll(logsDir, 0755)

	logFile, err := os.OpenFile(
		filepath.Join(logsDir, "fenrir.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644,
	)
	if err != nil {
		log.Fatalf("❌ Не удалось открыть лог-файл: %v", err)
	}
	defer logFile.Close()

	// Дублируем вывод: и в консоль, и в файл
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(multiWriter)
	log.SetFlags(log.LstdFlags)

	log.Println("═══════════════════════════════════════════")
	log.Println("🚀 Запуск Fenrir AI Web UI")
	log.Println("═══════════════════════════════════════════")

	// === 2. Конфиг ===
	cfg, err := config.Load("fenrir.json")
	if err != nil {
		log.Fatalf("❌ Ошибка конфига: %v", err)
	}
	log.Printf("✅ Конфиг загружен: %s:%d", cfg.Gateway.Bind, cfg.Gateway.Port)

	// === 3. База данных ===
	database, err := db.New(cfg.Database.Path)
	if err != nil {
		log.Fatalf("❌ Ошибка БД: %v", err)
	}
	defer database.Close()

	// === 4. Создаём дефолтного admin ===
	users, _ := database.ListUsers()
	if len(users) == 0 {
		hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
		database.CreateUser("admin", string(hash), "admin", "Administrator")
		log.Println("👤 Создан admin (логин: admin, пароль: admin)")
	}

	// === 5. HTTP-сервер ===
	server := webui.NewServer(cfg, database)

	srv := &http.Server{
		Addr:         cfg.GetWebUIAddr(),
		Handler:      server.Router(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 180 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// === 6. Запуск в фоне ===
	go func() {
		log.Printf("🌐 Web UI доступен: http://%s", cfg.GetWebUIAddr())
		log.Println("📦 Нажмите Ctrl+C для остановки")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Ошибка сервера: %v", err)
		}
	}()

	// === 7. Graceful shutdown ===
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Остановка сервера...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("❌ Ошибка остановки: %v", err)
	}
	log.Println("✅ Сервер остановлен")
}
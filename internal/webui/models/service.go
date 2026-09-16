package models

import (
    "fmt"
    "log"

    "fenrir/internal/config"
    "fenrir/internal/models"
)

// Service — сервис управления AI-моделями
type Service struct {
    config *config.Config
    models map[string]models.Interface
}

// NewService — создаёт сервис и инициализирует модели из конфига
func NewService(cfg *config.Config) *Service {
    s := &Service{
        config: cfg,
        models: make(map[string]models.Interface),
    }
    s.initModels()
    return s
}

func (s *Service) initModels() {
    for name, provider := range s.config.Models.Providers {
        switch provider.Type {
        case "ollama":
            modelName := "llama3.2:3b"
            if len(provider.Models) > 0 {
                modelName = provider.Models[0]
            }
            s.models[name] = models.NewOllama(provider.URL, modelName)
            log.Printf("✅ Загружена модель: %s (Ollama/%s)", name, modelName)

        default:
            log.Printf("⚠️  Неизвестный тип провайдера: %s", provider.Type)
        }
    }

    if len(s.models) == 0 {
        log.Println("⚠️  Модели не найдены, создаю дефолтную (ollama/llama3.2:3b)")
        s.models["llama3.2"] = models.NewOllama("http://localhost:11434", "llama3.2:3b")
    }
}

// GetModel — возвращает модель по имени
func (s *Service) GetModel(name string) (models.Interface, error) {
    model, ok := s.models[name]
    if !ok {
        // Если модель не найдена — возвращаем первую доступную
        for _, m := range s.models {
            return m, nil
        }
        return nil, fmt.Errorf("модель '%s' не найдена", name)
    }
    return model, nil
}

// ListModels — возвращает список доступных моделей
func (s *Service) ListModels() []string {
    var names []string
    for name := range s.models {
        names = append(names, name)
    }
    return names
}
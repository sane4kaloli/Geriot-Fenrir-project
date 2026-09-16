package models

import (
    "bufio"
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
)

// Ollama — интеграция с локальным Ollama
type Ollama struct {
    BaseModel
    url    string
    model  string
    client *http.Client
}

type ollamaRequest struct {
    Model   string                 `json:"model"`
    Prompt  string                 `json:"prompt"`
    Stream  bool                   `json:"stream"`
    Options map[string]interface{} `json:"options,omitempty"`
}

type ollamaResponse struct {
    Response string `json:"response"`
    Done     bool   `json:"done"`
    Error    string `json:"error,omitempty"`
}

// NewOllama — создаёт новую Ollama-модель
func NewOllama(url, model string) *Ollama {
    if url == "" {
        url = "http://localhost:11434"
    }
    return &Ollama{
        BaseModel: BaseModel{name: "ollama"},
        url:       url,
        model:     model,
        client: &http.Client{
            Timeout: 120 * time.Second,
        },
    }
}

// Generate — синхронная генерация
func (o *Ollama) Generate(prompt string) string {
    req := ollamaRequest{
        Model:  o.model,
        Prompt: prompt,
        Stream: false,
        Options: map[string]interface{}{
            "temperature": 0.7,
            "top_p":       0.9,
        },
    }

    jsonData, err := json.Marshal(req)
    if err != nil {
        return fmt.Sprintf("❌ Ошибка сериализации: %v", err)
    }

    resp, err := o.client.Post(
        fmt.Sprintf("%s/api/generate", o.url),
        "application/json",
        bytes.NewBuffer(jsonData),
    )
    if err != nil {
        return fmt.Sprintf("❌ Ошибка подключения к Ollama: %v\n\nПроверьте:\n1. Запущен ли Ollama? (ollama serve)\n2. Скачана ли модель? (ollama pull %s)", err, o.model)
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return fmt.Sprintf("❌ Ошибка чтения ответа: %v", err)
    }

    var result ollamaResponse
    if err := json.Unmarshal(body, &result); err != nil {
        return fmt.Sprintf("❌ Ошибка парсинга: %v\nОтвет: %s", err, string(body))
    }

    if result.Error != "" {
        return fmt.Sprintf("❌ Ошибка модели: %s", result.Error)
    }

    return result.Response
}

// StreamGenerate — стриминг ответа
func (o *Ollama) StreamGenerate(prompt string) <-chan string {
    ch := make(chan string, 100)

    go func() {
        defer close(ch)

        req := ollamaRequest{
            Model:  o.model,
            Prompt: prompt,
            Stream: true,
            Options: map[string]interface{}{
                "temperature": 0.7,
                "top_p":       0.9,
            },
        }

        jsonData, err := json.Marshal(req)
        if err != nil {
            ch <- fmt.Sprintf("❌ Ошибка: %v", err)
            return
        }

        resp, err := o.client.Post(
            fmt.Sprintf("%s/api/generate", o.url),
            "application/json",
            bytes.NewBuffer(jsonData),
        )
        if err != nil {
            ch <- fmt.Sprintf("❌ Ошибка подключения к Ollama: %v\n\nУбедитесь, что Ollama запущен: ollama serve", err)
            return
        }
        defer resp.Body.Close()

        		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
            var result ollamaResponse
            if err := json.Unmarshal([]byte(line), &result); err != nil {
                continue
            }

            if result.Error != "" {
                ch <- fmt.Sprintf("❌ Ошибка модели: %s", result.Error)
                return
            }

            if result.Response != "" {
                ch <- result.Response
            }

            if result.Done {
                break
            }
        }

        if err := scanner.Err(); err != nil {
            ch <- fmt.Sprintf("❌ Ошибка чтения стрима: %v", err)
        }
    }()

    return ch
}

// Name — возвращает имя модели
func (o *Ollama) Name() string {
    return fmt.Sprintf("ollama/%s", o.model)
}
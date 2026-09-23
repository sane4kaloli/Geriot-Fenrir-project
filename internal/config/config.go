package config

import (
    "encoding/json"
    "fmt"
    "os"
    "time"
)

type Config struct {
    Gateway  GatewayConfig  `json:"gateway"`
    WebUI    WebUIConfig    `json:"webui"`
    Database DatabaseConfig `json:"database"`
    Models   ModelsConfig   `json:"models"`
    Agents   AgentsConfig   `json:"agents"`
}

type GatewayConfig struct {
    Bind string     `json:"bind"`
    Port int        `json:"port"`
    Auth AuthConfig `json:"auth"`
}

type AuthConfig struct {
    Mode       string                `json:"mode"`
    Token      string                `json:"token,omitempty"`
    Users      map[string]UserConfig `json:"users,omitempty"`
    SessionTTL string                `json:"sessionTTL,omitempty"`
}

type UserConfig struct {
    Login        string `json:"login,omitempty"`
    PasswordHash string `json:"passwordHash,omitempty"`
    Role         string `json:"role"`
    DisplayName  string `json:"displayName,omitempty"`
}

type WebUIConfig struct {
    Enabled          bool     `json:"enabled"`
    SessionTTL       string   `json:"sessionTTL,omitempty"`
    FileMaxBytes     int64    `json:"fileMaxBytes,omitempty"`
    FileDenyPatterns []string `json:"fileDenyPatterns,omitempty"`
}

type DatabaseConfig struct {
    Path           string `json:"path"`
    MaxConnections int    `json:"maxConnections,omitempty"`
}

type ModelsConfig struct {
    Providers map[string]ProviderConfig `json:"providers"`
    Default   string                    `json:"default"`
}

type ProviderConfig struct {
    ID      string                 `json:"id"`
    Name    string                 `json:"name"`
    Type    string                 `json:"type"`
    URL     string                 `json:"url,omitempty"`
    APIKey  string                 `json:"apiKey,omitempty"`
    Models  []string               `json:"models"`
    Agents  []AgentConfig          `json:"agents"`
    Params  map[string]interface{} `json:"params,omitempty"`
    Enabled bool                   `json:"enabled"`
}

type AgentConfig struct {
    ID          string   `json:"id"`
    Name        string   `json:"name"`
    Description string   `json:"description,omitempty"`
    Model       string   `json:"model"`
    Prompt      string   `json:"prompt"`
    Skills      []string `json:"skills,omitempty"`
    Temperature float64  `json:"temperature,omitempty"`
    MaxTokens   int      `json:"maxTokens,omitempty"`
    Enabled     bool     `json:"enabled"`
    IsDefault   bool     `json:"isDefault,omitempty"`
}

type AgentsConfig struct {
    Workspace string `json:"workspace"`
    Main      string `json:"main"`
}

func Load(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("read config: %w", err)
    }

    var cfg Config
    if err := json.Unmarshal(data, &cfg); err != nil {
        return nil, fmt.Errorf("parse config: %w", err)
    }

    if cfg.WebUI.SessionTTL == "" {
        cfg.WebUI.SessionTTL = "168h"
    }
    if cfg.WebUI.FileMaxBytes == 0 {
        cfg.WebUI.FileMaxBytes = 25 * 1024 * 1024
    }
    if len(cfg.WebUI.FileDenyPatterns) == 0 {
        cfg.WebUI.FileDenyPatterns = []string{".exe", ".dll", ".so", ".bin"}
    }
    if cfg.Agents.Workspace == "" {
        cfg.Agents.Workspace = "./workspace"
    }
    if cfg.Database.Path == "" {
        cfg.Database.Path = "./data/fenrir.db"
    }
    if cfg.Database.MaxConnections == 0 {
        cfg.Database.MaxConnections = 10
    }
    if cfg.Models.Default == "" {
        cfg.Models.Default = "llama2"
    }

    return &cfg, nil
}

func (c *Config) GetWebUIAddr() string {
    return fmt.Sprintf("%s:%d", c.Gateway.Bind, c.Gateway.Port)
}

func (c *Config) GetSessionTTL() time.Duration {
    d, _ := time.ParseDuration(c.WebUI.SessionTTL)
    if d == 0 {
        return 168 * time.Hour
    }
    return d
}




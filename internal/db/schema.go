package db

import "time"

type User struct {
    ID           int64     `db:"id" json:"id"`
    Login        string    `db:"login" json:"login"`
    PasswordHash string    `db:"password_hash" json:"-"`
    Role         string    `db:"role" json:"role"`
    DisplayName  string    `db:"display_name" json:"displayName"`
    CreatedAt    time.Time `db:"created_at" json:"createdAt"`
    UpdatedAt    time.Time `db:"updated_at" json:"updatedAt"`
}

type Session struct {
    ID        string    `db:"id" json:"id"`
    UserID    int64     `db:"user_id" json:"userId"`
    CreatedAt time.Time `db:"created_at" json:"createdAt"`
    ExpiresAt time.Time `db:"expires_at" json:"expiresAt"`
}

type Chat struct {
    ID        string    `db:"id" json:"id"`
    UserID    int64     `db:"user_id" json:"userId"`
    Title     string    `db:"title" json:"title"`
    Model     string    `db:"model" json:"model"`
    Mode      string    `db:"mode" json:"mode"`
    CreatedAt time.Time `db:"created_at" json:"createdAt"`
    UpdatedAt time.Time `db:"updated_at" json:"updatedAt"`
}

type Message struct {
    ID         int64     `db:"id" json:"id"`
    ChatID     string    `db:"chat_id" json:"chatId"`
    Role       string    `db:"role" json:"role"`
    Content    string    `db:"content" json:"content"`
    Model      string    `db:"model" json:"model"`
    Mode       string    `db:"mode" json:"mode"`
    Tokens     int       `db:"tokens" json:"tokens"`
    DurationMs int       `db:"duration_ms" json:"durationMs"`
    CreatedAt  time.Time `db:"created_at" json:"createdAt"`
}

type Skill struct {
    ID          int64     `db:"id" json:"id"`
    Name        string    `db:"name" json:"name"`
    Description string    `db:"description" json:"description"`
    Enabled     bool      `db:"enabled" json:"enabled"`
    UpdatedAt   time.Time `db:"updated_at" json:"updatedAt"`
}

type Setting struct {
    Key       string    `db:"key" json:"key"`
    Value     string    `db:"value" json:"value"`
    UpdatedAt time.Time `db:"updated_at" json:"updatedAt"`
}

type AuditLog struct {
    ID        int64     `db:"id" json:"id"`
    UserID    int64     `db:"user_id" json:"userId"`
    Action    string    `db:"action" json:"action"`
    Target    string    `db:"target" json:"target"`
    Result    string    `db:"result" json:"result"`
    CreatedAt time.Time `db:"created_at" json:"createdAt"`
}

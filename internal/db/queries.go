package db

import (
    "database/sql"
    "fmt"
    "time"
)

// ===== USERS =====

func (d *DB) CreateUser(login, passwordHash, role, displayName string) (*User, error) {
    res, err := d.conn.Exec(
        `INSERT INTO users (login, password_hash, role, display_name) VALUES (?, ?, ?, ?)`,
        login, passwordHash, role, displayName,
    )
    if err != nil {
        return nil, fmt.Errorf("создание пользователя: %w", err)
    }
    id, _ := res.LastInsertId()
    return d.GetUserByID(id)
}

func (d *DB) GetUserByID(id int64) (*User, error) {
    var u User
    err := d.conn.QueryRow(
        `SELECT id, login, password_hash, role, display_name, created_at, updated_at 
         FROM users WHERE id = ?`, id,
    ).Scan(&u.ID, &u.Login, &u.PasswordHash, &u.Role, &u.DisplayName, &u.CreatedAt, &u.UpdatedAt)
    if err != nil {
        return nil, err
    }
    return &u, nil
}

func (d *DB) GetUserByLogin(login string) (*User, error) {
    var u User
    err := d.conn.QueryRow(
        `SELECT id, login, password_hash, role, display_name, created_at, updated_at 
         FROM users WHERE login = ?`, login,
    ).Scan(&u.ID, &u.Login, &u.PasswordHash, &u.Role, &u.DisplayName, &u.CreatedAt, &u.UpdatedAt)
    if err != nil {
        return nil, err
    }
    return &u, nil
}

func (d *DB) ListUsers() ([]User, error) {
    rows, err := d.conn.Query(`SELECT id, login, role, display_name, created_at, updated_at FROM users ORDER BY id`)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var users []User
    for rows.Next() {
        var u User
        if err := rows.Scan(&u.ID, &u.Login, &u.Role, &u.DisplayName, &u.CreatedAt, &u.UpdatedAt); err != nil {
            return nil, err
        }
        users = append(users, u)
    }
    return users, nil
}

func (d *DB) DeleteUser(id int64) error {
    _, err := d.conn.Exec(`DELETE FROM users WHERE id = ?`, id)
    return err
}

// ===== SESSIONS =====

func (d *DB) CreateSession(id string, userID int64, expiresAt time.Time) error {
    _, err := d.conn.Exec(
        `INSERT INTO sessions (id, user_id, expires_at) VALUES (?, ?, ?)`,
        id, userID, expiresAt,
    )
    return err
}

func (d *DB) GetSession(id string) (*Session, error) {
    var s Session
    err := d.conn.QueryRow(
        `SELECT id, user_id, created_at, expires_at FROM sessions WHERE id = ? AND expires_at > ?`,
        id, time.Now(),
    ).Scan(&s.ID, &s.UserID, &s.CreatedAt, &s.ExpiresAt)
    if err != nil {
        return nil, err
    }
    return &s, nil
}

func (d *DB) DeleteSession(id string) error {
    _, err := d.conn.Exec(`DELETE FROM sessions WHERE id = ?`, id)
    return err
}

func (d *DB) CleanupExpiredSessions() error {
    _, err := d.conn.Exec(`DELETE FROM sessions WHERE expires_at < ?`, time.Now())
    return err
}

// ===== CHATS =====

func (d *DB) CreateChat(id string, userID int64, title, model, mode string) (*Chat, error) {
    _, err := d.conn.Exec(
        `INSERT INTO chats (id, user_id, title, model, mode) VALUES (?, ?, ?, ?, ?)`,
        id, userID, title, model, mode,
    )
    if err != nil {
        return nil, err
    }
    return d.GetChat(id)
}

func (d *DB) GetChat(id string) (*Chat, error) {
    var c Chat
    err := d.conn.QueryRow(
        `SELECT id, user_id, title, model, mode, created_at, updated_at FROM chats WHERE id = ?`,
        id,
    ).Scan(&c.ID, &c.UserID, &c.Title, &c.Model, &c.Mode, &c.CreatedAt, &c.UpdatedAt)
    if err != nil {
        return nil, err
    }
    return &c, nil
}

func (d *DB) ListChatsByUser(userID int64) ([]Chat, error) {
    rows, err := d.conn.Query(
        `SELECT id, user_id, title, model, mode, created_at, updated_at 
         FROM chats WHERE user_id = ? ORDER BY updated_at DESC`,
        userID,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var chats []Chat
    for rows.Next() {
        var c Chat
        if err := rows.Scan(&c.ID, &c.UserID, &c.Title, &c.Model, &c.Mode, &c.CreatedAt, &c.UpdatedAt); err != nil {
            return nil, err
        }
        chats = append(chats, c)
    }
    return chats, nil
}

func (d *DB) ListAllChats() ([]Chat, error) {
    rows, err := d.conn.Query(
        `SELECT id, user_id, title, model, mode, created_at, updated_at 
         FROM chats ORDER BY updated_at DESC`,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var chats []Chat
    for rows.Next() {
        var c Chat
        if err := rows.Scan(&c.ID, &c.UserID, &c.Title, &c.Model, &c.Mode, &c.CreatedAt, &c.UpdatedAt); err != nil {
            return nil, err
        }
        chats = append(chats, c)
    }
    return chats, nil
}

func (d *DB) UpdateChatTitle(id, title string) error {
    _, err := d.conn.Exec(
        `UPDATE chats SET title = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
        title, id,
    )
    return err
}

func (d *DB) DeleteChat(id string) error {
    _, err := d.conn.Exec(`DELETE FROM chats WHERE id = ?`, id)
    return err
}

// ===== MESSAGES =====

func (d *DB) AddMessage(chatID, role, content, model, mode string, tokens, durationMs int) (*Message, error) {
    res, err := d.conn.Exec(
        `INSERT INTO messages (chat_id, role, content, model, mode, tokens, duration_ms) 
         VALUES (?, ?, ?, ?, ?, ?, ?)`,
        chatID, role, content, model, mode, tokens, durationMs,
    )
    if err != nil {
        return nil, err
    }
    d.conn.Exec(`UPDATE chats SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`, chatID)
    id, _ := res.LastInsertId()
    return d.GetMessage(id)
}

func (d *DB) GetMessage(id int64) (*Message, error) {
    var m Message
    err := d.conn.QueryRow(
        `SELECT id, chat_id, role, content, model, mode, tokens, duration_ms, created_at 
         FROM messages WHERE id = ?`, id,
    ).Scan(&m.ID, &m.ChatID, &m.Role, &m.Content, &m.Model, &m.Mode, &m.Tokens, &m.DurationMs, &m.CreatedAt)
    if err != nil {
        return nil, err
    }
    return &m, nil
}

func (d *DB) ListMessages(chatID string, limit int) ([]Message, error) {
    if limit <= 0 {
        limit = 100
    }
    rows, err := d.conn.Query(
        `SELECT id, chat_id, role, content, model, mode, tokens, duration_ms, created_at 
         FROM messages WHERE chat_id = ? ORDER BY id ASC LIMIT ?`,
        chatID, limit,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var messages []Message
    for rows.Next() {
        var m Message
        if err := rows.Scan(&m.ID, &m.ChatID, &m.Role, &m.Content, &m.Model, &m.Mode, &m.Tokens, &m.DurationMs, &m.CreatedAt); err != nil {
            return nil, err
        }
        messages = append(messages, m)
    }
    return messages, nil
}

// ===== SKILLS =====

func (d *DB) ListSkills() ([]Skill, error) {
    rows, err := d.conn.Query(`SELECT id, name, description, enabled, updated_at FROM skills ORDER BY name`)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var skills []Skill
    for rows.Next() {
        var s Skill
        var enabled int
        if err := rows.Scan(&s.ID, &s.Name, &s.Description, &enabled, &s.UpdatedAt); err != nil {
            return nil, err
        }
        s.Enabled = enabled == 1
        skills = append(skills, s)
    }
    return skills, nil
}

func (d *DB) SetSkillEnabled(name string, enabled bool) error {
    val := 0
    if enabled {
        val = 1
    }
    _, err := d.conn.Exec(
        `UPDATE skills SET enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE name = ?`,
        val, name,
    )
    return err
}

// ===== SETTINGS =====

func (d *DB) GetSetting(key string) (string, error) {
    var value string
    err := d.conn.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
    if err == sql.ErrNoRows {
        return "", nil
    }
    return value, err
}

func (d *DB) SetSetting(key, value string) error {
    _, err := d.conn.Exec(
        `INSERT INTO settings (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
         ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`,
        key, value,
    )
    return err
}

func (d *DB) GetAllSettings() (map[string]string, error) {
    rows, err := d.conn.Query(`SELECT key, value FROM settings`)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    result := make(map[string]string)
    for rows.Next() {
        var k, v string
        if err := rows.Scan(&k, &v); err != nil {
            return nil, err
        }
        result[k] = v
    }
    return result, nil
}

// ===== AUDIT =====

func (d *DB) AddAudit(userID int64, action, target, result string) error {
    _, err := d.conn.Exec(
        `INSERT INTO audit_log (user_id, action, target, result) VALUES (?, ?, ?, ?)`,
        userID, action, target, result,
    )
    return err
}

func (d *DB) ListAudit(limit int) ([]AuditLog, error) {
    if limit <= 0 {
        limit = 100
    }
    rows, err := d.conn.Query(
        `SELECT id, user_id, action, target, result, created_at 
         FROM audit_log ORDER BY id DESC LIMIT ?`, limit,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var logs []AuditLog
    for rows.Next() {
        var a AuditLog
        if err := rows.Scan(&a.ID, &a.UserID, &a.Action, &a.Target, &a.Result, &a.CreatedAt); err != nil {
            return nil, err
        }
        logs = append(logs, a)
    }
    return logs, nil
}
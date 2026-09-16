package db

import (
    "database/sql"
    "fmt"
    "log"
    "time"

    _ "modernc.org/sqlite"
)

type DB struct {
    conn *sql.DB
}

func New(path string) (*DB, error) {
    conn, err := sql.Open("sqlite", path)
    if err != nil {
        return nil, fmt.Errorf("открытие БД: %w", err)
    }

    conn.SetMaxOpenConns(10)
    conn.SetMaxIdleConns(5)
    conn.SetConnMaxLifetime(time.Hour)

    if err := conn.Ping(); err != nil {
        return nil, fmt.Errorf("ping БД: %w", err)
    }

    db := &DB{conn: conn}

    if err := db.migrate(); err != nil {
        return nil, fmt.Errorf("миграция: %w", err)
    }

    log.Printf("✅ БД инициализирована: %s", path)
    return db, nil
}

func (d *DB) Close() error {
    return d.conn.Close()
}

func (d *DB) Conn() *sql.DB {
    return d.conn
}

func (d *DB) migrate() error {
    schema := `
    CREATE TABLE IF NOT EXISTS users (
        id            INTEGER PRIMARY KEY AUTOINCREMENT,
        login         TEXT NOT NULL UNIQUE,
        password_hash TEXT NOT NULL,
        role          TEXT NOT NULL DEFAULT 'user',
        display_name  TEXT,
        created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
        updated_at    DATETIME DEFAULT CURRENT_TIMESTAMP
    );

    CREATE TABLE IF NOT EXISTS sessions (
        id         TEXT PRIMARY KEY,
        user_id    INTEGER NOT NULL,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        expires_at DATETIME NOT NULL,
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
    );

    CREATE TABLE IF NOT EXISTS chats (
        id         TEXT PRIMARY KEY,
        user_id    INTEGER NOT NULL,
        title      TEXT NOT NULL DEFAULT 'Новый чат',
        model      TEXT,
        mode       TEXT DEFAULT 'chat',
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
    );

    CREATE TABLE IF NOT EXISTS messages (
        id          INTEGER PRIMARY KEY AUTOINCREMENT,
        chat_id     TEXT NOT NULL,
        role        TEXT NOT NULL,
        content     TEXT NOT NULL,
        model       TEXT,
        mode        TEXT,
        tokens      INTEGER DEFAULT 0,
        duration_ms INTEGER DEFAULT 0,
        created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
        FOREIGN KEY (chat_id) REFERENCES chats(id) ON DELETE CASCADE
    );

    CREATE TABLE IF NOT EXISTS skills (
        id          INTEGER PRIMARY KEY AUTOINCREMENT,
        name        TEXT NOT NULL UNIQUE,
        description TEXT,
        enabled     INTEGER DEFAULT 0,
        updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
    );

    CREATE TABLE IF NOT EXISTS settings (
        key        TEXT PRIMARY KEY,
        value      TEXT,
        updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );

    CREATE TABLE IF NOT EXISTS audit_log (
        id         INTEGER PRIMARY KEY AUTOINCREMENT,
        user_id    INTEGER,
        action     TEXT NOT NULL,
        target     TEXT,
        result     TEXT,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );

    CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
    CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);
    CREATE INDEX IF NOT EXISTS idx_chats_user ON chats(user_id);
    CREATE INDEX IF NOT EXISTS idx_messages_chat ON messages(chat_id);
    CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_log(created_at);
    `

    if _, err := d.conn.Exec(schema); err != nil {
        return fmt.Errorf("схема: %w", err)
    }

    return d.seed()
}

func (d *DB) seed() error {
    defaults := map[string]string{
        "default_model":      "llama2",
        "default_agent_mode": "fast",
        "default_language":   "ru",
        "theme":              "dark",
    }

    for key, value := range defaults {
        if _, err := d.conn.Exec(
            `INSERT OR IGNORE INTO settings (key, value) VALUES (?, ?)`,
            key, value,
        ); err != nil {
            return err
        }
    }

    skills := []struct {
        Name        string
        Description string
        Enabled     int
    }{
        {"web_search", "Поиск в интернете", 1},
        {"code_executor", "Выполнение кода", 0},
        {"memory", "Долговременная память", 1},
        {"file_reader", "Чтение файлов", 1},
    }

    for _, s := range skills {
        if _, err := d.conn.Exec(
            `INSERT OR IGNORE INTO skills (name, description, enabled) VALUES (?, ?, ?)`,
            s.Name, s.Description, s.Enabled,
        ); err != nil {
            return err
        }
    }

    return nil
}

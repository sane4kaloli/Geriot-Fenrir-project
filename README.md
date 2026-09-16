# Fenrir AI Web UI

Веб-интерфейс для локального AI-ассистента **Fenrir** — с ролями admin/user, чатом, файлами и управлением.

## Быстрый старт

```bash
# Установка Ollama
curl -fsSL https://ollama.com/install.sh | sh
ollama pull llama3.2:3b

# Запуск
cd fenrir-webui
go run cmd/fenrir/main.go
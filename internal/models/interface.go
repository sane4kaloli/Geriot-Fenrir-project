package models

// Interface — общий интерфейс для всех AI-моделей
type Interface interface {
    // Generate — синхронная генерация ответа
    Generate(prompt string) string

    // StreamGenerate — стриминг ответа по частям
    StreamGenerate(prompt string) <-chan string

    // Name — имя модели
    Name() string
}

// BaseModel — базовая структура для всех моделей
type BaseModel struct {
    name string
}

func (m *BaseModel) Name() string {
    return m.name
}
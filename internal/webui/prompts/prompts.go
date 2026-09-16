package prompts

import "fmt"

func Chat(message, agentMode, username string) string {
    modeDesc := getModeDescription(agentMode)
    return fmt.Sprintf(`Ты — Fenrir, интеллектуальный AI-агент.
Пользователь: %s
Режим: %s

%s

Ответь на вопрос пользователя кратко и по делу.
Используй Markdown для форматирования.

Вопрос: %s
Ответ:`, username, agentMode, modeDesc, message)
}

func Slides(topic, agentMode, username string) string {
    modeDesc := getModeDescription(agentMode)
    return fmt.Sprintf(`Ты — эксперт по созданию презентаций.
Пользователь: %s
Режим: %s

%s

Создай структуру слайдов для темы: "%s"

Ответ ТОЛЬКО в формате JSON, без лишнего текста:
{
    "title": "Название презентации",
    "slides": [
        {"title": "Заголовок слайда", "content": "Содержание", "type": "content"}
    ]
}

Создай 5-7 слайдов.`, username, agentMode, modeDesc, topic)
}

func Tables(query, agentMode, username string) string {
    modeDesc := getModeDescription(agentMode)
    return fmt.Sprintf(`Ты — аналитик данных.
Пользователь: %s
Режим: %s

%s

Создай таблицу для запроса: "%s"

Ответ ТОЛЬКО в формате JSON:
{
    "title": "Название таблицы",
    "headers": ["Колонка 1", "Колонка 2"],
    "rows": [["Значение 1", "Значение 2"]],
    "summary": "Краткий вывод"
}

Сгенерируй 5-10 строк.`, username, agentMode, modeDesc, query)
}

func Research(topic, agentMode, username string) string {
    modeDesc := getModeDescription(agentMode)
    return fmt.Sprintf(`Ты — исследовательский агент.
Пользователь: %s
Режим: %s

%s

Проведи глубокое исследование по теме: "%s"

Ответ оформи в структурированном виде:
## Резюме
Краткое резюме

## Основные факты
- Факт 1
- Факт 2

## Тренды
- Тренд 1

## Источники
- Источник 1`, username, agentMode, modeDesc, topic)
}

func getModeDescription(mode string) string {
	switch mode {
	case "fast":
		return "Режим БЫСТРЫЙ: давай краткие, точные ответы без лишних деталей."
	case "reasoning", "expert":
		return `Режим РАССУЖДЕНИЕ: думай пошагово, объясняй ход мыслей.

ФОРМАТ ОТВЕТА (обязательно соблюдай):
🤔 Размышляю...
[Краткое вступление — о чём задача]

📝 Шаг 1: [название шага]
[Что делаем и почему]

📝 Шаг 2: [название шага]
[Что делаем и почему]

[Столько шагов, сколько нужно]

✅ Итог:
[Финальный ответ]`
	default:
		return "Режим СТАНДАРТНЫЙ."
	}
}
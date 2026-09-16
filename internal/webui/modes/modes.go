package modes

import (
    "encoding/json"
    "fmt"
    "strings"
)

type SlidesResponse struct {
    Title  string `json:"title"`
    Slides []struct {
        Title   string `json:"title"`
        Content string `json:"content"`
        Type    string `json:"type"`
    } `json:"slides"`
}

type TableResponse struct {
    Title   string     `json:"title"`
    Headers []string   `json:"headers"`
    Rows    [][]string `json:"rows"`
    Summary string     `json:"summary"`
}

type ResearchResponse struct {
    Summary  string `json:"summary"`
    Raw      string `json:"raw"`
}

type TextResponse struct {
    Answer  string   `json:"answer"`
    Sources []string `json:"sources,omitempty"`
}

func ParseSlides(raw string) (*SlidesResponse, error) {
    jsonStr := extractJSON(raw)
    if jsonStr == "" {
        return nil, fmt.Errorf("не удалось найти JSON в ответе")
    }
    var result SlidesResponse
    if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
        return nil, fmt.Errorf("ошибка парсинга JSON: %w", err)
    }
    return &result, nil
}

func ParseTable(raw string) (*TableResponse, error) {
    jsonStr := extractJSON(raw)
    if jsonStr == "" {
        return nil, fmt.Errorf("не удалось найти JSON в ответе")
    }
    var result TableResponse
    if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
        return nil, fmt.Errorf("ошибка парсинга JSON: %w", err)
    }
    return &result, nil
}

func ParseResearch(raw string) *ResearchResponse {
    return &ResearchResponse{
        Summary: "",
        Raw:     raw,
    }
}

func ParseText(raw string, files []string) *TextResponse {
    return &TextResponse{
        Answer:  raw,
        Sources: files,
    }
}

func extractJSON(raw string) string {
    start := strings.Index(raw, "{")
    end := strings.LastIndex(raw, "}")
    if start == -1 || end == -1 || end < start {
        return ""
    }
    return raw[start : end+1]
}
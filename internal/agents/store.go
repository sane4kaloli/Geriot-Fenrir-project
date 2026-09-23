package agents

import (
    "errors"
    "sync"

    "fenrir/internal/config"
)

var (
    ErrProviderNotFound = errors.New("provider not found")
    ErrAgentNotFound    = errors.New("agent not found")
)

type Store struct {
    mu        sync.RWMutex
    providers map[string]config.ProviderConfig
    defaultID string
    cfg       *config.Config
}

func NewStore(cfg *config.Config) *Store {
    return &Store{
        providers: cfg.Models.Providers,
        defaultID: cfg.Models.Default,
        cfg:       cfg,
    }
}

func (s *Store) ListProviders() []*config.ProviderConfig {
    s.mu.RLock()
    defer s.mu.RUnlock()
    out := make([]*config.ProviderConfig, 0, len(s.providers))
    for _, p := range s.providers {
    pCopy := p
    out = append(out, &pCopy)
}
    return out
}

func (s *Store) GetProvider(id string) (*config.ProviderConfig, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    p, ok := s.providers[id]
    if !ok {
        return nil, ErrProviderNotFound
    }
    return &p, nil
}

func (s *Store) ListAgents() []AgentInfo {
    s.mu.RLock()
    defer s.mu.RUnlock()
    var out []AgentInfo
    for pid, p := range s.providers {
        if !p.Enabled {
            continue
        }
        for _, a := range p.Agents {
            if !a.Enabled {
                continue
            }
            out = append(out, AgentInfo{
                ID:          a.ID,
                Name:        a.Name,
                Description: a.Description,
                ProviderID:  pid,
                Provider:    p.Name,
                Model:       a.Model,
                Skills:      a.Skills,
                IsDefault:   a.IsDefault,
            })
        }
    }
    return out
}

func (s *Store) GetAgent(agentID string) (*AgentInfo, *config.AgentConfig, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    for pid, p := range s.providers {
        for i := range p.Agents {
            a := &p.Agents[i]
            if a.ID == agentID {
                return &AgentInfo{
                    ID:          a.ID,
                    Name:        a.Name,
                    Description: a.Description,
                    ProviderID:  pid,
                    Provider:    p.Name,
                    Model:       a.Model,
                    Skills:      a.Skills,
                    IsDefault:   a.IsDefault,
                }, a, nil
            }
        }
    }
    return nil, nil, ErrAgentNotFound
}

func (s *Store) GetAgentsByProvider(providerID string) ([]config.AgentConfig, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    p, ok := s.providers[providerID]
    if !ok {
        return nil, ErrProviderNotFound
    }
    return p.Agents, nil
}

type AgentInfo struct {
    ID          string   `json:"id"`
    Name        string   `json:"name"`
    Description string   `json:"description,omitempty"`
    ProviderID  string   `json:"providerId"`
    Provider    string   `json:"provider"`
    Model       string   `json:"model"`
    Skills      []string `json:"skills,omitempty"`
    IsDefault   bool     `json:"isDefault,omitempty"`
}



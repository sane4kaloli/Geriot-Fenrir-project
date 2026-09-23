package webui

import (
    "net/http"

    "github.com/go-chi/chi/v5"
)

func (s *Server) handleListProviders(w http.ResponseWriter, r *http.Request) {
    providers := s.agentStore.ListProviders()
    writeJSON(w, http.StatusOK, map[string]interface{}{
        "providers": providers,
    })
}

func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
    agents := s.agentStore.ListAgents()
    writeJSON(w, http.StatusOK, map[string]interface{}{
        "agents": agents,
    })
}

func (s *Server) handleGetAgent(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    info, _, err := s.agentStore.GetAgent(id)
    if err != nil {
        writeError(w, http.StatusNotFound, "Агент не найден", err.Error())
        return
    }
    writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleListProviderAgents(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    agents, err := s.agentStore.GetAgentsByProvider(id)
    if err != nil {
        writeError(w, http.StatusNotFound, "Провайдер не найден", err.Error())
        return
    }
    writeJSON(w, http.StatusOK, map[string]interface{}{
        "agents": agents,
    })
}

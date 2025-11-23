package handler

import (
	"encoding/json"
	"net/http"

	"pr-reviewer-service/internal/model"
	"pr-reviewer-service/internal/service"
)

type Handler struct {
	service *service.Service
}

func New(service *service.Service) *Handler {
	return &Handler{service: service}
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, model.ErrorResponse{
		Error: model.ErrorDetail{
			Code:    code,
			Message: message,
		},
	})
}

func (h *Handler) CreateTeam(w http.ResponseWriter, r *http.Request) {
	var team model.Team
	if err := json.NewDecoder(r.Body).Decode(&team); err != nil {
		writeError(w, http.StatusBadRequest, model.ErrNotFound, "invalid request body")
		return
	}

	createdTeam, err := h.service.CreateTeam(team)
	if err != nil {
		if err.Error() == model.ErrTeamExists {
			writeError(w, http.StatusBadRequest, model.ErrTeamExists, "team_name already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, model.ErrNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"team": createdTeam,
	})
}

func (h *Handler) GetTeam(w http.ResponseWriter, r *http.Request) {
	teamName := r.URL.Query().Get("team_name")
	if teamName == "" {
		writeError(w, http.StatusBadRequest, model.ErrNotFound, "team_name query parameter is required")
		return
	}

	team, err := h.service.GetTeam(teamName)
	if err != nil {
		if err.Error() == model.ErrNotFound {
			writeError(w, http.StatusNotFound, model.ErrNotFound, "team not found")
			return
		}
		writeError(w, http.StatusInternalServerError, model.ErrNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, team)
}

func (h *Handler) SetUserActive(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID   string `json:"user_id"`
		IsActive bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, model.ErrNotFound, "invalid request body")
		return
	}

	user, err := h.service.SetUserActive(req.UserID, req.IsActive)
	if err != nil {
		if err.Error() == model.ErrNotFound {
			writeError(w, http.StatusNotFound, model.ErrNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, model.ErrNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user": user,
	})
}

func (h *Handler) CreatePR(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PullRequestID   string `json:"pull_request_id"`
		PullRequestName string `json:"pull_request_name"`
		AuthorID        string `json:"author_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, model.ErrNotFound, "invalid request body")
		return
	}

	pr, err := h.service.CreatePR(req.PullRequestID, req.PullRequestName, req.AuthorID)
	if err != nil {
		if err.Error() == model.ErrPRExists {
			writeError(w, http.StatusConflict, model.ErrPRExists, "PR id already exists")
			return
		}
		if err.Error() == model.ErrNotFound {
			writeError(w, http.StatusNotFound, model.ErrNotFound, "author not found")
			return
		}
		writeError(w, http.StatusInternalServerError, model.ErrNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"pr": pr,
	})
}

func (h *Handler) MergePR(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PullRequestID string `json:"pull_request_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, model.ErrNotFound, "invalid request body")
		return
	}

	pr, err := h.service.MergePR(req.PullRequestID)
	if err != nil {
		if err.Error() == model.ErrNotFound {
			writeError(w, http.StatusNotFound, model.ErrNotFound, "PR not found")
			return
		}
		writeError(w, http.StatusInternalServerError, model.ErrNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"pr": pr,
	})
}

func (h *Handler) ReassignReviewer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PullRequestID string `json:"pull_request_id"`
		OldUserID     string `json:"old_user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, model.ErrNotFound, "invalid request body")
		return
	}

	pr, replacedBy, err := h.service.ReassignReviewer(req.PullRequestID, req.OldUserID)
	if err != nil {
		if err.Error() == model.ErrNotFound {
			writeError(w, http.StatusNotFound, model.ErrNotFound, "PR or user not found")
			return
		}
		if err.Error() == model.ErrPRMerged {
			writeError(w, http.StatusConflict, model.ErrPRMerged, "cannot reassign on merged PR")
			return
		}
		if err.Error() == model.ErrNotAssigned {
			writeError(w, http.StatusConflict, model.ErrNotAssigned, "reviewer is not assigned to this PR")
			return
		}
		if err.Error() == model.ErrNoCandidate {
			writeError(w, http.StatusConflict, model.ErrNoCandidate, "no active replacement candidate in team")
			return
		}
		writeError(w, http.StatusInternalServerError, model.ErrNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"pr":          pr,
		"replaced_by": replacedBy,
	})
}

func (h *Handler) GetUserReviews(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		writeError(w, http.StatusBadRequest, model.ErrNotFound, "user_id query parameter is required")
		return
	}

	prs, err := h.service.GetUserReviews(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, model.ErrNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user_id":       userID,
		"pull_requests": prs,
	})
}

func (h *Handler) GetStatistics(w http.ResponseWriter, r *http.Request) {
	stats, err := h.service.GetStatistics()
	if err != nil {
		writeError(w, http.StatusInternalServerError, model.ErrNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

func (h *Handler) DeactivateTeam(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TeamName string `json:"team_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, model.ErrNotFound, "invalid request body")
		return
	}

	result, err := h.service.DeactivateTeam(req.TeamName)
	if err != nil {
		if err.Error() == model.ErrNotFound {
			writeError(w, http.StatusNotFound, model.ErrNotFound, "team not found")
			return
		}
		writeError(w, http.StatusInternalServerError, model.ErrNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

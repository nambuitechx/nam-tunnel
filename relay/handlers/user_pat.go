package relay_handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	relay_models "github.com/nambuitechx/nam-tunnel/relay/models"
	relay_utils "github.com/nambuitechx/nam-tunnel/relay/utils"
)

const (
	defaultPatLifetimeDays = 90
	maxPatLifetimeDays     = 365
)

type UserPatHandler struct {
	users *relay_models.UserRepository
	pats  *relay_models.UserPatRepository
}

func NewUserPatHandler(users *relay_models.UserRepository, pats *relay_models.UserPatRepository) *UserPatHandler {
	return &UserPatHandler{users: users, pats: pats}
}

type issuePatRequest struct {
	ExpiresInDays *int `json:"expires_in_days"`
}

type issuePatResponse struct {
	Token      string     `json:"token"`
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
}

func (h *UserPatHandler) Issue(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("user_id")

	user, err := h.users.GetByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, relay_models.ErrUserNotFound) {
			relay_utils.WriteJSONError(w, "user not found", http.StatusNotFound)
			return
		}
		relay_utils.WriteJSONError(w, "get user", http.StatusInternalServerError)
		return
	}
	if !user.Active {
		relay_utils.WriteJSONError(w, "user is inactive", http.StatusForbidden)
		return
	}

	var req issuePatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		relay_utils.WriteJSONError(w, "invalid json", http.StatusBadRequest)
		return
	}

	expiresInDays := defaultPatLifetimeDays
	if req.ExpiresInDays != nil {
		if *req.ExpiresInDays <= 0 || *req.ExpiresInDays > maxPatLifetimeDays {
			relay_utils.WriteJSONError(w, "expires_in_days must be between 1 and 365", http.StatusBadRequest)
			return
		}
		expiresInDays = *req.ExpiresInDays
	}

	plainToken, tokenHash, err := relay_utils.GeneratePatToken()
	if err != nil {
		relay_utils.WriteJSONError(w, "issue pat", http.StatusInternalServerError)
		return
	}

	expiresAt := time.Now().UTC().Add(time.Duration(expiresInDays) * 24 * time.Hour)
	pat, err := h.pats.Create(r.Context(), userID, tokenHash, expiresAt)
	if err != nil {
		relay_utils.WriteJSONError(w, "issue pat", http.StatusInternalServerError)
		return
	}

	relay_utils.WriteJSON(w, http.StatusCreated, issuePatResponse{
		Token:      plainToken,
		ID:         pat.ID,
		UserID:     pat.UserID,
		CreatedAt:  pat.CreatedAt,
		ExpiresAt:  pat.ExpiresAt,
		LastUsedAt: pat.LastUsedAt,
	})
}

func (h *UserPatHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("user_id")

	if _, err := h.users.GetByID(r.Context(), userID); err != nil {
		if errors.Is(err, relay_models.ErrUserNotFound) {
			relay_utils.WriteJSONError(w, "user not found", http.StatusNotFound)
			return
		}
		relay_utils.WriteJSONError(w, "get user", http.StatusInternalServerError)
		return
	}

	pats, err := h.pats.ListByUserID(r.Context(), userID)
	if err != nil {
		relay_utils.WriteJSONError(w, "list pats", http.StatusInternalServerError)
		return
	}
	if pats == nil {
		pats = []relay_models.UserPat{}
	}
	relay_utils.WriteJSON(w, http.StatusOK, pats)
}

func (h *UserPatHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("user_id")
	patID := r.PathValue("id")

	pat, err := h.pats.GetByIDForUser(r.Context(), patID, userID)
	if err != nil {
		if errors.Is(err, relay_models.ErrUserPatNotFound) {
			relay_utils.WriteJSONError(w, "pat not found", http.StatusNotFound)
			return
		}
		relay_utils.WriteJSONError(w, "get pat", http.StatusInternalServerError)
		return
	}
	relay_utils.WriteJSON(w, http.StatusOK, pat)
}

func (h *UserPatHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("user_id")
	patID := r.PathValue("id")

	if err := h.pats.DeleteForUser(r.Context(), patID, userID); err != nil {
		if errors.Is(err, relay_models.ErrUserPatNotFound) {
			relay_utils.WriteJSONError(w, "pat not found", http.StatusNotFound)
			return
		}
		relay_utils.WriteJSONError(w, "delete pat", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *UserPatHandler) DeleteAll(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("user_id")

	if _, err := h.users.GetByID(r.Context(), userID); err != nil {
		if errors.Is(err, relay_models.ErrUserNotFound) {
			relay_utils.WriteJSONError(w, "user not found", http.StatusNotFound)
			return
		}
		relay_utils.WriteJSONError(w, "get user", http.StatusInternalServerError)
		return
	}

	if err := h.pats.DeleteByUserID(r.Context(), userID); err != nil {
		relay_utils.WriteJSONError(w, "delete pats", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

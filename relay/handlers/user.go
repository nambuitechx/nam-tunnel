package relay_handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	relay_models "github.com/nambuitechx/nam-tunnel/relay/models"
	relay_utils "github.com/nambuitechx/nam-tunnel/relay/utils"
)

type UserHandler struct {
	users *relay_models.UserRepository
}

func NewUserHandler(users *relay_models.UserRepository) *UserHandler {
	return &UserHandler{users: users}
}

type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type updateUserRequest struct {
	Password *string `json:"password"`
	Active   *bool   `json:"active"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		relay_utils.WriteJSONError(w, "invalid json", http.StatusBadRequest)
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		relay_utils.WriteJSONError(w, "username and password are required", http.StatusBadRequest)
		return
	}

	user, err := h.users.Create(r.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, relay_models.ErrUsernameTaken) {
			relay_utils.WriteJSONError(w, "username already taken", http.StatusConflict)
			return
		}
		relay_utils.WriteJSONError(w, "create user", http.StatusInternalServerError)
		return
	}

	relay_utils.WriteJSON(w, http.StatusCreated, user)
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	users, err := h.users.List(r.Context())
	if err != nil {
		relay_utils.WriteJSONError(w, "list users", http.StatusInternalServerError)
		return
	}
	if users == nil {
		users = []relay_models.User{}
	}
	relay_utils.WriteJSON(w, http.StatusOK, users)
}

func (h *UserHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	user, err := h.users.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, relay_models.ErrUserNotFound) {
			relay_utils.WriteJSONError(w, "user not found", http.StatusNotFound)
			return
		}
		relay_utils.WriteJSONError(w, "get user", http.StatusInternalServerError)
		return
	}
	relay_utils.WriteJSON(w, http.StatusOK, user)
}

func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req updateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		relay_utils.WriteJSONError(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.Password == nil && req.Active == nil {
		relay_utils.WriteJSONError(w, "at least one field is required", http.StatusBadRequest)
		return
	}
	if req.Password != nil && *req.Password == "" {
		relay_utils.WriteJSONError(w, "password cannot be empty", http.StatusBadRequest)
		return
	}

	user, err := h.users.Update(r.Context(), id, req.Password, req.Active)
	if err != nil {
		if errors.Is(err, relay_models.ErrUserNotFound) {
			relay_utils.WriteJSONError(w, "user not found", http.StatusNotFound)
			return
		}
		relay_utils.WriteJSONError(w, "update user", http.StatusInternalServerError)
		return
	}
	relay_utils.WriteJSON(w, http.StatusOK, user)
}

func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.users.Delete(r.Context(), id); err != nil {
		if errors.Is(err, relay_models.ErrUserNotFound) {
			relay_utils.WriteJSONError(w, "user not found", http.StatusNotFound)
			return
		}
		relay_utils.WriteJSONError(w, "delete user", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		relay_utils.WriteJSONError(w, "invalid json", http.StatusBadRequest)
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		relay_utils.WriteJSONError(w, "username and password are required", http.StatusBadRequest)
		return
	}

	user, err := h.users.Authenticate(r.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, relay_models.ErrInvalidCredentials) {
			relay_utils.WriteJSONError(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		relay_utils.WriteJSONError(w, "login", http.StatusInternalServerError)
		return
	}
	relay_utils.WriteJSON(w, http.StatusOK, user)
}

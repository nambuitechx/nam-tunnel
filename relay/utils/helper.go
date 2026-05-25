package relay_utils

import (
	"encoding/json"
	"net/http"
)

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func WriteJSONError(w http.ResponseWriter, message string, status int) {
	WriteJSON(w, status, map[string]string{"error": message})
}

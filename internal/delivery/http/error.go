package handlers

import (
	"encoding/json"
	"net/http"
)

func status(id int) string {
	m := map[int]string{
		200: "OK",
		400: "StatusBadRequest",
		401: "Unauthorized",
		403: "Forbidden",
		404: "StatusNotFound",
		429: "too many requests",
		500: "internal server error",
	}
	value, _ := m[id]
	return value
}

func writeError(w http.ResponseWriter, code int, err error, flag int) {
	if flag == 1 {
		json.NewEncoder(w).Encode(map[string]string{"error": status(code)})
		json.NewEncoder(w).Encode(map[string]string{"message": err.Error()})
	}
	json.NewEncoder(w).Encode(map[string]string{"error": status(code)})
}

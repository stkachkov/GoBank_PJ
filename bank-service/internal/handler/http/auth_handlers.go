package http

import (
	"GoBank_PJ/bank-service/internal/models"
	"GoBank_PJ/bank-service/internal/service"
	"encoding/json"
	"errors"
	"net/http"
)

func (h *Handler) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	var req models.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "Invalid request payload", err)
		return
	}
	if req.Username == "" || req.Email == "" || req.Password == "" {
		sendErrorResponse(w, http.StatusBadRequest, "Username, email, and password are required", nil)
		return
	}
	newUser, err := h.AuthService.Register(r.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrUserExists) {
			sendErrorResponse(w, http.StatusConflict, err.Error(), err)
			return
		}
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to register user", err)
		return
	}
	sendSuccessResponse(w, http.StatusCreated, "User registered successfully", newUser)
}
func (h *Handler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "Invalid request payload", err)
		return
	}
	if req.Username == "" || req.Password == "" {
		sendErrorResponse(w, http.StatusBadRequest, "Username and password are required", nil)
		return
	}
	token, err := h.AuthService.Login(r.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) || errors.Is(err, service.ErrInvalidPassword) {
			sendErrorResponse(w, http.StatusUnauthorized, "Invalid credentials", err)
			return
		}
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to log in", err)
		return
	}
	sendSuccessResponse(w, http.StatusOK, "Login successful", map[string]string{"token": token})
}

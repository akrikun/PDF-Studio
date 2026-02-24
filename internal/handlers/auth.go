package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"pdf-studio/internal/config"
	"pdf-studio/internal/middleware"
	"pdf-studio/internal/services"
)

type AuthHandler struct {
	AuthSvc *services.AuthService
	Cfg     *config.Config
}

func NewAuthHandler(authSvc *services.AuthService, cfg *config.Config) *AuthHandler {
	return &AuthHandler{AuthSvc: authSvc, Cfg: cfg}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.AuthSvc.Authenticate(req.Email, req.Password)
	if err != nil {
		jsonError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	session, err := h.AuthSvc.CreateSession(user.ID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    session.Token,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.Cfg.IsProduction(),
		SameSite: http.SameSiteLaxMode,
		Expires:  session.ExpiresAt,
	})

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"user": map[string]interface{}{
			"id":    user.ID,
			"email": user.Email,
			"role":  user.Role,
		},
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_token")
	if err == nil {
		h.AuthSvc.DestroySession(cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.Cfg.IsProduction(),
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})

	jsonResponse(w, http.StatusOK, map[string]string{"message": "logged out"})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil {
		jsonError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"id":    user.ID,
		"email": user.Email,
		"role":  user.Role,
	})
}

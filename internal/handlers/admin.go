package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"

	"pdf-studio/internal/middleware"
	"pdf-studio/internal/models"
	"pdf-studio/internal/services"
)

type AdminHandler struct {
	DB      *sqlx.DB
	AuthSvc *services.AuthService
}

func NewAdminHandler(db *sqlx.DB, authSvc *services.AuthService) *AdminHandler {
	return &AdminHandler{DB: db, AuthSvc: authSvc}
}

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	var users []models.User
	err := h.DB.Select(&users, "SELECT id, email, role, active, created_at, updated_at FROM users ORDER BY created_at DESC")
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to list users")
		return
	}
	jsonResponse(w, http.StatusOK, users)
}

func (h *AdminHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string      `json:"email"`
		Password string      `json:"password"`
		Role     models.Role `json:"role"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if req.Email == "" || req.Password == "" {
		jsonError(w, http.StatusBadRequest, "email and password required")
		return
	}

	if req.Role == "" {
		req.Role = models.RoleViewer
	}

	if req.Role != models.RoleAdmin && req.Role != models.RoleEditor && req.Role != models.RoleViewer {
		jsonError(w, http.StatusBadRequest, "invalid role")
		return
	}

	user, err := h.AuthSvc.CreateUser(req.Email, req.Password, req.Role)
	if err != nil {
		jsonError(w, http.StatusConflict, "failed to create user: "+err.Error())
		return
	}

	jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"id":    user.ID,
		"email": user.Email,
		"role":  user.Role,
	})
}

func (h *AdminHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	currentUser := middleware.GetUser(r)
	userID := mux.Vars(r)["id"]

	var req struct {
		Email    *string      `json:"email"`
		Password *string      `json:"password"`
		Role     *models.Role `json:"role"`
		Active   *bool        `json:"active"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request")
		return
	}

	// Check user exists
	var user models.User
	if err := h.DB.Get(&user, "SELECT * FROM users WHERE id=$1", userID); err != nil {
		jsonError(w, http.StatusNotFound, "user not found")
		return
	}

	// Prevent self-deactivation and self-role-downgrade
	if currentUser != nil && currentUser.ID == userID {
		if req.Active != nil && !*req.Active {
			jsonError(w, http.StatusBadRequest, "you cannot deactivate yourself")
			return
		}
		if req.Role != nil && *req.Role != models.RoleAdmin {
			jsonError(w, http.StatusBadRequest, "you cannot remove your own admin role")
			return
		}
	}

	if req.Email != nil {
		h.DB.Exec("UPDATE users SET email=$1, updated_at=now() WHERE id=$2", *req.Email, userID)
	}
	if req.Role != nil {
		if *req.Role != models.RoleAdmin && *req.Role != models.RoleEditor && *req.Role != models.RoleViewer {
			jsonError(w, http.StatusBadRequest, "invalid role")
			return
		}
		h.DB.Exec("UPDATE users SET role=$1, updated_at=now() WHERE id=$2", *req.Role, userID)
		// Invalidate sessions on role change
		h.AuthSvc.DestroyUserSessions(userID)
	}
	if req.Active != nil {
		h.DB.Exec("UPDATE users SET active=$1, updated_at=now() WHERE id=$2", *req.Active, userID)
		if !*req.Active {
			h.AuthSvc.DestroyUserSessions(userID)
		}
	}
	if req.Password != nil && *req.Password != "" {
		hash, err := h.AuthSvc.HashPassword(*req.Password)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to hash password")
			return
		}
		h.DB.Exec("UPDATE users SET password_hash=$1, updated_at=now() WHERE id=$2", hash, userID)
	}

	jsonResponse(w, http.StatusOK, map[string]string{"message": "user updated"})
}

func (h *AdminHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	userID := mux.Vars(r)["id"]

	var user models.User
	if err := h.DB.Get(&user, "SELECT id, email, role, active, created_at, updated_at FROM users WHERE id=$1", userID); err != nil {
		jsonError(w, http.StatusNotFound, "user not found")
		return
	}

	jsonResponse(w, http.StatusOK, user)
}

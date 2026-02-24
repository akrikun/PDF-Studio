package handlers

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"

	"pdf-studio/internal/middleware"
	"pdf-studio/internal/services"
)

type ViewHandler struct {
	templates *template.Template
	AuthSvc   *services.AuthService
}

func NewViewHandler(templatesDir string, authSvc *services.AuthService) *ViewHandler {
	tmpl := template.Must(template.ParseGlob(filepath.Join(templatesDir, "*.html")))
	return &ViewHandler{templates: tmpl, AuthSvc: authSvc}
}

func (h *ViewHandler) render(w http.ResponseWriter, name string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("[view] template error: %v", err)
		http.Error(w, "Internal Server Error", 500)
	}
}

func (h *ViewHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	// If already authenticated, redirect to dashboard
	cookie, err := r.Cookie("session_token")
	if err == nil && cookie.Value != "" {
		if _, _, err := h.AuthSvc.ValidateSession(cookie.Value); err == nil {
			http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
			return
		}
	}
	h.render(w, "login.html", nil)
}

func (h *ViewHandler) DashboardPage(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	session := middleware.GetSession(r)
	csrf, _ := h.AuthSvc.GenerateCSRFToken(session.ID)
	h.render(w, "dashboard.html", map[string]interface{}{
		"User": user,
		"CSRF": csrf,
	})
}

func (h *ViewHandler) EditorPage(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	session := middleware.GetSession(r)
	csrf, _ := h.AuthSvc.GenerateCSRFToken(session.ID)
	h.render(w, "editor.html", map[string]interface{}{
		"User": user,
		"CSRF": csrf,
	})
}

func (h *ViewHandler) AdminPage(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	session := middleware.GetSession(r)
	csrf, _ := h.AuthSvc.GenerateCSRFToken(session.ID)
	h.render(w, "admin.html", map[string]interface{}{
		"User": user,
		"CSRF": csrf,
	})
}

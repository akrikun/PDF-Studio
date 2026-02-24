package main

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"

	"pdf-studio/internal/config"
	"pdf-studio/internal/database"
	"pdf-studio/internal/handlers"
	"pdf-studio/internal/middleware"
	"pdf-studio/internal/models"
	"pdf-studio/internal/services"
)

func main() {
	cfg := config.Load()
	log.Printf("[startup] PDF Studio starting on port %s (env=%s)", cfg.AppPort, cfg.AppEnv)

	// Database
	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("[startup] database connection failed: %v", err)
	}
	defer db.Close()
	log.Println("[startup] database connected")

	// Migrations
	if err := database.RunMigrations(db, "migrations"); err != nil {
		log.Fatalf("[startup] migrations failed: %v", err)
	}
	log.Println("[startup] migrations applied")

	// Services
	authSvc := services.NewAuthService(db)
	storageSvc := services.NewStorageService(db, cfg)
	pdfSvc := services.NewPDFService(cfg, storageSvc)

	// Seed admin
	if err := authSvc.SeedAdmin(cfg.AdminEmail, cfg.AdminPassword); err != nil {
		log.Printf("[startup] admin seed error: %v", err)
	} else {
		log.Printf("[startup] admin user ready: %s", cfg.AdminEmail)
	}

	// Clean expired sessions on startup
	authSvc.CleanExpiredSessions()

	// Handlers
	authHandler := handlers.NewAuthHandler(authSvc, cfg)
	adminHandler := handlers.NewAdminHandler(db, authSvc)
	docHandler := handlers.NewDocumentHandler(db, storageSvc, pdfSvc)
	pageHandler := handlers.NewPageHandler(db)
	fileHandler := handlers.NewFileHandler(storageSvc, cfg)
	viewHandler := handlers.NewViewHandler("web/templates", authSvc)

	// Router
	r := mux.NewRouter()

	// Global middleware
	r.Use(middleware.Logging)
	r.Use(middleware.SecureHeaders)

	// Static files
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	// Public pages
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}).Methods("GET")
	r.HandleFunc("/login", viewHandler.LoginPage).Methods("GET")

	// Auth API (public)
	r.HandleFunc("/auth/login", authHandler.Login).Methods("POST")

	// Auth API (authenticated)
	authAPI := r.PathPrefix("").Subrouter()
	authAPI.Use(middleware.APIAuthRequired(authSvc))
	authAPI.HandleFunc("/auth/logout", authHandler.Logout).Methods("POST")
	authAPI.HandleFunc("/me", authHandler.Me).Methods("GET")

	// Authenticated pages
	pages := r.PathPrefix("").Subrouter()
	pages.Use(middleware.AuthRequired(authSvc))
	pages.HandleFunc("/dashboard", viewHandler.DashboardPage).Methods("GET")
	pages.HandleFunc("/editor/{id}", viewHandler.EditorPage).Methods("GET")

	// Admin pages
	adminPages := r.PathPrefix("/admin").Subrouter()
	adminPages.Use(middleware.AuthRequired(authSvc))
	adminPages.Use(middleware.RoleRequired(models.RoleAdmin))
	adminPages.HandleFunc("", viewHandler.AdminPage).Methods("GET")

	// Admin API
	adminAPI := r.PathPrefix("/admin").Subrouter()
	adminAPI.Use(middleware.APIAuthRequired(authSvc))
	adminAPI.Use(middleware.RoleRequired(models.RoleAdmin))
	adminAPI.HandleFunc("/users", adminHandler.ListUsers).Methods("GET")
	adminAPI.HandleFunc("/users", adminHandler.CreateUser).Methods("POST")
	adminAPI.HandleFunc("/users/{id}", adminHandler.GetUser).Methods("GET")
	adminAPI.HandleFunc("/users/{id}", adminHandler.UpdateUser).Methods("PUT")

	// Documents API
	docAPI := r.PathPrefix("/api").Subrouter()
	docAPI.Use(middleware.APIAuthRequired(authSvc))

	docAPI.HandleFunc("/documents", docHandler.ListDocuments).Methods("GET")
	docAPI.HandleFunc("/documents", docHandler.CreateDocument).Methods("POST")
	docAPI.HandleFunc("/documents/{id}", docHandler.GetDocument).Methods("GET")
	docAPI.HandleFunc("/documents/{id}", docHandler.UpdateDocument).Methods("PUT")
	docAPI.HandleFunc("/documents/{id}", docHandler.DeleteDocument).Methods("DELETE")
	docAPI.HandleFunc("/documents/{id}/versions", docHandler.ListVersions).Methods("GET")
	docAPI.HandleFunc("/documents/{id}/versions", docHandler.CreateVersion).Methods("POST")
	docAPI.HandleFunc("/documents/{id}/generate-pdf", docHandler.GeneratePDF).Methods("POST")
	docAPI.HandleFunc("/documents/{id}/files", fileHandler.ListFiles).Methods("GET")

	// Pages API
	docAPI.HandleFunc("/versions/{versionId}/pages", pageHandler.ListPages).Methods("GET")
	docAPI.HandleFunc("/versions/{versionId}/pages", pageHandler.AddPage).Methods("POST")
	docAPI.HandleFunc("/versions/{versionId}/pages/reorder", pageHandler.ReorderPages).Methods("PUT")
	docAPI.HandleFunc("/pages/{pageId}", pageHandler.GetPage).Methods("GET")
	docAPI.HandleFunc("/pages/{pageId}", pageHandler.UpdatePage).Methods("PUT")
	docAPI.HandleFunc("/pages/{pageId}", pageHandler.DeletePage).Methods("DELETE")

	// Files API
	docAPI.HandleFunc("/files/upload", fileHandler.Upload).Methods("POST")
	docAPI.HandleFunc("/files/{id}/download", fileHandler.Download).Methods("GET")

	// Start server
	addr := ":" + cfg.AppPort
	log.Printf("[startup] listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("[startup] server error: %v", err)
	}
}

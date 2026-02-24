package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"pdf-studio/internal/middleware"
	"pdf-studio/internal/models"
)

func TestRoleRequired_AllowsMatchingRole(t *testing.T) {
	handler := middleware.RoleRequired(models.RoleAdmin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	user := &models.User{Role: models.RoleAdmin}
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, user)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
}

func TestRoleRequired_DeniesNonMatchingRole(t *testing.T) {
	handler := middleware.RoleRequired(models.RoleAdmin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	user := &models.User{Role: models.RoleEditor}
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, user)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403, got %d", rr.Code)
	}
}

func TestRoleRequired_DeniesNoUser(t *testing.T) {
	handler := middleware.RoleRequired(models.RoleAdmin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", rr.Code)
	}
}

func TestRoleRequired_AllowsMultipleRoles(t *testing.T) {
	handler := middleware.RoleRequired(models.RoleAdmin, models.RoleEditor)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Editor should pass
	req := httptest.NewRequest("GET", "/", nil)
	user := &models.User{Role: models.RoleEditor}
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, user)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Editor expected 200, got %d", rr.Code)
	}

	// Viewer should fail
	req2 := httptest.NewRequest("GET", "/", nil)
	viewer := &models.User{Role: models.RoleViewer}
	ctx2 := context.WithValue(req2.Context(), middleware.UserContextKey, viewer)
	req2 = req2.WithContext(ctx2)

	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusForbidden {
		t.Errorf("Viewer expected 403, got %d", rr2.Code)
	}
}

func TestGetUser_ReturnsNilWithoutContext(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	user := middleware.GetUser(req)
	if user != nil {
		t.Error("Expected nil user without context")
	}
}

func TestGetUser_ReturnsUserFromContext(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	expected := &models.User{ID: "123", Email: "test@example.com", Role: models.RoleEditor}
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, expected)
	req = req.WithContext(ctx)

	user := middleware.GetUser(req)
	if user == nil {
		t.Fatal("Expected non-nil user")
	}
	if user.ID != expected.ID {
		t.Errorf("Expected user ID %s, got %s", expected.ID, user.ID)
	}
}

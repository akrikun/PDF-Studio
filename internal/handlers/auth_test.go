package handlers_test

import (
	"testing"

	"pdf-studio/internal/models"
)

func TestUserRoles(t *testing.T) {
	tests := []struct {
		role    models.Role
		isAdmin bool
		canEdit bool
	}{
		{models.RoleAdmin, true, true},
		{models.RoleEditor, false, true},
		{models.RoleViewer, false, false},
	}

	for _, tt := range tests {
		u := &models.User{Role: tt.role}
		if u.IsAdmin() != tt.isAdmin {
			t.Errorf("Role %s: IsAdmin() = %v, want %v", tt.role, u.IsAdmin(), tt.isAdmin)
		}
		if u.CanEdit() != tt.canEdit {
			t.Errorf("Role %s: CanEdit() = %v, want %v", tt.role, u.CanEdit(), tt.canEdit)
		}
	}
}

func TestRoleConstants(t *testing.T) {
	if models.RoleAdmin != "admin" {
		t.Errorf("RoleAdmin = %q, want %q", models.RoleAdmin, "admin")
	}
	if models.RoleEditor != "editor" {
		t.Errorf("RoleEditor = %q, want %q", models.RoleEditor, "editor")
	}
	if models.RoleViewer != "viewer" {
		t.Errorf("RoleViewer = %q, want %q", models.RoleViewer, "viewer")
	}
}

func TestViewerCannotEdit(t *testing.T) {
	viewer := &models.User{Role: models.RoleViewer}
	if viewer.CanEdit() {
		t.Error("Viewer should not be able to edit")
	}
	if viewer.IsAdmin() {
		t.Error("Viewer should not be admin")
	}
}

func TestEditorCanEditButNotAdmin(t *testing.T) {
	editor := &models.User{Role: models.RoleEditor}
	if !editor.CanEdit() {
		t.Error("Editor should be able to edit")
	}
	if editor.IsAdmin() {
		t.Error("Editor should not be admin")
	}
}

func TestAdminCanEditAndIsAdmin(t *testing.T) {
	admin := &models.User{Role: models.RoleAdmin}
	if !admin.CanEdit() {
		t.Error("Admin should be able to edit")
	}
	if !admin.IsAdmin() {
		t.Error("Admin should be admin")
	}
}

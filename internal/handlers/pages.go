package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"

	"pdf-studio/internal/middleware"
	"pdf-studio/internal/models"
)

type PageHandler struct {
	DB *sqlx.DB
}

func NewPageHandler(db *sqlx.DB) *PageHandler {
	return &PageHandler{DB: db}
}

// checkVersionOwnership verifies the user owns the document that contains the version
func (h *PageHandler) checkVersionOwnership(versionID, userID string) bool {
	var count int
	err := h.DB.Get(&count, `
		SELECT COUNT(*) FROM document_versions dv
		JOIN documents d ON d.id = dv.document_id
		WHERE dv.id = $1 AND d.owner_id = $2`, versionID, userID)
	return err == nil && count > 0
}

func (h *PageHandler) ListPages(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	versionID := mux.Vars(r)["versionId"]

	if !h.checkVersionOwnership(versionID, user.ID) {
		jsonError(w, http.StatusNotFound, "version not found")
		return
	}

	var pages []models.Page
	err := h.DB.Select(&pages, "SELECT * FROM pages WHERE version_id=$1 ORDER BY page_index", versionID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to list pages")
		return
	}
	if pages == nil {
		pages = []models.Page{}
	}
	jsonResponse(w, http.StatusOK, pages)
}

func (h *PageHandler) GetPage(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	pageID := mux.Vars(r)["pageId"]

	var page models.Page
	err := h.DB.Get(&page, "SELECT * FROM pages WHERE id=$1", pageID)
	if err != nil {
		jsonError(w, http.StatusNotFound, "page not found")
		return
	}

	if !h.checkVersionOwnership(page.VersionID, user.ID) {
		jsonError(w, http.StatusNotFound, "page not found")
		return
	}

	jsonResponse(w, http.StatusOK, page)
}

func (h *PageHandler) UpdatePage(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if !user.CanEdit() {
		jsonError(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	pageID := mux.Vars(r)["pageId"]

	var page models.Page
	err := h.DB.Get(&page, "SELECT * FROM pages WHERE id=$1", pageID)
	if err != nil {
		jsonError(w, http.StatusNotFound, "page not found")
		return
	}

	if !h.checkVersionOwnership(page.VersionID, user.ID) {
		jsonError(w, http.StatusNotFound, "page not found")
		return
	}

	var req struct {
		ContentHTML string `json:"content_html"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request")
		return
	}

	_, err = h.DB.Exec("UPDATE pages SET content_html=$1, updated_at=now() WHERE id=$2", req.ContentHTML, pageID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to update page")
		return
	}

	// Update document timestamp
	h.DB.Exec(`UPDATE documents SET updated_at=now() WHERE id=(
		SELECT document_id FROM document_versions WHERE id=$1
	)`, page.VersionID)

	jsonResponse(w, http.StatusOK, map[string]string{"message": "page updated"})
}

func (h *PageHandler) AddPage(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if !user.CanEdit() {
		jsonError(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	versionID := mux.Vars(r)["versionId"]

	if !h.checkVersionOwnership(versionID, user.ID) {
		jsonError(w, http.StatusNotFound, "version not found")
		return
	}

	var req struct {
		ContentHTML string `json:"content_html"`
		AfterIndex  *int   `json:"after_index"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.ContentHTML = "<p></p>"
	}
	if req.ContentHTML == "" {
		req.ContentHTML = "<p></p>"
	}

	// Get max page index
	var maxIdx int
	h.DB.Get(&maxIdx, "SELECT COALESCE(MAX(page_index), -1) FROM pages WHERE version_id=$1", versionID)

	newIdx := maxIdx + 1
	if req.AfterIndex != nil && *req.AfterIndex < maxIdx {
		newIdx = *req.AfterIndex + 1
		// Shift pages after the insertion point
		h.DB.Exec("UPDATE pages SET page_index = page_index + 1 WHERE version_id=$1 AND page_index >= $2", versionID, newIdx)
	}

	var page models.Page
	err := h.DB.QueryRow(
		"INSERT INTO pages(version_id, page_index, content_html) VALUES($1, $2, $3) RETURNING id, version_id, page_index, content_html, created_at, updated_at",
		versionID, newIdx, req.ContentHTML,
	).Scan(&page.ID, &page.VersionID, &page.PageIndex, &page.ContentHTML, &page.CreatedAt, &page.UpdatedAt)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to add page")
		return
	}

	jsonResponse(w, http.StatusCreated, page)
}

func (h *PageHandler) DeletePage(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if !user.CanEdit() {
		jsonError(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	pageID := mux.Vars(r)["pageId"]

	var page models.Page
	err := h.DB.Get(&page, "SELECT * FROM pages WHERE id=$1", pageID)
	if err != nil {
		jsonError(w, http.StatusNotFound, "page not found")
		return
	}

	if !h.checkVersionOwnership(page.VersionID, user.ID) {
		jsonError(w, http.StatusNotFound, "page not found")
		return
	}

	// Don't delete last page
	var count int
	h.DB.Get(&count, "SELECT COUNT(*) FROM pages WHERE version_id=$1", page.VersionID)
	if count <= 1 {
		jsonError(w, http.StatusBadRequest, "cannot delete the last page")
		return
	}

	h.DB.Exec("DELETE FROM pages WHERE id=$1", pageID)
	// Reindex remaining pages
	h.DB.Exec(`
		WITH reindexed AS (
			SELECT id, ROW_NUMBER() OVER (ORDER BY page_index) - 1 AS new_index
			FROM pages WHERE version_id=$1
		)
		UPDATE pages SET page_index = reindexed.new_index
		FROM reindexed WHERE pages.id = reindexed.id`, page.VersionID)

	jsonResponse(w, http.StatusOK, map[string]string{"message": "page deleted"})
}

func (h *PageHandler) ReorderPages(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if !user.CanEdit() {
		jsonError(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	versionID := mux.Vars(r)["versionId"]

	if !h.checkVersionOwnership(versionID, user.ID) {
		jsonError(w, http.StatusNotFound, "version not found")
		return
	}

	var req struct {
		PageIDs []string `json:"page_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request")
		return
	}

	tx, err := h.DB.Beginx()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback()

	for i, pid := range req.PageIDs {
		tx.Exec("UPDATE pages SET page_index=$1 WHERE id=$2 AND version_id=$3", i, pid, versionID)
	}

	if err := tx.Commit(); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to reorder pages")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"message": "pages reordered"})
}

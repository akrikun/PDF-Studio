package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"

	"pdf-studio/internal/middleware"
	"pdf-studio/internal/models"
	"pdf-studio/internal/services"
)

type DocumentHandler struct {
	DB         *sqlx.DB
	StorageSvc *services.StorageService
	PDFSvc     *services.PDFService
}

func NewDocumentHandler(db *sqlx.DB, storageSvc *services.StorageService, pdfSvc *services.PDFService) *DocumentHandler {
	return &DocumentHandler{DB: db, StorageSvc: storageSvc, PDFSvc: pdfSvc}
}

func (h *DocumentHandler) ListDocuments(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	search := r.URL.Query().Get("q")

	var docs []models.Document
	var err error

	if search != "" {
		err = h.DB.Select(&docs, "SELECT * FROM documents WHERE owner_id=$1 AND title ILIKE '%' || $2 || '%' ORDER BY updated_at DESC", user.ID, search)
	} else {
		err = h.DB.Select(&docs, "SELECT * FROM documents WHERE owner_id=$1 ORDER BY updated_at DESC", user.ID)
	}

	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to list documents")
		return
	}
	if docs == nil {
		docs = []models.Document{}
	}
	jsonResponse(w, http.StatusOK, docs)
}

func (h *DocumentHandler) CreateDocument(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if !user.CanEdit() {
		jsonError(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	var req struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Title == "" {
		req.Title = "Untitled Document"
	}

	tx, err := h.DB.Beginx()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback()

	var doc models.Document
	err = tx.QueryRow(
		"INSERT INTO documents(owner_id, title) VALUES($1, $2) RETURNING id, owner_id, title, status, created_at, updated_at",
		user.ID, req.Title,
	).Scan(&doc.ID, &doc.OwnerID, &doc.Title, &doc.Status, &doc.CreatedAt, &doc.UpdatedAt)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to create document")
		return
	}

	var version models.DocumentVersion
	err = tx.QueryRow(
		"INSERT INTO document_versions(document_id, version_num) VALUES($1, 1) RETURNING id, document_id, version_num, created_at",
		doc.ID,
	).Scan(&version.ID, &version.DocumentID, &version.VersionNum, &version.CreatedAt)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to create version")
		return
	}

	_, err = tx.Exec(
		"INSERT INTO pages(version_id, page_index, content_html) VALUES($1, 0, $2)",
		version.ID, "<p>Start typing here...</p>",
	)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to create page")
		return
	}

	if err := tx.Commit(); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to commit")
		return
	}

	jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"document": doc,
		"version":  version,
	})
}

func (h *DocumentHandler) GetDocument(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	docID := mux.Vars(r)["id"]

	var doc models.Document
	err := h.DB.Get(&doc, "SELECT * FROM documents WHERE id=$1 AND owner_id=$2", docID, user.ID)
	if err != nil {
		jsonError(w, http.StatusNotFound, "document not found")
		return
	}

	var versions []models.DocumentVersion
	h.DB.Select(&versions, "SELECT * FROM document_versions WHERE document_id=$1 ORDER BY version_num DESC", docID)

	var files []models.File
	h.DB.Select(&files,
		`SELECT id, owner_id, document_id, version_id, kind, original_name, storage_path, mime, size, checksum, created_at
		 FROM files WHERE document_id=$1 AND owner_id=$2 ORDER BY created_at DESC`, docID, user.ID)

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"document": doc,
		"versions": versions,
		"files":    files,
	})
}

func (h *DocumentHandler) UpdateDocument(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if !user.CanEdit() {
		jsonError(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	docID := mux.Vars(r)["id"]

	var count int
	h.DB.Get(&count, "SELECT COUNT(*) FROM documents WHERE id=$1 AND owner_id=$2", docID, user.ID)
	if count == 0 {
		jsonError(w, http.StatusNotFound, "document not found")
		return
	}

	var req struct {
		Title  *string `json:"title"`
		Status *string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if req.Title != nil {
		h.DB.Exec("UPDATE documents SET title=$1, updated_at=now() WHERE id=$2", *req.Title, docID)
	}
	if req.Status != nil {
		h.DB.Exec("UPDATE documents SET status=$1, updated_at=now() WHERE id=$2", *req.Status, docID)
	}

	jsonResponse(w, http.StatusOK, map[string]string{"message": "document updated"})
}

func (h *DocumentHandler) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if !user.CanEdit() {
		jsonError(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	docID := mux.Vars(r)["id"]

	result, err := h.DB.Exec("DELETE FROM documents WHERE id=$1 AND owner_id=$2", docID, user.ID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to delete document")
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		jsonError(w, http.StatusNotFound, "document not found")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"message": "document deleted"})
}

func (h *DocumentHandler) ListVersions(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	docID := mux.Vars(r)["id"]

	var count int
	h.DB.Get(&count, "SELECT COUNT(*) FROM documents WHERE id=$1 AND owner_id=$2", docID, user.ID)
	if count == 0 {
		jsonError(w, http.StatusNotFound, "document not found")
		return
	}

	var versions []models.DocumentVersion
	h.DB.Select(&versions, "SELECT * FROM document_versions WHERE document_id=$1 ORDER BY version_num DESC", docID)

	jsonResponse(w, http.StatusOK, versions)
}

func (h *DocumentHandler) CreateVersion(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if !user.CanEdit() {
		jsonError(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	docID := mux.Vars(r)["id"]

	var count int
	h.DB.Get(&count, "SELECT COUNT(*) FROM documents WHERE id=$1 AND owner_id=$2", docID, user.ID)
	if count == 0 {
		jsonError(w, http.StatusNotFound, "document not found")
		return
	}

	tx, err := h.DB.Beginx()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback()

	var maxVer int
	tx.Get(&maxVer, "SELECT COALESCE(MAX(version_num), 0) FROM document_versions WHERE document_id=$1", docID)

	newVerNum := maxVer + 1

	var version models.DocumentVersion
	err = tx.QueryRow(
		"INSERT INTO document_versions(document_id, version_num) VALUES($1, $2) RETURNING id, document_id, version_num, created_at",
		docID, newVerNum,
	).Scan(&version.ID, &version.DocumentID, &version.VersionNum, &version.CreatedAt)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to create version")
		return
	}

	// Copy pages from previous version
	var latestVersionID string
	err = tx.Get(&latestVersionID, "SELECT id FROM document_versions WHERE document_id=$1 AND version_num=$2", docID, maxVer)
	if err == nil {
		var pages []models.Page
		tx.Select(&pages, "SELECT * FROM pages WHERE version_id=$1 ORDER BY page_index", latestVersionID)
		for _, p := range pages {
			tx.Exec("INSERT INTO pages(version_id, page_index, content_html) VALUES($1, $2, $3)",
				version.ID, p.PageIndex, p.ContentHTML)
		}
	} else {
		tx.Exec("INSERT INTO pages(version_id, page_index, content_html) VALUES($1, 0, '<p></p>')", version.ID)
	}

	if err := tx.Commit(); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to commit")
		return
	}

	h.DB.Exec("UPDATE documents SET updated_at=now() WHERE id=$1", docID)

	jsonResponse(w, http.StatusCreated, version)
}

func (h *DocumentHandler) GeneratePDF(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if !user.CanEdit() {
		jsonError(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	docID := mux.Vars(r)["id"]

	var doc models.Document
	err := h.DB.Get(&doc, "SELECT * FROM documents WHERE id=$1 AND owner_id=$2", docID, user.ID)
	if err != nil {
		jsonError(w, http.StatusNotFound, "document not found")
		return
	}

	var version models.DocumentVersion
	err = h.DB.Get(&version, "SELECT * FROM document_versions WHERE document_id=$1 ORDER BY version_num DESC LIMIT 1", docID)
	if err != nil {
		jsonError(w, http.StatusNotFound, "no versions found")
		return
	}

	var pages []models.Page
	err = h.DB.Select(&pages, "SELECT * FROM pages WHERE version_id=$1 ORDER BY page_index", version.ID)
	if err != nil || len(pages) == 0 {
		jsonError(w, http.StatusBadRequest, "no pages found")
		return
	}

	pdfBytes, err := h.PDFSvc.GeneratePDF(pages)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "PDF generation failed: "+err.Error())
		return
	}

	file, err := h.StorageSvc.SaveFile(
		user.ID, &docID, &version.ID,
		models.FileKindGeneratedPDF,
		doc.Title+".pdf",
		"application/pdf",
		bytes.NewReader(pdfBytes),
	)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to save PDF: "+err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"message": "PDF generated successfully",
		"file":    file,
	})
}

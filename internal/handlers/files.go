package handlers

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"pdf-studio/internal/config"
	"pdf-studio/internal/middleware"
	"pdf-studio/internal/models"
	"pdf-studio/internal/services"
)

type FileHandler struct {
	StorageSvc *services.StorageService
	Cfg        *config.Config
}

func NewFileHandler(storageSvc *services.StorageService, cfg *config.Config) *FileHandler {
	return &FileHandler{StorageSvc: storageSvc, Cfg: cfg}
}

func (h *FileHandler) Upload(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if !user.CanEdit() {
		jsonError(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.Cfg.MaxUploadSize)
	if err := r.ParseMultipartForm(h.Cfg.MaxUploadSize); err != nil {
		jsonError(w, http.StatusBadRequest, "file too large")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "no file provided")
		return
	}
	defer file.Close()

	docID := r.FormValue("document_id")
	versionID := r.FormValue("version_id")

	var docPtr, verPtr *string
	if docID != "" {
		docPtr = &docID
	}
	if versionID != "" {
		verPtr = &versionID
	}

	mime := header.Header.Get("Content-Type")
	if mime == "" {
		mime = "application/octet-stream"
	}

	kind := models.FileKindUpload
	if r.FormValue("kind") == "asset" {
		kind = models.FileKindAsset
	}

	saved, err := h.StorageSvc.SaveFile(user.ID, docPtr, verPtr, kind, header.Filename, mime, file)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to save file: "+err.Error())
		return
	}

	jsonResponse(w, http.StatusCreated, saved)
}

func (h *FileHandler) Download(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	fileID := mux.Vars(r)["id"]

	f, err := h.StorageSvc.GetFileByID(fileID, user.ID)
	if err != nil {
		jsonError(w, http.StatusNotFound, "file not found")
		return
	}

	data, err := h.StorageSvc.GetFileContent(f)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to read file")
		return
	}

	w.Header().Set("Content-Type", f.Mime)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+f.OriginalName+"\"")
	w.Header().Set("Content-Length", strconv.FormatInt(f.Size, 10))
	w.Write(data)
}

func (h *FileHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	docID := mux.Vars(r)["id"]

	files, err := h.StorageSvc.ListFilesByDocument(docID, user.ID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to list files")
		return
	}
	if files == nil {
		files = []models.File{}
	}
	jsonResponse(w, http.StatusOK, files)
}

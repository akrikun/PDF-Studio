package services

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"pdf-studio/internal/config"
	"pdf-studio/internal/models"
)

type StorageService struct {
	DB  *sqlx.DB
	Cfg *config.Config
}

func NewStorageService(db *sqlx.DB, cfg *config.Config) *StorageService {
	return &StorageService{DB: db, Cfg: cfg}
}

func (s *StorageService) SaveFile(ownerID string, docID *string, versionID *string, kind models.FileKind, originalName string, mime string, reader io.Reader) (*models.File, error) {
	// Read content
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	size := int64(len(data))
	checksum := computeChecksum(data)

	var storagePath string

	if s.Cfg.StorageMode == "db" && size <= s.Cfg.MaxUploadSize {
		storagePath = "db://" + uuid.New().String()
		f := &models.File{
			OwnerID:      ownerID,
			DocumentID:   docID,
			VersionID:    versionID,
			Kind:         kind,
			OriginalName: originalName,
			StoragePath:  storagePath,
			Mime:         mime,
			Size:         size,
			Checksum:     checksum,
		}

		err = s.DB.QueryRow(
			`INSERT INTO files(owner_id, document_id, version_id, kind, original_name, storage_path, mime, size, checksum, file_data)
			 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id, created_at`,
			f.OwnerID, f.DocumentID, f.VersionID, f.Kind, f.OriginalName, f.StoragePath, f.Mime, f.Size, f.Checksum, data,
		).Scan(&f.ID, &f.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("save file to db: %w", err)
		}
		return f, nil
	}

	// Filesystem storage
	var subdir string
	switch kind {
	case models.FileKindGeneratedPDF:
		subdir = "pdfs"
	default:
		subdir = "uploads"
	}

	dir := filepath.Join(s.Cfg.StoragePath, subdir, ownerID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create dir: %w", err)
	}

	ext := filepath.Ext(originalName)
	filename := uuid.New().String() + ext
	storagePath = filepath.Join(subdir, ownerID, filename)
	fullPath := filepath.Join(s.Cfg.StoragePath, storagePath)

	if err := os.WriteFile(fullPath, data, 0644); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}

	f := &models.File{
		OwnerID:      ownerID,
		DocumentID:   docID,
		VersionID:    versionID,
		Kind:         kind,
		OriginalName: originalName,
		StoragePath:  storagePath,
		Mime:         mime,
		Size:         size,
		Checksum:     checksum,
	}

	err = s.DB.QueryRow(
		`INSERT INTO files(owner_id, document_id, version_id, kind, original_name, storage_path, mime, size, checksum)
		 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id, created_at`,
		f.OwnerID, f.DocumentID, f.VersionID, f.Kind, f.OriginalName, f.StoragePath, f.Mime, f.Size, f.Checksum,
	).Scan(&f.ID, &f.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("save file record: %w", err)
	}

	return f, nil
}

func (s *StorageService) GetFileContent(f *models.File) ([]byte, error) {
	if s.Cfg.StorageMode == "db" && len(f.StoragePath) > 5 && f.StoragePath[:5] == "db://" {
		var data []byte
		err := s.DB.Get(&data, "SELECT file_data FROM files WHERE id=$1", f.ID)
		if err != nil {
			return nil, fmt.Errorf("read file from db: %w", err)
		}
		return data, nil
	}

	fullPath := filepath.Join(s.Cfg.StoragePath, f.StoragePath)
	return os.ReadFile(fullPath)
}

func (s *StorageService) GetFileByID(fileID, ownerID string) (*models.File, error) {
	var f models.File
	err := s.DB.Get(&f, "SELECT id, owner_id, document_id, version_id, kind, original_name, storage_path, mime, size, checksum, created_at FROM files WHERE id=$1 AND owner_id=$2", fileID, ownerID)
	if err != nil {
		return nil, fmt.Errorf("file not found")
	}
	return &f, nil
}

func (s *StorageService) ListFilesByDocument(docID, ownerID string) ([]models.File, error) {
	var files []models.File
	err := s.DB.Select(&files, "SELECT id, owner_id, document_id, version_id, kind, original_name, storage_path, mime, size, checksum, created_at FROM files WHERE document_id=$1 AND owner_id=$2 ORDER BY created_at DESC", docID, ownerID)
	if err != nil {
		return nil, err
	}
	return files, nil
}

// GetFileByIDInternal returns a file without ownership check — for internal use only (PDF generation).
func (s *StorageService) GetFileByIDInternal(fileID string) (*models.File, error) {
	var f models.File
	err := s.DB.Get(&f, "SELECT id, owner_id, document_id, version_id, kind, original_name, storage_path, mime, size, checksum, created_at FROM files WHERE id=$1", fileID)
	if err != nil {
		return nil, fmt.Errorf("file not found")
	}
	return &f, nil
}

func computeChecksum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

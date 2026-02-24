package models

import "time"

type FileKind string

const (
	FileKindUpload      FileKind = "upload"
	FileKindAsset       FileKind = "asset"
	FileKindGeneratedPDF FileKind = "generated_pdf"
)

type File struct {
	ID           string    `db:"id" json:"id"`
	OwnerID      string    `db:"owner_id" json:"owner_id"`
	DocumentID   *string   `db:"document_id" json:"document_id,omitempty"`
	VersionID    *string   `db:"version_id" json:"version_id,omitempty"`
	Kind         FileKind  `db:"kind" json:"kind"`
	OriginalName string    `db:"original_name" json:"original_name"`
	StoragePath  string    `db:"storage_path" json:"storage_path"`
	Mime         string    `db:"mime" json:"mime"`
	Size         int64     `db:"size" json:"size"`
	Checksum     string    `db:"checksum" json:"checksum"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
}

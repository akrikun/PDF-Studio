package models

import "time"

type Document struct {
	ID        string    `db:"id" json:"id"`
	OwnerID   string    `db:"owner_id" json:"owner_id"`
	Title     string    `db:"title" json:"title"`
	Status    string    `db:"status" json:"status"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

type DocumentVersion struct {
	ID         string    `db:"id" json:"id"`
	DocumentID string    `db:"document_id" json:"document_id"`
	VersionNum int       `db:"version_num" json:"version_num"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
}

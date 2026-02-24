package models

import "time"

type Page struct {
	ID          string    `db:"id" json:"id"`
	VersionID   string    `db:"version_id" json:"version_id"`
	PageIndex   int       `db:"page_index" json:"page_index"`
	ContentHTML string    `db:"content_html" json:"content_html"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

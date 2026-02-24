package models

import "time"

type Role string

const (
	RoleAdmin  Role = "admin"
	RoleEditor Role = "editor"
	RoleViewer Role = "viewer"
)

type User struct {
	ID           string    `db:"id" json:"id"`
	Email        string    `db:"email" json:"email"`
	PasswordHash string    `db:"password_hash" json:"-"`
	Role         Role      `db:"role" json:"role"`
	Active       bool      `db:"active" json:"active"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}

func (u *User) IsAdmin() bool  { return u.Role == RoleAdmin }
func (u *User) IsEditor() bool { return u.Role == RoleEditor }
func (u *User) IsViewer() bool { return u.Role == RoleViewer }
func (u *User) CanEdit() bool  { return u.Role == RoleAdmin || u.Role == RoleEditor }

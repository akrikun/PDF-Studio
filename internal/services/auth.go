package services

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"

	"pdf-studio/internal/models"
)

type AuthService struct {
	DB *sqlx.DB
}

func NewAuthService(db *sqlx.DB) *AuthService {
	return &AuthService{DB: db}
}

func (s *AuthService) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func (s *AuthService) CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func (s *AuthService) Authenticate(email, password string) (*models.User, error) {
	var user models.User
	err := s.DB.Get(&user, "SELECT * FROM users WHERE email=$1 AND active=true", email)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	if !s.CheckPassword(user.PasswordHash, password) {
		return nil, fmt.Errorf("invalid credentials")
	}
	return &user, nil
}

func (s *AuthService) CreateSession(userID string) (*models.Session, error) {
	token, err := generateToken(64)
	if err != nil {
		return nil, err
	}

	session := &models.Session{
		UserID:    userID,
		Token:     token,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	err = s.DB.QueryRow(
		`INSERT INTO sessions(user_id, token, expires_at) VALUES($1, $2, $3) RETURNING id, created_at`,
		session.UserID, session.Token, session.ExpiresAt,
	).Scan(&session.ID, &session.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	return session, nil
}

func (s *AuthService) ValidateSession(token string) (*models.User, *models.Session, error) {
	var session models.Session
	err := s.DB.Get(&session, "SELECT * FROM sessions WHERE token=$1 AND expires_at > now()", token)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid session")
	}

	var user models.User
	err = s.DB.Get(&user, "SELECT * FROM users WHERE id=$1 AND active=true", session.UserID)
	if err != nil {
		return nil, nil, fmt.Errorf("user not found")
	}

	return &user, &session, nil
}

func (s *AuthService) DestroySession(token string) error {
	_, err := s.DB.Exec("DELETE FROM sessions WHERE token=$1", token)
	return err
}

func (s *AuthService) DestroyUserSessions(userID string) error {
	_, err := s.DB.Exec("DELETE FROM sessions WHERE user_id=$1", userID)
	return err
}

func (s *AuthService) CleanExpiredSessions() error {
	_, err := s.DB.Exec("DELETE FROM sessions WHERE expires_at < now()")
	return err
}

func (s *AuthService) GenerateCSRFToken(sessionID string) (string, error) {
	token, err := generateToken(32)
	if err != nil {
		return "", err
	}
	_, err = s.DB.Exec("INSERT INTO csrf_tokens(token, session_id) VALUES($1, $2)", token, sessionID)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (s *AuthService) ValidateCSRFToken(token, sessionID string) bool {
	var count int
	err := s.DB.Get(&count, "SELECT COUNT(*) FROM csrf_tokens WHERE token=$1 AND session_id=$2", token, sessionID)
	if err != nil || count == 0 {
		return false
	}
	// Delete after use (one-time token)
	s.DB.Exec("DELETE FROM csrf_tokens WHERE token=$1", token)
	return true
}

func (s *AuthService) CreateUser(email, password string, role models.Role) (*models.User, error) {
	hash, err := s.HashPassword(password)
	if err != nil {
		return nil, err
	}

	user := &models.User{
		Email:        email,
		PasswordHash: hash,
		Role:         role,
		Active:       true,
	}

	err = s.DB.QueryRow(
		`INSERT INTO users(email, password_hash, role) VALUES($1, $2, $3) RETURNING id, created_at, updated_at`,
		user.Email, user.PasswordHash, user.Role,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	return user, nil
}

func (s *AuthService) SeedAdmin(email, password string) error {
	var count int
	s.DB.Get(&count, "SELECT COUNT(*) FROM users WHERE email=$1", email)
	if count > 0 {
		return nil
	}
	_, err := s.CreateUser(email, password, models.RoleAdmin)
	return err
}

func generateToken(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:]), nil
}

package config

import (
	"os"
	"strconv"
)

type Config struct {
	AppPort       string
	AppSecret     string
	AppEnv        string
	DBHost        string
	DBPort        string
	DBUser        string
	DBPassword    string
	DBName        string
	DBSSLMode     string
	GotenbergURL  string
	StorageMode   string
	StoragePath   string
	MaxUploadSize int64
	AdminEmail    string
	AdminPassword string
}

func Load() *Config {
	maxUpload, _ := strconv.ParseInt(getEnv("MAX_UPLOAD_SIZE", "10485760"), 10, 64)
	return &Config{
		AppPort:       getEnv("APP_PORT", "8080"),
		AppSecret:     getEnv("APP_SECRET", "change-me-to-a-random-64-char-string"),
		AppEnv:        getEnv("APP_ENV", "development"),
		DBHost:        getEnv("DB_HOST", "localhost"),
		DBPort:        getEnv("DB_PORT", "5432"),
		DBUser:        getEnv("DB_USER", "pdfstudio"),
		DBPassword:    getEnv("DB_PASSWORD", "pdfstudio_secret"),
		DBName:        getEnv("DB_NAME", "pdfstudio"),
		DBSSLMode:     getEnv("DB_SSLMODE", "disable"),
		GotenbergURL:  getEnv("GOTENBERG_URL", "http://gotenberg:3000"),
		StorageMode:   getEnv("STORAGE_MODE", "fs"),
		StoragePath:   getEnv("STORAGE_PATH", "/data"),
		MaxUploadSize: maxUpload,
		AdminEmail:    getEnv("ADMIN_EMAIL", "admin@pdfstudio.local"),
		AdminPassword: getEnv("ADMIN_PASSWORD", "Admin123!"),
	}
}

func (c *Config) DSN() string {
	return "host=" + c.DBHost +
		" port=" + c.DBPort +
		" user=" + c.DBUser +
		" password=" + c.DBPassword +
		" dbname=" + c.DBName +
		" sslmode=" + c.DBSSLMode
}

func (c *Config) IsProduction() bool {
	return c.AppEnv == "production"
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

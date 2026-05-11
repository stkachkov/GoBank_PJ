package config
import (
	"errors"
	"fmt"
	"os"
	"strconv"
)
type Config struct {
	DatabaseURL             string
	ServerPort              int
	JwtSecret               string
	PgpPrivateKey           string
	PgpPrivateKeyPassphrase string
	HmacSecretKey           string
	OtpServiceBaseURL       string
	SmtpHost                string
	SmtpPort                int
	SmtpUsername            string
	SmtpPassword            string
	SmtpFrom                string
}
func LoadConfig() (*Config, error) {
	serverPortStr := os.Getenv("SERVER_PORT")
	if serverPortStr == "" {
		serverPortStr = "8080"
	}
	serverPort, err := strconv.Atoi(serverPortStr)
	if err != nil {
		return nil, fmt.Errorf("invalid SERVER_PORT: %w", err)
	}
	smtpPortStr := os.Getenv("SMTP_PORT")
	if smtpPortStr == "" {
		smtpPortStr = "587"
	}
	smtpPort, err := strconv.Atoi(smtpPortStr)
	if err != nil {
		return nil, fmt.Errorf("invalid SMTP_PORT: %w", err)
	}
	cfg := &Config{
		DatabaseURL:             os.Getenv("DATABASE_URL"),
		ServerPort:              serverPort,
		JwtSecret:               os.Getenv("JWT_SECRET"),
		PgpPrivateKey:           os.Getenv("PGP_PRIVATE_KEY"),
		PgpPrivateKeyPassphrase: os.Getenv("PGP_PRIVATE_KEY_PASSPHRASE"),
		HmacSecretKey:           os.Getenv("HMAC_SECRET_KEY"),
		SmtpHost:                os.Getenv("SMTP_HOST"),
		SmtpPort:                smtpPort,
		SmtpUsername:            os.Getenv("SMTP_USERNAME"),
		SmtpPassword:            os.Getenv("SMTP_PASSWORD"),
		SmtpFrom:                os.Getenv("SMTP_FROM"),
	}
	if cfg.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL environment variable is required")
	}
	if cfg.JwtSecret == "" {
		return nil, errors.New("JWT_SECRET environment variable is required")
	}
	if cfg.PgpPrivateKey == "" {
		return nil, errors.New("PGP_PRIVATE_KEY environment variable is required")
	}
	if cfg.HmacSecretKey == "" {
		return nil, errors.New("HMAC_SECRET_KEY environment variable is required")
	}
	return cfg, nil
}

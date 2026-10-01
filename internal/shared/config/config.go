// Package config carrega a configuração da aplicação a partir de variáveis de ambiente.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

// Config reúne todas as configurações de execução.
type Config struct {
	Port          string
	LogLevel      string
	DatabaseURL   string
	JWTSecret     string
	JWTTTLMinutes int
	AdminEmail    string
	AdminPassword string
}

const minJWTSecretLen = 32

// Load lê e valida as variáveis de ambiente.
func Load() (Config, error) {
	ttl, err := strconv.Atoi(getenv("JWT_TTL_MINUTES", "60"))
	if err != nil || ttl <= 0 {
		return Config{}, errors.New("JWT_TTL_MINUTES deve ser um inteiro positivo")
	}

	cfg := Config{
		Port:          getenv("APP_PORT", "8080"),
		LogLevel:      getenv("LOG_LEVEL", "info"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		JWTSecret:     os.Getenv("JWT_SECRET"),
		JWTTTLMinutes: ttl,
		AdminEmail:    os.Getenv("ADMIN_EMAIL"),
		AdminPassword: os.Getenv("ADMIN_PASSWORD"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL é obrigatória")
	}
	if len(cfg.JWTSecret) < minJWTSecretLen {
		return Config{}, fmt.Errorf("JWT_SECRET deve ter ao menos %d caracteres", minJWTSecretLen)
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

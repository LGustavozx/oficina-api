// Package config carrega a configuração da aplicação a partir de variáveis de ambiente.
package config

import (
	"errors"
	"fmt"
	"os"
	"time"
)

const minJWTSecretLength = 32

type Config struct {
	HTTPPort    string
	DatabaseURL string

	JWTSecret string
	JWTTTL    time.Duration

	// Administrador criado na primeira execução, quando não há usuários.
	AdminEmail    string
	AdminPassword string
}

func Load() (Config, error) {
	cfg := Config{
		HTTPPort:      getenv("HTTP_PORT", "8080"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		JWTSecret:     os.Getenv("JWT_SECRET"),
		AdminEmail:    os.Getenv("ADMIN_EMAIL"),
		AdminPassword: os.Getenv("ADMIN_PASSWORD"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("config: DATABASE_URL é obrigatória")
	}

	if len(cfg.JWTSecret) < minJWTSecretLength {
		return Config{}, fmt.Errorf("config: JWT_SECRET é obrigatória e deve ter ao menos %d caracteres", minJWTSecretLength)
	}

	ttl, err := time.ParseDuration(getenv("JWT_TTL", "1h"))
	if err != nil || ttl <= 0 {
		return Config{}, errors.New("config: JWT_TTL inválida (exemplo: 1h, 30m)")
	}
	cfg.JWTTTL = ttl

	if (cfg.AdminEmail == "") != (cfg.AdminPassword == "") {
		return Config{}, errors.New("config: ADMIN_EMAIL e ADMIN_PASSWORD devem ser informadas juntas")
	}

	return cfg, nil
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

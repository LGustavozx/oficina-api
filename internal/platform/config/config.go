// Package config carrega a configuração da aplicação a partir de variáveis de ambiente.
package config

import (
	"errors"
	"os"
)

// Config reúne tudo que a aplicação precisa saber do ambiente em que está rodando.
type Config struct {
	HTTPPort    string
	DatabaseURL string
}

// Load lê as variáveis de ambiente e valida as obrigatórias.
// HTTP_PORT é opcional (padrão 8080); DATABASE_URL é obrigatória.
func Load() (Config, error) {
	cfg := Config{
		HTTPPort:    getenv("HTTP_PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("config: DATABASE_URL é obrigatória")
	}

	return cfg, nil
}

// getenv devolve o valor da variável ou o padrão quando ela está vazia/ausente.
func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

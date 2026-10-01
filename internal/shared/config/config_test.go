package config

import (
	"strings"
	"testing"
)

func setBase(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/db")
	t.Setenv("JWT_SECRET", strings.Repeat("a", 32))
	t.Setenv("JWT_TTL_MINUTES", "")
	t.Setenv("APP_PORT", "")
}

func TestLoad_Valida(t *testing.T) {
	setBase(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if cfg.Port != "8080" || cfg.JWTTTLMinutes != 60 {
		t.Errorf("defaults incorretos: %+v", cfg)
	}
}

func TestLoad_Erros(t *testing.T) {
	casos := map[string]func(t *testing.T){
		"sem database url": func(t *testing.T) { t.Setenv("DATABASE_URL", "") },
		"segredo curto":    func(t *testing.T) { t.Setenv("JWT_SECRET", "curto") },
		"ttl invalido":     func(t *testing.T) { t.Setenv("JWT_TTL_MINUTES", "abc") },
		"ttl zero":         func(t *testing.T) { t.Setenv("JWT_TTL_MINUTES", "0") },
	}
	for nome, mutar := range casos {
		t.Run(nome, func(t *testing.T) {
			setBase(t)
			mutar(t)
			if _, err := Load(); err == nil {
				t.Fatal("esperava erro")
			}
		})
	}
}

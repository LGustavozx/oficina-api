package config

import (
	"strings"
	"testing"
	"time"
)

const validSecret = "0123456789abcdef0123456789abcdef"

func setBaseEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("JWT_SECRET", validSecret)
	t.Setenv("HTTP_PORT", "")
	t.Setenv("JWT_TTL", "")
	t.Setenv("ADMIN_EMAIL", "")
	t.Setenv("ADMIN_PASSWORD", "")
}

func TestLoadDefaults(t *testing.T) {
	setBaseEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if cfg.HTTPPort != "8080" {
		t.Errorf("HTTPPort = %q, esperado 8080", cfg.HTTPPort)
	}
	if cfg.DatabaseURL != "postgres://x" {
		t.Errorf("DatabaseURL = %q", cfg.DatabaseURL)
	}
	if cfg.JWTTTL != time.Hour {
		t.Errorf("JWTTTL = %v, esperado 1h", cfg.JWTTTL)
	}
}

func TestLoadCustomValues(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("HTTP_PORT", "9090")
	t.Setenv("JWT_TTL", "30m")
	t.Setenv("ADMIN_EMAIL", "admin@oficina.com")
	t.Setenv("ADMIN_PASSWORD", "senha-forte-123")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if cfg.HTTPPort != "9090" || cfg.JWTTTL != 30*time.Minute {
		t.Errorf("valores não respeitados: %+v", cfg)
	}
	if cfg.AdminEmail != "admin@oficina.com" || cfg.AdminPassword != "senha-forte-123" {
		t.Errorf("admin não carregado: %+v", cfg)
	}
}

func TestLoadErrors(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantMsg string
	}{
		{"sem DATABASE_URL", map[string]string{"DATABASE_URL": ""}, "DATABASE_URL"},
		{"sem JWT_SECRET", map[string]string{"JWT_SECRET": ""}, "JWT_SECRET"},
		{"JWT_SECRET curta", map[string]string{"JWT_SECRET": "curta"}, "JWT_SECRET"},
		{"JWT_TTL inválida", map[string]string{"JWT_TTL": "abc"}, "JWT_TTL"},
		{"JWT_TTL negativa", map[string]string{"JWT_TTL": "-5m"}, "JWT_TTL"},
		{"admin sem senha", map[string]string{"ADMIN_EMAIL": "a@b.com"}, "ADMIN_"},
		{"admin sem e-mail", map[string]string{"ADMIN_PASSWORD": "senha-forte-123"}, "ADMIN_"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setBaseEnv(t)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			_, err := Load()
			if err == nil {
				t.Fatal("esperado erro")
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("erro = %q, esperado mencionar %q", err, tt.wantMsg)
			}
		})
	}
}

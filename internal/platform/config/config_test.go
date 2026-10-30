package config

import "testing"

func TestLoad(t *testing.T) {
	t.Run("usa o valor padrão da porta", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "postgres://x")
		t.Setenv("HTTP_PORT", "")

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
	})

	t.Run("respeita a porta informada", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "postgres://x")
		t.Setenv("HTTP_PORT", "9090")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if cfg.HTTPPort != "9090" {
			t.Errorf("HTTPPort = %q, esperado 9090", cfg.HTTPPort)
		}
	})

	t.Run("falha sem DATABASE_URL", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "")

		if _, err := Load(); err == nil {
			t.Fatal("esperado erro quando DATABASE_URL está vazia")
		}
	})
}

package relogio

import (
	"testing"
	"time"
)

func TestSistema_EmUTC(t *testing.T) {
	var r Relogio = Sistema{}
	agora := r.Agora()
	if agora.Location() != time.UTC {
		t.Errorf("esperado UTC, obtido %v", agora.Location())
	}
	if time.Since(agora) > time.Minute {
		t.Error("hora do sistema muito distante da atual")
	}
}

func TestFixo_Avancar(t *testing.T) {
	base := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	f := &Fixo{Instante: base}
	if !f.Agora().Equal(base) {
		t.Fatal("Fixo deveria devolver o instante configurado")
	}
	f.Avancar(90 * time.Minute)
	if want := base.Add(90 * time.Minute); !f.Agora().Equal(want) {
		t.Errorf("Agora() = %v, esperado %v", f.Agora(), want)
	}
}

package clock

import (
	"testing"
	"time"
)

func TestSystem_InUTC(t *testing.T) {
	var c Clock = System{}
	now := c.Now()
	if now.Location() != time.UTC {
		t.Errorf("esperado UTC, obtido %v", now.Location())
	}
	if time.Since(now) > time.Minute {
		t.Error("hora do sistema muito distante da atual")
	}
}

func TestFixed_Advance(t *testing.T) {
	base := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	f := &Fixed{Instant: base}
	if !f.Now().Equal(base) {
		t.Fatal("Fixed deveria devolver o instante configurado")
	}
	f.Advance(90 * time.Minute)
	if want := base.Add(90 * time.Minute); !f.Now().Equal(want) {
		t.Errorf("Now() = %v, esperado %v", f.Now(), want)
	}
}

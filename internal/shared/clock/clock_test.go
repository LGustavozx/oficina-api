package clock

import (
	"testing"
	"time"
)

func TestFixed(t *testing.T) {
	instante := time.Date(2026, 3, 10, 14, 30, 0, 0, time.UTC)
	var c Clock = Fixed{Time: instante}

	if got := c.Now(); !got.Equal(instante) {
		t.Errorf("Now() = %v, esperado %v", got, instante)
	}
}

func TestSystem(t *testing.T) {
	var c Clock = System{}

	antes := time.Now().UTC().Add(-time.Second)
	got := c.Now()
	depois := time.Now().UTC().Add(time.Second)

	if got.Before(antes) || got.After(depois) {
		t.Errorf("Now() = %v fora da janela [%v, %v]", got, antes, depois)
	}
	if got.Location() != time.UTC {
		t.Errorf("Location = %v, esperado UTC", got.Location())
	}
}

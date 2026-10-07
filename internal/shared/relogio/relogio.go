// Package relogio abstrai a leitura da hora atual, permitindo testes determinísticos.
package relogio

import "time"

// Relogio fornece o instante atual.
type Relogio interface {
	Agora() time.Time
}

// Sistema usa o relógio real, sempre em UTC.
type Sistema struct{}

// Agora devolve a hora atual em UTC.
func (Sistema) Agora() time.Time { return time.Now().UTC() }

// Fixo é um relógio controlável, destinado a testes.
type Fixo struct{ Instante time.Time }

// Agora devolve o instante configurado.
func (f *Fixo) Agora() time.Time { return f.Instante }

// Avancar move o relógio fixo para frente.
func (f *Fixo) Avancar(d time.Duration) { f.Instante = f.Instante.Add(d) }

// Package clock abstrai a leitura da hora atual, permitindo testes determinísticos.
package clock

import "time"

// Clock fornece o instante atual.
type Clock interface {
	Now() time.Time
}

// System usa o relógio real, sempre em UTC.
type System struct{}

// Now devolve a hora atual em UTC.
func (System) Now() time.Time { return time.Now().UTC() }

// Fixed é um relógio controlável, destinado a testes.
type Fixed struct{ Instant time.Time }

// Now devolve o instante configurado.
func (f *Fixed) Now() time.Time { return f.Instant }

// Advance move o relógio fixo para frente.
func (f *Fixed) Advance(d time.Duration) { f.Instant = f.Instant.Add(d) }

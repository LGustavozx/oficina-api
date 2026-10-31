// Package clock abstrai o horário atual para que o código que depende dele
// possa ser testado com valores exatos.
package clock

import "time"

type Clock interface {
	Now() time.Time
}

// System é o relógio real; sempre em UTC.
type System struct{}

func (System) Now() time.Time { return time.Now().UTC() }

// Fixed é o relógio parado usado nos testes.
type Fixed struct {
	Time time.Time
}

func (f Fixed) Now() time.Time { return f.Time }

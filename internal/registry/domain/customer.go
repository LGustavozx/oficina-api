package domain

import (
	"context"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"

	"oficina-api/internal/shared/apperr"
	"oficina-api/internal/shared/text"
)

const (
	CodeInvalidName      = "INVALID_NAME"
	CodeInvalidEmail     = "INVALID_EMAIL"
	CodeInvalidPhone     = "INVALID_PHONE"
	CodeCustomerNotFound = "CUSTOMER_NOT_FOUND"
	CodeDocumentTaken    = "DOCUMENT_ALREADY_REGISTERED"
	CodeCustomerInUse    = "CUSTOMER_IN_USE"

	minNameLength = 2
	maxNameLength = 120
	maxEmailLen   = 254
	minPhoneDigit = 10
	maxPhoneDigit = 13
)

// CustomerData é a entrada bruta, como chega da API, ainda sem validação.
type CustomerData struct {
	Name     string
	Document string
	Email    string
	Phone    string
}

type Customer struct {
	ID        uuid.UUID
	Name      string
	Document  Document
	Email     string // opcional
	Phone     string // opcional, só dígitos
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewCustomer(id uuid.UUID, data CustomerData, now time.Time) (Customer, error) {
	return Customer{ID: id, CreatedAt: now}.apply(data, now)
}

// Update devolve uma cópia com os novos dados; o ID e a data de criação não mudam.
func (c Customer) Update(data CustomerData, now time.Time) (Customer, error) {
	return c.apply(data, now)
}

func (c Customer) apply(data CustomerData, now time.Time) (Customer, error) {
	name, ok := text.Clean(data.Name, minNameLength, maxNameLength)
	if !ok {
		return Customer{}, apperr.Validation(CodeInvalidName, "o nome deve ter entre 2 e 120 caracteres")
	}

	document, err := NewDocument(data.Document)
	if err != nil {
		return Customer{}, err
	}

	email, err := normalizeOptionalEmail(data.Email)
	if err != nil {
		return Customer{}, err
	}

	phone, err := normalizeOptionalPhone(data.Phone)
	if err != nil {
		return Customer{}, err
	}

	c.Name, c.Document, c.Email, c.Phone, c.UpdatedAt = name, document, email, phone, now
	return c, nil
}

func normalizeOptionalEmail(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return "", nil
	}

	addr, err := mail.ParseAddress(value)
	if err != nil || addr.Address != value || len(value) > maxEmailLen {
		return "", apperr.Validation(CodeInvalidEmail, "e-mail inválido")
	}
	return value, nil
}

func normalizeOptionalPhone(raw string) (string, error) {
	digits := strings.NewReplacer(" ", "", "(", "", ")", "", "-", "", "+", "", ".", "").Replace(raw)
	if digits == "" {
		return "", nil
	}

	if !allDigits(digits) || len(digits) < minPhoneDigit || len(digits) > maxPhoneDigit {
		return "", apperr.Validation(CodeInvalidPhone, "telefone inválido")
	}
	return digits, nil
}

type CustomerRepository interface {
	Create(ctx context.Context, customer Customer) error
	Update(ctx context.Context, customer Customer) error
	Delete(ctx context.Context, id uuid.UUID) error
	FindByID(ctx context.Context, id uuid.UUID) (Customer, error)
	FindByDocument(ctx context.Context, document Document) (Customer, error)
	List(ctx context.Context, limit, offset int) ([]Customer, error)
}

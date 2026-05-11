package models

import (
	"time"

	"github.com/google/uuid"
)

type Card struct {
	ID                  uuid.UUID `json:"id"`
	AccountID           uuid.UUID `json:"account_id"`
	CardNumber          string    `json:"card_number"`
	ExpiryDate          string    `json:"expiry_date"`
	CardNumberEncrypted string    `json:"-"`
	ExpiryDateEncrypted string    `json:"-"`
	CvvHash             []byte    `json:"-"`
	HMACtag             []byte    `json:"-"`
	IsVirtual           bool      `json:"is_virtual"`
	CVV                 string    `json:"cvv,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

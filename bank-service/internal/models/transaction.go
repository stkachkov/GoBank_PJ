package models

import (
	"time"

	"github.com/google/uuid"
)

type Transaction struct {
	ID                   uuid.UUID  `json:"id"`
	AccountID            uuid.UUID  `json:"account_id"`
	TransactionType      string     `json:"transaction_type"`
	Amount               float64    `json:"amount"`
	Currency             string     `json:"currency"`
	Description          string     `json:"description"`
	TransactionTimestamp time.Time  `json:"transaction_timestamp"`
	RelatedAccountID     *uuid.UUID `json:"related_account_id,omitempty"`
}

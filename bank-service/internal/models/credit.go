package models

import (
	"time"

	"github.com/google/uuid"
)

type Credit struct {
	ID                 uuid.UUID  `json:"id"`
	UserID             uuid.UUID  `json:"user_id"`
	AccountID          uuid.UUID  `json:"account_id"`
	AmountGranted      float64    `json:"amount_granted"`
	InterestRate       float64    `json:"interest_rate"`
	TermMonths         int        `json:"term_months"`
	MonthlyPayment     float64    `json:"monthly_payment"`
	OutstandingBalance float64    `json:"outstanding_balance"`
	Status             string     `json:"status"`
	GrantedAt          time.Time  `json:"granted_at"`
	ClosedAt           *time.Time `json:"closed_at,omitempty"`
}

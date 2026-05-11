package models

import (
	"time"

	"github.com/google/uuid"
)

type PaymentSchedule struct {
	ID           uuid.UUID  `json:"id"`
	CreditID     uuid.UUID  `json:"credit_id"`
	DueDate      time.Time  `json:"due_date"`
	AmountDue    float64    `json:"amount_due"`
	AmountPaid   float64    `json:"amount_paid"`
	IsPaid       bool       `json:"is_paid"`
	OverdueFines float64    `json:"overdue_fines"`
	PaidAt       *time.Time `json:"paid_at,omitempty"`
}

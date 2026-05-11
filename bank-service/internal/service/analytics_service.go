package service

import (
	"GoBank_PJ/bank-service/internal/repository/postgres"
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
)

type CreditAnalyticsDTO struct {
	TotalCreditsActive    int     `json:"total_credits_active"`
	TotalOutstandingLoans float64 `json:"total_outstanding_loans"`
	TotalMonthlyPayments  float64 `json:"total_monthly_payments"`
}
type AnalyticsService struct {
	accountRepo         postgres.AccountRepository
	transactionRepo     postgres.TransactionRepository
	creditRepo          postgres.CreditRepository
	paymentScheduleRepo postgres.PaymentScheduleRepository
}

func NewAnalyticsService(
	accountRepo postgres.AccountRepository,
	transactionRepo postgres.TransactionRepository,
	creditRepo postgres.CreditRepository,
	paymentScheduleRepo postgres.PaymentScheduleRepository,
) *AnalyticsService {
	return &AnalyticsService{
		accountRepo:         accountRepo,
		transactionRepo:     transactionRepo,
		creditRepo:          creditRepo,
		paymentScheduleRepo: paymentScheduleRepo,
	}
}
func (s *AnalyticsService) GetMonthlyIncomeExpense(ctx context.Context, userID uuid.UUID, year, month int) (income, expense float64, err error) {
	accounts, err := s.accountRepo.GetAccountsByUserID(ctx, userID)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get accounts for user %s: %w", userID, err)
	}
	if len(accounts) == 0 {
		return 0, 0, nil
	}
	startOfMonth := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	endOfMonth := startOfMonth.AddDate(0, 1, 0).Add(-time.Nanosecond)
	totalIncome := 0.0
	totalExpense := 0.0
	for _, acc := range accounts {
		transactions, err := s.transactionRepo.GetTransactionsByAccountID(ctx, acc.ID)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to get transactions for account %s: %w", acc.ID, err)
		}
		for _, tx := range transactions {
			if tx.TransactionTimestamp.After(startOfMonth) && tx.TransactionTimestamp.Before(endOfMonth) {
				if tx.Amount > 0 {
					totalIncome += tx.Amount
				} else {
					totalExpense += tx.Amount
				}
			}
		}
	}
	return totalIncome, math.Abs(totalExpense), nil
}
func (s *AnalyticsService) GetCreditLoadAnalytics(ctx context.Context, userID uuid.UUID) (*CreditAnalyticsDTO, error) {
	credits, err := s.creditRepo.GetCreditsByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get credits for user %s: %w", userID, err)
	}
	analytics := &CreditAnalyticsDTO{}
	for _, credit := range credits {
		if credit.Status == "active" || credit.Status == "overdue" {
			analytics.TotalCreditsActive++
			analytics.TotalOutstandingLoans += credit.OutstandingBalance
			analytics.TotalMonthlyPayments += credit.MonthlyPayment
		}
	}
	return analytics, nil
}
func (s *AnalyticsService) PredictAccountBalance(ctx context.Context, accountID uuid.UUID, days int) (float64, error) {
	if days <= 0 || days > 365 {
		return 0, fmt.Errorf("invalid prediction period: days must be between 1 and 365")
	}
	account, err := s.accountRepo.GetAccountByID(ctx, accountID)
	if err != nil {
		return 0, fmt.Errorf("failed to get account %s: %w", accountID, err)
	}
	currentBalance := account.Balance
	predictionDate := time.Now().AddDate(0, 0, days)
	credits, err := s.creditRepo.GetCreditsByUserID(ctx, account.UserID)
	if err != nil {
		return 0, fmt.Errorf("failed to get credits for account's user %s: %w", account.UserID, err)
	}
	for _, credit := range credits {
		if credit.AccountID == accountID && (credit.Status == "active" || credit.Status == "overdue") {
			schedules, err := s.paymentScheduleRepo.GetPaymentSchedulesByCreditID(ctx, credit.ID)
			if err != nil {
				return 0, fmt.Errorf("failed to get payment schedules for credit %s: %w", credit.ID, err)
			}
			for _, schedule := range schedules {
				if !schedule.IsPaid && schedule.DueDate.Before(predictionDate) {
					currentBalance -= schedule.AmountDue
					currentBalance -= schedule.OverdueFines
				}
			}
		}
	}
	return currentBalance, nil
}

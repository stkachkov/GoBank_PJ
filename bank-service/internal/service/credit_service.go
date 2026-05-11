package service

import (
	"GoBank_PJ/bank-service/internal/models"
	"GoBank_PJ/bank-service/internal/repository/postgres"
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
)

var (
	ErrCreditNotFound      = errors.New("credit not found")
	ErrInvalidCreditAmount = errors.New("invalid credit amount")
	ErrInvalidTerm         = errors.New("invalid credit term")
	ErrCreditProcessing    = errors.New("credit processing error")
)

type CreditService struct {
	db                  *postgres.DB
	creditRepo          postgres.CreditRepository
	paymentScheduleRepo postgres.PaymentScheduleRepository
	accountRepo         postgres.AccountRepository
	transactionRepo     postgres.TransactionRepository
}

func NewCreditService(
	db *postgres.DB,
	creditRepo postgres.CreditRepository,
	paymentScheduleRepo postgres.PaymentScheduleRepository,
	accountRepo postgres.AccountRepository,
	transactionRepo postgres.TransactionRepository,
) *CreditService {
	return &CreditService{
		db:                  db,
		creditRepo:          creditRepo,
		paymentScheduleRepo: paymentScheduleRepo,
		accountRepo:         accountRepo,
		transactionRepo:     transactionRepo,
	}
}
func (s *CreditService) ApplyForCredit(
	ctx context.Context,
	userID, accountID uuid.UUID,
	amount float64,
	termMonths int,
	interestRate float64,
) (*models.Credit, error) {
	if amount <= 0 {
		return nil, ErrInvalidCreditAmount
	}
	if termMonths <= 0 {
		return nil, ErrInvalidTerm
	}
	monthlyInterestRate := (interestRate / 100) / 12
	var monthlyPayment float64
	if monthlyInterestRate == 0 {
		monthlyPayment = amount / float64(termMonths)
	} else {
		numerator := monthlyInterestRate * math.Pow(1+monthlyInterestRate, float64(termMonths))
		denominator := math.Pow(1+monthlyInterestRate, float64(termMonths)) - 1
		if denominator == 0 {
			return nil, fmt.Errorf("%w: cannot calculate monthly payment, denominator is zero", ErrCreditProcessing)
		}
		monthlyPayment = amount * (numerator / denominator)
	}
	credit := &models.Credit{
		ID:                 uuid.New(),
		UserID:             userID,
		AccountID:          accountID,
		AmountGranted:      amount,
		InterestRate:       interestRate,
		TermMonths:         termMonths,
		MonthlyPayment:     monthlyPayment,
		OutstandingBalance: amount,
		Status:             "active",
		GrantedAt:          time.Now(),
		ClosedAt:           nil,
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to begin transaction for credit application: %v", ErrCreditProcessing, err)
	}
	defer tx.Rollback()
	creditRepoTx := s.creditRepo.WithTx(tx)
	paymentScheduleRepoTx := s.paymentScheduleRepo.WithTx(tx)
	if err := creditRepoTx.CreateCredit(ctx, credit); err != nil {
		return nil, fmt.Errorf("%w: failed to create credit in repository: %v", ErrCreditProcessing, err)
	}
	currentBalance := amount
	for i := 1; i <= termMonths; i++ {
		interest := currentBalance * monthlyInterestRate
		principal := monthlyPayment - interest
		if i == termMonths {
			principal = currentBalance
			monthlyPayment = currentBalance + interest
		}
		currentBalance -= principal
		schedule := &models.PaymentSchedule{
			ID:           uuid.New(),
			CreditID:     credit.ID,
			DueDate:      time.Now().AddDate(0, i, 0),
			AmountDue:    monthlyPayment,
			AmountPaid:   0.0,
			IsPaid:       false,
			OverdueFines: 0.0,
			PaidAt:       nil,
		}
		if err := paymentScheduleRepoTx.CreatePaymentSchedule(ctx, schedule); err != nil {
			return nil, fmt.Errorf("%w: failed to create payment schedule entry: %v", ErrCreditProcessing, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("%w: failed to commit transaction for credit application: %v", ErrCreditProcessing, err)
	}
	return credit, nil
}
func (s *CreditService) GetCreditByID(ctx context.Context, id uuid.UUID) (*models.Credit, error) {
	credit, err := s.creditRepo.GetCreditByID(ctx, id)
	if err != nil {
		if err.Error() == fmt.Sprintf("credit with ID %s not found", id) {
			return nil, ErrCreditNotFound
		}
		return nil, fmt.Errorf("failed to get credit from repository: %w", err)
	}
	return credit, nil
}
func (s *CreditService) GetCreditsByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Credit, error) {
	credits, err := s.creditRepo.GetCreditsByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get credits by user ID from repository: %w", err)
	}
	return credits, nil
}
func (s *CreditService) GetPaymentSchedule(ctx context.Context, creditID uuid.UUID) ([]*models.PaymentSchedule, error) {
	schedule, err := s.paymentScheduleRepo.GetPaymentSchedulesByCreditID(ctx, creditID)
	if err != nil {
		return nil, fmt.Errorf("failed to get payment schedule from repository: %w", err)
	}
	return schedule, nil
}
func (s *CreditService) ProcessMonthlyPayment(ctx context.Context, creditID, accountID uuid.UUID, paymentAmount float64) error {
	if paymentAmount <= 0 {
		return ErrInvalidAmount
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("%w: failed to begin transaction for payment processing: %v", ErrCreditProcessing, err)
	}
	defer tx.Rollback()
	creditRepoTx := s.creditRepo.WithTx(tx)
	accountRepoTx := s.accountRepo.WithTx(tx)
	paymentScheduleRepoTx := s.paymentScheduleRepo.WithTx(tx)
	transactionRepoTx := s.transactionRepo.WithTx(tx)
	credit, err := creditRepoTx.GetCreditByID(ctx, creditID)
	if err != nil {
		return fmt.Errorf("%w: failed to get credit for payment: %v", ErrCreditNotFound, err)
	}
	if credit.AccountID != accountID {
		return fmt.Errorf("%w: payment account does not match credit's linked account", ErrCreditProcessing)
	}
	account, err := accountRepoTx.GetAccountByID(ctx, accountID)
	if err != nil {
		return fmt.Errorf("%w: failed to get payment account: %v", ErrCreditProcessing, err)
	}
	if account.Balance < paymentAmount {
		return ErrInsufficientFunds
	}
	account.Balance -= paymentAmount
	account.UpdatedAt = time.Now()
	if err := accountRepoTx.UpdateAccount(ctx, account); err != nil {
		return fmt.Errorf("%w: failed to debit payment from account: %v", ErrCreditProcessing, err)
	}
	credit.OutstandingBalance -= paymentAmount
	if credit.OutstandingBalance <= 0 {
		credit.OutstandingBalance = 0
		credit.Status = "paid"
		closedAt := time.Now()
		credit.ClosedAt = &closedAt
	}
	if err := creditRepoTx.UpdateCredit(ctx, credit); err != nil {
		return fmt.Errorf("%w: failed to update credit balance: %v", ErrCreditProcessing, err)
	}
	schedules, err := paymentScheduleRepoTx.GetPaymentSchedulesByCreditID(ctx, creditID)
	if err != nil {
		return fmt.Errorf("%w: failed to get payment schedules for credit: %v", ErrCreditProcessing, err)
	}
	var updatedSchedule *models.PaymentSchedule
	for _, ps := range schedules {
		if !ps.IsPaid && ps.DueDate.Before(time.Now().AddDate(0, 0, 1)) {
			ps.AmountPaid += paymentAmount
			ps.IsPaid = ps.AmountPaid >= ps.AmountDue
			ps.PaidAt = nil
			if ps.IsPaid {
				now := time.Now()
				ps.PaidAt = &now
			}
			ps.OverdueFines = 0
			updatedSchedule = ps
			break
		}
	}
	if updatedSchedule == nil {
		fmt.Printf("Warning: No matching payment schedule found for credit %s to apply payment of %f\n", creditID, paymentAmount)
	} else {
		if err := paymentScheduleRepoTx.UpdatePaymentSchedule(ctx, updatedSchedule); err != nil {
			return fmt.Errorf("%w: failed to update payment schedule entry: %v", ErrCreditProcessing, err)
		}
	}
	transaction := &models.Transaction{
		ID:                   uuid.New(),
		AccountID:            accountID,
		TransactionType:      "credit_payment",
		Amount:               -paymentAmount,
		Currency:             account.Currency,
		Description:          fmt.Sprintf("Payment for credit %s", creditID),
		TransactionTimestamp: time.Now(),
		RelatedAccountID:     nil,
	}
	if err := transactionRepoTx.CreateTransaction(ctx, transaction); err != nil {
		return fmt.Errorf("failed to record credit payment transaction: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%w: failed to commit transaction for payment processing: %v", ErrCreditProcessing, err)
	}
	return nil
}

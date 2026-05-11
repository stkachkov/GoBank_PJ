package service

import (
	"GoBank_PJ/bank-service/internal/models"
	"GoBank_PJ/bank-service/internal/repository/postgres"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

var (
	ErrAccountNotFound      = errors.New("account not found")
	ErrInsufficientFunds    = errors.New("insufficient funds")
	ErrInvalidAmount        = errors.New("invalid amount")
	ErrSelfTransfer         = errors.New("cannot transfer to the same account")
	ErrCurrencyMismatch     = errors.New("currency mismatch for transfer")
	ErrAccountAlreadyExists = errors.New("account with this number already exists for this user")
)

type AccountService struct {
	db                  *postgres.DB
	accountRepo         postgres.AccountRepository
	transactionRepo     postgres.TransactionRepository
	userRepo            postgres.UserRepository
	notificationService *NotificationService
}

func NewAccountService(db *postgres.DB, accountRepo postgres.AccountRepository, transactionRepo postgres.TransactionRepository, userRepo postgres.UserRepository, notificationService *NotificationService) *AccountService {
	return &AccountService{
		db:                  db,
		accountRepo:         accountRepo,
		transactionRepo:     transactionRepo,
		userRepo:            userRepo,
		notificationService: notificationService,
	}
}
func (s *AccountService) CreateAccount(ctx context.Context, userID uuid.UUID, currency string) (*models.Account, error) {
	accountNumber := generateAccountNumber()
	existingAccount, err := s.accountRepo.GetAccountByAccountNumber(ctx, accountNumber)
	if err != nil && err.Error() != fmt.Sprintf("account with number %s not found", accountNumber) {
		return nil, fmt.Errorf("failed to check for existing account number: %w", err)
	}
	if existingAccount != nil {
		return nil, ErrAccountAlreadyExists
	}
	account := &models.Account{
		ID:            uuid.New(),
		UserID:        userID,
		AccountNumber: accountNumber,
		Balance:       0.00,
		Currency:      currency,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	if err := s.accountRepo.CreateAccount(ctx, account); err != nil {
		return nil, fmt.Errorf("failed to create account in repository: %w", err)
	}
	return account, nil
}
func (s *AccountService) GetAccountByID(ctx context.Context, id uuid.UUID) (*models.Account, error) {
	account, err := s.accountRepo.GetAccountByID(ctx, id)
	if err != nil {
		if err.Error() == fmt.Sprintf("account with ID %s not found", id) {
			return nil, ErrAccountNotFound
		}
		return nil, fmt.Errorf("failed to get account from repository: %w", err)
	}
	return account, nil
}
func (s *AccountService) GetAccountsByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Account, error) {
	accounts, err := s.accountRepo.GetAccountsByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get accounts by user ID from repository: %w", err)
	}
	return accounts, nil
}
func (s *AccountService) Deposit(ctx context.Context, accountID uuid.UUID, amount float64) (*models.Account, error) {
	if amount <= 0 {
		return nil, ErrInvalidAmount
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction for deposit: %w", err)
	}
	defer tx.Rollback()
	account, err := s.accountRepo.GetAccountByID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account for deposit: %w", err)
	}
	account.Balance += amount
	account.UpdatedAt = time.Now()
	if err := s.accountRepo.UpdateAccount(ctx, account); err != nil {
		return nil, fmt.Errorf("failed to update account balance: %w", err)
	}
	transaction := &models.Transaction{
		ID:                   uuid.New(),
		AccountID:            account.ID,
		TransactionType:      "deposit",
		Amount:               amount,
		Currency:             account.Currency,
		Description:          "Deposit funds",
		TransactionTimestamp: time.Now(),
	}
	if err := s.transactionRepo.CreateTransaction(ctx, transaction); err != nil {
		return nil, fmt.Errorf("failed to record deposit transaction: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction for deposit: %w", err)
	}
	return account, nil
}
func (s *AccountService) Withdraw(ctx context.Context, accountID uuid.UUID, amount float64) (*models.Account, error) {
	if amount <= 0 {
		return nil, ErrInvalidAmount
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction for withdrawal: %w", err)
	}
	defer tx.Rollback()
	account, err := s.accountRepo.GetAccountByID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account for withdrawal: %w", err)
	}
	if account.Balance < amount {
		return nil, ErrInsufficientFunds
	}
	account.Balance -= amount
	account.UpdatedAt = time.Now()
	if err := s.accountRepo.UpdateAccount(ctx, account); err != nil {
		return nil, fmt.Errorf("failed to update account balance: %w", err)
	}
	transaction := &models.Transaction{
		ID:                   uuid.New(),
		AccountID:            account.ID,
		TransactionType:      "withdrawal",
		Amount:               -amount,
		Currency:             account.Currency,
		Description:          "Withdraw funds",
		TransactionTimestamp: time.Now(),
	}
	if err := s.transactionRepo.CreateTransaction(ctx, transaction); err != nil {
		return nil, fmt.Errorf("failed to record withdrawal transaction: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction for withdrawal: %w", err)
	}

	go func() {
		user, err := s.userRepo.FindUserByID(context.Background(), account.UserID.String())
		if err != nil {
			logrus.Errorf("failed to find user for withdrawal notification: %v", err)
			return
		}
		subject := "Уведомление о списании средств"
		body := fmt.Sprintf("Здравствуйте, %s! С вашего счета %s успешно списано %.2f %s.", user.Username, account.AccountNumber, amount, account.Currency)
		if err := s.notificationService.SendEmail(context.Background(), user.Email, subject, body); err != nil {
			logrus.Errorf("failed to send withdrawal notification: %v", err)
		}
	}()

	return account, nil
}
func (s *AccountService) Transfer(ctx context.Context, fromAccountID, toAccountID uuid.UUID, amount float64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	if fromAccountID == toAccountID {
		return ErrSelfTransfer
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction for transfer: %w", err)
	}
	defer tx.Rollback()
	fromAccount, err := s.accountRepo.GetAccountByID(ctx, fromAccountID)
	if err != nil {
		return fmt.Errorf("failed to retrieve source account: %w", err)
	}
	toAccount, err := s.accountRepo.GetAccountByID(ctx, toAccountID)
	if err != nil {
		return fmt.Errorf("failed to retrieve destination account: %w", err)
	}
	if fromAccount.Currency != toAccount.Currency {
		return ErrCurrencyMismatch
	}
	if fromAccount.Balance < amount {
		return ErrInsufficientFunds
	}
	fromAccount.Balance -= amount
	fromAccount.UpdatedAt = time.Now()
	toAccount.Balance += amount
	toAccount.UpdatedAt = time.Now()
	if err := s.accountRepo.UpdateAccount(ctx, fromAccount); err != nil {
		return fmt.Errorf("failed to update source account balance: %w", err)
	}
	if err := s.accountRepo.UpdateAccount(ctx, toAccount); err != nil {
		return fmt.Errorf("failed to update destination account balance: %w", err)
	}
	transferOutTx := &models.Transaction{
		ID:                   uuid.New(),
		AccountID:            fromAccount.ID,
		TransactionType:      "transfer_out",
		Amount:               -amount,
		Currency:             fromAccount.Currency,
		Description:          fmt.Sprintf("Transfer to account %s", toAccount.AccountNumber),
		RelatedAccountID:     &toAccount.ID,
		TransactionTimestamp: time.Now(),
	}
	if err := s.transactionRepo.CreateTransaction(ctx, transferOutTx); err != nil {
		return fmt.Errorf("failed to record transfer out transaction: %w", err)
	}
	transferInTx := &models.Transaction{
		ID:                   uuid.New(),
		AccountID:            toAccount.ID,
		TransactionType:      "transfer_in",
		Amount:               amount,
		Currency:             toAccount.Currency,
		Description:          fmt.Sprintf("Transfer from account %s", fromAccount.AccountNumber),
		RelatedAccountID:     &fromAccount.ID,
		TransactionTimestamp: time.Now(),
	}
	if err := s.transactionRepo.CreateTransaction(ctx, transferInTx); err != nil {
		return fmt.Errorf("failed to record transfer in transaction: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction for transfer: %w", err)
	}

	go func() {
		senderUser, err := s.userRepo.FindUserByID(context.Background(), fromAccount.UserID.String())
		if err != nil {
			logrus.Errorf("failed to find sender user for notification: %v", err)
			return
		}
		recipientUser, err := s.userRepo.FindUserByID(context.Background(), toAccount.UserID.String())
		if err != nil {
			logrus.Errorf("failed to find recipient user for notification: %v", err)
			return
		}

		senderSubject := "Уведомление о переводе средств"
		senderBody := fmt.Sprintf("Здравствуйте, %s! Вы успешно перевели %.2f %s на счет %s.", senderUser.Username, amount, fromAccount.Currency, toAccount.AccountNumber)
		if err := s.notificationService.SendEmail(context.Background(), senderUser.Email, senderSubject, senderBody); err != nil {
			logrus.Errorf("failed to send transfer notification to sender: %v", err)
		}

		recipientSubject := "Уведомление о пополнении счета"
		recipientBody := fmt.Sprintf("Здравствуйте, %s! Ваш счет пополнен на %.2f %s со счета %s.", recipientUser.Username, amount, toAccount.Currency, fromAccount.AccountNumber)
		if err := s.notificationService.SendEmail(context.Background(), recipientUser.Email, recipientSubject, recipientBody); err != nil {
			logrus.Errorf("failed to send transfer notification to recipient: %v", err)
		}
	}()

	return nil
}
func generateAccountNumber() string {
	return uuid.New().String()[:12]
}

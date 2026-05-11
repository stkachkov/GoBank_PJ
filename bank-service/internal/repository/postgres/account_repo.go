package postgres

import (
	"GoBank_PJ/bank-service/internal/models"
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

type AccountRepository interface {
	CreateAccount(ctx context.Context, account *models.Account) error
	GetAccountByID(ctx context.Context, id uuid.UUID) (*models.Account, error)
	GetAccountsByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Account, error)
	GetAccountByAccountNumber(ctx context.Context, accountNumber string) (*models.Account, error)
	UpdateAccount(ctx context.Context, account *models.Account) error
	DeleteAccount(ctx context.Context, id uuid.UUID) error
	WithTx(tx *sql.Tx) AccountRepository
}
type PostgresAccountRepository struct {
	querier Querier
}

func NewPostgresAccountRepository(querier Querier) *PostgresAccountRepository {
	return &PostgresAccountRepository{querier: querier}
}
func (r *PostgresAccountRepository) WithTx(tx *sql.Tx) AccountRepository {
	return &PostgresAccountRepository{querier: tx}
}
func (r *PostgresAccountRepository) CreateAccount(ctx context.Context, account *models.Account) error {
	query := `
		INSERT INTO accounts (id, user_id, account_number, balance, currency, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at`
	err := r.querier.QueryRowContext(ctx, query,
		account.ID,
		account.UserID,
		account.AccountNumber,
		account.Balance,
		account.Currency,
		account.CreatedAt,
		account.UpdatedAt,
	).Scan(&account.ID, &account.CreatedAt, &account.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create account: %w", err)
	}
	return nil
}
func (r *PostgresAccountRepository) GetAccountByID(ctx context.Context, id uuid.UUID) (*models.Account, error) {
	account := &models.Account{}
	query := `
		SELECT id, user_id, account_number, balance, currency, created_at, updated_at
		FROM accounts
		WHERE id = $1`
	err := r.querier.QueryRowContext(ctx, query, id).Scan(
		&account.ID,
		&account.UserID,
		&account.AccountNumber,
		&account.Balance,
		&account.Currency,
		&account.CreatedAt,
		&account.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("account with ID %s not found", id)
		}
		return nil, fmt.Errorf("failed to get account by ID: %w", err)
	}
	return account, nil
}
func (r *PostgresAccountRepository) GetAccountsByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Account, error) {
	query := `
		SELECT id, user_id, account_number, balance, currency, created_at, updated_at
		FROM accounts
		WHERE user_id = $1`
	rows, err := r.querier.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get accounts by user ID: %w", err)
	}
	defer rows.Close()
	var accounts []*models.Account
	for rows.Next() {
		account := &models.Account{}
		if err := rows.Scan(
			&account.ID,
			&account.UserID,
			&account.AccountNumber,
			&account.Balance,
			&account.Currency,
			&account.CreatedAt,
			&account.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan account row: %w", err)
		}
		accounts = append(accounts, account)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration: %w", err)
	}
	return accounts, nil
}
func (r *PostgresAccountRepository) GetAccountByAccountNumber(ctx context.Context, accountNumber string) (*models.Account, error) {
	account := &models.Account{}
	query := `
		SELECT id, user_id, account_number, balance, currency, created_at, updated_at
		FROM accounts
		WHERE account_number = $1`
	err := r.querier.QueryRowContext(ctx, query, accountNumber).Scan(
		&account.ID,
		&account.UserID,
		&account.AccountNumber,
		&account.Balance,
		&account.Currency,
		&account.CreatedAt,
		&account.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("account with number %s not found", accountNumber)
		}
		return nil, fmt.Errorf("failed to get account by account number: %w", err)
	}
	return account, nil
}
func (r *PostgresAccountRepository) UpdateAccount(ctx context.Context, account *models.Account) error {
	query := `
		UPDATE accounts
		SET balance = $1, updated_at = $2
		WHERE id = $3`
	res, err := r.querier.ExecContext(ctx, query,
		account.Balance,
		account.UpdatedAt,
		account.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update account: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("account with ID %s not found for update", account.ID)
	}
	return nil
}
func (r *PostgresAccountRepository) DeleteAccount(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM accounts WHERE id = $1`
	res, err := r.querier.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete account: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("account with ID %s not found for delete", id)
	}
	return nil
}

package postgres

import (
	"GoBank_PJ/bank-service/internal/models"
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

type TransactionRepository interface {
	CreateTransaction(ctx context.Context, transaction *models.Transaction) error
	GetTransactionByID(ctx context.Context, id uuid.UUID) (*models.Transaction, error)
	GetTransactionsByAccountID(ctx context.Context, accountID uuid.UUID) ([]*models.Transaction, error)
	GetTransactionsByAccountIDAndType(ctx context.Context, accountID uuid.UUID, transactionType string) ([]*models.Transaction, error)
	WithTx(tx *sql.Tx) TransactionRepository
}
type PostgresTransactionRepository struct {
	querier Querier
}

func NewPostgresTransactionRepository(querier Querier) *PostgresTransactionRepository {
	return &PostgresTransactionRepository{querier: querier}
}
func (r *PostgresTransactionRepository) WithTx(tx *sql.Tx) TransactionRepository {
	return &PostgresTransactionRepository{querier: tx}
}
func (r *PostgresTransactionRepository) CreateTransaction(ctx context.Context, transaction *models.Transaction) error {
	query := `
		INSERT INTO transactions (id, account_id, transaction_type, amount, currency, description, transaction_timestamp, related_account_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, transaction_timestamp`
	var relatedAccountID uuid.NullUUID
	if transaction.RelatedAccountID != nil {
		relatedAccountID = uuid.NullUUID{UUID: *transaction.RelatedAccountID, Valid: true}
	} else {
		relatedAccountID = uuid.NullUUID{Valid: false}
	}
	err := r.querier.QueryRowContext(ctx, query,
		transaction.ID,
		transaction.AccountID,
		transaction.TransactionType,
		transaction.Amount,
		transaction.Currency,
		transaction.Description,
		transaction.TransactionTimestamp,
		relatedAccountID,
	).Scan(&transaction.ID, &transaction.TransactionTimestamp)
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}
	return nil
}
func (r *PostgresTransactionRepository) GetTransactionByID(ctx context.Context, id uuid.UUID) (*models.Transaction, error) {
	transaction := &models.Transaction{}
	var relatedAccountID uuid.NullUUID
	query := `
		SELECT id, account_id, transaction_type, amount, currency, description, transaction_timestamp, related_account_id
		FROM transactions
		WHERE id = $1`
	err := r.querier.QueryRowContext(ctx, query, id).Scan(
		&transaction.ID,
		&transaction.AccountID,
		&transaction.TransactionType,
		&transaction.Amount,
		&transaction.Currency,
		&transaction.Description,
		&transaction.TransactionTimestamp,
		&relatedAccountID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("transaction with ID %s not found", id)
		}
		return nil, fmt.Errorf("failed to get transaction by ID: %w", err)
	}
	if relatedAccountID.Valid {
		transaction.RelatedAccountID = &relatedAccountID.UUID
	}
	return transaction, nil
}
func (r *PostgresTransactionRepository) GetTransactionsByAccountID(ctx context.Context, accountID uuid.UUID) ([]*models.Transaction, error) {
	query := `
		SELECT id, account_id, transaction_type, amount, currency, description, transaction_timestamp, related_account_id
		FROM transactions
		WHERE account_id = $1
		ORDER BY transaction_timestamp DESC`
	rows, err := r.querier.QueryContext(ctx, query, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions by account ID: %w", err)
	}
	defer rows.Close()
	var transactions []*models.Transaction
	for rows.Next() {
		transaction := &models.Transaction{}
		var relatedAccountID uuid.NullUUID
		if err := rows.Scan(
			&transaction.ID,
			&transaction.AccountID,
			&transaction.TransactionType,
			&transaction.Amount,
			&transaction.Currency,
			&transaction.Description,
			&transaction.TransactionTimestamp,
			&relatedAccountID,
		); err != nil {
			return nil, fmt.Errorf("failed to scan transaction row: %w", err)
		}
		if relatedAccountID.Valid {
			transaction.RelatedAccountID = &relatedAccountID.UUID
		}
		transactions = append(transactions, transaction)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration: %w", err)
	}
	return transactions, nil
}
func (r *PostgresTransactionRepository) GetTransactionsByAccountIDAndType(ctx context.Context, accountID uuid.UUID, transactionType string) ([]*models.Transaction, error) {
	query := `
		SELECT id, account_id, transaction_type, amount, currency, description, transaction_timestamp, related_account_id
		FROM transactions
		WHERE account_id = $1 AND transaction_type = $2
		ORDER BY transaction_timestamp DESC`
	rows, err := r.querier.QueryContext(ctx, query, accountID, transactionType)
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions by account ID and type: %w", err)
	}
	defer rows.Close()
	var transactions []*models.Transaction
	for rows.Next() {
		transaction := &models.Transaction{}
		var relatedAccountID uuid.NullUUID
		if err := rows.Scan(
			&transaction.ID,
			&transaction.AccountID,
			&transaction.TransactionType,
			&transaction.Amount,
			&transaction.Currency,
			&transaction.Description,
			&transaction.TransactionTimestamp,
			&relatedAccountID,
		); err != nil {
			return nil, fmt.Errorf("failed to scan transaction row: %w", err)
		}
		if relatedAccountID.Valid {
			transaction.RelatedAccountID = &relatedAccountID.UUID
		}
		transactions = append(transactions, transaction)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration: %w", err)
	}
	return transactions, nil
}

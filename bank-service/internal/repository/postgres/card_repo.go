package postgres

import (
	"GoBank_PJ/bank-service/internal/models"
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

type CardRepository interface {
	CreateCard(ctx context.Context, card *models.Card) error
	GetCardByID(ctx context.Context, id uuid.UUID) (*models.Card, error)
	GetCardsByAccountID(ctx context.Context, accountID uuid.UUID) ([]*models.Card, error)
	UpdateCard(ctx context.Context, card *models.Card) error
	DeleteCard(ctx context.Context, id uuid.UUID) error
	WithTx(tx *sql.Tx) CardRepository
}
type PostgresCardRepository struct {
	querier Querier
}

func NewPostgresCardRepository(querier Querier) *PostgresCardRepository {
	return &PostgresCardRepository{querier: querier}
}
func (r *PostgresCardRepository) WithTx(tx *sql.Tx) CardRepository {
	return &PostgresCardRepository{querier: tx}
}
func (r *PostgresCardRepository) CreateCard(ctx context.Context, card *models.Card) error {
	query := `
		INSERT INTO cards (id, account_id, card_number_encrypted, expiry_date_encrypted, cvv_hash, hmac_tag, is_virtual, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at`
	err := r.querier.QueryRowContext(ctx, query,
		card.ID,
		card.AccountID,
		card.CardNumberEncrypted,
		card.ExpiryDateEncrypted,
		card.CvvHash,
		card.HMACtag,
		card.IsVirtual,
		card.CreatedAt,
		card.UpdatedAt,
	).Scan(&card.ID, &card.CreatedAt, &card.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create card: %w", err)
	}
	return nil
}
func (r *PostgresCardRepository) GetCardByID(ctx context.Context, id uuid.UUID) (*models.Card, error) {
	card := &models.Card{}
	query := `
		SELECT id, account_id, card_number_encrypted, expiry_date_encrypted, cvv_hash, hmac_tag, is_virtual, created_at, updated_at
		FROM cards
		WHERE id = $1`
	err := r.querier.QueryRowContext(ctx, query, id).Scan(
		&card.ID,
		&card.AccountID,
		&card.CardNumberEncrypted,
		&card.ExpiryDateEncrypted,
		&card.CvvHash,
		&card.HMACtag,
		&card.IsVirtual,
		&card.CreatedAt,
		&card.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("card with ID %s not found", id)
		}
		return nil, fmt.Errorf("failed to get card by ID: %w", err)
	}
	return card, nil
}
func (r *PostgresCardRepository) GetCardsByAccountID(ctx context.Context, accountID uuid.UUID) ([]*models.Card, error) {
	query := `
		SELECT id, account_id, card_number_encrypted, expiry_date_encrypted, cvv_hash, hmac_tag, is_virtual, created_at, updated_at
		FROM cards
		WHERE account_id = $1`
	rows, err := r.querier.QueryContext(ctx, query, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cards by account ID: %w", err)
	}
	defer rows.Close()
	var cards []*models.Card
	for rows.Next() {
		card := &models.Card{}
		if err := rows.Scan(
			&card.ID,
			&card.AccountID,
			&card.CardNumberEncrypted,
			&card.ExpiryDateEncrypted,
			&card.CvvHash,
			&card.HMACtag,
			&card.IsVirtual,
			&card.CreatedAt,
			&card.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan card row: %w", err)
		}
		cards = append(cards, card)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration: %w", err)
	}
	return cards, nil
}
func (r *PostgresCardRepository) UpdateCard(ctx context.Context, card *models.Card) error {
	query := `
		UPDATE cards
		SET is_virtual = $1, updated_at = $2
		WHERE id = $3`
	res, err := r.querier.ExecContext(ctx, query,
		card.IsVirtual,
		card.UpdatedAt,
		card.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update card: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("card with ID %s not found for update", card.ID)
	}
	return nil
}
func (r *PostgresCardRepository) DeleteCard(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM cards WHERE id = $1`
	res, err := r.querier.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete card: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("card with ID %s not found for delete", id)
	}
	return nil
}

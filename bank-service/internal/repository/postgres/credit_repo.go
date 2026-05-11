package postgres

import (
	"GoBank_PJ/bank-service/internal/models"
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

type CreditRepository interface {
	CreateCredit(ctx context.Context, credit *models.Credit) error
	GetCreditByID(ctx context.Context, id uuid.UUID) (*models.Credit, error)
	GetCreditsByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Credit, error)
	UpdateCredit(ctx context.Context, credit *models.Credit) error
	DeleteCredit(ctx context.Context, id uuid.UUID) error
	WithTx(tx *sql.Tx) CreditRepository
}
type PostgresCreditRepository struct {
	querier Querier
}

func NewPostgresCreditRepository(querier Querier) *PostgresCreditRepository {
	return &PostgresCreditRepository{querier: querier}
}
func (r *PostgresCreditRepository) WithTx(tx *sql.Tx) CreditRepository {
	return &PostgresCreditRepository{querier: tx}
}
func (r *PostgresCreditRepository) CreateCredit(ctx context.Context, credit *models.Credit) error {
	query := `
		INSERT INTO credits (id, user_id, account_id, amount_granted, interest_rate, term_months, monthly_payment, outstanding_balance, status, granted_at, closed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, granted_at, closed_at`
	var closedAt sql.NullTime
	if credit.ClosedAt != nil {
		closedAt = sql.NullTime{Time: *credit.ClosedAt, Valid: true}
	} else {
		closedAt = sql.NullTime{Valid: false}
	}
	err := r.querier.QueryRowContext(ctx, query,
		credit.ID,
		credit.UserID,
		credit.AccountID,
		credit.AmountGranted,
		credit.InterestRate,
		credit.TermMonths,
		credit.MonthlyPayment,
		credit.OutstandingBalance,
		credit.Status,
		credit.GrantedAt,
		closedAt,
	).Scan(&credit.ID, &credit.GrantedAt, &closedAt)
	if err != nil {
		return fmt.Errorf("failed to create credit: %w", err)
	}
	if closedAt.Valid {
		credit.ClosedAt = &closedAt.Time
	}
	return nil
}
func (r *PostgresCreditRepository) GetCreditByID(ctx context.Context, id uuid.UUID) (*models.Credit, error) {
	credit := &models.Credit{}
	var closedAt sql.NullTime
	query := `
		SELECT id, user_id, account_id, amount_granted, interest_rate, term_months, monthly_payment, outstanding_balance, status, granted_at, closed_at
		FROM credits
		WHERE id = $1`
	err := r.querier.QueryRowContext(ctx, query, id).Scan(
		&credit.ID,
		&credit.UserID,
		&credit.AccountID,
		&credit.AmountGranted,
		&credit.InterestRate,
		&credit.TermMonths,
		&credit.MonthlyPayment,
		&credit.OutstandingBalance,
		&credit.Status,
		&credit.GrantedAt,
		&closedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("credit with ID %s not found", id)
		}
		return nil, fmt.Errorf("failed to get credit by ID: %w", err)
	}
	if closedAt.Valid {
		credit.ClosedAt = &closedAt.Time
	}
	return credit, nil
}
func (r *PostgresCreditRepository) GetCreditsByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Credit, error) {
	query := `
		SELECT id, user_id, account_id, amount_granted, interest_rate, term_months, monthly_payment, outstanding_balance, status, granted_at, closed_at
		FROM credits
		WHERE user_id = $1
		ORDER BY granted_at DESC`
	rows, err := r.querier.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get credits by user ID: %w", err)
	}
	defer rows.Close()
	var credits []*models.Credit
	for rows.Next() {
		credit := &models.Credit{}
		var closedAt sql.NullTime
		if err := rows.Scan(
			&credit.ID,
			&credit.UserID,
			&credit.AccountID,
			&credit.AmountGranted,
			&credit.InterestRate,
			&credit.TermMonths,
			&credit.MonthlyPayment,
			&credit.OutstandingBalance,
			&credit.Status,
			&credit.GrantedAt,
			&closedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan credit row: %w", err)
		}
		if closedAt.Valid {
			credit.ClosedAt = &closedAt.Time
		}
		credits = append(credits, credit)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration: %w", err)
	}
	return credits, nil
}
func (r *PostgresCreditRepository) UpdateCredit(ctx context.Context, credit *models.Credit) error {
	query := `
		UPDATE credits
		SET outstanding_balance = $1, status = $2, closed_at = $3
		WHERE id = $4`
	var closedAt sql.NullTime
	if credit.ClosedAt != nil {
		closedAt = sql.NullTime{Time: *credit.ClosedAt, Valid: true}
	} else {
		closedAt = sql.NullTime{Valid: false}
	}
	res, err := r.querier.ExecContext(ctx, query,
		credit.OutstandingBalance,
		credit.Status,
		closedAt,
		credit.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update credit: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("credit with ID %s not found for update", credit.ID)
	}
	return nil
}
func (r *PostgresCreditRepository) DeleteCredit(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM credits WHERE id = $1`
	res, err := r.querier.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete credit: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("credit with ID %s not found for delete", id)
	}
	return nil
}

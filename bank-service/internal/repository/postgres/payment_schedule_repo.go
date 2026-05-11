package postgres

import (
	"GoBank_PJ/bank-service/internal/models"
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

type PaymentScheduleRepository interface {
	CreatePaymentSchedule(ctx context.Context, schedule *models.PaymentSchedule) error
	GetPaymentScheduleByID(ctx context.Context, id uuid.UUID) (*models.PaymentSchedule, error)
	GetPaymentSchedulesByCreditID(ctx context.Context, creditID uuid.UUID) ([]*models.PaymentSchedule, error)
	UpdatePaymentSchedule(ctx context.Context, schedule *models.PaymentSchedule) error
	DeletePaymentSchedule(ctx context.Context, id uuid.UUID) error
	WithTx(tx *sql.Tx) PaymentScheduleRepository
}
type PostgresPaymentScheduleRepository struct {
	querier Querier
}

func NewPostgresPaymentScheduleRepository(querier Querier) *PostgresPaymentScheduleRepository {
	return &PostgresPaymentScheduleRepository{querier: querier}
}
func (r *PostgresPaymentScheduleRepository) WithTx(tx *sql.Tx) PaymentScheduleRepository {
	return &PostgresPaymentScheduleRepository{querier: tx}
}
func (r *PostgresPaymentScheduleRepository) CreatePaymentSchedule(ctx context.Context, schedule *models.PaymentSchedule) error {
	query := `
		INSERT INTO payment_schedules (id, credit_id, due_date, amount_due, amount_paid, is_paid, overdue_fines, paid_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, paid_at`
	var paidAt sql.NullTime
	if schedule.PaidAt != nil {
		paidAt = sql.NullTime{Time: *schedule.PaidAt, Valid: true}
	} else {
		paidAt = sql.NullTime{Valid: false}
	}
	err := r.querier.QueryRowContext(ctx, query,
		schedule.ID,
		schedule.CreditID,
		schedule.DueDate,
		schedule.AmountDue,
		schedule.AmountPaid,
		schedule.IsPaid,
		schedule.OverdueFines,
		paidAt,
	).Scan(&schedule.ID, &paidAt)
	if err != nil {
		return fmt.Errorf("failed to create payment schedule: %w", err)
	}
	if paidAt.Valid {
		schedule.PaidAt = &paidAt.Time
	}
	return nil
}
func (r *PostgresPaymentScheduleRepository) GetPaymentScheduleByID(ctx context.Context, id uuid.UUID) (*models.PaymentSchedule, error) {
	schedule := &models.PaymentSchedule{}
	var paidAt sql.NullTime
	query := `
		SELECT id, credit_id, due_date, amount_due, amount_paid, is_paid, overdue_fines, paid_at
		FROM payment_schedules
		WHERE id = $1`
	err := r.querier.QueryRowContext(ctx, query, id).Scan(
		&schedule.ID,
		&schedule.CreditID,
		&schedule.DueDate,
		&schedule.AmountDue,
		&schedule.AmountPaid,
		&schedule.IsPaid,
		&schedule.OverdueFines,
		&paidAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("payment schedule with ID %s not found", id)
		}
		return nil, fmt.Errorf("failed to get payment schedule by ID: %w", err)
	}
	if paidAt.Valid {
		schedule.PaidAt = &paidAt.Time
	}
	return schedule, nil
}
func (r *PostgresPaymentScheduleRepository) GetPaymentSchedulesByCreditID(ctx context.Context, creditID uuid.UUID) ([]*models.PaymentSchedule, error) {
	query := `
		SELECT id, credit_id, due_date, amount_due, amount_paid, is_paid, overdue_fines, paid_at
		FROM payment_schedules
		WHERE credit_id = $1
		ORDER BY due_date ASC`
	rows, err := r.querier.QueryContext(ctx, query, creditID)
	if err != nil {
		return nil, fmt.Errorf("failed to get payment schedules by credit ID: %w", err)
	}
	defer rows.Close()
	var schedules []*models.PaymentSchedule
	for rows.Next() {
		schedule := &models.PaymentSchedule{}
		var paidAt sql.NullTime
		if err := rows.Scan(
			&schedule.ID,
			&schedule.CreditID,
			&schedule.DueDate,
			&schedule.AmountDue,
			&schedule.AmountPaid,
			&schedule.IsPaid,
			&schedule.OverdueFines,
			&paidAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan payment schedule row: %w", err)
		}
		if paidAt.Valid {
			schedule.PaidAt = &paidAt.Time
		}
		schedules = append(schedules, schedule)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration: %w", err)
	}
	return schedules, nil
}
func (r *PostgresPaymentScheduleRepository) UpdatePaymentSchedule(ctx context.Context, schedule *models.PaymentSchedule) error {
	query := `
		UPDATE payment_schedules
		SET amount_paid = $1, is_paid = $2, overdue_fines = $3, paid_at = $4
		WHERE id = $5`
	var paidAt sql.NullTime
	if schedule.PaidAt != nil {
		paidAt = sql.NullTime{Time: *schedule.PaidAt, Valid: true}
	} else {
		paidAt = sql.NullTime{Valid: false}
	}
	res, err := r.querier.ExecContext(ctx, query,
		schedule.AmountPaid,
		schedule.IsPaid,
		schedule.OverdueFines,
		paidAt,
		schedule.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update payment schedule: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("payment schedule with ID %s not found for update", schedule.ID)
	}
	return nil
}
func (r *PostgresPaymentScheduleRepository) DeletePaymentSchedule(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM payment_schedules WHERE id = $1`
	res, err := r.querier.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete payment schedule: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("payment schedule with ID %s not found for delete", id)
	}
	return nil
}

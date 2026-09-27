package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/Zaya-AS/tg-expense-bot/internal/report"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ReportRepository struct {
	pool *pgxpool.Pool
}

func NewReportRepository(pool *pgxpool.Pool) *ReportRepository {
	return &ReportRepository{pool: pool}
}

func (r *ReportRepository) CurrentMonth(
	ctx context.Context,
	telegramUserID int64,
) (time.Time, []report.CategoryTotal, error) {
	var userID int64
	var timezone string

	err := r.pool.QueryRow(ctx, `
		SELECT id, timezone
		FROM users
		WHERE telegram_user_id = $1
	`, telegramUserID).Scan(&userID, &timezone)
	if err != nil {
		return time.Time{}, nil, fmt.Errorf("find report user: %w", err)
	}

	location, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, nil, fmt.Errorf("load user timezone: %w", err)
	}

	start, end := report.MonthBounds(time.Now(), location)

	rows, err := r.pool.Query(ctx, `
		SELECT c.name, SUM(e.amount_minor)::bigint
		FROM expenses AS e
		JOIN categories AS c
			ON c.id = e.category_id AND c.user_id = e.user_id
		WHERE e.user_id = $1
			AND e.deleted_at IS NULL
			AND e.spent_at >= $2
			AND e.spent_at < $3
		GROUP BY c.name
		ORDER BY SUM(e.amount_minor) DESC, c.name
	`, userID, start, end)
	if err != nil {
		return time.Time{}, nil, fmt.Errorf("query monthly expenses: %w", err)
	}
	defer rows.Close()

	var totals []report.CategoryTotal
	for rows.Next() {
		var item report.CategoryTotal
		if err := rows.Scan(&item.Name, &item.AmountMinor); err != nil {
			return time.Time{}, nil, fmt.Errorf("scan monthly total: %w", err)
		}
		totals = append(totals, item)
	}
	if err := rows.Err(); err != nil {
		return time.Time{}, nil, fmt.Errorf("read monthly totals: %w", err)
	}

	return start, totals, nil
}

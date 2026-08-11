package greetings

import (
	"context"
	"database/sql"
	"errors"
)


type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) GetRandomByPeriod(ctx context.Context, period string) (*Greeting, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT id, greeting, period
		 FROM "Greetings"
		 WHERE period = $1
		 ORDER BY random()
		 LIMIT 1`,
		period,
	)

	var g Greeting
	if err := row.Scan(&g.ID, &g.Greeting, &g.Period); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &g, nil
}
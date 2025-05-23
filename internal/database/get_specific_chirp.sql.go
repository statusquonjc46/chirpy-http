// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.29.0
// source: get_specific_chirp.sql

package database

import (
	"context"

	"github.com/google/uuid"
)

const getSpecificChirp = `-- name: GetSpecificChirp :one
SELECT id, created_at, updated_at, body, user_id FROM chirps WHERE id=$1
`

func (q *Queries) GetSpecificChirp(ctx context.Context, id uuid.UUID) (Chirp, error) {
	row := q.db.QueryRowContext(ctx, getSpecificChirp, id)
	var i Chirp
	err := row.Scan(
		&i.ID,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.Body,
		&i.UserID,
	)
	return i, err
}

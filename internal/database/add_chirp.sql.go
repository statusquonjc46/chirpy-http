// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.29.0
// source: add_chirp.sql

package database

import (
	"context"

	"github.com/google/uuid"
)

const addChirp = `-- name: AddChirp :one
INSERT INTO chirps (id, created_at, updated_at, body, user_id)
VALUES (
        gen_random_uuid(), NOW(), NOW(), $1, $2
)
RETURNING id, created_at, updated_at, body, user_id
`

type AddChirpParams struct {
	Body   string
	UserID uuid.NullUUID
}

func (q *Queries) AddChirp(ctx context.Context, arg AddChirpParams) (Chirp, error) {
	row := q.db.QueryRowContext(ctx, addChirp, arg.Body, arg.UserID)
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

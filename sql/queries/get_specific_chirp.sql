-- name: GetSpecificChirp :one
SELECT * FROM chirps WHERE id=$1;

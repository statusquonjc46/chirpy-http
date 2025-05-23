-- name: UserandHashLookup :one
SELECT * FROM users WHERE email=$1;

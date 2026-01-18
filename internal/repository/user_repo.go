package repository

import (
	"context"
	"errors"

	"github.com/Ay0wa/ws-messenger/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepo struct {
	db *pgxpool.Pool
}

func NewUserRepo(db *pgxpool.Pool) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(ctx context.Context, user models.User) (models.User, error) {
	query := `
        INSERT INTO users (email, password_hash, display_name)
        VALUES ($1, $2, $3)
        RETURNING id, created_at
    `
	err := r.db.QueryRow(ctx, query, user.Email, user.PasswordHash, user.DisplayName).Scan(&user.ID, &user.CreatedAt)
	return user, err
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (models.User, error) {
	query := `
        SELECT id, email, password_hash, display_name, created_at
        FROM users
        WHERE email = $1
    `
	var user models.User
	err := r.db.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.DisplayName,
		&user.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.User{}, pgx.ErrNoRows
	}
	return user, err
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (models.User, error) {
	query := `
        SELECT id, email, password_hash, display_name, created_at
        FROM users
        WHERE id = $1
    `
	var user models.User
	err := r.db.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.DisplayName,
		&user.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.User{}, pgx.ErrNoRows
	}
	return user, err
}

package repository

import (
	"context"
	"errors"
	"time"

	"culinary-book/backend/models"

	"github.com/jackc/pgx/v5"
)

type UserRepository struct {
	db *pgx.Conn
}

func NewUserRepository(db *pgx.Conn) *UserRepository {
	return &UserRepository{db: db}
}

// CreateUser создаёт нового пользователя
func (r *UserRepository) CreateUser(user *models.User) error {
	ctx := context.Background()

	query := `
		INSERT INTO users (username, password_hash, email, created_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`

	err := r.db.QueryRow(ctx, query,
		user.Username,
		user.PasswordHash,
		user.Email,
		time.Now(),
	).Scan(&user.ID, &user.CreatedAt)

	return err
}

// GetUserByUsername получает пользователя по имени
func (r *UserRepository) GetUserByUsername(username string) (*models.User, error) {
	ctx := context.Background()

	query := `
		SELECT id, username, password_hash, email, created_at
		FROM users
		WHERE username = $1
	`

	var user models.User
	err := r.db.QueryRow(ctx, query, username).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.Email,
		&user.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errors.New("пользователь не найден")
		}
		return nil, err
	}

	return &user, nil
}

// GetUserByID получает пользователя по ID
func (r *UserRepository) GetUserByID(userID int) (*models.User, error) {
	ctx := context.Background()

	query := `
		SELECT id, username, email, created_at
		FROM users
		WHERE id = $1
	`

	var user models.User
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errors.New("пользователь не найден")
		}
		return nil, err
	}

	return &user, nil
}

// UsernameExists проверяет, существует ли username
func (r *UserRepository) UsernameExists(username string) (bool, error) {
	ctx := context.Background()

	query := `
		SELECT COUNT(*) FROM users WHERE username = $1
	`

	var count int
	err := r.db.QueryRow(ctx, query, username).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

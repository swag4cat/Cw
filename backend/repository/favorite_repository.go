package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type FavoriteRepository struct {
	db *pgx.Conn
}

func NewFavoriteRepository(db *pgx.Conn) *FavoriteRepository {
	return &FavoriteRepository{db: db}
}

func (r *FavoriteRepository) AddFavorite(userID, recipeID int) error {
	ctx := context.Background()

	query := `
		INSERT INTO favorites (user_id, recipe_id, created_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, recipe_id) DO NOTHING
	`

	_, err := r.db.Exec(ctx, query, userID, recipeID, time.Now())
	return err
}

func (r *FavoriteRepository) RemoveFavorite(userID, recipeID int) error {
	ctx := context.Background()

	query := `
		DELETE FROM favorites
		WHERE user_id = $1 AND recipe_id = $2
	`

	result, err := r.db.Exec(ctx, query, userID, recipeID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return errors.New("рецепт не найден в избранном")
	}

	return nil
}

func (r *FavoriteRepository) GetFavoriteRecipes(userID int) ([]int, error) {
	ctx := context.Background()

	query := `
		SELECT recipe_id FROM favorites
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var recipeIDs []int
	for rows.Next() {
		var recipeID int
		if err := rows.Scan(&recipeID); err != nil {
			return nil, err
		}
		recipeIDs = append(recipeIDs, recipeID)
	}

	return recipeIDs, nil
}

func (r *FavoriteRepository) IsFavorite(userID, recipeID int) (bool, error) {
	ctx := context.Background()

	query := `
		SELECT COUNT(*) FROM favorites
		WHERE user_id = $1 AND recipe_id = $2
	`

	var count int
	err := r.db.QueryRow(ctx, query, userID, recipeID).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"culinary-book/backend/models"

	"github.com/jackc/pgx/v5"
)

type RecipeRepository struct {
	db *pgx.Conn
}

func NewRecipeRepository(db *pgx.Conn) *RecipeRepository {
	return &RecipeRepository{db: db}
}

// CreateRecipe создаёт новый рецепт
func (r *RecipeRepository) CreateRecipe(recipe *models.Recipe) error {
	ctx := context.Background()

	ingredientsJSON, _ := json.Marshal(recipe.Ingredients)

	query := `
		INSERT INTO recipes (user_id, title, description, ingredients, instructions, cooking_time, difficulty, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at
	`

	err := r.db.QueryRow(ctx, query,
		recipe.UserID,
		recipe.Title,
		recipe.Description,
		ingredientsJSON,
		recipe.Instructions,
		recipe.CookingTime,
		recipe.Difficulty,
		time.Now(),
		time.Now(),
	).Scan(&recipe.ID, &recipe.CreatedAt, &recipe.UpdatedAt)

	return err
}

// GetRecipesByUserID получает рецепты пользователя
func (r *RecipeRepository) GetRecipesByUserID(userID int) ([]models.Recipe, error) {
	ctx := context.Background()

	query := `
		SELECT id, user_id, title, description, ingredients, instructions, cooking_time, difficulty, created_at, updated_at
		FROM recipes
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var recipes []models.Recipe
	for rows.Next() {
		var recipe models.Recipe
		var ingredientsJSON []byte

		err := rows.Scan(
			&recipe.ID,
			&recipe.UserID,
			&recipe.Title,
			&recipe.Description,
			&ingredientsJSON,
			&recipe.Instructions,
			&recipe.CookingTime,
			&recipe.Difficulty,
			&recipe.CreatedAt,
			&recipe.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		json.Unmarshal(ingredientsJSON, &recipe.Ingredients)
		recipes = append(recipes, recipe)
	}

	return recipes, nil
}

// GetRecipeByID получает рецепт по ID
func (r *RecipeRepository) GetRecipeByID(recipeID int) (*models.Recipe, error) {
	ctx := context.Background()

	query := `
		SELECT id, user_id, title, description, ingredients, instructions, cooking_time, difficulty, created_at, updated_at
		FROM recipes
		WHERE id = $1
	`

	var recipe models.Recipe
	var ingredientsJSON []byte

	err := r.db.QueryRow(ctx, query, recipeID).Scan(
		&recipe.ID,
		&recipe.UserID,
		&recipe.Title,
		&recipe.Description,
		&ingredientsJSON,
		&recipe.Instructions,
		&recipe.CookingTime,
		&recipe.Difficulty,
		&recipe.CreatedAt,
		&recipe.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errors.New("рецепт не найден")
		}
		return nil, err
	}

	json.Unmarshal(ingredientsJSON, &recipe.Ingredients)
	return &recipe, nil
}

// UpdateRecipe обновляет рецепт
func (r *RecipeRepository) UpdateRecipe(recipe *models.Recipe) error {
	ctx := context.Background()

	ingredientsJSON, _ := json.Marshal(recipe.Ingredients)

	query := `
		UPDATE recipes
		SET title = $1, description = $2, ingredients = $3, instructions = $4, cooking_time = $5, difficulty = $6, updated_at = $7
		WHERE id = $8 AND user_id = $9
		RETURNING updated_at
	`

	err := r.db.QueryRow(ctx, query,
		recipe.Title,
		recipe.Description,
		ingredientsJSON,
		recipe.Instructions,
		recipe.CookingTime,
		recipe.Difficulty,
		time.Now(),
		recipe.ID,
		recipe.UserID,
	).Scan(&recipe.UpdatedAt)

	return err
}

// DeleteRecipe удаляет рецепт
func (r *RecipeRepository) DeleteRecipe(recipeID, userID int) error {
	ctx := context.Background()

	query := `
		DELETE FROM recipes
		WHERE id = $1 AND user_id = $2
	`

	result, err := r.db.Exec(ctx, query, recipeID, userID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return errors.New("рецепт не найден или нет прав на удаление")
	}

	return nil
}

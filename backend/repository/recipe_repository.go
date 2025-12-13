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

func (r *RecipeRepository) CreateRecipe(recipe *models.Recipe) error {
	ctx := context.Background()

	ingredientsJSON, _ := json.Marshal(recipe.Ingredients)

	query := `
		INSERT INTO recipes
		(user_id, title, description, ingredients, instructions,
		 cooking_time, difficulty, image_base64, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
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
		recipe.ImageBase64,
		time.Now(),
		time.Now(),
	).Scan(&recipe.ID, &recipe.CreatedAt, &recipe.UpdatedAt)

	return err
}

func (r *RecipeRepository) GetRecipesByUserID(userID int) ([]models.Recipe, error) {
	ctx := context.Background()

	favoriteRepo := NewFavoriteRepository(r.db)
	favoriteIDs, err := favoriteRepo.GetFavoriteRecipes(userID)
	if err != nil {
		return nil, err
	}

	favoriteMap := make(map[int]bool)
	for _, id := range favoriteIDs {
		favoriteMap[id] = true
	}

	query := `
		SELECT id, user_id, title, description, ingredients, instructions,
		       cooking_time, difficulty, image_base64, created_at, updated_at
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
			&recipe.ImageBase64,
			&recipe.CreatedAt,
			&recipe.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		json.Unmarshal(ingredientsJSON, &recipe.Ingredients)

		recipe.IsFavorite = favoriteMap[recipe.ID]

		recipes = append(recipes, recipe)
	}

	return recipes, nil
}

func (r *RecipeRepository) GetRecipeByID(recipeID int) (*models.Recipe, error) {
	ctx := context.Background()

	query := `
		SELECT id, user_id, title, description, ingredients, instructions,
		       cooking_time, difficulty, image_base64, created_at, updated_at
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
		&recipe.ImageBase64,
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

func (r *RecipeRepository) UpdateRecipe(recipe *models.Recipe) error {
	ctx := context.Background()

	ingredientsJSON, _ := json.Marshal(recipe.Ingredients)

	query := `
		UPDATE recipes
		SET title = $1, description = $2, ingredients = $3, instructions = $4,
		    cooking_time = $5, difficulty = $6, image_base64 = $7, updated_at = $8
		WHERE id = $9 AND user_id = $10
		RETURNING updated_at
	`

	err := r.db.QueryRow(ctx, query,
		recipe.Title,
		recipe.Description,
		ingredientsJSON,
		recipe.Instructions,
		recipe.CookingTime,
		recipe.Difficulty,
		recipe.ImageBase64,
		time.Now(),
		recipe.ID,
		recipe.UserID,
	).Scan(&recipe.UpdatedAt)

	return err
}

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

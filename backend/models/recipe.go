package models

import (
	"time"
)

type Recipe struct {
	ID           int                    `json:"id"`
	UserID       int                    `json:"user_id"`
	Title        string                 `json:"title" binding:"required"`
	Description  string                 `json:"description,omitempty"`
	Ingredients  []string               `json:"ingredients,omitempty"`
	Instructions string                 `json:"instructions,omitempty"`
	CookingTime  int                    `json:"cooking_time,omitempty"`
	Difficulty   string                 `json:"difficulty,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	Extra        map[string]interface{} `json:"-"`
}

type RecipeRequest struct {
	Title        string   `json:"title" binding:"required"`
	Description  string   `json:"description,omitempty"`
	Ingredients  []string `json:"ingredients,omitempty"`
	Instructions string   `json:"instructions,omitempty"`
	CookingTime  int      `json:"cooking_time,omitempty"`
	Difficulty   string   `json:"difficulty,omitempty"`
}

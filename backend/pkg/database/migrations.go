package database

import (
	"context"
	"log"
)

func Migrate(db *pgxpool.Pool) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			username VARCHAR(50) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			email VARCHAR(100),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`,

			`CREATE TABLE IF NOT EXISTS recipes (
				id SERIAL PRIMARY KEY,
				user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
				title VARCHAR(200) NOT NULL,
				description TEXT,
				ingredients JSONB,
				instructions TEXT,
				cooking_time INTEGER,
				difficulty VARCHAR(20),
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
				)`,

				`CREATE INDEX IF NOT EXISTS idx_recipes_user_id ON recipes(user_id)`,
	}

	ctx := context.Background()
	for _, query := range queries {
		if _, err := db.Exec(ctx, query); err != nil {
			return err
		}
	}

	log.Println("Миграции выполнены успешно")
	return nil
}

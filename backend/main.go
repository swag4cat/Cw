package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"culinary-book/backend/auth"
	"culinary-book/backend/models"
	"culinary-book/backend/repository"

	"github.com/jackc/pgx/v5"
)

var db *pgx.Conn
var userRepo *repository.UserRepository
var recipeRepo *repository.RecipeRepository
var favoriteRepo *repository.FavoriteRepository

func initDB() error {
	connStr := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var err error
	db, err = pgx.Connect(ctx, connStr)
	if err != nil {
		return fmt.Errorf("не удалось подключиться к БД: %w", err)
	}

	if err := db.Ping(ctx); err != nil {
		return fmt.Errorf("не удалось проверить подключение к БД: %w", err)
	}

	userRepo = repository.NewUserRepository(db)
	recipeRepo = repository.NewRecipeRepository(db)
	favoriteRepo = repository.NewFavoriteRepository(db)

	log.Println("✅ Подключение к PostgreSQL установлено")
	return nil
}

func createTables() error {
	ctx := context.Background()

	_, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			username VARCHAR(50) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			email VARCHAR(100),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS recipes (
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
		)
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(ctx, `
		CREATE INDEX IF NOT EXISTS idx_recipes_user_id ON recipes(user_id)
	`)
	if err != nil {
		return err
	}

	log.Println("✅ Таблицы созданы/проверены")
	return nil
}

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, `{"error": "Требуется авторизация"}`, http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, `{"error": "Неверный формат токена"}`, http.StatusUnauthorized)
			return
		}

		tokenString := parts[1]
		_, err := auth.ValidateJWT(tokenString)
		if err != nil {
			http.Error(w, `{"error": "Невалидный токен"}`, http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, `{"error": "Метод не разрешен"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Неверный формат данных"}`, http.StatusBadRequest)
		return
	}

	if len(req.Username) < 3 {
		http.Error(w, `{"error": "Имя пользователя должно быть не менее 3 символов"}`, http.StatusBadRequest)
		return
	}
	if len(req.Password) < 6 {
		http.Error(w, `{"error": "Пароль должен быть не менее 6 символов"}`, http.StatusBadRequest)
		return
	}

	exists, err := userRepo.UsernameExists(req.Username)
	if err != nil {
		http.Error(w, `{"error": "Ошибка сервера"}`, http.StatusInternalServerError)
		return
	}
	if exists {
		http.Error(w, `{"error": "Пользователь с таким именем уже существует"}`, http.StatusConflict)
		return
	}

	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		http.Error(w, `{"error": "Ошибка при обработке пароля"}`, http.StatusInternalServerError)
		return
	}

	user := &models.User{
		Username:     req.Username,
		PasswordHash: passwordHash,
	}

	if err := userRepo.CreateUser(user); err != nil {
		http.Error(w, `{"error": "Ошибка при создании пользователя"}`, http.StatusInternalServerError)
		return
	}

	token, err := auth.GenerateJWT(user.ID, user.Username)
	if err != nil {
		http.Error(w, `{"error": "Ошибка при создании токена"}`, http.StatusInternalServerError)
		return
	}

	response := models.AuthResponse{
		Status:  "ok",
		Message: "Регистрация успешна",
		Token:   token,
		User:    user,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, `{"error": "Метод не разрешен"}`, http.StatusMethodNotAllowed)
		return
	}

	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Неверный формат данных"}`, http.StatusBadRequest)
		return
	}

	user, err := userRepo.GetUserByUsername(req.Username)
	if err != nil {
		http.Error(w, `{"error": "Неверное имя пользователя или пароль"}`, http.StatusUnauthorized)
		return
	}

	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		http.Error(w, `{"error": "Неверное имя пользователя или пароль"}`, http.StatusUnauthorized)
		return
	}

	token, err := auth.GenerateJWT(user.ID, user.Username)
	if err != nil {
		http.Error(w, `{"error": "Ошибка при создании токена"}`, http.StatusInternalServerError)
		return
	}

	response := models.AuthResponse{
		Status:  "ok",
		Message: "Вход выполнен успешно",
		Token:   token,
		User:    user,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func recipesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := map[string]interface{}{
		"status":  "ok",
		"message": "API рецептов работает. Используйте /api/my-recipes для получения своих рецептов",
		"data": []map[string]interface{}{
			{
				"id":          1,
				"title":       "Борщ",
				"description": "Традиционный суп",
				"difficulty":  "medium",
				"time":        90,
			},
			{
				"id":          2,
				"title":       "Оливье",
				"description": "Салат на Новый год",
				"difficulty":  "easy",
				"time":        60,
			},
		},
	}

	json.NewEncoder(w).Encode(response)
}

func myRecipesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, `{"error": "Требуется авторизация"}`, http.StatusUnauthorized)
		return
	}

	tokenString := strings.Split(authHeader, " ")[1]
	userID, err := auth.GetUserIDFromToken(tokenString)
	if err != nil {
		http.Error(w, `{"error": "Невалидный токен"}`, http.StatusUnauthorized)
		return
	}

	recipes, err := recipeRepo.GetRecipesByUserID(userID)
	if err != nil {
		http.Error(w, `{"error": "Ошибка при получении рецептов"}`, http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status":  "ok",
		"count":   len(recipes),
		"recipes": recipes,
	}

	json.NewEncoder(w).Encode(response)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ctx := context.Background()
	var dbStatus string
	if err := db.Ping(ctx); err != nil {
		dbStatus = "disconnected"
	} else {
		dbStatus = "connected"
	}

	response := map[string]interface{}{
		"status":    "ok",
		"message":   "Сервер кулинарной книги работает!",
		"timestamp": time.Now().Format(time.RFC3339),
		"database":  dbStatus,
		"version":   "1.0.0",
	}

	json.NewEncoder(w).Encode(response)
}

func createRecipeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, `{"error": "Метод не разрешен"}`, http.StatusMethodNotAllowed)
		return
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, `{"error": "Требуется авторизация"}`, http.StatusUnauthorized)
		return
	}

	tokenString := strings.Split(authHeader, " ")[1]
	userID, err := auth.GetUserIDFromToken(tokenString)
	if err != nil {
		http.Error(w, `{"error": "Невалидный токен"}`, http.StatusUnauthorized)
		return
	}

	var recipeReq struct {
		Title        string   `json:"title"`
		Description  string   `json:"description"`
		Ingredients  []string `json:"ingredients"`
		Instructions string   `json:"instructions"`
		CookingTime  int      `json:"cooking_time"`
		Difficulty   string   `json:"difficulty"`
		ImageBase64  string   `json:"image_base64"`
	}

	if err := json.NewDecoder(r.Body).Decode(&recipeReq); err != nil {
		http.Error(w, `{"error": "Неверный формат данных"}`, http.StatusBadRequest)
		return
	}

	recipe := &models.Recipe{
		UserID:       userID,
		Title:        recipeReq.Title,
		Description:  recipeReq.Description,
		Ingredients:  recipeReq.Ingredients,
		Instructions: recipeReq.Instructions,
		CookingTime:  recipeReq.CookingTime,
		Difficulty:   recipeReq.Difficulty,
		ImageBase64:  recipeReq.ImageBase64,
	}

	if err := recipeRepo.CreateRecipe(recipe); err != nil {
		http.Error(w, `{"error": "Ошибка при создании рецепта: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status":  "ok",
		"message": "Рецепт успешно создан",
		"recipe":  recipe,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func updateRecipeHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != "PUT" {
        http.Error(w, `{"error": "Метод не разрешен"}`, http.StatusMethodNotAllowed)
        return
    }

    authHeader := r.Header.Get("Authorization")
    if authHeader == "" {
        http.Error(w, `{"error": "Требуется авторизация"}`, http.StatusUnauthorized)
        return
    }

    tokenString := strings.Split(authHeader, " ")[1]
    userID, err := auth.GetUserIDFromToken(tokenString)
    if err != nil {
        http.Error(w, `{"error": "Невалидный токен"}`, http.StatusUnauthorized)
        return
    }

    var recipeReq struct {
        ID           int      `json:"id"`
        Title        string   `json:"title"`
        Description  string   `json:"description"`
        Ingredients  []string `json:"ingredients"`
        Instructions string   `json:"instructions"`
        CookingTime  int      `json:"cooking_time"`
        Difficulty   string   `json:"difficulty"`
        ImageBase64  string   `json:"image_base64"`
    }

    if err := json.NewDecoder(r.Body).Decode(&recipeReq); err != nil {
        http.Error(w, `{"error": "Неверный формат данных"}`, http.StatusBadRequest)
        return
    }

    if recipeReq.ID == 0 {
        http.Error(w, `{"error": "ID рецепта не указан"}`, http.StatusBadRequest)
        return
    }

    recipe := &models.Recipe{
        ID:           recipeReq.ID,
        UserID:       userID,
        Title:        recipeReq.Title,
        Description:  recipeReq.Description,
        Ingredients:  recipeReq.Ingredients,
        Instructions: recipeReq.Instructions,
        CookingTime:  recipeReq.CookingTime,
        Difficulty:   recipeReq.Difficulty,
        ImageBase64:  recipeReq.ImageBase64,
    }

    if err := recipeRepo.UpdateRecipe(recipe); err != nil {
        if strings.Contains(err.Error(), "no rows") {
            http.Error(w, `{"error": "Рецепт не найден или нет прав на редактирование"}`, http.StatusNotFound)
            return
        }
        http.Error(w, `{"error": "Ошибка при обновлении рецепта: `+err.Error()+`"}`, http.StatusInternalServerError)
        return
    }

    response := map[string]interface{}{
        "status":  "ok",
        "message": "Рецепт успешно обновлен",
        "recipe":  recipe,
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

func deleteRecipeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		http.Error(w, `{"error": "Метод не разрешен"}`, http.StatusMethodNotAllowed)
		return
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, `{"error": "Требуется авторизация"}`, http.StatusUnauthorized)
		return
	}

	tokenString := strings.Split(authHeader, " ")[1]
	userID, err := auth.GetUserIDFromToken(tokenString)
	if err != nil {
		http.Error(w, `{"error": "Невалидный токен"}`, http.StatusUnauthorized)
		return
	}

	recipeIDStr := r.URL.Query().Get("id")
	if recipeIDStr == "" {
		http.Error(w, `{"error": "ID рецепта не указан"}`, http.StatusBadRequest)
		return
	}

	recipeID, err := strconv.Atoi(recipeIDStr)
	if err != nil {
		http.Error(w, `{"error": "Неверный ID рецепта"}`, http.StatusBadRequest)
		return
	}

	if err := recipeRepo.DeleteRecipe(recipeID, userID); err != nil {
		http.Error(w, `{"error": "Ошибка при удалении рецепта: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status":  "ok",
		"message": "Рецепт успешно удален",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func favoritesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, `{"error": "Метод не разрешен"}`, http.StatusMethodNotAllowed)
		return
	}

	authHeader := r.Header.Get("Authorization")
	tokenString := strings.Split(authHeader, " ")[1]
	userID, err := auth.GetUserIDFromToken(tokenString)
	if err != nil {
		http.Error(w, `{"error": "Невалидный токен"}`, http.StatusUnauthorized)
		return
	}

	favoriteIDs, err := favoriteRepo.GetFavoriteRecipes(userID)
	if err != nil {
		http.Error(w, `{"error": "Ошибка при получении избранного"}`, http.StatusInternalServerError)
		return
	}

	var favoriteRecipes []models.Recipe
	for _, recipeID := range favoriteIDs {
		recipe, err := recipeRepo.GetRecipeByID(recipeID)
		if err == nil {
			recipe.IsFavorite = true
			favoriteRecipes = append(favoriteRecipes, *recipe)
		}
	}

	response := map[string]interface{}{
		"status":   "ok",
		"count":    len(favoriteRecipes),
		"recipes":  favoriteRecipes,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func addFavoriteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, `{"error": "Метод не разрешен"}`, http.StatusMethodNotAllowed)
		return
	}

	authHeader := r.Header.Get("Authorization")
	tokenString := strings.Split(authHeader, " ")[1]
	userID, err := auth.GetUserIDFromToken(tokenString)
	if err != nil {
		http.Error(w, `{"error": "Невалидный токен"}`, http.StatusUnauthorized)
		return
	}

	var req struct {
		RecipeID int `json:"recipe_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Неверный формат данных"}`, http.StatusBadRequest)
		return
	}

	_, err = recipeRepo.GetRecipeByID(req.RecipeID)
	if err != nil {
		http.Error(w, `{"error": "Рецепт не найден"}`, http.StatusNotFound)
		return
	}

	if err := favoriteRepo.AddFavorite(userID, req.RecipeID); err != nil {
		http.Error(w, `{"error": "Ошибка при добавлении в избранное"}`, http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status":  "ok",
		"message": "Рецепт добавлен в избранное",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func removeFavoriteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		http.Error(w, `{"error": "Метод не разрешен"}`, http.StatusMethodNotAllowed)
		return
	}

	authHeader := r.Header.Get("Authorization")
	tokenString := strings.Split(authHeader, " ")[1]
	userID, err := auth.GetUserIDFromToken(tokenString)
	if err != nil {
		http.Error(w, `{"error": "Невалидный токен"}`, http.StatusUnauthorized)
		return
	}

	recipeIDStr := r.URL.Query().Get("recipe_id")
	if recipeIDStr == "" {
		http.Error(w, `{"error": "ID рецепта не указан"}`, http.StatusBadRequest)
		return
	}

	recipeID, err := strconv.Atoi(recipeIDStr)
	if err != nil {
		http.Error(w, `{"error": "Неверный ID рецепта"}`, http.StatusBadRequest)
		return
	}

	if err := favoriteRepo.RemoveFavorite(userID, recipeID); err != nil {
		http.Error(w, `{"error": "Ошибка при удалении из избранного"}`, http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status":  "ok",
		"message": "Рецепт удален из избранного",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	log.Println("🚀 Запуск сервера кулинарной книги...")

	if err := initDB(); err != nil {
		log.Printf("⚠️  Предупреждение: %v", err)
		log.Println("⚠️  Сервер запустится без базы данных")
	} else {
		defer db.Close(context.Background())

		if err := createTables(); err != nil {
			log.Printf("⚠️  Не удалось создать таблицы: %v", err)
		}
	}

	http.HandleFunc("/api/health", healthHandler)
	http.HandleFunc("/api/register", registerHandler)
	http.HandleFunc("/api/login", loginHandler)
	http.HandleFunc("/api/recipes", recipesHandler)
	http.HandleFunc("/api/my-recipes", authMiddleware(myRecipesHandler))
	http.HandleFunc("/api/create-recipe", authMiddleware(createRecipeHandler))
	http.HandleFunc("/api/update-recipe", authMiddleware(updateRecipeHandler))
	http.HandleFunc("/api/delete-recipe", authMiddleware(deleteRecipeHandler))
	http.HandleFunc("/api/favorites", authMiddleware(favoritesHandler))
	http.HandleFunc("/api/favorites/add", authMiddleware(addFavoriteHandler))
	http.HandleFunc("/api/favorites/remove", authMiddleware(removeFavoriteHandler))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"app":        "Кулинарная книга",
			"version":    "1.0.0",
			"author":     "Кондратов Семён",
			"status":     "работает",
			"database":   "PostgreSQL",
			"container":  "Docker",
			"auth":       "JWT",
			"endpoints": []string{
				"POST /api/register",
				"POST /api/login",
				"GET  /api/recipes",
				"GET  /api/my-recipes (требует Bearer token)",
			},
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("📡 Сервер запущен на порту %s", port)
	log.Printf("🌐 Откройте в браузере: http://localhost:%s", port)
	log.Printf("🔧 Проверка здоровья: http://localhost:%s/api/health", port)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal("Ошибка запуска сервера:", err)
	}
}

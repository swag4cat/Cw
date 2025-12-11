package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// Структуры данных
type User struct {
	ID        int       `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type Recipe struct {
	ID           int       `json:"id"`
	UserID       int       `json:"user_id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Ingredients  []string  `json:"ingredients"`
	Instructions string    `json:"instructions"`
	CookingTime  int       `json:"cooking_time"`
	Difficulty   string    `json:"difficulty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type AuthResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Token   string `json:"token"`
	User    *User  `json:"user"`
}

type RecipesResponse struct {
	Status  string   `json:"status"`
	Message string   `json:"message"`
	Count   int      `json:"count"`
	Recipes []Recipe `json:"recipes"`
}

// Эмодзи символы
const (
	iconFood     = "🍳"
	iconRecipe   = "📝"
	iconSearch   = "🔍"
	iconTime     = "⏱"
	iconCalendar = "📅"
	iconUser     = "👤"
	iconAdd      = "➕"
	iconDelete   = "🗑"
	iconClose    = "✕"
	iconSuccess  = "✓"
	iconError    = "✗"
	iconBullet   = "•"
)

// Глобальные переменные
var (
	myApp           fyne.App
	myWindow        fyne.Window
	currentToken    string
	currentUser     *User
	recipeList      *widget.List
	recipes         []Recipe
	filteredRecipes []Recipe
	statusLabel     *widget.Label
	searchEntry     *widget.Entry
)

func getAPIURL() string {
	if url := os.Getenv("API_URL"); url != "" {
		return url
	}
	return "http://localhost:8080/api"
}

func main() {
	myApp = app.New()
	myWindow = myApp.NewWindow(fmt.Sprintf("%s Кулинарная книга v1.0", iconFood))
	myWindow.Resize(fyne.NewSize(900, 700))

	initUI()
	showAuthWindow()

	myWindow.ShowAndRun()
}

func initUI() {
	statusLabel = widget.NewLabel(fmt.Sprintf("%s Статус: Не авторизован", iconTime))
	statusLabel.TextStyle = fyne.TextStyle{Bold: true}

	// Поле поиска
	searchEntry = widget.NewEntry()
	searchEntry.SetPlaceHolder(fmt.Sprintf("%s Поиск рецептов...", iconSearch))
	searchEntry.OnChanged = func(searchText string) {
		if searchText == "" {
			filteredRecipes = recipes
		} else {
			filteredRecipes = []Recipe{}
			searchLower := strings.ToLower(searchText)
			for _, recipe := range recipes {
				if strings.Contains(strings.ToLower(recipe.Title), searchLower) ||
					strings.Contains(strings.ToLower(recipe.Description), searchLower) ||
					containsIngredient(recipe.Ingredients, searchLower) {
					filteredRecipes = append(filteredRecipes, recipe)
				}
			}
		}
		recipeList.Refresh()
	}

	recipeList = widget.NewList(
		func() int {
			if searchEntry.Text == "" {
				return len(recipes)
			}
			return len(filteredRecipes)
		},
		func() fyne.CanvasObject {
			return container.NewVBox(
				widget.NewLabel("Название"),
				widget.NewLabel("Описание"),
			)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			var recipe Recipe
			if searchEntry.Text == "" {
				if i < len(recipes) {
					recipe = recipes[i]
				} else {
					return
				}
			} else {
				if i < len(filteredRecipes) {
					recipe = filteredRecipes[i]
				} else {
					return
				}
			}

			vbox := o.(*fyne.Container)
			title := vbox.Objects[0].(*widget.Label)
			desc := vbox.Objects[1].(*widget.Label)

			// Иконка сложности
			difficultyIcon := "📊"
			switch recipe.Difficulty {
			case "легкая":
				difficultyIcon = "🟢"
			case "средняя":
				difficultyIcon = "🟡"
			case "сложная":
				difficultyIcon = "🔴"
			}

			title.SetText(fmt.Sprintf("%s %s", iconRecipe, recipe.Title))
			desc.SetText(fmt.Sprintf("%s %d мин | %s %s | %s %s",
				iconTime,
				recipe.CookingTime,
				difficultyIcon,
				recipe.Difficulty,
				iconCalendar,
				recipe.CreatedAt.Format("02.01"),
			))
		},
	)

	recipeList.OnSelected = func(id widget.ListItemID) {
		var recipe Recipe
		if searchEntry.Text == "" {
			if id < len(recipes) {
				recipe = recipes[id]
			} else {
				return
			}
		} else {
			if id < len(filteredRecipes) {
				recipe = filteredRecipes[id]
			} else {
				return
			}
		}
		showRecipeDetails(recipe)
		recipeList.Unselect(id)
	}
}

func containsIngredient(ingredients []string, search string) bool {
	for _, ing := range ingredients {
		if strings.Contains(strings.ToLower(ing), search) {
			return true
		}
	}
	return false
}

func showAuthWindow() {
	myWindow.SetTitle(fmt.Sprintf("%s Кулинарная книга - Авторизация", iconFood))

	username := widget.NewEntry()
	username.SetPlaceHolder("Имя пользователя")

	password := widget.NewPasswordEntry()
	password.SetPlaceHolder("Пароль")

	confirmPassword := widget.NewPasswordEntry()
	confirmPassword.SetPlaceHolder("Подтвердите пароль")

	loginForm := container.NewVBox(
		widget.NewLabelWithStyle(fmt.Sprintf("%s Вход в систему", iconUser), fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		username,
		password,
		widget.NewButton(fmt.Sprintf("%s Войти", iconSuccess), func() {
			if username.Text == "" || password.Text == "" {
				dialog.ShowError(fmt.Errorf("%s Заполните все поля", iconError), myWindow)
				return
			}
			login(username.Text, password.Text)
		}),
	)

	registerForm := container.NewVBox(
		widget.NewLabelWithStyle(fmt.Sprintf("%s Регистрация", iconAdd), fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		username,
		password,
		confirmPassword,
		widget.NewButton(fmt.Sprintf("%s Зарегистрироваться", iconAdd), func() {
			if password.Text != confirmPassword.Text {
				dialog.ShowError(fmt.Errorf("%s Пароли не совпадают", iconError), myWindow)
				return
			}
			if len(username.Text) < 3 {
				dialog.ShowError(fmt.Errorf("%s Имя пользователя должно быть не менее 3 символов", iconError), myWindow)
				return
			}
			if len(password.Text) < 6 {
				dialog.ShowError(fmt.Errorf("%s Пароль должен быть не менее 6 символов", iconError), myWindow)
				return
			}
			register(username.Text, password.Text)
		}),
	)

	tabs := container.NewAppTabs(
		container.NewTabItem(fmt.Sprintf("%s Вход", iconUser), loginForm),
		container.NewTabItem(fmt.Sprintf("%s Регистрация", iconAdd), registerForm),
	)

	mainContent := container.NewVBox(
		widget.NewLabelWithStyle(fmt.Sprintf("%s Кулинарная книга", iconFood), fyne.TextAlignCenter, fyne.TextStyle{
			Bold:   true,
			Italic: true,
		}),
		widget.NewLabel("Ваша личная коллекция рецептов"),
		widget.NewSeparator(),
		tabs,
	)

	myWindow.SetContent(container.NewCenter(mainContent))
}

func showMainWindow() {
	myWindow.SetTitle(fmt.Sprintf("%s Кулинарная книга - %s %s",
		iconFood, iconUser, currentUser.Username))

	refreshBtn := widget.NewButton(fmt.Sprintf("%s Обновить", iconSuccess), func() { loadRecipes() })
	addBtn := widget.NewButton(fmt.Sprintf("%s Добавить рецепт", iconAdd), func() { showAddRecipeForm() })
	logoutBtn := widget.NewButton(fmt.Sprintf("%s Выйти", iconClose), func() {
		currentToken = ""
		currentUser = nil
		recipes = []Recipe{}
		filteredRecipes = []Recipe{}
		showAuthWindow()
	})

	topPanel := container.NewVBox(
		container.NewHBox(
			statusLabel,
			layout.NewSpacer(),
			widget.NewLabel(fmt.Sprintf("%s %s", iconUser, currentUser.Username)),
		),
		container.NewBorder(
			nil,
			nil,
			widget.NewLabel(iconSearch),
			nil,
			searchEntry,
		),
		container.NewHBox(refreshBtn, addBtn, logoutBtn),
		widget.NewSeparator(),
	)

	content := container.NewBorder(
		topPanel,
		nil,
		nil,
		nil,
		container.NewScroll(recipeList),
	)

	myWindow.SetContent(content)
	loadRecipes()
}

func login(username, password string) {
	statusLabel.SetText(fmt.Sprintf("%s Статус: Вход...", iconTime))

	data, _ := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})

	resp, err := http.Post(getAPIURL()+"/login", "application/json", bytes.NewBuffer(data))
	if err != nil {
		dialog.ShowError(fmt.Errorf("%s Ошибка подключения: %v", iconError, err), myWindow)
		statusLabel.SetText(fmt.Sprintf("%s Статус: Ошибка подключения", iconError))
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		dialog.ShowError(fmt.Errorf("%s Ошибка авторизации: %s", iconError, string(body)), myWindow)
		statusLabel.SetText(fmt.Sprintf("%s Статус: Ошибка авторизации", iconError))
		return
	}

	var authResp AuthResponse
	json.Unmarshal(body, &authResp)

	if authResp.Status == "ok" {
		currentToken = authResp.Token
		currentUser = authResp.User
		statusLabel.SetText(fmt.Sprintf("%s Статус: Авторизован", iconSuccess))
		showMainWindow()
	} else {
		dialog.ShowError(fmt.Errorf("%s Ошибка: %s", iconError, authResp.Message), myWindow)
		statusLabel.SetText(fmt.Sprintf("%s Статус: Ошибка", iconError))
	}
}

func register(username, password string) {
	statusLabel.SetText(fmt.Sprintf("%s Статус: Регистрация...", iconTime))

	data, _ := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})

	resp, err := http.Post(getAPIURL()+"/register", "application/json", bytes.NewBuffer(data))
	if err != nil {
		dialog.ShowError(fmt.Errorf("%s Ошибка подключения: %v", iconError, err), myWindow)
		statusLabel.SetText(fmt.Sprintf("%s Статус: Ошибка подключения", iconError))
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		dialog.ShowError(fmt.Errorf("%s Ошибка регистрации: %s", iconError, string(body)), myWindow)
		statusLabel.SetText(fmt.Sprintf("%s Статус: Ошибка регистрации", iconError))
		return
	}

	var authResp AuthResponse
	json.Unmarshal(body, &authResp)

	if authResp.Status == "ok" {
		currentToken = authResp.Token
		currentUser = authResp.User
		statusLabel.SetText(fmt.Sprintf("%s Статус: Авторизован", iconSuccess))
		showMainWindow()
	} else {
		dialog.ShowError(fmt.Errorf("%s Ошибка: %s", iconError, authResp.Message), myWindow)
		statusLabel.SetText(fmt.Sprintf("%s Статус: Ошибка", iconError))
	}
}

func loadRecipes() {
	if currentToken == "" {
		return
	}

	statusLabel.SetText(fmt.Sprintf("%s Статус: Загрузка рецептов...", iconTime))

	client := &http.Client{}
	req, _ := http.NewRequest("GET", getAPIURL()+"/my-recipes", nil)
	req.Header.Set("Authorization", "Bearer "+currentToken)

	resp, err := client.Do(req)
	if err != nil {
		dialog.ShowError(fmt.Errorf("%s Ошибка загрузки: %v", iconError, err), myWindow)
		statusLabel.SetText(fmt.Sprintf("%s Статус: Ошибка загрузки", iconError))
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		dialog.ShowError(fmt.Errorf("%s Ошибка: %s", iconError, string(body)), myWindow)
		statusLabel.SetText(fmt.Sprintf("%s Статус: Ошибка", iconError))
		return
	}

	var recipesResp RecipesResponse
	json.Unmarshal(body, &recipesResp)

	if recipesResp.Status == "ok" {
		recipes = recipesResp.Recipes
		filteredRecipes = recipes
		recipeList.Refresh()
		statusLabel.SetText(fmt.Sprintf("%s Статус: %d рецептов загружено",
			iconSuccess, len(recipes)))
	} else {
		dialog.ShowError(fmt.Errorf("%s Ошибка: %s", iconError, recipesResp.Message), myWindow)
		statusLabel.SetText(fmt.Sprintf("%s Статус: Ошибка", iconError))
	}
}

func showAddRecipeForm() {
	dialogWindow := myApp.NewWindow(fmt.Sprintf("%s Новый рецепт", iconAdd))
	dialogWindow.Resize(fyne.NewSize(500, 600))

	titleEntry := widget.NewEntry()
	titleEntry.SetPlaceHolder("Название рецепта")

	descEntry := widget.NewMultiLineEntry()
	descEntry.SetPlaceHolder("Описание рецепта")
	descEntry.Wrapping = fyne.TextWrapWord

	ingredientsEntry := widget.NewMultiLineEntry()
	ingredientsEntry.SetPlaceHolder("Ингредиенты (каждый с новой строки)")
	ingredientsEntry.Wrapping = fyne.TextWrapWord

	instructionsEntry := widget.NewMultiLineEntry()
	instructionsEntry.SetPlaceHolder("Инструкции по приготовлению")
	instructionsEntry.Wrapping = fyne.TextWrapWord

	timeEntry := widget.NewEntry()
	timeEntry.SetPlaceHolder("Время приготовления (минуты)")

	difficultyEntry := widget.NewSelect([]string{"легкая", "средняя", "сложная"}, nil)
	difficultyEntry.PlaceHolder = "Выберите сложность"

	form := widget.NewForm(
		widget.NewFormItem("Название:", titleEntry),
		widget.NewFormItem("Описание:", descEntry),
		widget.NewFormItem("Ингредиенты:", ingredientsEntry),
		widget.NewFormItem("Инструкции:", instructionsEntry),
		widget.NewFormItem("Время (мин):", timeEntry),
		widget.NewFormItem("Сложность:", difficultyEntry),
	)

	form.OnSubmit = func() {
		if titleEntry.Text == "" {
			dialog.ShowError(fmt.Errorf("%s Введите название рецепта", iconError), dialogWindow)
			return
		}
		if difficultyEntry.Selected == "" {
			dialog.ShowError(fmt.Errorf("%s Выберите сложность", iconError), dialogWindow)
			return
		}

		createRecipe(
			titleEntry.Text,
			descEntry.Text,
			parseIngredients(ingredientsEntry.Text),
			instructionsEntry.Text,
			timeEntry.Text,
			difficultyEntry.Selected,
		)
		dialogWindow.Close()
	}

	form.OnCancel = func() {
		dialogWindow.Close()
	}

	form.SubmitText = fmt.Sprintf("%s Сохранить", iconSuccess)
	form.CancelText = fmt.Sprintf("%s Отмена", iconClose)

	dialogWindow.SetContent(container.NewVBox(
		widget.NewLabelWithStyle(fmt.Sprintf("%s Новый рецепт", iconRecipe),
			fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		form,
	))

	dialogWindow.Show()
}

func parseIngredients(text string) []string {
	var ingredients []string
	for _, line := range strings.Split(text, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			ingredients = append(ingredients, trimmed)
		}
	}
	return ingredients
}

func createRecipe(title, description string, ingredients []string, instructions, timeStr, difficulty string) {
	statusLabel.SetText(fmt.Sprintf("%s Статус: Создание рецепта...", iconTime))

	cookingTime := 0
	if n, err := strconv.Atoi(timeStr); err == nil {
		cookingTime = n
	}

	recipeData := map[string]interface{}{
		"title":        title,
		"description":  description,
		"ingredients":  ingredients,
		"instructions": instructions,
		"cooking_time": cookingTime,
		"difficulty":   difficulty,
	}

	jsonData, _ := json.Marshal(recipeData)

	client := &http.Client{}
	req, _ := http.NewRequest("POST", getAPIURL()+"/create-recipe", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+currentToken)

	resp, err := client.Do(req)
	if err != nil {
		dialog.ShowError(fmt.Errorf("%s Ошибка: %v", iconError, err), myWindow)
		statusLabel.SetText(fmt.Sprintf("%s Статус: Ошибка создания", iconError))
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 200 {
		dialog.ShowInformation(fmt.Sprintf("%s Успех", iconSuccess),
			"Рецепт успешно создан!", myWindow)
		loadRecipes()
	} else {
		dialog.ShowError(fmt.Errorf("%s Ошибка: %s", iconError, string(body)), myWindow)
		statusLabel.SetText(fmt.Sprintf("%s Статус: Ошибка", iconError))
	}
}

func showRecipeDetails(recipe Recipe) {
	dialogWindow := myApp.NewWindow(fmt.Sprintf("%s %s", iconFood, recipe.Title))
	dialogWindow.Resize(fyne.NewSize(600, 700))

	titleLabel := widget.NewLabelWithStyle(fmt.Sprintf("%s %s", iconRecipe, recipe.Title),
		fyne.TextAlignCenter, fyne.TextStyle{
			Bold:   true,
			Italic: true,
		})

	// Иконка сложности
	difficultyIcon := "📊"
	switch recipe.Difficulty {
	case "легкая":
		difficultyIcon = "🟢"
	case "средняя":
		difficultyIcon = "🟡"
	case "сложная":
		difficultyIcon = "🔴"
	}

	infoCard := container.NewVBox(
		widget.NewLabelWithStyle(fmt.Sprintf("%s Информация о рецепте", iconFood),
			fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		widget.NewLabel(fmt.Sprintf("%s Время приготовления: %d минут", iconTime, recipe.CookingTime)),
		widget.NewLabel(fmt.Sprintf("%s Сложность: %s %s", difficultyIcon, recipe.Difficulty, difficultyIcon)),
		widget.NewLabel(fmt.Sprintf("%s Добавлен: %s", iconCalendar, recipe.CreatedAt.Format("02.01.2006 15:04"))),
	)

	ingredientsBox := container.NewVBox(
		widget.NewLabelWithStyle(fmt.Sprintf("%s Ингредиенты", iconAdd),
			fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
	)
	for _, ing := range recipe.Ingredients {
		ingredientsBox.Add(widget.NewLabel(fmt.Sprintf("%s %s", iconBullet, ing)))
	}

	instructionsBox := container.NewVBox(
		widget.NewLabelWithStyle(fmt.Sprintf("%s Приготовление", iconFood),
			fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		widget.NewLabel(recipe.Instructions),
	)

	deleteBtn := widget.NewButton(fmt.Sprintf("%s Удалить", iconDelete), func() {
		confirmDialog := dialog.NewConfirm(fmt.Sprintf("%s Удаление рецепта", iconDelete),
			fmt.Sprintf("Вы уверены, что хотите удалить рецепт \"%s\"?\n%s Это действие нельзя отменить.",
				recipe.Title, iconError),
			func(confirmed bool) {
				if confirmed {
					deleteRecipe(recipe.ID)
					dialogWindow.Close()
				}
			}, dialogWindow)
		confirmDialog.Show()
	})

	closeBtn := widget.NewButton(fmt.Sprintf("%s Закрыть", iconClose), func() {
		dialogWindow.Close()
	})

	content := container.NewVBox(
		titleLabel,
		infoCard,
		ingredientsBox,
		instructionsBox,
		container.NewCenter(
			container.NewHBox(
				deleteBtn,
				closeBtn,
			),
		),
	)

	scroll := container.NewScroll(content)
	dialogWindow.SetContent(scroll)
	dialogWindow.Show()
}

func deleteRecipe(recipeID int) {
	statusLabel.SetText(fmt.Sprintf("%s Статус: Удаление рецепта...", iconTime))

	client := &http.Client{}
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/delete-recipe?id=%d", getAPIURL(), recipeID), nil)
	req.Header.Set("Authorization", "Bearer "+currentToken)

	resp, err := client.Do(req)
	if err != nil {
		dialog.ShowError(fmt.Errorf("%s Ошибка удаления: %v", iconError, err), myWindow)
		statusLabel.SetText(fmt.Sprintf("%s Статус: Ошибка удаления", iconError))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		dialog.ShowInformation(fmt.Sprintf("%s Успех", iconSuccess),
			"Рецепт успешно удален!", myWindow)
		loadRecipes()
	} else {
		body, _ := io.ReadAll(resp.Body)
		dialog.ShowError(fmt.Errorf("%s Ошибка: %s", iconError, string(body)), myWindow)
		statusLabel.SetText(fmt.Sprintf("%s Статус: Ошибка", iconError))
	}
}

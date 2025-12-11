package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
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

// Глобальные переменные
var (
	myApp        fyne.App
	myWindow     fyne.Window
	currentToken string
	currentUser  *User
	recipeList   *widget.List
	recipes      []Recipe
	statusLabel  *widget.Label
)

// Функция для получения URL API
func getAPIURL() string {
	if url := os.Getenv("API_URL"); url != "" {
		return url
	}
	return "http://localhost:8080/api"
}

func main() {
	myApp = app.New()
	myWindow = myApp.NewWindow("🍳 Кулинарная книга v1.0")
	myWindow.Resize(fyne.NewSize(900, 700))

	initUI()
	showAuthWindow()

	myWindow.ShowAndRun()
}

func initUI() {
	statusLabel = widget.NewLabel("Статус: Не авторизован")
	statusLabel.TextStyle = fyne.TextStyle{Bold: true}

	recipeList = widget.NewList(
		func() int { return len(recipes) },
		func() fyne.CanvasObject {
			return container.NewBorder(
				nil,
				nil,
				widget.NewIcon(theme.FileIcon()),
				nil,
				container.NewVBox(
					widget.NewLabel("Название"),
					widget.NewLabel("Описание"),
				),
			)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			recipe := recipes[i]
			cont := o.(*fyne.Container)
			vbox := cont.Objects[0].(*fyne.Container)
			title := vbox.Objects[0].(*widget.Label)
			desc := vbox.Objects[1].(*widget.Label)

			// Эмодзи для сложности
			difficultyEmoji := "📊"
			switch recipe.Difficulty {
			case "легкая":
				difficultyEmoji = "🟢"
			case "средняя":
				difficultyEmoji = "🟡"
			case "сложная":
				difficultyEmoji = "🔴"
			}

			title.SetText("🍴 " + recipe.Title)
			desc.SetText(fmt.Sprintf("⏱ %d мин | %s %s | 📅 %s",
				recipe.CookingTime,
				difficultyEmoji,
				recipe.Difficulty,
				recipe.CreatedAt.Format("02.01"),
			))
		},
	)

	recipeList.OnSelected = func(id widget.ListItemID) {
		showRecipeDetails(recipes[id])
		recipeList.Unselect(id)
	}
}

func showAuthWindow() {
	myWindow.SetTitle("🍳 Кулинарная книга - Авторизация")

	username := widget.NewEntry()
	username.SetPlaceHolder("Имя пользователя (мин. 3 символа)")

	password := widget.NewPasswordEntry()
	password.SetPlaceHolder("Пароль (мин. 6 символов)")

	confirmPassword := widget.NewPasswordEntry()
	confirmPassword.SetPlaceHolder("Подтвердите пароль")

	loginForm := container.NewVBox(
		widget.NewLabelWithStyle("Вход в систему", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		username,
		password,
		widget.NewButtonWithIcon("Войти", theme.LoginIcon(), func() {
			if username.Text == "" || password.Text == "" {
				dialog.ShowError(fmt.Errorf("Заполните все поля"), myWindow)
				return
			}
			login(username.Text, password.Text)
		}),
	)

	registerForm := container.NewVBox(
		widget.NewLabelWithStyle("Регистрация", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		username,
		password,
		confirmPassword,
		widget.NewButtonWithIcon("Зарегистрироваться", theme.ConfirmIcon(), func() {
			if password.Text != confirmPassword.Text {
				dialog.ShowError(fmt.Errorf("Пароли не совпадают"), myWindow)
				return
			}
			if len(username.Text) < 3 {
				dialog.ShowError(fmt.Errorf("Имя пользователя должно быть не менее 3 символов"), myWindow)
				return
			}
			if len(password.Text) < 6 {
				dialog.ShowError(fmt.Errorf("Пароль должен быть не менее 6 символов"), myWindow)
				return
			}
			register(username.Text, password.Text)
		}),
	)

	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("Вход", theme.LoginIcon(), loginForm),
		container.NewTabItemWithIcon("Регистрация", theme.ContentAddIcon(), registerForm),
	)

	mainContent := container.NewVBox(
		widget.NewLabelWithStyle("🍳 Кулинарная книга", fyne.TextAlignCenter, fyne.TextStyle{
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
	myWindow.SetTitle(fmt.Sprintf("🍳 Кулинарная книга - %s", currentUser.Username))

	refreshBtn := widget.NewButtonWithIcon("Обновить", theme.ViewRefreshIcon(), func() { loadRecipes() })
	addBtn := widget.NewButtonWithIcon("Добавить рецепт", theme.ContentAddIcon(), func() { showAddRecipeForm() })
	logoutBtn := widget.NewButtonWithIcon("Выйти", theme.LogoutIcon(), func() {
		currentToken = ""
		currentUser = nil
		recipes = []Recipe{}
		showAuthWindow()
	})

	userInfo := fmt.Sprintf("👤 %s | 📅 Регистрация: %s",
		currentUser.Username,
		currentUser.CreatedAt.Format("02.01.2006"),
	)

	topPanel := container.NewVBox(
		container.NewHBox(
			statusLabel,
			layout.NewSpacer(),
			widget.NewLabel(userInfo),
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
	statusLabel.SetText("Статус: Вход...")

	data, _ := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})

	resp, err := http.Post(getAPIURL()+"/login", "application/json", bytes.NewBuffer(data))
	if err != nil {
		dialog.ShowError(fmt.Errorf("Ошибка подключения: %v", err), myWindow)
		statusLabel.SetText("Статус: Ошибка подключения")
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		dialog.ShowError(fmt.Errorf("Ошибка авторизации: %s", string(body)), myWindow)
		statusLabel.SetText("Статус: Ошибка авторизации")
		return
	}

	var authResp AuthResponse
	json.Unmarshal(body, &authResp)

	if authResp.Status == "ok" {
		currentToken = authResp.Token
		currentUser = authResp.User
		statusLabel.SetText("Статус: Авторизован ✓")
		showMainWindow()
	} else {
		dialog.ShowError(fmt.Errorf("Ошибка: %s", authResp.Message), myWindow)
		statusLabel.SetText("Статус: Ошибка")
	}
}

func register(username, password string) {
	statusLabel.SetText("Статус: Регистрация...")

	data, _ := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})

	resp, err := http.Post(getAPIURL()+"/register", "application/json", bytes.NewBuffer(data))
	if err != nil {
		dialog.ShowError(fmt.Errorf("Ошибка подключения: %v", err), myWindow)
		statusLabel.SetText("Статус: Ошибка подключения")
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		dialog.ShowError(fmt.Errorf("Ошибка регистрации: %s", string(body)), myWindow)
		statusLabel.SetText("Статус: Ошибка регистрации")
		return
	}

	var authResp AuthResponse
	json.Unmarshal(body, &authResp)

	if authResp.Status == "ok" {
		currentToken = authResp.Token
		currentUser = authResp.User
		statusLabel.SetText("Статус: Авторизован ✓")
		showMainWindow()
	} else {
		dialog.ShowError(fmt.Errorf("Ошибка: %s", authResp.Message), myWindow)
		statusLabel.SetText("Статус: Ошибка")
	}
}

func loadRecipes() {
	if currentToken == "" {
		return
	}

	statusLabel.SetText("Статус: Загрузка рецептов...")

	client := &http.Client{}
	req, _ := http.NewRequest("GET", getAPIURL()+"/my-recipes", nil)
	req.Header.Set("Authorization", "Bearer "+currentToken)

	resp, err := client.Do(req)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Ошибка загрузки: %v", err), myWindow)
		statusLabel.SetText("Статус: Ошибка загрузки")
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		dialog.ShowError(fmt.Errorf("Ошибка: %s", string(body)), myWindow)
		statusLabel.SetText("Статус: Ошибка")
		return
	}

	var recipesResp RecipesResponse
	json.Unmarshal(body, &recipesResp)

	if recipesResp.Status == "ok" {
		recipes = recipesResp.Recipes
		recipeList.Refresh()
		statusLabel.SetText(fmt.Sprintf("Статус: %d рецептов загружено ✓", len(recipes)))
	} else {
		dialog.ShowError(fmt.Errorf("Ошибка: %s", recipesResp.Message), myWindow)
		statusLabel.SetText("Статус: Ошибка")
	}
}

func showAddRecipeForm() {
	dialogWindow := myApp.NewWindow("📝 Новый рецепт")
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
		// Валидация
		if titleEntry.Text == "" {
			dialog.ShowError(fmt.Errorf("Введите название рецепта"), dialogWindow)
			return
		}
		if difficultyEntry.Selected == "" {
			dialog.ShowError(fmt.Errorf("Выберите сложность"), dialogWindow)
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

	form.SubmitText = "Сохранить"
	form.CancelText = "Отмена"

	dialogWindow.SetContent(container.NewVBox(
		widget.NewLabelWithStyle("📝 Новый рецепт", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		form,
	))

	dialogWindow.Show()
}

func parseIngredients(text string) []string {
	var ingredients []string
	lines := splitLines(text)
	for _, line := range lines {
		if trimmed := trim(line); trimmed != "" {
			ingredients = append(ingredients, trimmed)
		}
	}
	return ingredients
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func trim(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func createRecipe(title, description string, ingredients []string, instructions, timeStr, difficulty string) {
	statusLabel.SetText("Статус: Создание рецепта...")

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
		dialog.ShowError(fmt.Errorf("Ошибка: %v", err), myWindow)
		statusLabel.SetText("Статус: Ошибка создания")
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 200 {
		dialog.ShowInformation("✅ Успех", "Рецепт успешно создан!", myWindow)
		loadRecipes()
	} else {
		dialog.ShowError(fmt.Errorf("Ошибка: %s", string(body)), myWindow)
		statusLabel.SetText("Статус: Ошибка")
	}
}

func showRecipeDetails(recipe Recipe) {
	dialogWindow := myApp.NewWindow(fmt.Sprintf("🍳 %s", recipe.Title))
	dialogWindow.Resize(fyne.NewSize(600, 700))

	titleLabel := widget.NewLabelWithStyle(recipe.Title, fyne.TextAlignCenter, fyne.TextStyle{
		Bold:   true,
		Italic: true,
	})
	// TextSize не поддерживается в этой версии Fyne, убираем

	difficultyEmoji := "📊"
	switch recipe.Difficulty {
	case "легкая":
		difficultyEmoji = "🟢"
	case "средняя":
		difficultyEmoji = "🟡"
	case "сложная":
		difficultyEmoji = "🔴"
	}

	infoCard := container.NewVBox(
		widget.NewLabelWithStyle("📋 Информация о рецепте", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		container.NewHBox(
			widget.NewIcon(theme.InfoIcon()), // Заменили FileTimeIcon на InfoIcon
			widget.NewLabel(fmt.Sprintf("⏱ Время: %d минут", recipe.CookingTime)),
			layout.NewSpacer(),
			widget.NewIcon(theme.InfoIcon()),
			widget.NewLabel(fmt.Sprintf("%s Сложность: %s", difficultyEmoji, recipe.Difficulty)),
		),
		container.NewHBox(
			widget.NewIcon(theme.InfoIcon()), // Заменили CalendarIcon на InfoIcon
			widget.NewLabel(fmt.Sprintf("📅 Добавлен: %s", recipe.CreatedAt.Format("02.01.2006 15:04"))),
		),
	)

	ingredientsBox := container.NewVBox(
		widget.NewLabelWithStyle("🛒 Ингредиенты", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
	)
	for _, ing := range recipe.Ingredients {
		ingredientsBox.Add(container.NewHBox(
			widget.NewIcon(theme.DocumentCreateIcon()),
			widget.NewLabel(fmt.Sprintf("  %s", ing)),
		))
	}

	instructionsBox := container.NewVBox(
		widget.NewLabelWithStyle("👨‍🍳 Приготовление", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		widget.NewLabel(recipe.Instructions),
	)

	deleteBtn := widget.NewButtonWithIcon("Удалить", theme.DeleteIcon(), func() {
		confirmDialog := dialog.NewConfirm("🗑 Удаление рецепта",
			fmt.Sprintf("Вы уверены, что хотите удалить рецепт \"%s\"?\nЭто действие нельзя отменить.", recipe.Title),
			func(confirmed bool) {
				if confirmed {
					deleteRecipe(recipe.ID)
					dialogWindow.Close()
				}
			}, dialogWindow)
		confirmDialog.SetDismissText("Отмена")
		confirmDialog.SetConfirmText("Удалить")
		confirmDialog.Show()
	})
	deleteBtn.Importance = widget.DangerImportance

	closeBtn := widget.NewButtonWithIcon("Закрыть", theme.CancelIcon(), func() {
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
	statusLabel.SetText("Статус: Удаление рецепта...")

	client := &http.Client{}
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/delete-recipe?id=%d", getAPIURL(), recipeID), nil)
	req.Header.Set("Authorization", "Bearer "+currentToken)

	resp, err := client.Do(req)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Ошибка удаления: %v", err), myWindow)
		statusLabel.SetText("Статус: Ошибка удаления")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		dialog.ShowInformation("✅ Успех", "Рецепт успешно удален!", myWindow)
		loadRecipes()
	} else {
		body, _ := io.ReadAll(resp.Body)
		dialog.ShowError(fmt.Errorf("Ошибка: %s", string(body)), myWindow)
		statusLabel.SetText("Статус: Ошибка")
	}
}

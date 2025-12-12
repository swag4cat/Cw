package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
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
	ImageBase64  string    `json:"image_base64,omitempty"`
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

// Эмодзи символы (исправляем проблемные)
const (
	iconFood     = "🍳"
	iconRecipe   = "📋"
	iconSearch   = "🔎"
	iconTime     = "🕐"
	iconCalendar = "📆"
	iconUser     = "👤"
	iconAdd      = "➕"
	iconDelete   = "🗑"
	iconClose    = "❌"
	iconSuccess  = "✅"
	iconError    = "❎"
	iconBullet   = "•"
	iconRefresh  = "🔄"
	iconExit     = "🚪"
	iconEdit     = "✏️" // ДОБАВЬТЕ ЭТУ СТРОКУ
)

// Глобальные переменные
var (
	myApp           fyne.App
	myWindow        fyne.Window
	currentToken    string
	currentUser     *User
	recipeGrid      *fyne.Container
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

	// Создаём Grid для карточек
	recipeGrid = container.NewGridWrap(fyne.NewSize(250, 200))
}

func truncateText(text string, maxLength int) string {
	if len(text) <= maxLength {
		return text
	}
	return text[:maxLength] + "..."
}

// Функция для обновления Grid
func updateRecipeGrid() {
	recipeGrid.Objects = nil

	var displayRecipes []Recipe
	if searchEntry.Text == "" {
		displayRecipes = recipes
	} else {
		displayRecipes = filteredRecipes
	}

	for _, recipe := range displayRecipes {
		recipeGrid.Add(createRecipeCard(recipe))
	}
	recipeGrid.Refresh()
}

// Функция создания карточки рецепта
func createRecipeCard(recipe Recipe) fyne.CanvasObject {
	// Создаём изображение для карточки
	var imageResource fyne.Resource

	if recipe.ImageBase64 != "" && len(recipe.ImageBase64) > 100 {
		// Пробуем декодировать base64
		imgData, err := base64.StdEncoding.DecodeString(recipe.ImageBase64)
		if err == nil {
			imageResource = fyne.NewStaticResource("recipe_"+strconv.Itoa(recipe.ID), imgData)
		}
	}

	// Если нет фото или ошибка декодирования - используем иконку
	if imageResource == nil {
		imageResource = theme.FileIcon()
	}

	// Создаём изображение
	cardImage := canvas.NewImageFromResource(imageResource)
	cardImage.FillMode = canvas.ImageFillContain
	cardImage.SetMinSize(fyne.NewSize(200, 120))

	// Создаём контейнер карточки
	cardContent := container.NewVBox(
		cardImage,
		widget.NewLabelWithStyle(recipe.Title, fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabel(fmt.Sprintf("%s %d мин | %s", iconTime, recipe.CookingTime, recipe.Difficulty)),
	)

	// Создаём кнопку (вместо карточки)
	cardButton := widget.NewButton("", func() {
		showRecipeDetails(recipe)
	})

	// Вставляем контент в кнопку
	cardContainer := container.NewStack(
		cardButton,
		cardContent,
	)

	return cardContainer
}

func containsIngredient(ingredients []string, search string) bool {
	searchLower := strings.ToLower(search)
	for _, ing := range ingredients {
		if strings.Contains(strings.ToLower(ing), searchLower) {
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

	refreshBtn := widget.NewButton(fmt.Sprintf("%s Обновить", iconRefresh), func() { loadRecipes() })
	addBtn := widget.NewButton(fmt.Sprintf("%s Добавить рецепт", iconAdd), func() {
		showAddRecipeFormWithImage()
	})
	logoutBtn := widget.NewButton(fmt.Sprintf("%s Выйти", iconExit), func() {
		currentToken = ""
		currentUser = nil
		recipes = []Recipe{}
		filteredRecipes = []Recipe{}
		showAuthWindow()
	})

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
		updateRecipeGrid()
	}

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

	// Используем Grid
	content := container.NewBorder(
		topPanel,
		nil,
		nil,
		nil,
		container.NewScroll(recipeGrid),
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
		updateRecipeGrid()
		statusLabel.SetText(fmt.Sprintf("%s Статус: %d рецептов загружено",
			iconSuccess, len(recipes)))
	} else {
		dialog.ShowError(fmt.Errorf("%s Ошибка: %s", iconError, recipesResp.Message), myWindow)
		statusLabel.SetText(fmt.Sprintf("%s Статус: Ошибка", iconError))
	}
}

func showAddRecipeFormWithImage() {
	dialogWindow := myApp.NewWindow(fmt.Sprintf("%s Новый рецепт с фото", iconAdd))
	dialogWindow.Resize(fyne.NewSize(500, 700))

	titleEntry := widget.NewEntry()
	titleEntry.SetPlaceHolder("Название рецепта")

	descEntry := widget.NewMultiLineEntry()
	descEntry.SetPlaceHolder("Описание рецепта")
	descEntry.Wrapping = fyne.TextWrapWord

	ingredientsEntry := widget.NewMultiLineEntry()
	ingredientsEntry.SetPlaceHolder("Ингредиенты (каждый с новой строки)")
	ingredientsEntry.Wrapping = fyne.TextWrapWord

	instructionsEntry := widget.NewMultiLineEntry()
	instructionsEntry.SetPlaceHolder("Инструкции по приготовления")
	instructionsEntry.Wrapping = fyne.TextWrapWord

	timeEntry := widget.NewEntry()
	timeEntry.SetPlaceHolder("Время приготовления (минуты)")

	difficultyEntry := widget.NewSelect([]string{"легкая", "средняя", "сложная"}, nil)
	difficultyEntry.PlaceHolder = "Выберите сложность"

	// Переменная для изображения
	var imageBase64 string

	// Превью изображения
	imagePreview := canvas.NewImageFromResource(theme.BrokenImageIcon())
	imagePreview.SetMinSize(fyne.NewSize(200, 150))
	imagePreview.FillMode = canvas.ImageFillContain

	// Функция для загрузки и отображения изображения
	loadAndDisplayImage := func(filePath string) {
		fmt.Printf("Загружаем изображение из: %s\n", filePath)

		imgBytes, err := os.ReadFile(filePath)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Ошибка чтения файла: %v", err), dialogWindow)
			return
		}

		// Проверяем размер (максимум 1MB для простоты)
		if len(imgBytes) > 1024*1024 {
			dialog.ShowError(fmt.Errorf("Изображение слишком большое (макс. 1MB)"), dialogWindow)
			return
		}

		// Конвертируем в base64
		imageBase64 = base64.StdEncoding.EncodeToString(imgBytes)

		// Показываем превью
		previewResource := fyne.NewStaticResource(
			filepath.Base(filePath),
			imgBytes,
		)
		imagePreview.Resource = previewResource
		imagePreview.Refresh()

		dialog.ShowInformation("✅", "Фото загружено!", dialogWindow)
	}

	// Кнопка выбора файла
	selectImageBtn := widget.NewButton("📁 Выбрать фото", func() {
		fileDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				if err != nil {
					fmt.Printf("Ошибка диалога: %v\n", err)
				}
				return
			}
			defer reader.Close()

			uri := reader.URI()
			fmt.Printf("Выбранный URI: %s\n", uri.String())

			// Преобразуем URI в путь файла
			filePath := ""
			if uri.Scheme() == "file" {
				filePath = uri.Path()
			} else {
				filePath = strings.TrimPrefix(uri.String(), "file://")
			}

			if filePath == "" {
				dialog.ShowError(fmt.Errorf("Не удалось получить путь к файлу"), dialogWindow)
				return
			}

			loadAndDisplayImage(filePath)
		}, dialogWindow)

		// Фильтруем только изображения
		fileDialog.SetFilter(storage.NewExtensionFileFilter([]string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp"}))


		fileDialog.Show()
	})

	// Удалить фото
	removeImageBtn := widget.NewButton("🗑 Удалить фото", func() {
		imageBase64 = ""
		imagePreview.Resource = theme.BrokenImageIcon()
		imagePreview.Refresh()
		dialog.ShowInformation("✅", "Фото удалено", dialogWindow)
	})

	imageControls := container.NewVBox(
		widget.NewLabel("📸 Фото рецепта (опционально):"),
		container.NewCenter(imagePreview),
		container.NewHBox(
			selectImageBtn,
			removeImageBtn,
		),
	)

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

		createRecipeWithImage(
			titleEntry.Text,
			descEntry.Text,
			parseIngredients(ingredientsEntry.Text),
			instructionsEntry.Text,
			timeEntry.Text,
			difficultyEntry.Selected,
			imageBase64,
		)
		dialogWindow.Close()
	}

	form.OnCancel = func() {
		dialogWindow.Close()
	}

	form.SubmitText = "Сохранить"
	form.CancelText = "Отмена"

	dialogWindow.SetContent(container.NewVBox(
		widget.NewLabelWithStyle(fmt.Sprintf("%s Новый рецепт", iconRecipe),
			fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		imageControls,
		widget.NewSeparator(),
		form,
	))

	dialogWindow.Show()
}

func showEditRecipeForm(recipe Recipe) {
    dialogWindow := myApp.NewWindow(fmt.Sprintf("%s Редактирование: %s", iconEdit, recipe.Title))
    dialogWindow.Resize(fyne.NewSize(500, 700))

    titleEntry := widget.NewEntry()
    titleEntry.SetText(recipe.Title) // Предзаполняем текущим значением

    descEntry := widget.NewMultiLineEntry()
    descEntry.SetText(recipe.Description)
    descEntry.Wrapping = fyne.TextWrapWord

    ingredientsEntry := widget.NewMultiLineEntry()
    ingredientsEntry.SetText(strings.Join(recipe.Ingredients, "\n"))
    ingredientsEntry.Wrapping = fyne.TextWrapWord

    instructionsEntry := widget.NewMultiLineEntry()
    instructionsEntry.SetText(recipe.Instructions)
    instructionsEntry.Wrapping = fyne.TextWrapWord

    timeEntry := widget.NewEntry()
    timeEntry.SetText(strconv.Itoa(recipe.CookingTime))

    difficultyEntry := widget.NewSelect([]string{"легкая", "средняя", "сложная"}, func(selected string) {
        // Обработчик выбора
    })
    difficultyEntry.Selected = recipe.Difficulty // Устанавливаем текущее значение
    difficultyEntry.PlaceHolder = "Выберите сложность"

    // Переменная для изображения
    var imageBase64 string = recipe.ImageBase64 // Начинаем с текущего изображения

    // Превью изображения
    imagePreview := canvas.NewImageFromResource(theme.BrokenImageIcon())
    imagePreview.SetMinSize(fyne.NewSize(200, 150))
    imagePreview.FillMode = canvas.ImageFillContain

    // Если есть текущее изображение - показываем его
    if recipe.ImageBase64 != "" && len(recipe.ImageBase64) > 100 {
        imgData, err := base64.StdEncoding.DecodeString(recipe.ImageBase64)
        if err == nil {
            previewResource := fyne.NewStaticResource("current_image", imgData)
            imagePreview.Resource = previewResource
            imagePreview.Refresh()
        }
    }

    // Функция для загрузки и отображения изображения
    loadAndDisplayImage := func(filePath string) {
        fmt.Printf("Загружаем новое изображение из: %s\n", filePath)

        imgBytes, err := os.ReadFile(filePath)
        if err != nil {
            dialog.ShowError(fmt.Errorf("Ошибка чтения файла: %v", err), dialogWindow)
            return
        }

        if len(imgBytes) > 1024*1024 {
            dialog.ShowError(fmt.Errorf("Изображение слишком большое (макс. 1MB)"), dialogWindow)
            return
        }

        // Конвертируем в base64
        imageBase64 = base64.StdEncoding.EncodeToString(imgBytes)

        // Показываем превью
        previewResource := fyne.NewStaticResource(
            filepath.Base(filePath),
            imgBytes,
        )
        imagePreview.Resource = previewResource
        imagePreview.Refresh()

        dialog.ShowInformation("✅", "Новое фото загружено!", dialogWindow)
    }

    // Кнопка выбора файла
    selectImageBtn := widget.NewButton("📁 Выбрать новое фото", func() {
        fileDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
            if err != nil || reader == nil {
                if err != nil {
                    fmt.Printf("Ошибка диалога: %v\n", err)
                }
                return
            }
            defer reader.Close()

            uri := reader.URI()
            fmt.Printf("Выбранный URI: %s\n", uri.String())

            filePath := ""
            if uri.Scheme() == "file" {
                filePath = uri.Path()
            } else {
                filePath = strings.TrimPrefix(uri.String(), "file://")
            }

            if filePath == "" {
                dialog.ShowError(fmt.Errorf("Не удалось получить путь к файлу"), dialogWindow)
                return
            }

            loadAndDisplayImage(filePath)
        }, dialogWindow)

        fileDialog.SetFilter(storage.NewExtensionFileFilter([]string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp"}))
        fileDialog.Resize(fyne.NewSize(600, 400))
        fileDialog.Show()
    })

    // Удалить фото
    removeImageBtn := widget.NewButton("🗑 Удалить фото", func() {
        imageBase64 = ""
        imagePreview.Resource = theme.BrokenImageIcon()
        imagePreview.Refresh()
        dialog.ShowInformation("✅", "Фото удалено", dialogWindow)
    })

    imageControls := container.NewVBox(
        widget.NewLabel("📸 Фото рецепта:"),
        widget.NewLabel("(оставьте текущее или загрузите новое)"),
        container.NewCenter(imagePreview),
        container.NewHBox(
            selectImageBtn,
            removeImageBtn,
        ),
    )

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

        updateRecipeWithImage(
            recipe.ID, // Передаем ID редактируемого рецепта
            titleEntry.Text,
            descEntry.Text,
            parseIngredients(ingredientsEntry.Text),
            instructionsEntry.Text,
            timeEntry.Text,
            difficultyEntry.Selected,
            imageBase64,
        )
        dialogWindow.Close()
    }

    form.OnCancel = func() {
        dialogWindow.Close()
    }

    form.SubmitText = "Сохранить изменения"
    form.CancelText = "Отмена"

    dialogWindow.SetContent(container.NewVBox(
        widget.NewLabelWithStyle(fmt.Sprintf("%s Редактирование рецепта", iconEdit),
            fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
        imageControls,
        widget.NewSeparator(),
        form,
    ))

    dialogWindow.Show()
}

func parseIngredients(text string) []string {
	var ingredients []string
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			ingredients = append(ingredients, trimmed)
		}
	}
	return ingredients
}

func createRecipeWithImage(title, description string, ingredients []string, instructions, timeStr, difficulty, imageBase64 string) {
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
		"image_base64": imageBase64,
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

func updateRecipeWithImage(recipeID int, title, description string, ingredients []string, instructions, timeStr, difficulty, imageBase64 string) {
    statusLabel.SetText(fmt.Sprintf("%s Статус: Обновление рецепта...", iconTime))

    cookingTime := 0
    if n, err := strconv.Atoi(timeStr); err == nil {
        cookingTime = n
    }

    recipeData := map[string]interface{}{
        "id":           recipeID, // Ключевое поле для идентификации
        "title":        title,
        "description":  description,
        "ingredients":  ingredients,
        "instructions": instructions,
        "cooking_time": cookingTime,
        "difficulty":   difficulty,
        "image_base64": imageBase64,
    }

    jsonData, _ := json.Marshal(recipeData)

    client := &http.Client{}
    // Используем PUT запрос и endpoint /api/update-recipe
    req, _ := http.NewRequest("PUT", getAPIURL()+"/update-recipe", bytes.NewBuffer(jsonData))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+currentToken)

    resp, err := client.Do(req)
    if err != nil {
        dialog.ShowError(fmt.Errorf("%s Ошибка при обновлении: %v", iconError, err), myWindow)
        statusLabel.SetText(fmt.Sprintf("%s Статус: Ошибка обновления", iconError))
        return
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)

    if resp.StatusCode == 200 {
        dialog.ShowInformation(fmt.Sprintf("%s Успех", iconSuccess),
            "Рецепт успешно обновлен!", myWindow)
        loadRecipes() // Перезагружаем список рецептов
    } else {
        // Парсим сообщение об ошибке
        var errorResp map[string]interface{}
        if err := json.Unmarshal(body, &errorResp); err == nil {
            if msg, ok := errorResp["error"].(string); ok {
                dialog.ShowError(fmt.Errorf("%s Ошибка: %s", iconError, msg), myWindow)
            } else {
                dialog.ShowError(fmt.Errorf("%s Ошибка: Неизвестная ошибка сервера", iconError), myWindow)
            }
        } else {
            dialog.ShowError(fmt.Errorf("%s Ошибка: %s", iconError, string(body)), myWindow)
        }
        statusLabel.SetText(fmt.Sprintf("%s Статус: Ошибка обновления", iconError))
    }
}

func showRecipeDetails(recipe Recipe) {
    dialogWindow := myApp.NewWindow(fmt.Sprintf("%s %s", iconFood, recipe.Title))
    dialogWindow.Resize(fyne.NewSize(650, 800))

    titleLabel := widget.NewLabelWithStyle(fmt.Sprintf("%s %s", iconRecipe, recipe.Title),
        fyne.TextAlignCenter, fyne.TextStyle{
            Bold:   true,
            Italic: true,
        })

    // Показываем фото если есть
    var imageContainer fyne.CanvasObject

    if recipe.ImageBase64 != "" && len(recipe.ImageBase64) > 100 {
        imgData, err := base64.StdEncoding.DecodeString(recipe.ImageBase64)
        if err == nil {
            imageResource := fyne.NewStaticResource("recipe_detail", imgData)
            recipeImage := canvas.NewImageFromResource(imageResource)
            recipeImage.FillMode = canvas.ImageFillContain
            recipeImage.SetMinSize(fyne.NewSize(300, 200))
            imageContainer = recipeImage
        }
    }

    if imageContainer == nil {
        imageContainer = widget.NewLabel("📷 Фото не загружено")
    }

    // 1. БЛОК С ОПИСАНИЕМ (ДОБАВЛЕНО)
    var descriptionBox fyne.CanvasObject
    if recipe.Description != "" {
        descriptionLabel := widget.NewLabel(recipe.Description)
        descriptionLabel.Wrapping = fyne.TextWrapWord

        descriptionBox = container.NewVBox(
            widget.NewLabelWithStyle(fmt.Sprintf("%s Описание", iconFood),
                fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
            widget.NewSeparator(),
            descriptionLabel,
        )
    } else {
        descriptionBox = widget.NewLabel("📝 Описание отсутствует")
    }

    difficultyIcon := "📊"
    switch recipe.Difficulty {
    case "легкая":
        difficultyIcon = "🟢"
    case "средняя":
        difficultyIcon = "🟡"
    case "сложная":
        difficultyIcon = "🔴"
    }

    // 2. ИНФОРМАЦИОННАЯ КАРТОЧКА
    infoCard := container.NewVBox(
        widget.NewLabelWithStyle(fmt.Sprintf("%s Информация о рецепте", iconFood),
            fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
        widget.NewSeparator(),
        widget.NewLabel(fmt.Sprintf("%s Время приготовления: %d минут", iconTime, recipe.CookingTime)),
        widget.NewLabel(fmt.Sprintf("%s Сложность: %s %s", difficultyIcon, recipe.Difficulty, difficultyIcon)),
        widget.NewLabel(fmt.Sprintf("%s Добавлен: %s", iconCalendar, recipe.CreatedAt.Format("02.01.2006 15:04"))),
    )

    // 3. ИНГРЕДИЕНТЫ
    ingredientsBox := container.NewVBox(
        widget.NewLabelWithStyle(fmt.Sprintf("%s Ингредиенты", iconAdd),
            fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
        widget.NewSeparator(),
    )
    for _, ing := range recipe.Ingredients {
        ingredientsBox.Add(widget.NewLabel(fmt.Sprintf("%s %s", iconBullet, ing)))
    }

    // 4. ИНСТРУКЦИИ С ПРОКРУТКОЙ (УЛУЧШЕННАЯ ВЕРСИЯ)
    instructionsTitle := widget.NewLabelWithStyle(fmt.Sprintf("%s Приготовление", iconFood),
        fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

    instructionsText := widget.NewLabel(recipe.Instructions)
    instructionsText.Wrapping = fyne.TextWrapWord

    instructionsContainer := container.NewVBox(
        instructionsTitle,
        widget.NewSeparator(),
        instructionsText,
    )

    var instructionsBox fyne.CanvasObject = instructionsContainer
    if len(recipe.Instructions) > 300 {
        instructionsBox = container.NewScroll(instructionsContainer)
        instructionsBox.(*container.Scroll).SetMinSize(fyne.NewSize(0, 200))
    }

    // Добавляем кнопки редактирования и удаления
    editBtn := widget.NewButton(fmt.Sprintf("%s Редактировать", iconEdit), func() {
	    dialogWindow.Close()
	    showEditRecipeForm(recipe) // Эту функцию создадим на следующем шаге
    })

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

    // 5. СОБИРАЕМ ВСЕ БЛОКИ ВМЕСТЕ
    content := container.NewVBox(
        titleLabel,
        container.NewCenter(imageContainer),
        descriptionBox,     // ← ОПИСАНИЕ ДОБАВЛЕНО ЗДЕСЬ
        infoCard,
        ingredientsBox,
        instructionsBox,
        container.NewCenter(
            container.NewHBox(
		editBtn,
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

package main

import (
	"fmt"
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/devalexandre/llmschat/database"
	"github.com/devalexandre/llmschat/llm"
	"github.com/devalexandre/llmschat/themes/dracula"
)

type ChatMessage struct {
	Text   string
	Sender string
	IsAI   bool
}

// Global variables
var (
	currentModel string
	mainScroll   *container.Scroll
)

func main() {
	// Initialize database
	fmt.Println("Initializing database...")
	if err := database.InitDB(); err != nil {
		fmt.Printf("Failed to initialize database: %v\n", err)
	}
	fmt.Println("Database initialized.")

	defer database.Close()

	a := app.New()
	a.Settings().SetTheme(&dracula.DraculaTheme{})
	w := a.NewWindow("AI Chat")
	w.Resize(fyne.NewSize(900, 700))

	// Chat messages container with padding
	messages := container.NewVBox()
	messagesContainer := container.NewPadded(messages)
	mainScroll = container.NewScroll(messagesContainer)
	mainScroll.SetMinSize(fyne.NewSize(600, 600))

	// Create model selection
	modelSelect := widget.NewSelect([]string{}, func(value string) {
		currentModel = value
	})
	modelSelect.Hide() // Hide initially

	// Check if we have API key configured and load available models
	settings, err := database.GetSettings()
	if err == nil && settings != nil && settings.APIKey != "" {
		// Get models for the current company
		models, err := database.GetModelsByCompany(settings.CompanyID)
		if err == nil && len(models) > 0 {
			modelNames := make([]string, len(models))
			for i, model := range models {
				modelNames[i] = model.Name
			}
			modelSelect.Options = modelNames

			// Set current model from settings
			for _, model := range models {
				if model.ID == settings.ModelID {
					modelSelect.SetSelected(model.Name)
					currentModel = model.Name
					break
				}
			}
			modelSelect.Show()
		}
	}

	// Styled input field
	input := widget.NewMultiLineEntry()
	input.SetPlaceHolder("Type your message...")
	input.SetMinRowsVisible(3)
	input.Resize(fyne.NewSize(500, 60))

	// Styled send button
	sendFunc := func() {
		userMessage := input.Text
		if userMessage != "" {
			AddMessage(messages, userMessage, "You", false)
			input.SetText("")
			// Get AI response with current model in stream mode
			go func() {
				// Create initial AI message container
				aiMessage := container.NewVBox()
				senderLabel := widget.NewLabel("AI")
				senderLabel.TextStyle = fyne.TextStyle{Italic: true}

				loadingLabel := widget.NewLabel("Loading...")
				aiMessage.Add(loadingLabel)
				messages.Refresh()

				stream, err := llm.GetResponseStream(userMessage, currentModel)
				aiMessage.Remove(loadingLabel)
				if err != nil {
					AddMessage(messages, fmt.Sprintf("Error: %v", err), "System", true)
					return
				}

				messageLabel := widget.NewRichText()
				messageLabel.Wrapping = fyne.TextWrapWord
				messageBox := container.NewVBox(messageLabel)
				messageContainer := container.NewBorder(
					nil, nil, layout.NewSpacer(), layout.NewSpacer(),
					messageBox,
				)

				aiMessage.Add(senderLabel)
				aiMessage.Add(messageContainer)
				aiMessage.Add(widget.NewSeparator())
				messages.Add(aiMessage)

				fullText := ""
				for chunk := range stream {
					fullText += chunk
					messageLabel.ParseMarkdown(fullText)
					messageLabel.Refresh()
					mainScroll.ScrollToBottom()
				}
			}()
		}
	}

	send := widget.NewButtonWithIcon("Send", theme.MailSendIcon(), sendFunc)
	send.Resize(fyne.NewSize(100, 60))

	input.OnSubmitted = func(s string) {
		sendFunc()
	}

	// Create a container with layout that respects sizes
	inputWrapper := container.NewHBox(layout.NewSpacer())
	inputWrapper.Add(input)

	// Create the input container with proper layout
	inputContainer := container.NewBorder(
		nil, nil, nil, send,
		container.NewStack(
			input,
		),
	)

	// Create sidebar with chat history
	sidebar := createSidebar(w)

	// Main content with model selector above messages
	mainContent := container.NewBorder(
		modelSelect, // Place model selector at top
		container.NewPadded(inputContainer),
		nil,
		nil,
		mainScroll,
	)

	content := container.NewHSplit(
		sidebar,
		mainContent,
	)
	content.SetOffset(0.2)

	w.SetContent(content)
	w.ShowAndRun()
}

func AddMessage(messages *fyne.Container, text, sender string, isAI bool) {
	// Create standard text label
	messageLabel := widget.NewRichTextFromMarkdown(text)

	// Create message container with proper alignment and styling
	messageBox := container.NewPadded(messageLabel)
	var messageContainer *fyne.Container

	if isAI {
		// AI message styling (left-aligned)
		messageContainer = container.NewHBox(
			messageBox,
			layout.NewSpacer(),
		)
	} else {
		// User message styling (right-aligned)
		messageContainer = container.NewHBox(
			layout.NewSpacer(),
			messageBox,
		)
	}

	// Add sender label
	senderLabel := widget.NewLabel(fmt.Sprintf("%s", sender))
	senderLabel.TextStyle = fyne.TextStyle{Italic: true}

	// Add message with padding
	messages.Add(container.NewVBox(
		senderLabel,
		messageContainer,
		widget.NewSeparator(),
	))

	messages.Refresh()
	mainScroll.ScrollToBottom()
}

func createSidebar(w fyne.Window) fyne.CanvasObject {
	// Create styled sidebar
	title := widget.NewLabelWithStyle("Chat History", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	separator := widget.NewSeparator()

	// Create new chat button
	newChatBtn := widget.NewButtonWithIcon("New Chat", theme.ContentAddIcon(), func() {
		// Implement new chat functionality
	})

	// Create settings button
	settingsBtn := widget.NewButtonWithIcon("Settings", theme.SettingsIcon(), func() {
		showSettingsModal(w)
	})

	// Sidebar content with settings at bottom
	topContent := container.NewVBox(
		title,
		separator,
		newChatBtn,
		widget.NewSeparator(),
	)

	// Create a container that pushes settings to bottom
	content := container.NewBorder(
		topContent,
		container.NewVBox(
			widget.NewSeparator(),
			settingsBtn,
		),
		nil, nil,
		layout.NewSpacer(), // This spacer pushes settings to bottom
	)

	return container.NewPadded(content)
}

func showSettingsModal(w fyne.Window) {
	// Create form fields with increased width
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Enter your name")
	nameEntry.Resize(fyne.NewSize(300, 36))

	apiKeyEntry := widget.NewPasswordEntry()
	apiKeyEntry.SetPlaceHolder("Enter your API key")
	apiKeyEntry.Resize(fyne.NewSize(300, 36))

	// Get companies from database
	companies, err := database.GetCompanies()
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to load companies: %v", err), w)
		return
	}

	// Create company names slice for select widget
	companyNames := make([]string, len(companies))
	companyMap := make(map[string]int) // Map company names to IDs
	for i, company := range companies {
		companyNames[i] = company.Name
		companyMap[company.Name] = company.ID
	}

	// Create model selection (will be updated based on company selection)
	var selectedCompanyID int
	var selectedModelID int
	modelSelect := widget.NewSelect([]string{}, func(value string) {
		// Find model ID from selected value
		models, err := database.GetModelsByCompany(selectedCompanyID)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to load models: %v", err), w)
			return
		}
		for _, model := range models {
			if model.Name == value {
				selectedModelID = model.ID
				break
			}
		}
	})
	modelSelect.Resize(fyne.NewSize(300, 36))
	modelSelect.Hide() // Hide initially until company is selected

	// Create company selection
	companySelect := widget.NewSelect(companyNames, func(value string) {
		selectedCompanyID = companyMap[value]
		// Load models for selected company
		models, err := database.GetModelsByCompany(selectedCompanyID)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to load models: %v", err), w)
			return
		}
		modelNames := make([]string, len(models))
		for i, model := range models {
			modelNames[i] = model.Name
		}
		modelSelect.Options = modelNames
		if len(modelNames) > 0 {
			modelSelect.SetSelected(modelNames[0])
			modelSelect.Show()
			modelSelect.Refresh()
		}
	})
	companySelect.Resize(fyne.NewSize(300, 36))

	// Load current settings if they exist
	if settings, err := database.GetSettings(); err == nil && settings != nil {
		nameEntry.SetText(settings.Name)
		apiKeyEntry.SetText(settings.APIKey)
		// Set company
		for name, id := range companyMap {
			if id == settings.CompanyID {
				companySelect.SetSelected(name)
				break
			}
		}
	}

	// Create form with wider layout
	formContainer := container.NewVBox(
		widget.NewForm(
			&widget.FormItem{Text: "Name", Widget: nameEntry},
			&widget.FormItem{Text: "Company", Widget: companySelect},
			&widget.FormItem{Text: "Model", Widget: modelSelect},
			&widget.FormItem{Text: "API Key", Widget: apiKeyEntry},
		),
	)

	// Create buttons
	saveBtn := widget.NewButton("Save", func() {
		if modelSelect.Selected == "" {
			dialog.ShowError(fmt.Errorf("Please select a model"), w)
			return
		}

		// Save settings to database
		err := database.SaveSettings(
			nameEntry.Text,
			selectedCompanyID,
			selectedModelID,
			apiKeyEntry.Text,
		)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to save settings: %v", err), w)
			return
		}
		dialog.ShowInformation("Success", "Settings saved", w)
	})
	cancelBtn := widget.NewButton("Cancel", func() {})

	// Create button container
	buttons := container.NewHBox(
		layout.NewSpacer(),
		cancelBtn,
		saveBtn,
	)

	// Create main container with padding
	content := container.NewVBox(
		formContainer,
		widget.NewSeparator(),
		buttons,
	)

	// Show custom dialog with increased size
	d := dialog.NewCustom("Settings", "", content, w)
	d.Resize(fyne.NewSize(400, 350))
	d.Show()

	// Trigger initial model list population if company is selected
	if companySelect.Selected != "" {
		companySelect.OnChanged(companySelect.Selected)
	}
}

func GetAIResponse(prompt string) string {
	response, err := llm.GetResponse(prompt, currentModel)
	if err != nil {
		fmt.Printf("Failed to get response: %v\n", err)
		return fmt.Sprintf("Error: %v", err)
	}

	defer func() {
		if r := recover(); r != nil {
			log.Fatalf("Application panicked: %v", r)
		}
	}()

	return response
}

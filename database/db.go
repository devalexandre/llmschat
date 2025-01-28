package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

type Company struct {
	ID      int
	Name    string
	BaseURL string
}

type Model struct {
	ID        int
	Name      string
	CompanyID int
}

type Settings struct {
	ID        int
	Name      string
	CompanyID int
	ModelID   int
	APIKey    string
}

var db *sql.DB

func InitDB() error {
	// Create database directory if it doesn't exist
	dbDir := "data"
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		log.Printf("Failed to create database directory: %v", err)
		return fmt.Errorf("failed to create database directory: %v", err)
	}

	dbPath := filepath.Join(dbDir, "chat.db")
	log.Printf("Opening database at: %s", dbPath)

	// Open database connection
	var err error
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Printf("Failed to open database: %v", err)
		return fmt.Errorf("failed to open database: %v", err)
	}

	// Check if database is accessible
	if err := db.Ping(); err != nil {
		log.Printf("Failed to ping database: %v", err)
		return fmt.Errorf("failed to ping database: %v", err)
	}

	// Create tables
	if err := createTables(); err != nil {
		log.Printf("Failed to create tables: %v", err)
		return fmt.Errorf("failed to create tables: %v", err)
	}

	// Initialize default data
	if err := initializeDefaultData(); err != nil {
		log.Printf("Failed to initialize default data: %v", err)
		return fmt.Errorf("failed to initialize default data: %v", err)
	}

	log.Printf("Database initialization completed successfully")
	return nil
}

func createTables() error {
	log.Printf("Creating/updating tables...")

	// Companies table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS companies (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			base_url TEXT
		)
	`)
	if err != nil {
		log.Printf("Failed to create companies table: %v", err)
		return fmt.Errorf("failed to create companies table: %v", err)
	}
	log.Printf("Companies table created/verified successfully")

	// Models table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS models (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			company_id INTEGER,
			FOREIGN KEY (company_id) REFERENCES companies (id),
			UNIQUE(name, company_id)
		)
	`)
	if err != nil {
		log.Printf("Failed to create models table: %v", err)
		return fmt.Errorf("failed to create models table: %v", err)
	}
	log.Printf("Models table created/verified successfully")

	// Settings table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS settings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			company_id INTEGER,
			model_id INTEGER,
			api_key TEXT,
			FOREIGN KEY (company_id) REFERENCES companies (id),
			FOREIGN KEY (model_id) REFERENCES models (id)
		)
	`)
	if err != nil {
		log.Printf("Failed to create settings table: %v", err)
		return fmt.Errorf("failed to create settings table: %v", err)
	}
	log.Printf("Settings table created/verified successfully")

	return nil
}

func initializeDefaultData() error {
	log.Printf("Initializing default data...")
	
	// Default companies and their models
	companies := map[string][]string{
		"OpenAI": {
			"gpt-4",
			"gpt-4-turbo",
			"gpt-4-32k",
			"gpt-3.5-turbo",
			"gpt-3.5-turbo-16k",
		},
		"Anthropic": {
			"claude-2.1",
			"claude-2.0",
			"claude-instant-1.2",
		},
		"Google": {
			"palm-2",
			"gemini-pro",
		},
		"Meta": {
			"llama-2-70b",
			"llama-2-13b",
		},
		"Mistral": {
			"mistral-7b",
			"mixtral-8x7b",
		},
		"Deepseek": {
			"deepseek-chat",
			"deepseek-reasoner",
		},
	}

	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		log.Printf("Failed to begin transaction: %v", err)
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Insert companies and their models
	for companyName, models := range companies {
		log.Printf("Processing company: %s", companyName)
		
		// Insert company
		var baseURL string
		switch companyName {
		case "Deepseek":
			baseURL = "https://api.deepseek.com"
		case "OpenAI":
			baseURL = "https://api.openai.com"
		case "Anthropic":
			baseURL = "https://api.anthropic.com"
		case "Google":
			baseURL = "https://generativelanguage.googleapis.com"
		}
		
		result, err := tx.Exec("INSERT OR IGNORE INTO companies (name, base_url) VALUES (?, ?)", companyName, baseURL)
		if err != nil {
			log.Printf("Failed to insert company %s: %v", companyName, err)
			return fmt.Errorf("failed to insert company %s: %v", companyName, err)
		}

		// Get company ID
		var companyID int64
		if id, err := result.LastInsertId(); err == nil && id != 0 {
			companyID = id
			log.Printf("Created new company %s with ID %d", companyName, companyID)
		} else {
			// If company already exists, get its ID
			var id int64
			err := tx.QueryRow("SELECT id FROM companies WHERE name = ?", companyName).Scan(&id)
			if err != nil {
				log.Printf("Failed to get ID for existing company %s: %v", companyName, err)
				return fmt.Errorf("failed to get ID for existing company %s: %v", companyName, err)
			}
			companyID = id
			log.Printf("Found existing company %s with ID %d", companyName, companyID)
		}

		// Insert models for this company
		for _, modelName := range models {
			_, err = tx.Exec("INSERT OR IGNORE INTO models (name, company_id) VALUES (?, ?)",
				modelName, companyID)
			if err != nil {
				log.Printf("Failed to insert model %s for company %s: %v", modelName, companyName, err)
				return fmt.Errorf("failed to insert model %s for company %s: %v", modelName, companyName, err)
			}
		}
		log.Printf("Successfully added/updated models for company %s", companyName)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		log.Printf("Failed to commit transaction: %v", err)
		return fmt.Errorf("failed to commit transaction: %v", err)
	}
	log.Printf("Successfully initialized default data")
	return nil
}

// GetCompanies returns all companies
func GetCompanies() ([]Company, error) {
	rows, err := db.Query("SELECT id, name, base_url FROM companies ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var companies []Company
	for rows.Next() {
		var c Company
		if err := rows.Scan(&c.ID, &c.Name, &c.BaseURL); err != nil {
			return nil, err
		}
		companies = append(companies, c)
	}
	return companies, nil
}

// GetModelsByCompany returns all models for a given company
func GetModelsByCompany(companyID int) ([]Model, error) {
	rows, err := db.Query("SELECT id, name FROM models WHERE company_id = ? ORDER BY name", companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var models []Model
	for rows.Next() {
		var m Model
		if err := rows.Scan(&m.ID, &m.Name); err != nil {
			return nil, err
		}
		m.CompanyID = companyID
		models = append(models, m)
	}
	return models, nil
}

// SaveSettings saves user settings
func SaveSettings(name string, companyID, modelID int, apiKey string) error {
	// Delete existing settings first (we only keep one settings record)
	_, err := db.Exec("DELETE FROM settings")
	if err != nil {
		return err
	}

	// Insert new settings
	_, err = db.Exec(`
		INSERT INTO settings (name, company_id, model_id, api_key)
		VALUES (?, ?, ?, ?)
	`, name, companyID, modelID, apiKey)
	return err
}

// GetSettings retrieves the current settings
func GetSettings() (*Settings, error) {
	var s Settings
	err := db.QueryRow(`
		SELECT id, name, company_id, model_id, api_key
		FROM settings LIMIT 1
	`).Scan(&s.ID, &s.Name, &s.CompanyID, &s.ModelID, &s.APIKey)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// Close closes the database connection
func Close() {
	if db != nil {
		if err := db.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}
}

package cli

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/thotenn/aicarousel/internal/db/apikeys"
	"github.com/thotenn/aicarousel/internal/db/provsettings"
)

// App holds shared dependencies for all CLI commands.
type App struct {
	DB   *sql.DB
	Keys apikeys.Repo
	Prov provsettings.Repo
}

// providerMeta is the canonical ordered list of all known providers. Order
// matches the TS source so menus look the same.
var providerMeta = []struct {
	key    string
	name   string
	envKey string
}{
	{"cerebras", "Cerebras", "CEREBRAS_API_KEY"},
	{"groq", "Groq", "GROQ_API_KEY"},
	{"openrouter", "OpenRouter", "OPENROUTER_API_KEY"},
	{"gemini", "Gemini", "GEMINI_API_KEY"},
	{"ollama", "Ollama", "OLLAMA_ENABLED"},
	{"zai", "Z.ai", "ZAI_API_KEY"},
}

// providerEntry holds provider info enriched from the .env file.
type providerEntry struct {
	key    string
	name   string
	envKey string
	hasKey bool
	value  string
}

// buildProviderList returns providerMeta enriched with current .env values.
func buildProviderList() []providerEntry {
	out := make([]providerEntry, len(providerMeta))
	for i, pm := range providerMeta {
		val := os.Getenv(pm.envKey)
		if val == "" {
			val = GetValue(pm.envKey)
		}
		out[i] = providerEntry{
			key:    pm.key,
			name:   pm.name,
			envKey: pm.envKey,
			hasKey: val != "",
			value:  val,
		}
	}
	return out
}

// providerKeys returns the list of all known provider keys.
func providerKeys() []string {
	keys := make([]string, len(providerMeta))
	for i, pm := range providerMeta {
		keys[i] = pm.key
	}
	return keys
}

// Run starts the interactive main menu loop.
func Run(app *App) {
	for {
		ClearScreen()
		Header("🎠 AICarousel Setup")

		fmt.Println("  1. 🔧 Setup inicial (base de datos y migraciones)")
		fmt.Println("  2. 🔑 Gestionar API Keys de Providers")
		fmt.Println("  3. 🎫 Gestionar API Keys de la Aplicación")
		fmt.Println("  4. ⚡ Seleccionar/Deseleccionar Providers")
		fmt.Println("  5. 🎯 Gestionar Modelos de Providers")
		fmt.Println("  6. 📊 Ver estado actual")
		fmt.Println("  0. ❌ Salir")
		fmt.Println()

		choice, _ := Ask("> ")

		switch choice {
		case "1":
			RunSetup(app)
		case "2":
			ManageProviderKeys(app)
		case "3":
			ManageAppKeys(app)
		case "4":
			ManageProviderToggle(app)
		case "5":
			ManageModels(app)
		case "6":
			ShowStatus(app)
		case "0", "q", "exit", "salir":
			ClearScreen()
			Info("¡Hasta luego!")
			fmt.Println()
			os.Exit(0)
		}
	}
}

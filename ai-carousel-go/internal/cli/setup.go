package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/thotenn/aicarousel-go/internal/db"
	"github.com/thotenn/aicarousel-go/internal/providers"
)

// RunSetup implements menu option 1: initial database and environment setup.
func RunSetup(app *App) {
	ClearScreen()
	Section("🔧 Setup Inicial")

	ctx := context.Background()

	// Step 1 — data directory.
	Step("Verificando directorio de datos...", false)
	if err := os.MkdirAll("data", 0o755); err != nil {
		Error(fmt.Sprintf("No se pudo crear ./data: %v", err))
	} else {
		// Ensure .gitkeep exists.
		f, err := os.OpenFile("data/.gitkeep", os.O_CREATE|os.O_WRONLY, 0o644)
		if err == nil {
			f.Close() //nolint:errcheck
		}
		Step("Directorio de datos listo", true)
	}

	// Step 2 — .env file.
	Step("Verificando archivo .env...", false)
	if !Exists() {
		created, err := CreateFromTemplate()
		if err != nil {
			Error(fmt.Sprintf("Error creando .env: %v", err))
		} else if created {
			Step(fmt.Sprintf("Archivo .env creado en %s", Path()), true)
			Info("Recuerda configurar las API keys de los providers.")
		} else {
			// No template — write minimal .env.
			if werr := os.WriteFile(Path(), []byte("PORT=7123\n"), 0o600); werr != nil {
				Error(fmt.Sprintf("Error escribiendo .env: %v", werr))
			} else {
				Step(fmt.Sprintf("Archivo .env mínimo creado en %s", Path()), true)
			}
		}
	} else {
		Step("Archivo .env existente", true)
	}

	// Step 3 — migrations.
	Step("Ejecutando migraciones de base de datos...", false)
	if err := db.Up(app.DB); err != nil {
		Error(fmt.Sprintf("Error en migraciones: %v", err))
	} else {
		Step("Migraciones completadas", true)
	}

	// Step 4 — sync providers.
	Step("Sincronizando providers...", false)
	keys := make([]string, 0, len(providers.Meta))
	for k := range providers.Meta {
		keys = append(keys, k)
	}
	if err := app.Prov.Sync(ctx, keys); err != nil {
		Error(fmt.Sprintf("Error sincronizando providers: %v", err))
	} else {
		Step(fmt.Sprintf("%d providers sincronizados", len(keys)), true)
	}

	fmt.Println()
	Success("¡Setup completado exitosamente!")
	fmt.Println()
	Info("Próximos pasos:")
	fmt.Println("  1. Configura las API Keys de los providers (opción 2 del menú)")
	fmt.Println("  2. Crea una API Key para la aplicación (opción 3 del menú)")
	fmt.Println("  3. Inicia el servidor con: aicarousel-server")
	fmt.Println()

	PressEnter("")
}

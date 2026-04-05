package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/thotenn/aicarousel/internal/config"
	"github.com/thotenn/aicarousel/internal/db"
)

// ShowStatus implements menu option 6: display current system status.
func ShowStatus(app *App) {
	ClearScreen()
	Section("📊 Estado del Sistema")

	ctx := context.Background()

	// Database
	dbPath := config.Cfg.DBPath
	if dbPath == "" {
		dbPath = "data/aicarousel.db"
	}
	_, dbErr := os.Stat(dbPath)
	dbExists := dbErr == nil

	fmt.Println(Bold + "Base de Datos" + Reset)
	fmt.Printf("  Ubicación: %s\n", dbPath)
	if dbExists {
		fmt.Printf("  Estado:    %s\n", Color(Green, "✓ Conectada"))
		if err := db.Up(app.DB); err != nil {
			fmt.Printf("  Migraciones: %s\n", Color(Red, "✗ Error"))
		} else {
			fmt.Printf("  Migraciones: %s\n", Color(Green, "✓ Actualizadas"))
		}
	} else {
		fmt.Printf("  Estado:    %s\n", Color(Red, "✗ No existe"))
	}
	fmt.Println()

	// .env file
	fmt.Println(Bold + "Archivo .env" + Reset)
	if Exists() {
		fmt.Printf("  Estado: %s\n", Color(Green, "✓ Existe"))
	} else {
		fmt.Printf("  Estado: %s\n", Color(Red, "✗ No existe"))
	}
	port := GetValue("PORT")
	if port == "" {
		port = "7123 (default)"
	}
	fmt.Printf("  Puerto: %s\n", port)
	fmt.Println()

	// Providers
	fmt.Println(Bold + "Providers" + Reset)
	if dbExists {
		app.Prov.Sync(ctx, providerKeys()) //nolint:errcheck
		settings, _ := app.Prov.All(ctx)
		settingsMap := make(map[string]struct {
			enabled  int
			priority int
		})
		for _, s := range settings {
			settingsMap[s.ProviderKey] = struct {
				enabled  int
				priority int
			}{s.IsEnabled, s.Priority}
		}
		for _, pm := range providerMeta {
			s, ok := settingsMap[pm.key]
			hasKey := os.Getenv(pm.envKey) != ""
			var statusStr string
			if !hasKey {
				statusStr = Color(Red, "✗ Sin API Key")
			} else if !ok || s.enabled == 0 {
				statusStr = Color(Yellow, "⊘ Desactivado")
			} else {
				statusStr = Color(Green, fmt.Sprintf("✓ Activo (orden: %d)", s.priority))
			}
			fmt.Printf("  %-12s %s\n", pm.name, statusStr)
		}
	} else {
		for _, pm := range providerMeta {
			hasKey := os.Getenv(pm.envKey) != ""
			var statusStr string
			if hasKey {
				statusStr = Color(Green, "✓ API Key configurada")
			} else {
				statusStr = Color(Red, "✗ Sin API Key")
			}
			fmt.Printf("  %-12s %s\n", pm.name, statusStr)
		}
	}
	fmt.Println()

	// App API keys
	fmt.Println(Bold + "API Keys de la Aplicación" + Reset)
	if dbExists {
		keys, err := app.Keys.List(ctx)
		if err != nil {
			fmt.Printf("  %s\n", Color(Red, "✗ Error leyendo API Keys"))
		} else if len(keys) == 0 {
			fmt.Printf("  %s\n", Color(Yellow, "⚠ No hay API Keys creadas"))
			Info("  Crea una con la opción 3 del menú principal")
		} else {
			active := 0
			revoked := 0
			for _, k := range keys {
				if k.IsActive == 1 {
					active++
				} else {
					revoked++
				}
			}
			fmt.Printf("  Total:    %d\n", len(keys))
			fmt.Printf("  Activas:  %s\n", Color(Green, fmt.Sprintf("%d", active)))
			if revoked > 0 {
				fmt.Printf("  Revocadas: %s\n", Color(Red, fmt.Sprintf("%d", revoked)))
			}
		}
	} else {
		fmt.Printf("  %s\n", Color(Dim, "- Base de datos no inicializada"))
	}
	fmt.Println()

	// Server info
	if port == "7123 (default)" {
		port = "7123"
	}
	fmt.Println(Bold + "Servidor" + Reset)
	fmt.Printf("  Puerto:   %s\n", port)
	fmt.Printf("  URL:      http://localhost:%s\n", port)
	fmt.Println("  Iniciar:  aicarousel-server")
	fmt.Println()

	// Quick commands
	fmt.Println(Bold + "Comandos Rápidos" + Reset)
	fmt.Println("  aicarousel-setup     - Este menú")
	fmt.Println("  aicarousel-server    - Iniciar servidor")
	fmt.Println("  aicarousel-apikey list - Listar API Keys")
	fmt.Println()

	PressEnter("")
}

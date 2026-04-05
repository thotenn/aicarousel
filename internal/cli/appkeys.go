package cli

import (
	"context"
	"fmt"
)

// ManageAppKeys implements menu option 3: manage application API keys.
func ManageAppKeys(app *App) {
	ctx := context.Background()

	for {
		ClearScreen()
		Section("🎫 API Keys de la Aplicación")

		keys, err := app.Keys.List(ctx)
		if err != nil {
			Error(fmt.Sprintf("Error leyendo API Keys: %v", err))
			PressEnter("")
			return
		}

		if len(keys) == 0 {
			Info("No hay API Keys creadas.")
			fmt.Println()
			fmt.Println("  1. Crear nueva API Key")
			fmt.Println("  0. Volver")
			fmt.Println()
			n, _, _ := AskNumber("> ")
			if n == 1 {
				createAppKey(app, ctx)
			} else {
				return
			}
			continue
		}

		// Table: ID, Prefix, Nombre, Último uso, Estado, Usos
		headers := []string{"ID", "Prefix", "Nombre", "Último uso", "Estado", "Usos"}
		rows := make([][]string, len(keys))
		for i, k := range keys {
			name := "-"
			if k.Name.Valid {
				name = k.Name.String
			}
			lastUsed := "-"
			if k.LastUsedAt.Valid && len(k.LastUsedAt.String) >= 16 {
				lastUsed = k.LastUsedAt.String[:16]
			}
			var estado string
			if k.IsActive == 1 {
				estado = Color(Green, "✓ Activa")
			} else {
				estado = Color(Red, "✗ Revocada")
			}
			rows[i] = []string{
				fmt.Sprintf("%d", k.ID),
				k.KeyPrefix,
				name,
				lastUsed,
				estado,
				fmt.Sprintf("%d", k.UsageCount),
			}
		}
		Table(headers, rows, []int{4, 12, 15, 17, 12, 6})

		fmt.Println()
		fmt.Println("  1. Crear nueva API Key")
		fmt.Println("  2. Revocar API Key")
		fmt.Println("  3. Eliminar API Key")
		fmt.Println("  0. Volver")
		fmt.Println()

		n, _, _ := AskNumber("> ")
		switch n {
		case 1:
			createAppKey(app, ctx)
		case 2:
			revokeAppKey(app, ctx)
		case 3:
			deleteAppKey(app, ctx)
		default:
			return
		}
	}
}

func createAppKey(app *App, ctx context.Context) {
	fmt.Println()
	name, _ := Ask("Nombre para la nueva API Key (opcional): ")

	plain, rec, err := app.Keys.Create(ctx, name)
	if err != nil {
		Error(fmt.Sprintf("Error creando API Key: %v", err))
		PressEnter("")
		return
	}

	ApiKeyBox(plain)

	fmt.Println("  Detalles:")
	fmt.Printf("    ID:      %d\n", rec.ID)
	n := "(sin nombre)"
	if rec.Name.Valid {
		n = rec.Name.String
	}
	fmt.Printf("    Nombre:  %s\n", n)
	fmt.Printf("    Creada:  %s\n", rec.CreatedAt)
	fmt.Println()
	Info("Usa esta key en el header 'Authorization: Bearer <key>'")
	Info("o en el header 'x-api-key: <key>'")

	PressEnter("")
}

func revokeAppKey(app *App, ctx context.Context) {
	fmt.Println()
	id, ok, err := AskNumber("ID de la API Key a revocar: ")
	if err != nil || !ok || id == 0 {
		Info("Operación cancelada")
		PressEnter("")
		return
	}

	confirmed, _ := Confirm(fmt.Sprintf("¿Revocar API Key #%d?", id), false)
	if !confirmed {
		Info("Operación cancelada")
		PressEnter("")
		return
	}

	found, err := app.Keys.Revoke(ctx, int64(id))
	if err != nil {
		Error(fmt.Sprintf("Error: %v", err))
	} else if found {
		Success(fmt.Sprintf("API Key #%d revocada", id))
		Info("La key ya no funcionará para autenticarse")
	} else {
		Error(fmt.Sprintf("API Key #%d no encontrada", id))
	}
	PressEnter("")
}

func deleteAppKey(app *App, ctx context.Context) {
	fmt.Println()
	id, ok, err := AskNumber("ID de la API Key a eliminar: ")
	if err != nil || !ok || id == 0 {
		Info("Operación cancelada")
		PressEnter("")
		return
	}

	confirmed, _ := Confirm(fmt.Sprintf("¿ELIMINAR PERMANENTEMENTE API Key #%d?", id), false)
	if !confirmed {
		Info("Operación cancelada")
		PressEnter("")
		return
	}

	found, err := app.Keys.Delete(ctx, int64(id))
	if err != nil {
		Error(fmt.Sprintf("Error: %v", err))
	} else if found {
		Success(fmt.Sprintf("API Key #%d eliminada permanentemente", id))
	} else {
		Error(fmt.Sprintf("API Key #%d no encontrada", id))
	}
	PressEnter("")
}

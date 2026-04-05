package cli

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// ManageProviderToggle implements menu option 4: enable/disable providers and reorder.
func ManageProviderToggle(app *App) {
	ctx := context.Background()

	// Sync first so all known providers have rows.
	keys := providerKeys()
	app.Prov.Sync(ctx, keys) //nolint:errcheck

	for {
		ClearScreen()
		Section("⚡ Providers Activos")

		statuses := buildProviderStatuses(app, ctx)

		headers := []string{"#", "Provider", "API Key", "Habilitado", "Orden"}
		rows := make([][]string, len(statuses))
		for i, s := range statuses {
			var apiKeyCol, enabledCol, orderCol string
			if s.hasKey {
				apiKeyCol = Color(Green, "✓ Config.")
			} else {
				apiKeyCol = Color(Red, "✗ Falta")
			}
			if !s.hasKey {
				enabledCol = Color(Dim, "- N/A")
				orderCol = "-"
			} else if s.isEnabled {
				enabledCol = Checkbox(true) + " Activo"
				orderCol = strconv.Itoa(s.priority)
			} else {
				enabledCol = Checkbox(false) + " Inactivo"
				orderCol = "-"
			}
			rows[i] = []string{fmt.Sprintf("%d", i+1), s.name, apiKeyCol, enabledCol, orderCol}
		}
		Table(headers, rows, []int{3, 13, 11, 17, 7})

		fmt.Println()
		Info("Providers sin API Key no pueden activarse.")
		Info("El orden determina la rotación round-robin.")
		fmt.Println()
		fmt.Println("  1. Toggle provider (activar/desactivar)")
		fmt.Println("  2. Cambiar orden de rotación")
		fmt.Println("  0. Volver")
		fmt.Println()

		n, _, _ := AskNumber("> ")
		switch n {
		case 1:
			toggleProviderMenu(app, ctx, statuses)
		case 2:
			changeOrderMenu(app, ctx, statuses)
		default:
			return
		}
	}
}

type providerStatus struct {
	key       string
	name      string
	hasKey    bool
	isEnabled bool
	priority  int
}

func buildProviderStatuses(app *App, ctx context.Context) []providerStatus {
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

	out := make([]providerStatus, 0, len(providerMeta))
	for _, pm := range providerMeta {
		s, ok := settingsMap[pm.key]
		priority := 999
		isEnabled := false
		if ok {
			priority = s.priority
			isEnabled = s.enabled == 1
		}
		out = append(out, providerStatus{
			key:       pm.key,
			name:      pm.name,
			hasKey:    os.Getenv(pm.envKey) != "",
			isEnabled: isEnabled,
			priority:  priority,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].priority < out[j].priority
	})
	return out
}

func toggleProviderMenu(app *App, ctx context.Context, statuses []providerStatus) {
	fmt.Println()
	idx, ok, _ := AskNumber("Número del provider a toggle: ")
	if !ok || idx < 1 || idx > len(statuses) {
		return
	}
	p := statuses[idx-1]

	if !p.hasKey {
		fmt.Println()
		Warning(fmt.Sprintf("%s no tiene API Key configurada.", p.name))
		Info("Configura la API Key primero en la opción 2 del menú principal.")
		PressEnter("")
		return
	}

	newState := !p.isEnabled
	if _, err := app.Prov.Toggle(ctx, p.key, newState); err != nil {
		Error(fmt.Sprintf("Error: %v", err))
	} else {
		fmt.Println()
		if newState {
			Success(fmt.Sprintf("%s activado", p.name))
		} else {
			Success(fmt.Sprintf("%s desactivado", p.name))
		}
	}
	PressEnter("")
}

func changeOrderMenu(app *App, ctx context.Context, statuses []providerStatus) {
	active := make([]providerStatus, 0)
	for _, s := range statuses {
		if s.hasKey && s.isEnabled {
			active = append(active, s)
		}
	}

	if len(active) == 0 {
		fmt.Println()
		Warning("No hay providers activos para reordenar.")
		PressEnter("")
		return
	}

	fmt.Println()
	Info("Orden actual de rotación:")
	fmt.Println()
	for i, p := range active {
		fmt.Printf("  %d. %s\n", i+1, p.name)
	}

	fmt.Println()
	Info("Ingresa el nuevo orden separado por comas.")
	Info("Ejemplo: 2,1,3 para poner el segundo primero.")
	fmt.Println()

	input, err := Ask("Nuevo orden: ")
	if err != nil || strings.TrimSpace(input) == "" {
		return
	}

	parts := strings.Split(input, ",")
	if len(parts) != len(active) {
		Error(fmt.Sprintf("Debes especificar %d posiciones.", len(active)))
		PressEnter("")
		return
	}

	indices := make([]int, len(parts))
	seen := make(map[int]bool)
	valid := true
	for i, s := range parts {
		v, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil || v < 1 || v > len(active) {
			valid = false
			break
		}
		if seen[v-1] {
			valid = false
			break
		}
		seen[v-1] = true
		indices[i] = v - 1
	}
	if !valid {
		Error("Posiciones inválidas o con duplicados.")
		PressEnter("")
		return
	}

	orderedKeys := make([]string, len(indices))
	for i, idx := range indices {
		orderedKeys[i] = active[idx].key
	}

	if err := app.Prov.Reorder(ctx, orderedKeys); err != nil {
		Error(fmt.Sprintf("Error: %v", err))
	} else {
		fmt.Println()
		Success("Orden actualizado:")
		for i, k := range orderedKeys {
			for _, p := range active {
				if p.key == k {
					fmt.Printf("  %d. %s\n", i+1, p.name)
					break
				}
			}
		}
	}
	PressEnter("")
}

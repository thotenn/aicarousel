package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/thotenn/aicarousel-go/internal/modelsconfig"
)

// ManageModels implements menu option 5: manage models per provider.
func ManageModels(_ *App) {
	for {
		ClearScreen()
		Section("🎯 Gestión de Modelos")

		cfg, err := modelsconfig.Load()
		if err != nil {
			Error(fmt.Sprintf("Error cargando modelos: %v", err))
			PressEnter("")
			return
		}

		headers := []string{"#", "Provider", "Default", "Fallback", "Modelos"}
		rows := make([][]string, len(providerMeta))
		for i, pm := range providerMeta {
			pc, ok := cfg[pm.key]
			if !ok {
				rows[i] = []string{fmt.Sprintf("%d", i+1), pm.name, Color(Dim, "No config."), "-", "0"}
				continue
			}
			def := pc.Default
			if len(def) > 25 {
				def = def[:22] + "..."
			}
			rows[i] = []string{
				fmt.Sprintf("%d", i+1),
				pm.name,
				def,
				Checkbox(pc.EnableFallback),
				fmt.Sprintf("%d", len(pc.Models)),
			}
		}
		Table(headers, rows, []int{3, 13, 27, 9, 7})

		fmt.Println()
		Info("Selecciona un provider para gestionar sus modelos.")
		fmt.Println()
		fmt.Printf("  1-%d. Seleccionar provider\n", len(providerMeta))
		fmt.Println("  0. Volver")
		fmt.Println()

		n, ok, _ := AskNumber("> ")
		if !ok || n == 0 {
			return
		}
		if n >= 1 && n <= len(providerMeta) {
			pm := providerMeta[n-1]
			manageProviderModels(pm.key, pm.name)
		}
	}
}

func manageProviderModels(provKey, provName string) {
	for {
		ClearScreen()
		Section(fmt.Sprintf("🎯 Modelos de %s", provName))

		cfg, _ := modelsconfig.Load()
		pc, ok := cfg[provKey]

		if !ok {
			Warning("Este provider no tiene configuración de modelos.")
			Info("Agrega al menos un modelo para comenzar.")
			fmt.Println()
			fmt.Println("  1. Agregar modelo")
			fmt.Println("  0. Volver")
			fmt.Println()
			n, _, _ := AskNumber("> ")
			if n == 1 {
				addModelMenu(provKey)
			} else {
				return
			}
			continue
		}

		headers := []string{"#", "Modelo", "Default"}
		rows := make([][]string, len(pc.Models))
		for i, m := range pc.Models {
			disp := m
			if len(disp) > 40 {
				disp = disp[:37] + "..."
			}
			var def string
			if m == pc.Default {
				def = Color(Yellow, "★ Default")
			}
			rows[i] = []string{fmt.Sprintf("%d", i+1), disp, def}
		}
		Table(headers, rows, []int{3, 44, 11})

		fmt.Println()
		var fbStatus string
		if pc.EnableFallback {
			fbStatus = Color(Green, "✓ Activado")
		} else {
			fbStatus = Color(Red, "✗ Desactivado")
		}
		Info(fmt.Sprintf("Fallback intra-provider: %s", fbStatus))
		if pc.EnableFallback && len(pc.Models) > 1 {
			Info("Si el modelo default falla, se probarán los demás en orden.")
		}
		fmt.Println()
		fmt.Println("  1. Agregar modelo")
		fmt.Println("  2. Editar modelo")
		fmt.Println("  3. Eliminar modelo")
		fmt.Println("  4. Cambiar default")
		fmt.Println("  5. Toggle fallback")
		fmt.Println("  6. Reordenar modelos")
		fmt.Println("  0. Volver")
		fmt.Println()

		n, _, _ := AskNumber("> ")
		switch n {
		case 1:
			addModelMenu(provKey)
		case 2:
			editModelMenu(provKey, pc)
		case 3:
			deleteModelMenu(provKey, pc)
		case 4:
			changeDefaultMenu(provKey, pc)
		case 5:
			toggleFallbackMenu(provKey, pc)
		case 6:
			reorderModelsMenu(provKey, pc)
		default:
			return
		}
	}
}

func addModelMenu(provKey string) {
	fmt.Println()
	Info("Ingresa el identificador del modelo (ej: llama-3.3-70b-versatile)")
	fmt.Println()
	model, _ := Ask("Modelo: ")
	model = strings.TrimSpace(model)
	if model == "" {
		return
	}
	if err := modelsconfig.AddModel(provKey, model); err != nil {
		fmt.Println()
		Error(err.Error())
	} else {
		fmt.Println()
		Success(fmt.Sprintf("Modelo %q agregado.", model))
	}
	PressEnter("")
}

func editModelMenu(provKey string, pc modelsconfig.ProviderModelConfig) {
	fmt.Println()
	idx, ok, _ := AskNumber("Número del modelo a editar: ")
	if !ok || idx < 1 || idx > len(pc.Models) {
		return
	}
	old := pc.Models[idx-1]
	fmt.Println()
	Info(fmt.Sprintf("Modelo actual: %s", old))
	fmt.Println()
	newModel, _ := Ask("Nuevo nombre: ")
	newModel = strings.TrimSpace(newModel)
	if newModel == "" || newModel == old {
		return
	}
	if err := modelsconfig.UpdateModel(provKey, old, newModel); err != nil {
		fmt.Println()
		Error(err.Error())
	} else {
		fmt.Println()
		Success(fmt.Sprintf("Modelo actualizado a %q.", newModel))
	}
	PressEnter("")
}

func deleteModelMenu(provKey string, pc modelsconfig.ProviderModelConfig) {
	if len(pc.Models) == 1 {
		fmt.Println()
		Warning("No puedes eliminar el único modelo del provider.")
		PressEnter("")
		return
	}
	fmt.Println()
	idx, ok, _ := AskNumber("Número del modelo a eliminar: ")
	if !ok || idx < 1 || idx > len(pc.Models) {
		return
	}
	model := pc.Models[idx-1]
	if model == pc.Default {
		fmt.Println()
		Warning("No puedes eliminar el modelo default.")
		Info("Cambia el default primero.")
		PressEnter("")
		return
	}
	fmt.Println()
	Warning(fmt.Sprintf("¿Eliminar %q?", model))
	confirm, _ := Ask("Escribe 'si' para confirmar: ")
	if strings.ToLower(strings.TrimSpace(confirm)) != "si" {
		return
	}
	if err := modelsconfig.RemoveModel(provKey, model); err != nil {
		fmt.Println()
		Error(err.Error())
	} else {
		fmt.Println()
		Success(fmt.Sprintf("Modelo %q eliminado.", model))
	}
	PressEnter("")
}

func changeDefaultMenu(provKey string, pc modelsconfig.ProviderModelConfig) {
	fmt.Println()
	Info("Modelos disponibles:")
	fmt.Println()
	for i, m := range pc.Models {
		marker := ""
		if m == pc.Default {
			marker = Color(Yellow, " ★")
		}
		fmt.Printf("  %d. %s%s\n", i+1, m, marker)
	}
	fmt.Println()
	idx, ok, _ := AskNumber("Número del nuevo default: ")
	if !ok || idx < 1 || idx > len(pc.Models) {
		return
	}
	model := pc.Models[idx-1]
	if model == pc.Default {
		Info("Ese modelo ya es el default.")
		PressEnter("")
		return
	}
	if err := modelsconfig.SetDefaultModel(provKey, model); err != nil {
		fmt.Println()
		Error(err.Error())
	} else {
		fmt.Println()
		Success(fmt.Sprintf("Default cambiado a %q.", model))
	}
	PressEnter("")
}

func toggleFallbackMenu(provKey string, pc modelsconfig.ProviderModelConfig) {
	curr := "activado"
	action := "desactivar"
	if !pc.EnableFallback {
		curr = "desactivado"
		action = "activar"
	}
	fmt.Println()
	Info(fmt.Sprintf("Fallback actualmente: %s", curr))
	fmt.Println()
	if pc.EnableFallback {
		Info("Si desactivas el fallback, solo se usará el modelo default.")
	} else {
		Info("Si activas el fallback, cuando el default falle,")
		Info("se probarán los demás modelos antes de pasar al siguiente provider.")
	}
	fmt.Println()
	prompt := fmt.Sprintf("¿%s fallback? (si/no): ", strings.Title(action)) //nolint:staticcheck
	confirm, _ := Ask(prompt)
	if strings.ToLower(strings.TrimSpace(confirm)) != "si" {
		return
	}
	newVal, err := modelsconfig.ToggleFallback(provKey)
	if err != nil {
		fmt.Println()
		Error(err.Error())
	} else {
		fmt.Println()
		if newVal {
			Success("Fallback activado.")
		} else {
			Success("Fallback desactivado.")
		}
	}
	PressEnter("")
}

func reorderModelsMenu(provKey string, pc modelsconfig.ProviderModelConfig) {
	if len(pc.Models) < 2 {
		fmt.Println()
		Warning("Necesitas al menos 2 modelos para reordenar.")
		PressEnter("")
		return
	}
	fmt.Println()
	Info("Orden actual (para fallback):")
	fmt.Println()
	for i, m := range pc.Models {
		marker := ""
		if m == pc.Default {
			marker = Color(Yellow, " ★")
		}
		fmt.Printf("  %d. %s%s\n", i+1, m, marker)
	}
	fmt.Println()
	Info("Ingresa el nuevo orden separado por comas.")
	Info("Ejemplo: 2,1,3 para poner el segundo primero.")
	fmt.Println()
	input, _ := Ask("Nuevo orden: ")
	input = strings.TrimSpace(input)
	if input == "" {
		return
	}

	parts := strings.Split(input, ",")
	if len(parts) != len(pc.Models) {
		Error(fmt.Sprintf("Debes especificar %d posiciones.", len(pc.Models)))
		PressEnter("")
		return
	}

	indices := make([]int, len(parts))
	seen := make(map[int]bool)
	for i, s := range parts {
		v, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil || v < 1 || v > len(pc.Models) || seen[v-1] {
			Error("Posiciones inválidas o con duplicados.")
			PressEnter("")
			return
		}
		seen[v-1] = true
		indices[i] = v - 1
	}

	ordered := make([]string, len(indices))
	for i, idx := range indices {
		ordered[i] = pc.Models[idx]
	}

	if err := modelsconfig.ReorderModels(provKey, ordered); err != nil {
		fmt.Println()
		Error(err.Error())
	} else {
		fmt.Println()
		Success("Orden actualizado:")
		for i, m := range ordered {
			fmt.Printf("  %d. %s\n", i+1, m)
		}
	}
	PressEnter("")
}


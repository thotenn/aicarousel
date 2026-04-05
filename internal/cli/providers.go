package cli

import (
	"fmt"
)

// ManageProviderKeys implements menu option 2: manage provider API keys.
func ManageProviderKeys(_ *App) {
	for {
		ClearScreen()
		Section("🔑 API Keys de Providers")

		list := buildProviderList()

		headers := []string{"#", "Provider", "Variable", "Estado"}
		rows := make([][]string, len(list))
		for i, p := range list {
			var estado string
			if p.hasKey {
				estado = Color(Green, "✓ "+MaskKey(p.value))
			} else {
				estado = Color(Red, "✗ No configurada")
			}
			rows[i] = []string{fmt.Sprintf("%d", i+1), p.name, p.envKey, estado}
		}
		Table(headers, rows, []int{3, 13, 22, 28})

		fmt.Println()
		Info("Selecciona un provider para configurar su API Key")
		fmt.Println()

		n, ok, err := AskNumber("Opción (0 para volver): ")
		if err != nil || !ok || n == 0 {
			return
		}
		if n < 1 || n > len(list) {
			continue
		}
		configureProviderKey(list[n-1])
	}
}

func configureProviderKey(p providerEntry) {
	ClearScreen()
	Section(fmt.Sprintf("🔑 Configurar %s", p.name))

	fmt.Printf("  Variable:  %s\n", p.envKey)
	if p.hasKey {
		fmt.Printf("  Estado:    %s\n", Color(Green, "✓ Configurada ("+MaskKey(p.value)+")"))
	} else {
		fmt.Printf("  Estado:    %s\n", Color(Red, "✗ No configurada"))
	}
	fmt.Println()

	if p.hasKey {
		fmt.Println("  1. Actualizar API Key")
		fmt.Println("  2. Eliminar API Key")
		fmt.Println("  0. Volver")
		fmt.Println()
		n, _, _ := AskNumber("> ")
		switch n {
		case 1:
			updateProviderKey(p)
		case 2:
			deleteProviderKey(p)
		}
	} else {
		fmt.Println("  1. Agregar API Key")
		fmt.Println("  0. Volver")
		fmt.Println()
		n, _, _ := AskNumber("> ")
		if n == 1 {
			updateProviderKey(p)
		}
	}
}

func updateProviderKey(p providerEntry) {
	fmt.Println()
	Info(fmt.Sprintf("Ingresa la API Key para %s", p.name))
	Info("(La key se guardará en el archivo .env)")
	fmt.Println()

	newKey, err := Ask("API Key: ")
	if err != nil || newKey == "" {
		Error("API Key vacía, operación cancelada")
		PressEnter("")
		return
	}

	if err := WriteValue(p.envKey, newKey); err != nil {
		Error(fmt.Sprintf("Error guardando la key: %v", err))
	} else {
		fmt.Println()
		Success(p.envKey + " guardada correctamente")
		Success(fmt.Sprintf("Provider %s ahora disponible", p.name))
	}
	PressEnter("")
}

func deleteProviderKey(p providerEntry) {
	fmt.Println()
	ok, err := Confirm(fmt.Sprintf("¿Eliminar API Key de %s?", p.name), false)
	if err != nil || !ok {
		Info("Operación cancelada")
		PressEnter("")
		return
	}

	if err := RemoveValue(p.envKey); err != nil {
		Error(fmt.Sprintf("Error eliminando la key: %v", err))
	} else {
		fmt.Println()
		Success(p.envKey + " eliminada")
		Info(fmt.Sprintf("Provider %s ya no estará disponible", p.name))
	}
	PressEnter("")
}

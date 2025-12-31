/**
 * Provider toggle module - Enable/disable providers and change order.
 */

import { ask, askNumber, pressEnter, clearScreen } from "./utils/prompt.ts";
import { section, table, success, error, info, warning, colors, color, checkbox } from "./utils/display.ts";
import { hasEnvValue } from "./utils/env.ts";
import { getAllProviderSettings, toggleProvider, reorderProviders, syncProviders } from "../db/provider_settings.ts";
import { providers } from "../defaults/providers.ts";
import { migrate } from "../db/migrate.ts";

interface ProviderStatus {
  key: string;
  name: string;
  hasApiKey: boolean;
  isEnabled: boolean;
  priority: number;
}

function getProviderStatuses(): ProviderStatus[] {
  // Sync providers first
  syncProviders(Object.keys(providers));

  const settings = getAllProviderSettings();

  return Object.entries(providers).map(([key, provider]) => {
    const setting = settings.find((s) => s.provider_key === key);
    return {
      key,
      name: provider.name,
      hasApiKey: hasEnvValue(provider.apiKeyName),
      isEnabled: setting?.is_enabled === 1,
      priority: setting?.priority ?? 999,
    };
  }).sort((a, b) => a.priority - b.priority);
}

export async function manageProviderToggle(): Promise<void> {
  // Ensure migrations are run
  await migrate();

  while (true) {
    clearScreen();
    section("⚡ Providers Activos");

    const statuses = getProviderStatuses();

    // Build table
    const headers = ["#", "Provider", "API Key", "Habilitado", "Orden"];
    const rows = statuses.map((p, i) => {
      const apiKeyStatus = p.hasApiKey
        ? color("✓ Config.", colors.green)
        : color("✗ Falta", colors.red);

      const enabledStatus = p.hasApiKey
        ? checkbox(p.isEnabled) + (p.isEnabled ? " Activo" : " Inactivo")
        : color("- N/A", colors.dim);

      const order = p.hasApiKey && p.isEnabled ? String(p.priority) : "-";

      return [
        String(i + 1),
        p.name,
        apiKeyStatus,
        enabledStatus,
        order,
      ];
    });

    table(headers, rows, [4, 14, 12, 16, 8]);

    console.log();
    info("Providers sin API Key no pueden activarse.");
    info("El orden determina la rotación round-robin.");
    console.log();

    console.log("  1. Toggle provider (activar/desactivar)");
    console.log("  2. Cambiar orden de rotación");
    console.log("  0. Volver");
    console.log();

    const choice = await askNumber("> ");

    switch (choice) {
      case 1:
        await toggleProviderMenu(statuses);
        break;
      case 2:
        await changeOrderMenu(statuses);
        break;
      case 0:
      default:
        return;
    }
  }
}

async function toggleProviderMenu(statuses: ProviderStatus[]): Promise<void> {
  console.log();
  const idx = await askNumber("Número del provider a toggle: ");

  if (!idx || idx < 1 || idx > statuses.length) {
    return;
  }

  const provider = statuses[idx - 1];

  if (!provider) return;

  if (!provider.hasApiKey) {
    console.log();
    warning(`${provider.name} no tiene API Key configurada.`);
    info("Configura la API Key primero en la opción 2 del menú principal.");
    await pressEnter();
    return;
  }

  const newState = !provider.isEnabled;
  toggleProvider(provider.key, newState);

  console.log();
  if (newState) {
    success(`${provider.name} activado`);
  } else {
    success(`${provider.name} desactivado`);
  }

  await pressEnter();
}

async function changeOrderMenu(statuses: ProviderStatus[]): Promise<void> {
  // Only show active providers
  const activeProviders = statuses.filter((p) => p.hasApiKey && p.isEnabled);

  if (activeProviders.length === 0) {
    console.log();
    warning("No hay providers activos para reordenar.");
    await pressEnter();
    return;
  }

  console.log();
  info("Orden actual de rotación:");
  console.log();

  activeProviders.forEach((p, i) => {
    console.log(`  ${i + 1}. ${p.name}`);
  });

  console.log();
  info("Ingresa el nuevo orden separado por comas.");
  info("Ejemplo: 2,1,3 para poner el segundo primero.");
  console.log();

  const input = await ask("Nuevo orden: ");

  if (!input.trim()) {
    return;
  }

  const newOrder = input.split(",").map((s) => parseInt(s.trim(), 10) - 1);

  // Validate
  if (newOrder.length !== activeProviders.length) {
    error(`Debes especificar ${activeProviders.length} posiciones.`);
    await pressEnter();
    return;
  }

  const hasInvalid = newOrder.some((i) => isNaN(i) || i < 0 || i >= activeProviders.length);
  if (hasInvalid) {
    error("Posiciones inválidas.");
    await pressEnter();
    return;
  }

  const hasDuplicates = new Set(newOrder).size !== newOrder.length;
  if (hasDuplicates) {
    error("No se permiten duplicados.");
    await pressEnter();
    return;
  }

  // Apply new order
  const orderedKeys = newOrder.map((i) => activeProviders[i]!.key);
  reorderProviders(orderedKeys);

  console.log();
  success("Orden actualizado:");
  orderedKeys.forEach((key, i) => {
    const provider = activeProviders.find((p) => p.key === key);
    console.log(`  ${i + 1}. ${provider?.name}`);
  });

  await pressEnter();
}

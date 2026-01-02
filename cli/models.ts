/**
 * Models management module - Configure models per provider.
 */

import { ask, askNumber, pressEnter, clearScreen } from "./utils/prompt.ts";
import { section, table, success, error, info, warning, colors, color, checkbox } from "./utils/display.ts";
import { providers } from "../defaults/providers.ts";
import {
  getModelsConfig,
  getProviderConfig,
  addModel,
  removeModel,
  updateModel,
  setDefaultModel,
  toggleFallback,
  reorderModels,
  type ProviderModelConfig,
} from "../services/models_config.ts";

interface ProviderModelStatus {
  key: string;
  name: string;
  config: ProviderModelConfig | null;
}

function getProviderModelStatuses(): ProviderModelStatus[] {
  const config = getModelsConfig();

  return Object.entries(providers).map(([key, provider]) => ({
    key,
    name: provider.name,
    config: config[key] ?? null,
  }));
}

export async function manageModels(): Promise<void> {
  while (true) {
    clearScreen();
    section("🎯 Gestión de Modelos");

    const statuses = getProviderModelStatuses();

    // Build table
    const headers = ["#", "Provider", "Default", "Fallback", "Modelos"];
    const rows = statuses.map((p, i) => {
      if (!p.config) {
        return [
          String(i + 1),
          p.name,
          color("No config.", colors.dim),
          "-",
          "0",
        ];
      }

      const defaultModel = p.config.default.length > 25
        ? p.config.default.substring(0, 22) + "..."
        : p.config.default;

      return [
        String(i + 1),
        p.name,
        defaultModel,
        checkbox(p.config.enableFallback),
        String(p.config.models.length),
      ];
    });

    table(headers, rows, [4, 14, 28, 10, 8]);

    console.log();
    info("Selecciona un provider para gestionar sus modelos.");
    console.log();

    console.log("  1-" + statuses.length + ". Seleccionar provider");
    console.log("  0. Volver");
    console.log();

    const choice = await askNumber("> ");

    if (choice === 0 || choice === null) {
      return;
    }

    if (choice >= 1 && choice <= statuses.length) {
      const provider = statuses[choice - 1];
      if (provider) {
        await manageProviderModels(provider);
      }
    }
  }
}

async function manageProviderModels(provider: ProviderModelStatus): Promise<void> {
  while (true) {
    clearScreen();
    section(`🎯 Modelos de ${provider.name}`);

    const config = getProviderConfig(provider.key);

    if (!config) {
      warning("Este provider no tiene configuración de modelos.");
      info("Agrega al menos un modelo para comenzar.");
      console.log();

      console.log("  1. Agregar modelo");
      console.log("  0. Volver");
      console.log();

      const choice = await askNumber("> ");
      if (choice === 1) {
        await addModelMenu(provider.key);
      } else {
        return;
      }
      continue;
    }

    // Show current models
    const headers = ["#", "Modelo", "Default"];
    const rows = config.models.map((model, i) => {
      const isDefault = model === config.default;
      const modelDisplay = model.length > 40 ? model.substring(0, 37) + "..." : model;
      return [
        String(i + 1),
        modelDisplay,
        isDefault ? color("★ Default", colors.yellow) : "",
      ];
    });

    table(headers, rows, [4, 45, 12]);

    console.log();

    // Show fallback status
    const fallbackStatus = config.enableFallback
      ? color("✓ Activado", colors.green)
      : color("✗ Desactivado", colors.red);
    info(`Fallback intra-provider: ${fallbackStatus}`);

    if (config.enableFallback && config.models.length > 1) {
      info("Si el modelo default falla, se probarán los demás en orden.");
    }

    console.log();

    console.log("  1. Agregar modelo");
    console.log("  2. Editar modelo");
    console.log("  3. Eliminar modelo");
    console.log("  4. Cambiar default");
    console.log("  5. Toggle fallback");
    console.log("  6. Reordenar modelos");
    console.log("  0. Volver");
    console.log();

    const choice = await askNumber("> ");

    switch (choice) {
      case 1:
        await addModelMenu(provider.key);
        break;
      case 2:
        await editModelMenu(provider.key, config);
        break;
      case 3:
        await deleteModelMenu(provider.key, config);
        break;
      case 4:
        await changeDefaultMenu(provider.key, config);
        break;
      case 5:
        await toggleFallbackMenu(provider.key, config);
        break;
      case 6:
        await reorderModelsMenu(provider.key, config);
        break;
      case 0:
      default:
        return;
    }
  }
}

async function addModelMenu(providerKey: string): Promise<void> {
  console.log();
  info("Ingresa el identificador del modelo (ej: llama-3.3-70b-versatile)");
  console.log();

  const model = await ask("Modelo: ");

  if (!model.trim()) {
    return;
  }

  try {
    await addModel(providerKey, model.trim());
    console.log();
    success(`Modelo "${model.trim()}" agregado.`);
  } catch (err) {
    console.log();
    error((err as Error).message);
  }

  await pressEnter();
}

async function editModelMenu(providerKey: string, config: ProviderModelConfig): Promise<void> {
  console.log();
  const idx = await askNumber("Número del modelo a editar: ");

  if (!idx || idx < 1 || idx > config.models.length) {
    return;
  }

  const oldModel = config.models[idx - 1];
  if (!oldModel) return;

  console.log();
  info(`Modelo actual: ${oldModel}`);
  console.log();

  const newModel = await ask("Nuevo nombre: ");

  if (!newModel.trim() || newModel.trim() === oldModel) {
    return;
  }

  try {
    await updateModel(providerKey, oldModel, newModel.trim());
    console.log();
    success(`Modelo actualizado a "${newModel.trim()}".`);
  } catch (err) {
    console.log();
    error((err as Error).message);
  }

  await pressEnter();
}

async function deleteModelMenu(providerKey: string, config: ProviderModelConfig): Promise<void> {
  if (config.models.length === 1) {
    console.log();
    warning("No puedes eliminar el único modelo del provider.");
    await pressEnter();
    return;
  }

  console.log();
  const idx = await askNumber("Número del modelo a eliminar: ");

  if (!idx || idx < 1 || idx > config.models.length) {
    return;
  }

  const model = config.models[idx - 1];
  if (!model) return;

  if (model === config.default) {
    console.log();
    warning("No puedes eliminar el modelo default.");
    info("Cambia el default primero.");
    await pressEnter();
    return;
  }

  console.log();
  warning(`¿Eliminar "${model}"?`);
  const confirm = await ask("Escribe 'si' para confirmar: ");

  if (confirm.toLowerCase() !== "si") {
    return;
  }

  try {
    await removeModel(providerKey, model);
    console.log();
    success(`Modelo "${model}" eliminado.`);
  } catch (err) {
    console.log();
    error((err as Error).message);
  }

  await pressEnter();
}

async function changeDefaultMenu(providerKey: string, config: ProviderModelConfig): Promise<void> {
  console.log();
  info("Modelos disponibles:");
  console.log();

  config.models.forEach((model, i) => {
    const isDefault = model === config.default;
    const marker = isDefault ? color(" ★", colors.yellow) : "";
    console.log(`  ${i + 1}. ${model}${marker}`);
  });

  console.log();
  const idx = await askNumber("Número del nuevo default: ");

  if (!idx || idx < 1 || idx > config.models.length) {
    return;
  }

  const model = config.models[idx - 1];
  if (!model) return;

  if (model === config.default) {
    info("Ese modelo ya es el default.");
    await pressEnter();
    return;
  }

  try {
    await setDefaultModel(providerKey, model);
    console.log();
    success(`Default cambiado a "${model}".`);
  } catch (err) {
    console.log();
    error((err as Error).message);
  }

  await pressEnter();
}

async function toggleFallbackMenu(providerKey: string, config: ProviderModelConfig): Promise<void> {
  const currentStatus = config.enableFallback ? "activado" : "desactivado";
  const newStatus = config.enableFallback ? "desactivar" : "activar";

  console.log();
  info(`Fallback actualmente: ${currentStatus}`);
  console.log();

  if (config.enableFallback) {
    info("Si desactivas el fallback, solo se usará el modelo default.");
    info("Si falla, pasará directamente al siguiente provider.");
  } else {
    info("Si activas el fallback, cuando el default falle,");
    info("se probarán los demás modelos antes de pasar al siguiente provider.");
  }

  console.log();
  const confirm = await ask(`¿${newStatus.charAt(0).toUpperCase() + newStatus.slice(1)} fallback? (si/no): `);

  if (confirm.toLowerCase() !== "si") {
    return;
  }

  try {
    const newValue = await toggleFallback(providerKey);
    console.log();
    success(`Fallback ${newValue ? "activado" : "desactivado"}.`);
  } catch (err) {
    console.log();
    error((err as Error).message);
  }

  await pressEnter();
}

async function reorderModelsMenu(providerKey: string, config: ProviderModelConfig): Promise<void> {
  if (config.models.length < 2) {
    console.log();
    warning("Necesitas al menos 2 modelos para reordenar.");
    await pressEnter();
    return;
  }

  console.log();
  info("Orden actual (para fallback):");
  console.log();

  config.models.forEach((model, i) => {
    const isDefault = model === config.default;
    const marker = isDefault ? color(" ★", colors.yellow) : "";
    console.log(`  ${i + 1}. ${model}${marker}`);
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
  if (newOrder.length !== config.models.length) {
    error(`Debes especificar ${config.models.length} posiciones.`);
    await pressEnter();
    return;
  }

  const hasInvalid = newOrder.some((i) => isNaN(i) || i < 0 || i >= config.models.length);
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
  const orderedModels = newOrder.map((i) => config.models[i]!);

  try {
    await reorderModels(providerKey, orderedModels);
    console.log();
    success("Orden actualizado:");
    orderedModels.forEach((model, i) => {
      console.log(`  ${i + 1}. ${model}`);
    });
  } catch (err) {
    console.log();
    error((err as Error).message);
  }

  await pressEnter();
}

/**
 * Provider API Keys management module.
 */

import { ask, askNumber, pressEnter, clearScreen, confirm } from "./utils/prompt.ts";
import { section, table, success, error, info, colors, color, maskApiKey } from "./utils/display.ts";
import { hasEnvValue, getEnvValue, writeEnvValue, removeEnvValue } from "./utils/env.ts";
import { providers } from "../defaults/providers.ts";

interface ProviderInfo {
  key: string;
  name: string;
  apiKeyName: string;
  hasKey: boolean;
  maskedKey: string;
}

function getProviderList(): ProviderInfo[] {
  return Object.entries(providers).map(([key, provider]) => {
    const apiKeyValue = getEnvValue(provider.apiKeyName);
    return {
      key,
      name: provider.name,
      apiKeyName: provider.apiKeyName,
      hasKey: hasEnvValue(provider.apiKeyName),
      maskedKey: maskApiKey(apiKeyValue),
    };
  });
}

export async function manageProviderKeys(): Promise<void> {
  while (true) {
    clearScreen();
    section("🔑 API Keys de Providers");

    const providerList = getProviderList();

    // Build table
    const headers = ["#", "Provider", "Variable", "Estado"];
    const rows = providerList.map((p, i) => [
      String(i + 1),
      p.name,
      p.apiKeyName,
      p.hasKey
        ? color("✓ " + p.maskedKey, colors.green)
        : color("✗ No configurada", colors.red),
    ]);

    table(headers, rows, [4, 14, 22, 24]);

    console.log();
    info("Selecciona un provider para configurar su API Key");
    console.log();

    const choice = await askNumber("Opción (0 para volver): ");

    if (choice === null || choice === 0) {
      return;
    }

    const selectedProvider = providerList[choice - 1];
    if (!selectedProvider) {
      continue;
    }

    await configureProviderKey(selectedProvider);
  }
}

async function configureProviderKey(provider: ProviderInfo): Promise<void> {
  clearScreen();
  section(`🔑 Configurar ${provider.name}`);

  console.log(`  Variable:  ${provider.apiKeyName}`);
  console.log(`  Estado:    ${provider.hasKey
    ? color("✓ Configurada (" + provider.maskedKey + ")", colors.green)
    : color("✗ No configurada", colors.red)
  }`);
  console.log();

  if (provider.hasKey) {
    console.log("  1. Actualizar API Key");
    console.log("  2. Eliminar API Key");
    console.log("  0. Volver");
    console.log();

    const choice = await askNumber("> ");

    switch (choice) {
      case 1:
        await updateApiKey(provider);
        break;
      case 2:
        await deleteApiKey(provider);
        break;
      default:
        return;
    }
  } else {
    console.log("  1. Agregar API Key");
    console.log("  0. Volver");
    console.log();

    const choice = await askNumber("> ");

    if (choice === 1) {
      await updateApiKey(provider);
    }
  }
}

async function updateApiKey(provider: ProviderInfo): Promise<void> {
  console.log();
  info(`Ingresa la API Key para ${provider.name}`);
  info("(La key se guardará en el archivo .env)");
  console.log();

  const newKey = await ask("API Key: ");

  if (!newKey.trim()) {
    error("API Key vacía, operación cancelada");
    await pressEnter();
    return;
  }

  try {
    await writeEnvValue(provider.apiKeyName, newKey.trim());
    console.log();
    success(`${provider.apiKeyName} guardada correctamente`);
    success(`Provider ${provider.name} ahora disponible`);
  } catch (err) {
    error(`Error guardando la key: ${err}`);
  }

  await pressEnter();
}

async function deleteApiKey(provider: ProviderInfo): Promise<void> {
  console.log();
  const confirmed = await confirm(`¿Eliminar API Key de ${provider.name}?`);

  if (!confirmed) {
    info("Operación cancelada");
    await pressEnter();
    return;
  }

  try {
    await removeEnvValue(provider.apiKeyName);
    console.log();
    success(`${provider.apiKeyName} eliminada`);
    info(`Provider ${provider.name} ya no estará disponible`);
  } catch (err) {
    error(`Error eliminando la key: ${err}`);
  }

  await pressEnter();
}

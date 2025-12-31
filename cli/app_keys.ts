/**
 * Application API Keys management module.
 */

import { ask, askNumber, pressEnter, clearScreen, confirm } from "./utils/prompt.ts";
import { section, table, success, error, info, apiKeyBox, colors, color } from "./utils/display.ts";
import { createApiKey, listApiKeys, revokeApiKey, deleteApiKey } from "../db/api_keys.ts";
import { migrate } from "../db/migrate.ts";

export async function manageAppKeys(): Promise<void> {
  // Ensure migrations are run
  await migrate();

  while (true) {
    clearScreen();
    section("🎫 API Keys de la Aplicación");

    const keys = listApiKeys();

    if (keys.length === 0) {
      info("No hay API Keys creadas.");
      console.log();
      console.log("  1. Crear nueva API Key");
      console.log("  0. Volver");
      console.log();

      const choice = await askNumber("> ");
      if (choice === 1) {
        await createNewKey();
      } else {
        return;
      }
      continue;
    }

    // Build table
    const headers = ["ID", "Prefix", "Nombre", "Último uso", "Estado", "Usos"];
    const rows = keys.map((k) => [
      String(k.id),
      k.key_prefix,
      k.name || "-",
      k.last_used_at?.slice(0, 16) || "-",
      k.is_active
        ? color("✓ Activa", colors.green)
        : color("✗ Revocada", colors.red),
      String(k.usage_count),
    ]);

    table(headers, rows, [5, 13, 16, 18, 12, 8]);

    console.log();
    console.log("  1. Crear nueva API Key");
    console.log("  2. Revocar API Key");
    console.log("  3. Eliminar API Key");
    console.log("  0. Volver");
    console.log();

    const choice = await askNumber("> ");

    switch (choice) {
      case 1:
        await createNewKey();
        break;
      case 2:
        await revokeKey();
        break;
      case 3:
        await deleteKey();
        break;
      case 0:
      default:
        return;
    }
  }
}

async function createNewKey(): Promise<void> {
  console.log();
  const name = await ask("Nombre para la nueva API Key (opcional): ");

  try {
    const { key, record } = await createApiKey(name.trim() || undefined);

    apiKeyBox(key);

    console.log("  Detalles:");
    console.log(`    ID:      ${record.id}`);
    console.log(`    Nombre:  ${record.name || "(sin nombre)"}`);
    console.log(`    Creada:  ${record.created_at}`);
    console.log();

    info("Usa esta key en el header 'Authorization: Bearer <key>'");
    info("o en el header 'x-api-key: <key>'");

  } catch (err) {
    error(`Error creando API Key: ${err}`);
  }

  await pressEnter();
}

async function revokeKey(): Promise<void> {
  console.log();
  const id = await askNumber("ID de la API Key a revocar: ");

  if (!id) {
    info("Operación cancelada");
    await pressEnter();
    return;
  }

  const confirmed = await confirm(`¿Revocar API Key #${id}?`);

  if (!confirmed) {
    info("Operación cancelada");
    await pressEnter();
    return;
  }

  const result = revokeApiKey(id);

  if (result) {
    success(`API Key #${id} revocada`);
    info("La key ya no funcionará para autenticarse");
  } else {
    error(`API Key #${id} no encontrada`);
  }

  await pressEnter();
}

async function deleteKey(): Promise<void> {
  console.log();
  const id = await askNumber("ID de la API Key a eliminar: ");

  if (!id) {
    info("Operación cancelada");
    await pressEnter();
    return;
  }

  const confirmed = await confirm(`¿ELIMINAR PERMANENTEMENTE API Key #${id}?`);

  if (!confirmed) {
    info("Operación cancelada");
    await pressEnter();
    return;
  }

  const result = deleteApiKey(id);

  if (result) {
    success(`API Key #${id} eliminada permanentemente`);
  } else {
    error(`API Key #${id} no encontrada`);
  }

  await pressEnter();
}

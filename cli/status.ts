/**
 * Status module - Show current system status.
 */

import { pressEnter, clearScreen } from "./utils/prompt.ts";
import { section, success, error, info, colors, color, keyValue } from "./utils/display.ts";
import { envFileExists, hasEnvValue, getEnvValue } from "./utils/env.ts";
import { getAllProviderSettings, syncProviders } from "../db/provider_settings.ts";
import { listApiKeys } from "../db/api_keys.ts";
import { providers } from "../defaults/providers.ts";
import { migrate } from "../db/migrate.ts";
import { DB_PATH } from "../db/index.ts";
import { existsSync } from "fs";

export async function showStatus(): Promise<void> {
  clearScreen();
  section("📊 Estado del Sistema");

  // Database status
  const dbExists = existsSync(DB_PATH);
  console.log(color("Base de Datos", colors.bold));
  console.log(`  Ubicación: ${DB_PATH}`);
  console.log(`  Estado:    ${dbExists ? color("✓ Conectada", colors.green) : color("✗ No existe", colors.red)}`);

  if (dbExists) {
    try {
      await migrate();
      console.log(`  Migraciones: ${color("✓ Actualizadas", colors.green)}`);
    } catch {
      console.log(`  Migraciones: ${color("✗ Error", colors.red)}`);
    }
  }

  console.log();

  // Environment file
  const envExists = await envFileExists();
  console.log(color("Archivo .env", colors.bold));
  console.log(`  Estado: ${envExists ? color("✓ Existe", colors.green) : color("✗ No existe", colors.red)}`);
  console.log(`  Puerto: ${getEnvValue("PORT") || "7123 (default)"}`);

  console.log();

  // Providers status
  console.log(color("Providers", colors.bold));

  if (dbExists) {
    syncProviders(Object.keys(providers));
    const settings = getAllProviderSettings();

    for (const [key, provider] of Object.entries(providers)) {
      const setting = settings.find((s) => s.provider_key === key);
      const hasKey = hasEnvValue(provider.apiKeyName);
      const isEnabled = setting?.is_enabled === 1;

      let status: string;
      if (!hasKey) {
        status = color("✗ Sin API Key", colors.red);
      } else if (!isEnabled) {
        status = color("⊘ Desactivado", colors.yellow);
      } else {
        status = color(`✓ Activo (orden: ${setting?.priority})`, colors.green);
      }

      console.log(`  ${provider.name.padEnd(12)} ${status}`);
    }
  } else {
    for (const [, provider] of Object.entries(providers)) {
      const hasKey = hasEnvValue(provider.apiKeyName);
      const status = hasKey
        ? color("✓ API Key configurada", colors.green)
        : color("✗ Sin API Key", colors.red);
      console.log(`  ${provider.name.padEnd(12)} ${status}`);
    }
  }

  console.log();

  // Application API Keys
  console.log(color("API Keys de la Aplicación", colors.bold));

  if (dbExists) {
    try {
      const keys = listApiKeys();
      const active = keys.filter((k) => k.is_active).length;
      const revoked = keys.filter((k) => !k.is_active).length;

      if (keys.length === 0) {
        console.log(`  ${color("⚠ No hay API Keys creadas", colors.yellow)}`);
        info("  Crea una con la opción 3 del menú principal");
      } else {
        console.log(`  Total:    ${keys.length}`);
        console.log(`  Activas:  ${color(String(active), colors.green)}`);
        if (revoked > 0) {
          console.log(`  Revocadas: ${color(String(revoked), colors.red)}`);
        }
      }
    } catch {
      console.log(`  ${color("✗ Error leyendo API Keys", colors.red)}`);
    }
  } else {
    console.log(`  ${color("- Base de datos no inicializada", colors.dim)}`);
  }

  console.log();

  // Server info
  console.log(color("Servidor", colors.bold));
  const port = getEnvValue("PORT") || "7123";
  console.log(`  Puerto:   ${port}`);
  console.log(`  URL:      http://localhost:${port}`);
  console.log(`  Iniciar:  bun run dev`);

  console.log();

  // Quick commands
  console.log(color("Comandos Rápidos", colors.bold));
  console.log("  bun run setup          - Este menú");
  console.log("  bun run dev            - Iniciar servidor (desarrollo)");
  console.log("  bun run start          - Iniciar servidor (producción)");
  console.log("  bun run api-key list   - Listar API Keys");

  console.log();

  await pressEnter();
}

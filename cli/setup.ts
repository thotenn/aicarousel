/**
 * Setup module - Initial database and environment setup.
 */

import { pressEnter, clearScreen } from "./utils/prompt.ts";
import { section, success, error, step, info } from "./utils/display.ts";
import { envFileExists, createEnvFromTemplate, getEnvPath } from "./utils/env.ts";
import { migrate } from "../db/migrate.ts";
import { syncProviders } from "../db/provider_settings.ts";
import { providers } from "../defaults/providers.ts";
import { join } from "path";

export async function runSetup(): Promise<void> {
  clearScreen();
  section("🔧 Setup Inicial");

  try {
    // Step 1: Create data directory
    step("Verificando directorio de datos...");
    const dataDir = join(import.meta.dir, "..", "data");
    await Bun.write(join(dataDir, ".gitkeep"), "");
    step("Directorio de datos listo", true);

    // Step 2: Check/create .env file
    step("Verificando archivo .env...");
    const envExists = await envFileExists();
    if (!envExists) {
      await createEnvFromTemplate();
      step(`Archivo .env creado en ${getEnvPath()}`, true);
      info("Recuerda configurar las API keys de los providers.");
    } else {
      step("Archivo .env existente", true);
    }

    // Step 3: Run migrations
    step("Ejecutando migraciones de base de datos...");
    await migrate();
    step("Migraciones completadas", true);

    // Step 4: Sync providers
    step("Sincronizando providers...");
    const providerKeys = Object.keys(providers);
    syncProviders(providerKeys);
    step(`${providerKeys.length} providers sincronizados`, true);

    console.log();
    success("¡Setup completado exitosamente!");
    console.log();
    info("Próximos pasos:");
    console.log("  1. Configura las API Keys de los providers (opción 2 del menú)");
    console.log("  2. Crea una API Key para la aplicación (opción 3 del menú)");
    console.log("  3. Inicia el servidor con: bun run dev");
    console.log();

  } catch (err) {
    error(`Error durante el setup: ${err}`);
  }

  await pressEnter();
}

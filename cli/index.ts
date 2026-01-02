#!/usr/bin/env bun
/**
 * AICarousel Interactive CLI
 *
 * Main menu for all configuration options.
 */

import { ask, closePrompt, clearScreen } from "./utils/prompt.ts";
import { header, section, info } from "./utils/display.ts";
import { runSetup } from "./setup.ts";
import { manageProviderKeys } from "./providers.ts";
import { manageAppKeys } from "./app_keys.ts";
import { manageProviderToggle } from "./provider_toggle.ts";
import { manageModels } from "./models.ts";
import { showStatus } from "./status.ts";

async function mainMenu(): Promise<void> {
  while (true) {
    clearScreen();
    header("🎠 AICarousel Setup");

    console.log("  1. 🔧 Setup inicial (base de datos y migraciones)");
    console.log("  2. 🔑 Gestionar API Keys de Providers");
    console.log("  3. 🎫 Gestionar API Keys de la Aplicación");
    console.log("  4. ⚡ Seleccionar/Deseleccionar Providers");
    console.log("  5. 🎯 Gestionar Modelos de Providers");
    console.log("  6. 📊 Ver estado actual");
    console.log("  0. ❌ Salir");
    console.log();

    const choice = await ask("> ");

    switch (choice) {
      case "1":
        await runSetup();
        break;
      case "2":
        await manageProviderKeys();
        break;
      case "3":
        await manageAppKeys();
        break;
      case "4":
        await manageProviderToggle();
        break;
      case "5":
        await manageModels();
        break;
      case "6":
        await showStatus();
        break;
      case "0":
      case "q":
      case "exit":
        clearScreen();
        info("¡Hasta luego!");
        console.log();
        closePrompt();
        process.exit(0);
      default:
        // Invalid option, just redraw menu
        break;
    }
  }
}

// Run
mainMenu().catch((error) => {
  console.error("Error:", error);
  closePrompt();
  process.exit(1);
});

/**
 * CLI Input utilities using readline.
 */

import * as readline from "readline";

let rl: readline.Interface | null = null;

/**
 * Get or create readline interface.
 */
function getReadline(): readline.Interface {
  if (!rl) {
    rl = readline.createInterface({
      input: process.stdin,
      output: process.stdout,
    });
  }
  return rl;
}

/**
 * Close readline interface.
 */
export function closePrompt(): void {
  if (rl) {
    rl.close();
    rl = null;
  }
}

/**
 * Ask a question and return the answer.
 */
export function ask(question: string): Promise<string> {
  return new Promise((resolve) => {
    getReadline().question(question, (answer) => {
      resolve(answer.trim());
    });
  });
}

/**
 * Ask for a number input.
 */
export async function askNumber(question: string): Promise<number | null> {
  const answer = await ask(question);
  if (answer === "") return null;
  const num = parseInt(answer, 10);
  return isNaN(num) ? null : num;
}

/**
 * Ask for confirmation (y/n).
 */
export async function confirm(question: string, defaultYes = false): Promise<boolean> {
  const hint = defaultYes ? "[Y/n]" : "[y/N]";
  const answer = await ask(`${question} ${hint}: `);

  if (answer === "") return defaultYes;
  return answer.toLowerCase() === "y" || answer.toLowerCase() === "yes";
}

/**
 * Wait for Enter key.
 */
export async function pressEnter(message = "Presiona Enter para continuar..."): Promise<void> {
  await ask(`\n${message}`);
}

/**
 * Ask for a secret input (API key).
 * Note: In Bun/Node, hiding input is complex. We'll show it but warn user.
 */
export async function askSecret(question: string): Promise<string> {
  return ask(question);
}

/**
 * Clear the console.
 */
export function clearScreen(): void {
  console.clear();
}

/**
 * Show a menu and get selection.
 */
export async function menu(
  title: string,
  options: { key: string; label: string }[]
): Promise<string> {
  console.log(`\n${title}\n`);

  for (const opt of options) {
    console.log(`  ${opt.key}. ${opt.label}`);
  }

  console.log();
  const answer = await ask("> ");
  return answer;
}

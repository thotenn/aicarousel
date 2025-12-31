/**
 * CLI Display utilities for formatting output.
 */

/**
 * ANSI color codes.
 */
export const colors = {
  reset: "\x1b[0m",
  bold: "\x1b[1m",
  dim: "\x1b[2m",
  red: "\x1b[31m",
  green: "\x1b[32m",
  yellow: "\x1b[33m",
  blue: "\x1b[34m",
  magenta: "\x1b[35m",
  cyan: "\x1b[36m",
  white: "\x1b[37m",
  bgRed: "\x1b[41m",
  bgGreen: "\x1b[42m",
  bgYellow: "\x1b[43m",
  bgBlue: "\x1b[44m",
};

/**
 * Apply color to text.
 */
export function color(text: string, ...codes: string[]): string {
  return codes.join("") + text + colors.reset;
}

/**
 * Display a header box.
 */
export function header(title: string): void {
  const width = 60;
  const padding = Math.max(0, Math.floor((width - title.length - 2) / 2));
  const line = "═".repeat(width);

  console.log();
  console.log(color(`╔${line}╗`, colors.cyan));
  console.log(color(`║${" ".repeat(padding)}${title}${" ".repeat(width - padding - title.length)}║`, colors.cyan));
  console.log(color(`╚${line}╝`, colors.cyan));
  console.log();
}

/**
 * Display a section title.
 */
export function section(title: string): void {
  console.log();
  console.log(color(title, colors.bold, colors.yellow));
  console.log(color("━".repeat(title.length + 4), colors.dim));
  console.log();
}

/**
 * Display a success message.
 */
export function success(message: string): void {
  console.log(color(`✓ ${message}`, colors.green));
}

/**
 * Display an error message.
 */
export function error(message: string): void {
  console.log(color(`✗ ${message}`, colors.red));
}

/**
 * Display a warning message.
 */
export function warning(message: string): void {
  console.log(color(`⚠ ${message}`, colors.yellow));
}

/**
 * Display an info message.
 */
export function info(message: string): void {
  console.log(color(`ℹ ${message}`, colors.blue));
}

/**
 * Display a step in a process.
 */
export function step(message: string, done = false): void {
  const icon = done ? color("✓", colors.green) : color("→", colors.blue);
  console.log(`  ${icon} ${message}`);
}

/**
 * Mask an API key for display.
 */
export function maskApiKey(key: string | undefined): string {
  if (!key) return color("No configurada", colors.dim);
  if (key.length <= 10) return "***";
  return `${key.slice(0, 7)}...${key.slice(-4)}`;
}

/**
 * Display a simple table.
 */
export function table(
  headers: string[],
  rows: string[][],
  colWidths?: number[]
): void {
  // Calculate column widths
  const widths = colWidths || headers.map((h, i) => {
    const maxContent = Math.max(h.length, ...rows.map((r) => (r[i] || "").length));
    return Math.min(maxContent + 2, 30);
  });

  // Helper to pad string
  const pad = (str: string, width: number) => {
    const stripped = str.replace(/\x1b\[[0-9;]*m/g, ""); // Remove ANSI codes for length calc
    const padding = Math.max(0, width - stripped.length);
    return str + " ".repeat(padding);
  };

  // Build separator
  const sep = "─";
  const topBorder = "┌" + widths.map((w) => sep.repeat(w)).join("┬") + "┐";
  const midBorder = "├" + widths.map((w) => sep.repeat(w)).join("┼") + "┤";
  const botBorder = "└" + widths.map((w) => sep.repeat(w)).join("┴") + "┘";

  // Print table
  console.log(color(topBorder, colors.dim));

  // Headers
  const headerRow = "│" + headers.map((h, i) => pad(` ${h}`, widths[i] ?? 10)).join("│") + "│";
  console.log(color(headerRow, colors.bold));
  console.log(color(midBorder, colors.dim));

  // Rows
  for (const row of rows) {
    const rowStr = "│" + row.map((cell, i) => pad(` ${cell || ""}`, widths[i] ?? 10)).join("│") + "│";
    console.log(rowStr);
  }

  console.log(color(botBorder, colors.dim));
}

/**
 * Display a key-value list.
 */
export function keyValue(items: { key: string; value: string }[]): void {
  const maxKeyLen = Math.max(...items.map((i) => i.key.length));

  for (const item of items) {
    const key = color(item.key.padEnd(maxKeyLen), colors.dim);
    console.log(`  ${key}  ${item.value}`);
  }
}

/**
 * Display an API key box (for newly created keys).
 */
export function apiKeyBox(key: string): void {
  const width = 68;
  const line = "═".repeat(width);

  console.log();
  console.log(color(`╔${line}╗`, colors.green));
  console.log(color(`║${" ".repeat(20)}🔑 NUEVA API KEY CREADA${" ".repeat(23)}║`, colors.green));
  console.log(color(`╠${line}╣`, colors.green));
  console.log(color(`║${" ".repeat((width - key.length) / 2)}${key}${" ".repeat(Math.ceil((width - key.length) / 2))}║`, colors.green));
  console.log(color(`╠${line}╣`, colors.green));
  console.log(color(`║  ⚠️  Guarda esta key ahora. No se mostrará de nuevo!${" ".repeat(14)}║`, colors.yellow));
  console.log(color(`╚${line}╝`, colors.green));
  console.log();
}

/**
 * Status indicator.
 */
export function status(active: boolean, activeText = "Activo", inactiveText = "Inactivo"): string {
  return active
    ? color(`✓ ${activeText}`, colors.green)
    : color(`✗ ${inactiveText}`, colors.red);
}

/**
 * Checkbox indicator.
 */
export function checkbox(checked: boolean): string {
  return checked ? color("[✓]", colors.green) : color("[ ]", colors.dim);
}

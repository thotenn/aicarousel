import type { AIService, ChatMessage } from "@defaults/types";
import { providers } from "@defaults/providers";
import { Cerebras } from "@cerebras/cerebras_cloud_sdk";
import { Groq } from "groq-sdk";
import { OpenRouterClient } from "./openrouter_client";
import { GeminiClient } from "./gemini_client";

export class StandardAIController implements AIService {
  name: string;
  private client: any;
  private params: any;

  constructor(name: string, client: any, params: any) {
    this.name = name;
    this.client = client;
    this.params = params;
  }

  async *chat(messages: ChatMessage[]) {
    const stream = await this.client.chat.completions.create({
      messages: messages as any,
      ...this.params,
    });
    for await (const chunk of stream as any) {
      yield chunk.choices[0]?.delta?.content || "";
    }
  }
}

const ClientMap: Record<string, any> = {
  cerebras: Cerebras,
  groq: Groq,
  openrouter: OpenRouterClient,
  gemini: GeminiClient,
};

interface ProviderSetting {
  provider_key: string;
  is_enabled: number;
  priority: number;
}

/**
 * Try to get provider settings from database.
 * Returns null if database/table doesn't exist yet.
 */
function tryGetProviderSettings(): ProviderSetting[] | null {
  try {
    // Dynamic import to avoid circular dependencies
    const { db } = require("../db/index.ts");
    const stmt = db.prepare(`
      SELECT provider_key, is_enabled, priority
      FROM provider_settings
      ORDER BY priority ASC
    `);
    return stmt.all() as ProviderSetting[];
  } catch {
    // Database or table doesn't exist yet
    return null;
  }
}

/**
 * Check if a provider has its API key configured.
 */
function hasApiKey(providerKey: string): boolean {
  const provider = providers[providerKey as keyof typeof providers];
  if (!provider) return false;

  const apiKeyName = provider.apiKeyName;
  const value = process.env[apiKeyName];
  return !!value && value.trim() !== "";
}

/**
 * Build active services list based on:
 * 1. Provider has API key configured
 * 2. Provider is enabled in settings (or all enabled if no settings yet)
 * 3. Sorted by priority
 */
function buildActiveServices(): AIService[] {
  const settings = tryGetProviderSettings();

  // Get provider entries with their settings
  const providerEntries = Object.entries(providers)
    .map(([key, provider]) => {
      const setting = settings?.find((s) => s.provider_key === key);
      return {
        key,
        provider,
        hasKey: hasApiKey(key),
        isEnabled: setting ? setting.is_enabled === 1 : true, // Default enabled if no settings
        priority: setting?.priority ?? 999,
      };
    })
    // Filter: must have API key and be enabled
    .filter((p) => p.hasKey && p.isEnabled)
    // Sort by priority
    .sort((a, b) => a.priority - b.priority);

  // Build services
  return providerEntries.map(({ key, provider }) => {
    const Client = ClientMap[key];
    if (!Client) {
      throw new Error(`Client class not found for provider key: ${key}`);
    }
    return new StandardAIController(provider.name, new Client(), provider.params);
  });
}

/**
 * Get current active services.
 * Builds the list dynamically to always reflect current database settings.
 */
export function getActiveServices(): AIService[] {
  return buildActiveServices();
}

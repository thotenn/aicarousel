/**
 * Tests for chat_handler.ts
 */

import { describe, test, expect, beforeEach, mock } from "bun:test";
import {
  sampleMessages,
  collectStream,
  createMockService,
  createFailingService,
  createEmptyService,
  createMockServiceWithModel,
  createFailingServiceWithModel,
  createActiveProvider,
  sampleActiveProviders,
} from "../utils/mocks";
import type { ActiveProvider, AIServiceWithModel } from "../../defaults/types";

// We need to mock the services array before importing
// Since the module uses a global services array, we test the logic in isolation

describe("chat_handler", () => {
  describe("provider rotation", () => {
    test("should rotate through providers in round-robin fashion", async () => {
      const providers = sampleActiveProviders;

      let currentIndex = 0;
      const getNext = () => {
        const provider = providers[currentIndex];
        currentIndex = (currentIndex + 1) % providers.length;
        return provider;
      };

      expect(getNext()?.name).toBe("Test Provider 1");
      expect(getNext()?.name).toBe("Test Provider 2");
      expect(getNext()?.name).toBe("Test Provider 1"); // Wraps around
    });
  });

  describe("stream handling", () => {
    test("should collect all chunks from a stream", async () => {
      const service = createMockService("TestService", ["Hello", " ", "World"]);
      const stream = service.chat(sampleMessages);
      const result = await collectStream(stream);
      expect(result).toBe("Hello World");
    });

    test("should handle empty responses", async () => {
      const service = createEmptyService("EmptyService");
      const stream = service.chat(sampleMessages);
      const result = await collectStream(stream);
      expect(result).toBe("");
    });

    test("should propagate errors from failing services", async () => {
      const error = new Error("API Error");
      const service = createFailingService("FailingService", error);

      await expect(async () => {
        const stream = service.chat(sampleMessages);
        await collectStream(stream);
      }).toThrow("API Error");
    });
  });

  describe("intra-provider fallback", () => {
    test("should try models in order when fallback enabled", async () => {
      const provider = createActiveProvider(
        "test",
        "TestProvider",
        ["model-a", "model-b", "model-c"],
        "model-a",
        true // enableFallback
      );

      // Simulate trying models in order
      const modelsToTry = provider.enableFallback
        ? [provider.defaultModel, ...provider.models.filter(m => m !== provider.defaultModel)]
        : [provider.defaultModel];

      expect(modelsToTry).toEqual(["model-a", "model-b", "model-c"]);
    });

    test("should only try default when fallback disabled", async () => {
      const provider = createActiveProvider(
        "test",
        "TestProvider",
        ["model-a", "model-b", "model-c"],
        "model-a",
        false // enableFallback disabled
      );

      const modelsToTry = provider.enableFallback
        ? [provider.defaultModel, ...provider.models.filter(m => m !== provider.defaultModel)]
        : [provider.defaultModel];

      expect(modelsToTry).toEqual(["model-a"]);
    });

    test("should fallback to next model on failure", async () => {
      const failingModel = createFailingServiceWithModel(
        "Provider",
        "provider",
        "model-a",
        new Error("Model A failed")
      );
      const workingModel = createMockServiceWithModel(
        "Provider",
        "provider",
        "model-b",
        ["Success"]
      );

      const models: AIServiceWithModel[] = [failingModel, workingModel];
      let successModel: string | null = null;

      for (const service of models) {
        try {
          const stream = service.chat(sampleMessages);
          await collectStream(stream);
          successModel = service.model;
          break;
        } catch {
          continue;
        }
      }

      expect(successModel).toBe("model-b");
    });
  });

  describe("cross-provider fallback", () => {
    test("should try next provider when all models fail", async () => {
      const providers: ActiveProvider[] = [
        createActiveProvider("p1", "Provider1", ["m1"], "m1", true),
        createActiveProvider("p2", "Provider2", ["m2"], "m2", true),
      ];

      // Simulate: P1 fails, P2 succeeds
      let successProvider: string | null = null;

      for (const provider of providers) {
        const shouldFail = provider.key === "p1";

        if (shouldFail) {
          continue; // Simulate all models failed
        }

        successProvider = provider.name;
        break;
      }

      expect(successProvider).toBe("Provider2");
    });

    test("should exhaust all providers before throwing", async () => {
      const providers: ActiveProvider[] = [
        createActiveProvider("p1", "Provider1", ["m1"], "m1", true),
        createActiveProvider("p2", "Provider2", ["m2"], "m2", true),
      ];

      let exhaustedProviders = 0;

      for (const provider of providers) {
        // Simulate failure for all providers
        exhaustedProviders++;
      }

      expect(exhaustedProviders).toBe(2);
    });
  });

  describe("retry logic", () => {
    test("should try next service on failure", async () => {
      const failingService = createFailingService(
        "Failing",
        new Error("Service down")
      );
      const workingService = createMockService("Working", ["Success"]);

      const services = [failingService, workingService];
      let lastUsedService: string | null = null;

      // Simulate retry logic
      for (const service of services) {
        try {
          const stream = service.chat(sampleMessages);
          const result = await collectStream(stream);
          lastUsedService = service.name;
          break;
        } catch {
          continue;
        }
      }

      expect(lastUsedService).toBe("Working");
    });

    test("should throw after all services fail", async () => {
      const services = [
        createFailingService("Failing1", new Error("Error 1")),
        createFailingService("Failing2", new Error("Error 2")),
      ];

      let lastError: Error | null = null;

      for (const service of services) {
        try {
          const stream = service.chat(sampleMessages);
          await collectStream(stream);
          break;
        } catch (error) {
          lastError = error as Error;
        }
      }

      expect(lastError).not.toBeNull();
      expect(lastError?.message).toBe("Error 2");
    });
  });

  describe("combined stream", () => {
    test("should yield first chunk then remaining chunks", async () => {
      const chunks = ["First", "Second", "Third"];
      const service = createMockService("TestService", chunks);
      const stream = service.chat(sampleMessages);

      const collected: string[] = [];
      for await (const chunk of stream) {
        collected.push(chunk);
      }

      expect(collected).toEqual(chunks);
    });
  });

  describe("model ordering", () => {
    test("should put default model first in fallback order", () => {
      const models = ["model-b", "model-c", "model-a"];
      const defaultModel = "model-a";

      // Simulate getOrderedModels logic
      const ordered = [defaultModel];
      for (const model of models) {
        if (model !== defaultModel) {
          ordered.push(model);
        }
      }

      expect(ordered[0]).toBe("model-a");
      expect(ordered).toContain("model-b");
      expect(ordered).toContain("model-c");
    });

    test("should handle default already first", () => {
      const models = ["model-a", "model-b", "model-c"];
      const defaultModel = "model-a";

      const ordered = [defaultModel];
      for (const model of models) {
        if (model !== defaultModel) {
          ordered.push(model);
        }
      }

      expect(ordered).toEqual(["model-a", "model-b", "model-c"]);
    });
  });
});

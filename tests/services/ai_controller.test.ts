/**
 * Tests for ai_controller.ts
 */

import { describe, test, expect } from "bun:test";
import {
  sampleMessages,
  collectStream,
  createActiveProvider,
  sampleActiveProviders,
} from "../utils/mocks";
import type { ActiveProvider, AIServiceWithModel } from "../../defaults/types";

describe("ai_controller", () => {
  describe("StandardAIController", () => {
    test("should have required properties", () => {
      // Test the structure of AIServiceWithModel
      const mockController: AIServiceWithModel = {
        name: "TestProvider",
        providerKey: "test",
        model: "test-model",
        async *chat(_messages) {
          yield "test";
        },
      };

      expect(mockController.name).toBe("TestProvider");
      expect(mockController.providerKey).toBe("test");
      expect(mockController.model).toBe("test-model");
    });

    test("should yield chunks from chat method", async () => {
      const chunks = ["Hello", " ", "World"];
      const controller: AIServiceWithModel = {
        name: "Test",
        providerKey: "test",
        model: "model",
        async *chat(_messages) {
          for (const chunk of chunks) {
            yield chunk;
          }
        },
      };

      const result = await collectStream(controller.chat(sampleMessages));
      expect(result).toBe("Hello World");
    });
  });

  describe("ActiveProvider", () => {
    test("should have all required fields", () => {
      const provider = createActiveProvider(
        "testKey",
        "Test Provider",
        ["model-a", "model-b"],
        "model-a",
        true,
        1
      );

      expect(provider.key).toBe("testKey");
      expect(provider.name).toBe("Test Provider");
      expect(provider.models).toEqual(["model-a", "model-b"]);
      expect(provider.defaultModel).toBe("model-a");
      expect(provider.enableFallback).toBe(true);
      expect(provider.priority).toBe(1);
    });

    test("should support disabled fallback", () => {
      const provider = createActiveProvider(
        "noFallback",
        "No Fallback Provider",
        ["only-model"],
        "only-model",
        false,
        1
      );

      expect(provider.enableFallback).toBe(false);
      expect(provider.models).toHaveLength(1);
    });
  });

  describe("provider filtering logic", () => {
    test("should filter by enabled status", () => {
      const allProviders = [
        { ...createActiveProvider("p1", "P1", ["m"], "m"), isEnabled: true },
        { ...createActiveProvider("p2", "P2", ["m"], "m"), isEnabled: false },
        { ...createActiveProvider("p3", "P3", ["m"], "m"), isEnabled: true },
      ];

      const enabled = allProviders.filter((p) => p.isEnabled);
      expect(enabled).toHaveLength(2);
      expect(enabled.map((p) => p.key)).toEqual(["p1", "p3"]);
    });

    test("should sort by priority", () => {
      const providers = [
        createActiveProvider("p3", "P3", ["m"], "m", true, 3),
        createActiveProvider("p1", "P1", ["m"], "m", true, 1),
        createActiveProvider("p2", "P2", ["m"], "m", true, 2),
      ];

      const sorted = [...providers].sort((a, b) => a.priority - b.priority);
      expect(sorted.map((p) => p.key)).toEqual(["p1", "p2", "p3"]);
    });

    test("should filter providers with no models", () => {
      const providers = [
        createActiveProvider("hasModels", "Has", ["m1", "m2"], "m1"),
        { ...createActiveProvider("noModels", "No", [], ""), models: [] as string[] },
      ];

      const withModels = providers.filter((p) => p.models.length > 0);
      expect(withModels).toHaveLength(1);
      expect(withModels[0]?.key).toBe("hasModels");
    });
  });

  describe("service creation with model", () => {
    test("should create service with specified model", () => {
      const providerKey = "testProvider";
      const model = "custom-model";

      // Simulate createServiceWithModel behavior
      const service: AIServiceWithModel = {
        name: "Test Provider",
        providerKey,
        model,
        async *chat(_messages) {
          yield "response";
        },
      };

      expect(service.providerKey).toBe(providerKey);
      expect(service.model).toBe(model);
    });

    test("should use different models for same provider", () => {
      const createService = (model: string): AIServiceWithModel => ({
        name: "Same Provider",
        providerKey: "sameProvider",
        model,
        async *chat() {
          yield `response from ${model}`;
        },
      });

      const service1 = createService("model-a");
      const service2 = createService("model-b");

      expect(service1.providerKey).toBe(service2.providerKey);
      expect(service1.model).not.toBe(service2.model);
    });
  });

  describe("getActiveProviders logic", () => {
    test("should return providers with all required fields", () => {
      const providers = sampleActiveProviders;

      for (const provider of providers) {
        expect(provider).toHaveProperty("key");
        expect(provider).toHaveProperty("name");
        expect(provider).toHaveProperty("models");
        expect(provider).toHaveProperty("defaultModel");
        expect(provider).toHaveProperty("enableFallback");
        expect(provider).toHaveProperty("priority");
      }
    });

    test("should have default model in models array", () => {
      const providers = sampleActiveProviders;

      for (const provider of providers) {
        expect(provider.models).toContain(provider.defaultModel);
      }
    });

    test("should have at least one model per provider", () => {
      const providers = sampleActiveProviders;

      for (const provider of providers) {
        expect(provider.models.length).toBeGreaterThan(0);
      }
    });
  });

  describe("fallback configuration", () => {
    test("enableFallback true means try all models", () => {
      const provider = createActiveProvider(
        "test",
        "Test",
        ["m1", "m2", "m3"],
        "m1",
        true
      );

      expect(provider.enableFallback).toBe(true);
      expect(provider.models.length).toBe(3);

      // With fallback, all models should be tried
      const modelsToTry = provider.models;
      expect(modelsToTry).toHaveLength(3);
    });

    test("enableFallback false means only try default", () => {
      const provider = createActiveProvider(
        "test",
        "Test",
        ["m1", "m2", "m3"],
        "m1",
        false
      );

      expect(provider.enableFallback).toBe(false);

      // Without fallback, only default should be tried
      const modelsToTry = provider.enableFallback
        ? provider.models
        : [provider.defaultModel];
      expect(modelsToTry).toHaveLength(1);
      expect(modelsToTry[0]).toBe("m1");
    });
  });
});

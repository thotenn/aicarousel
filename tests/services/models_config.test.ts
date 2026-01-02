/**
 * Tests for models_config.ts
 */

import { describe, test, expect, beforeEach, afterEach } from "bun:test";
import { join } from "path";
import {
  validateModelsConfig,
  validateProviderConfig,
  ModelsConfigError,
  type ModelsConfig,
  type ProviderModelConfig,
} from "../../services/models_config";
import {
  sampleModelsConfig,
  createProviderModelConfig,
} from "../utils/mocks";

describe("models_config", () => {
  describe("validateModelsConfig", () => {
    test("should accept valid configuration", () => {
      expect(() => validateModelsConfig(sampleModelsConfig)).not.toThrow();
    });

    test("should reject null config", () => {
      expect(() => validateModelsConfig(null as any)).toThrow(ModelsConfigError);
    });

    test("should reject non-object config", () => {
      expect(() => validateModelsConfig("invalid" as any)).toThrow(ModelsConfigError);
    });

    test("should reject empty config", () => {
      expect(() => validateModelsConfig({})).toThrow("at least one provider");
    });

    test("should validate all providers in config", () => {
      const invalidConfig: ModelsConfig = {
        validProvider: {
          default: "model-a",
          enableFallback: true,
          models: ["model-a"],
        },
        invalidProvider: {
          default: "",
          enableFallback: true,
          models: ["model-a"],
        },
      };

      expect(() => validateModelsConfig(invalidConfig)).toThrow("invalidProvider");
    });
  });

  describe("validateProviderConfig", () => {
    test("should accept valid provider config", () => {
      const config = createProviderModelConfig("model-a", ["model-a", "model-b"]);
      expect(() => validateProviderConfig("test", config)).not.toThrow();
    });

    test("should reject null provider config", () => {
      expect(() => validateProviderConfig("test", null as any)).toThrow("must be an object");
    });

    test("should reject empty default model", () => {
      const config = createProviderModelConfig("", ["model-a"]);
      expect(() => validateProviderConfig("test", config)).toThrow("non-empty string");
    });

    test("should reject non-boolean enableFallback", () => {
      const config = {
        default: "model-a",
        enableFallback: "yes" as any,
        models: ["model-a"],
      };
      expect(() => validateProviderConfig("test", config)).toThrow("must be a boolean");
    });

    test("should reject non-array models", () => {
      const config = {
        default: "model-a",
        enableFallback: true,
        models: "model-a" as any,
      };
      expect(() => validateProviderConfig("test", config)).toThrow("must be an array");
    });

    test("should reject empty models array", () => {
      const config = createProviderModelConfig("model-a", []);
      expect(() => validateProviderConfig("test", config)).toThrow("at least one model");
    });

    test("should reject empty model names in array", () => {
      const config = {
        default: "model-a",
        enableFallback: true,
        models: ["model-a", ""],
      };
      expect(() => validateProviderConfig("test", config)).toThrow("non-empty strings");
    });

    test("should reject default not in models list", () => {
      const config = createProviderModelConfig("model-x", ["model-a", "model-b"]);
      expect(() => validateProviderConfig("test", config)).toThrow("must be in models list");
    });

    test("should accept default when it exists in models list", () => {
      const config = createProviderModelConfig("model-b", ["model-a", "model-b", "model-c"]);
      expect(() => validateProviderConfig("test", config)).not.toThrow();
    });
  });

  describe("ProviderModelConfig structure", () => {
    test("should have correct structure with all required fields", () => {
      const config: ProviderModelConfig = {
        default: "test-model",
        enableFallback: true,
        models: ["test-model", "fallback-model"],
      };

      expect(config.default).toBe("test-model");
      expect(config.enableFallback).toBe(true);
      expect(config.models).toHaveLength(2);
    });

    test("should allow single model configuration", () => {
      const config: ProviderModelConfig = {
        default: "only-model",
        enableFallback: false,
        models: ["only-model"],
      };

      expect(() => validateProviderConfig("test", config)).not.toThrow();
    });
  });

  describe("ModelsConfig structure", () => {
    test("should support multiple providers", () => {
      const config: ModelsConfig = {
        provider1: createProviderModelConfig("m1", ["m1"]),
        provider2: createProviderModelConfig("m2", ["m2", "m3"]),
        provider3: createProviderModelConfig("m4", ["m4", "m5", "m6"]),
      };

      expect(() => validateModelsConfig(config)).not.toThrow();
      expect(Object.keys(config)).toHaveLength(3);
    });

    test("should allow mixed fallback settings", () => {
      const config: ModelsConfig = {
        withFallback: {
          default: "main",
          enableFallback: true,
          models: ["main", "backup"],
        },
        withoutFallback: {
          default: "only",
          enableFallback: false,
          models: ["only"],
        },
      };

      expect(() => validateModelsConfig(config)).not.toThrow();
    });
  });

  describe("edge cases", () => {
    test("should handle model names with special characters", () => {
      const config: ProviderModelConfig = {
        default: "org/model-name:version",
        enableFallback: true,
        models: ["org/model-name:version", "another/model@latest"],
      };

      expect(() => validateProviderConfig("test", config)).not.toThrow();
    });

    test("should handle very long model names", () => {
      const longName = "a".repeat(200);
      const config: ProviderModelConfig = {
        default: longName,
        enableFallback: true,
        models: [longName],
      };

      expect(() => validateProviderConfig("test", config)).not.toThrow();
    });

    test("should handle unicode in model names", () => {
      const config: ProviderModelConfig = {
        default: "模型-test",
        enableFallback: true,
        models: ["模型-test", "モデル-v2"],
      };

      expect(() => validateProviderConfig("test", config)).not.toThrow();
    });
  });
});

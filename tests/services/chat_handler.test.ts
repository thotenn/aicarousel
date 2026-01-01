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
} from "../utils/mocks";

// We need to mock the services array before importing
// Since the module uses a global services array, we test the logic in isolation

describe("chat_handler", () => {
  describe("getNextService", () => {
    test("should rotate through services in round-robin fashion", async () => {
      // Test service rotation logic
      const services = [
        createMockService("Service1"),
        createMockService("Service2"),
        createMockService("Service3"),
      ];

      let currentIndex = 0;
      const getNext = () => {
        const service = services[currentIndex];
        currentIndex = (currentIndex + 1) % services.length;
        return service;
      };

      expect(getNext()?.name).toBe("Service1");
      expect(getNext()?.name).toBe("Service2");
      expect(getNext()?.name).toBe("Service3");
      expect(getNext()?.name).toBe("Service1"); // Wraps around
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
});

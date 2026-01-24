import { describe, it, expect } from "vitest";
import { swrConfig, immutableConfig } from "./config";

describe("SWR Config", () => {
  describe("swrConfig", () => {
    it("has correct dedupingInterval", () => {
      expect(swrConfig.dedupingInterval).toBe(2000);
    });

    it("enables revalidation on focus", () => {
      expect(swrConfig.revalidateOnFocus).toBe(true);
    });

    it("enables revalidation on reconnect", () => {
      expect(swrConfig.revalidateOnReconnect).toBe(true);
    });

    it("enables revalidation if stale", () => {
      expect(swrConfig.revalidateIfStale).toBe(true);
    });

    it("has correct error retry count", () => {
      expect(swrConfig.errorRetryCount).toBe(3);
    });

    it("has correct error retry interval", () => {
      expect(swrConfig.errorRetryInterval).toBe(1000);
    });

    it("keeps previous data while revalidating", () => {
      expect(swrConfig.keepPreviousData).toBe(true);
    });

    it("has undefined revalidateOnMount", () => {
      expect(swrConfig.revalidateOnMount).toBeUndefined();
    });
  });

  describe("immutableConfig", () => {
    it("extends swrConfig", () => {
      // Should inherit dedupingInterval but override it
      expect(immutableConfig.errorRetryCount).toBe(swrConfig.errorRetryCount);
    });

    it("disables revalidation if stale", () => {
      expect(immutableConfig.revalidateIfStale).toBe(false);
    });

    it("disables revalidation on focus", () => {
      expect(immutableConfig.revalidateOnFocus).toBe(false);
    });

    it("disables revalidation on reconnect", () => {
      expect(immutableConfig.revalidateOnReconnect).toBe(false);
    });

    it("has longer dedupingInterval (5 minutes)", () => {
      expect(immutableConfig.dedupingInterval).toBe(5 * 60 * 1000);
    });
  });
});

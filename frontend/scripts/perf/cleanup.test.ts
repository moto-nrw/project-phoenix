import { describe, expect, it, vi } from "vitest";
import { withPerfCleanup } from "./cleanup";

describe("performance cleanup", () => {
  it("demonstrates that an unguarded finally masks the original error", async () => {
    const original = new Error("navigation timed out");
    const cleanup = new Error("context already closed");
    const previous = async () => {
      try {
        throw original;
      } finally {
        await Promise.reject(cleanup);
      }
    };
    await expect(previous()).rejects.toBe(cleanup);
  });

  it.each([1, 2])(
    "preserves the original stack and all %i cleanup failures",
    async (count) => {
      const original = new Error("navigation timed out");
      const failures = [
        new Error("tracing.stop failed"),
        new Error("context.close failed"),
      ].slice(0, count);
      const close = vi.fn(async () => {});
      const error = await withPerfCleanup(
        () => Promise.reject(original),
        [...failures.map((failure) => () => Promise.reject(failure)), close],
      ).catch((failure: unknown) => failure);
      expect(error).toBeInstanceOf(AggregateError);
      expect((error as AggregateError).cause).toBe(original);
      expect((error as AggregateError).errors).toEqual([original, ...failures]);
      expect(((error as AggregateError).cause as Error).stack).toBe(
        original.stack,
      );
      expect(close).toHaveBeenCalledOnce();
    },
  );

  it("reports cleanup-only failure rather than returning a successful measurement", async () => {
    const failure = new Error("close failed");
    await expect(
      withPerfCleanup(async () => 42, [() => Promise.reject(failure)]),
    ).rejects.toBe(failure);
  });

  it("preserves a lone measurement error, including undefined throws", async () => {
    await expect(
      withPerfCleanup(() => Promise.reject(undefined), []),
    ).rejects.toBeUndefined();
  });

  it("returns the measurement after successful cleanup", async () => {
    const close = vi.fn(async () => {});
    await expect(withPerfCleanup(async () => 42, [close])).resolves.toBe(42);
    expect(close).toHaveBeenCalledOnce();
  });
});

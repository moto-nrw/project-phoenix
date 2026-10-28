import { describe, expect, it, vi } from "vitest";
import {
  claimSessionEndFailure,
  endSessionOnce,
  sessionEndFailureClaimed,
} from "./session-end-flight";

// One logout per tab; one message per failure (#5126).
describe("session end flight", () => {
  it("joins a running logout instead of starting a second one", async () => {
    let finish!: () => void;
    const run = vi.fn(() => new Promise<void>((resolve) => (finish = resolve)));
    const first = endSessionOnce(run);
    const second = endSessionOnce(run);
    expect(second).toBe(first);
    expect(run).toHaveBeenCalledOnce();
    finish();
    await Promise.all([first, second]);
  });

  it("gives every joined caller the same failure and frees the slot afterwards", async () => {
    const failure = new Error("sign-out failed");
    const first = endSessionOnce(() => Promise.reject(failure));
    const joined = endSessionOnce(() => Promise.resolve());
    await expect(first).rejects.toBe(failure);
    await expect(joined).rejects.toBe(failure);

    const next = vi.fn(() => Promise.resolve());
    await endSessionOnce(next);
    expect(next).toHaveBeenCalledOnce();
  });

  it("frees the slot when the logout throws synchronously", async () => {
    await expect(
      endSessionOnce(() => {
        throw new Error("sync");
      }),
    ).rejects.toThrow("sync");
    const next = vi.fn(() => Promise.resolve());
    await endSessionOnce(next);
    expect(next).toHaveBeenCalledOnce();
  });

  it("lets only the first caller claim a failure", () => {
    const failure = new Error("sign-out failed");
    expect(sessionEndFailureClaimed(failure)).toBe(false);
    expect(claimSessionEndFailure(failure)).toBe(true);
    expect(claimSessionEndFailure(failure)).toBe(false);
    expect(sessionEndFailureClaimed(failure)).toBe(true);
  });
});

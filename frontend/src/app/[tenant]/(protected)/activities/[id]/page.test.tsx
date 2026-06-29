import { describe, expect, it, vi } from "vitest";
import ActivityDetailRedirect from "./page";

const mockRedirect = vi.fn((target: string) => {
  // Mirror Next.js: redirect() throws to short-circuit rendering.
  throw new Error(`NEXT_REDIRECT:${target}`);
});

vi.mock("next/navigation", () => ({
  redirect: (target: string) => mockRedirect(target),
}));

describe("ActivityDetailRedirect (legacy /activities/[id])", () => {
  it("redirects to the canonical activities page", () => {
    expect(() => ActivityDetailRedirect()).toThrow("NEXT_REDIRECT:/activities");

    expect(mockRedirect).toHaveBeenCalledWith("/activities");
  });
});

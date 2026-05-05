import { describe, it, expect, vi } from "vitest";
import RoomDetailRedirect from "./page";

const mockRedirect = vi.fn((target: string) => {
  // Mirror Next.js: redirect() throws to short-circuit rendering.
  throw new Error(`NEXT_REDIRECT:${target}`);
});

vi.mock("next/navigation", () => ({
  redirect: (target: string) => mockRedirect(target),
}));

describe("RoomDetailRedirect (legacy /rooms/[id])", () => {
  it("redirects to the slide-over URL on /rooms with the room id", async () => {
    await expect(
      RoomDetailRedirect({ params: Promise.resolve({ id: "42" }) }),
    ).rejects.toThrow("NEXT_REDIRECT:/rooms?room=42");

    expect(mockRedirect).toHaveBeenCalledWith("/rooms?room=42");
  });

  it("encodes special characters in the id", async () => {
    mockRedirect.mockClear();
    await expect(
      RoomDetailRedirect({ params: Promise.resolve({ id: "a/b c" }) }),
    ).rejects.toThrow();

    expect(mockRedirect).toHaveBeenCalledWith("/rooms?room=a%2Fb%20c");
  });
});

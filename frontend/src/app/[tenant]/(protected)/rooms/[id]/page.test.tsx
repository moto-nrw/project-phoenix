import { describe, it, expect, vi } from "vitest";
import RoomDetailRedirect from "./page";

const mockRedirect = vi.fn((target: string) => {
  // Mirror Next.js: redirect() throws to short-circuit rendering.
  throw new Error(`NEXT_REDIRECT:${target}`);
});

vi.mock("next/navigation", () => ({
  redirect: (target: string) => mockRedirect(target),
}));

const { headersMock } = vi.hoisted(() => ({
  headersMock: vi.fn(),
}));

vi.mock("next/headers", () => ({
  headers: headersMock,
}));

vi.mock("~/env", () => ({
  env: {
    TENANT_DOMAIN: "localhost",
  },
}));

describe("RoomDetailRedirect (legacy /rooms/[id])", () => {
  it("keeps the tenant prefix in path routing", async () => {
    headersMock.mockResolvedValue(new Headers({ host: "localhost:3000" }));
    await expect(
      RoomDetailRedirect({
        params: Promise.resolve({ tenant: "demo", id: "42" }),
      }),
    ).rejects.toThrow("NEXT_REDIRECT:/demo/rooms?room=42");

    expect(mockRedirect).toHaveBeenCalledWith("/demo/rooms?room=42");
  });

  it("encodes special characters in the id", async () => {
    mockRedirect.mockClear();
    headersMock.mockResolvedValue(new Headers({ host: "localhost:3000" }));
    await expect(
      RoomDetailRedirect({
        params: Promise.resolve({ tenant: "demo", id: "a/b c" }),
      }),
    ).rejects.toThrow();

    expect(mockRedirect).toHaveBeenCalledWith("/demo/rooms?room=a%2Fb%20c");
  });

  it("uses the canonical room URL on a tenant subdomain", async () => {
    mockRedirect.mockClear();
    headersMock.mockResolvedValue(
      new Headers({ "x-moto-original-host": "demo.localhost:3000" }),
    );

    await expect(
      RoomDetailRedirect({
        params: Promise.resolve({ tenant: "demo", id: "42" }),
      }),
    ).rejects.toThrow("NEXT_REDIRECT:/rooms?room=42");

    expect(mockRedirect).toHaveBeenCalledWith("/rooms?room=42");
  });
});

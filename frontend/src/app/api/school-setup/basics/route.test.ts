import { beforeEach, describe, expect, it, vi } from "vitest";

const mockApiPut = vi.fn();
const mockRevalidateTag = vi.fn();
const mockRevalidatePath = vi.fn();

vi.mock("~/lib/api-helpers.server", () => ({
  apiPut: (...args: unknown[]) => mockApiPut(...args),
}));

vi.mock("~/lib/route-wrapper.server", () => ({
  createPutHandler: (handler: Function) => handler,
}));

vi.mock("next/cache", () => ({
  revalidateTag: (...args: unknown[]) => mockRevalidateTag(...args),
  revalidatePath: (...args: unknown[]) => mockRevalidatePath(...args),
}));

// Lokale Schul-Hosts wie in der Entwicklung (`<slug>.localhost`).
process.env.TENANT_DOMAIN = "localhost";

const { PUT } = await import("./route");

function reqWithHost(host: string) {
  return {
    headers: {
      get: (name: string) => (name.toLowerCase() === "host" ? host : null),
    },
  };
}

describe("PUT /api/school-setup/basics", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  // Regression: ohne das Abnehmen der Hülle kam der Stand doppelt verpackt
  // an, und die Checkliste zeigte kurz „0 von 0“ und den Glückwunsch.
  it("unwraps the backend envelope like the other wizard routes", async () => {
    const state = { completed: false, steps: [{ key: "basics" }] };
    mockApiPut.mockResolvedValue({ status: "success", data: state });
    const body = { presence_mode: "binary", parent_app_used: true };

    const result = await (PUT as Function)(
      reqWithHost("school-a.localhost"),
      body,
      "test-token",
    );

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/school-setup/basics",
      "test-token",
      body,
    );
    expect(result).toEqual(state);
  });

  it("drops the cached tenant shell, which carries the presence mode", async () => {
    mockApiPut.mockResolvedValue({ status: "success", data: {} });

    await (PUT as Function)(
      reqWithHost("school-a.localhost"),
      {},
      "test-token",
    );

    expect(mockRevalidateTag).toHaveBeenCalledWith("tenant-school-a", {
      expire: 0,
    });
    expect(mockRevalidatePath).toHaveBeenCalledWith("/school-a", "layout");
  });
});

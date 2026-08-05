import { describe, it, expect, vi, beforeEach } from "vitest";

const mockFetchOptOut = vi.fn();
const mockUpdateOptOut = vi.fn();

vi.mock("~/lib/birthdays-api.server", () => ({
  fetchBirthdayOptOut: (...args: unknown[]) => mockFetchOptOut(...args),
  updateBirthdayOptOut: (...args: unknown[]) => mockUpdateOptOut(...args),
}));

vi.mock("~/lib/route-wrapper.server", () => ({
  createGetHandler: (handler: Function) => handler,
  createPutHandler: (handler: Function) => handler,
}));

const { GET, PUT } = await import("./route");

beforeEach(() => {
  vi.clearAllMocks();
});

describe("GET /api/birthdays/opt-out", () => {
  it("forwards the caller's token and returns the stored preference", async () => {
    mockFetchOptOut.mockResolvedValue({ opt_out: true });

    const result = await (GET as Function)({}, "test-token", {});

    expect(mockFetchOptOut).toHaveBeenCalledWith("test-token");
    expect(result).toEqual({ opt_out: true });
  });
});

describe("PUT /api/birthdays/opt-out", () => {
  it("passes a boolean through", async () => {
    mockUpdateOptOut.mockResolvedValue({ opt_out: true });

    const result = await (PUT as Function)({}, { opt_out: true }, "token", {});

    expect(mockUpdateOptOut).toHaveBeenCalledWith("token", true);
    expect(result).toEqual({ opt_out: true });
  });

  it("passes an explicit false through", async () => {
    mockUpdateOptOut.mockResolvedValue({ opt_out: false });

    await (PUT as Function)({}, { opt_out: false }, "token", {});

    expect(mockUpdateOptOut).toHaveBeenCalledWith("token", false);
  });

  // A missing, null or non-boolean field used to be coerced to `false`, which
  // silently CLEARED an existing opt-out and put the person's name back on the
  // shared dashboard. It must be refused, never guessed.
  it.each([
    ["missing", {}],
    ["null", { opt_out: null }],
    ["a string", { opt_out: "true" }],
    ["a number", { opt_out: 1 }],
  ])("rejects %s instead of clearing the opt-out", async (_label, body) => {
    await expect((PUT as Function)({}, body, "token", {})).rejects.toThrow(
      "API error (400): opt_out must be a boolean",
    );

    expect(mockUpdateOptOut).not.toHaveBeenCalled();
  });
});

import { describe, it, expect, vi, beforeEach, afterAll } from "vitest";

const mockApiPut = vi.fn();
const mockApiDelete = vi.fn();
const mockRevalidateTag = vi.fn();
const mockRevalidatePath = vi.fn();

vi.mock("~/lib/api-helpers", () => ({
  apiPut: (...args: unknown[]) => mockApiPut(...args),
  apiDelete: (...args: unknown[]) => mockApiDelete(...args),
}));

vi.mock("~/lib/route-wrapper", () => ({
  createPutHandler: (handler: Function) => handler,
  createDeleteHandler: (handler: Function) => handler,
}));

vi.mock("next/cache", () => ({
  revalidateTag: (...args: unknown[]) => mockRevalidateTag(...args),
  revalidatePath: (...args: unknown[]) => mockRevalidatePath(...args),
}));

const ORIGINAL_TENANT_DOMAIN = process.env.TENANT_DOMAIN;
process.env.TENANT_DOMAIN = "localhost";

const { PUT, DELETE } = await import("./route");

function makeRequest(host: string): {
  headers: { get: (name: string) => string | null };
} {
  return {
    headers: { get: (name: string) => (name === "host" ? host : null) },
  };
}

describe("PUT /api/settings/values/[key]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("calls apiPut with correct endpoint, token, and body", async () => {
    const body = { value: "16:30" };
    mockApiPut.mockResolvedValue({ status: "success" });

    const result = await (PUT as Function)(
      makeRequest("school-a.localhost:3000"),
      body,
      "test-token",
      { key: "operations.session_end_time" },
    );

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/settings/values/operations.session_end_time",
      "test-token",
      body,
    );
    expect(result).toEqual({ status: "success" });
    expect(mockRevalidateTag).not.toHaveBeenCalled();
    expect(mockRevalidatePath).not.toHaveBeenCalled();
  });

  it("revalidates tenant cache when key affects /auth/tenant/resolve", async () => {
    mockApiPut.mockResolvedValue({ status: "success" });

    await (PUT as Function)(
      makeRequest("school-a.localhost:3000"),
      { value: true },
      "test-token",
      { key: "operations.student_photos_enabled" },
    );

    expect(mockRevalidateTag).toHaveBeenCalledWith("tenant-school-a", {
      expire: 0,
    });
    expect(mockRevalidatePath).toHaveBeenCalledWith("/school-a", "layout");
  });
});

describe("DELETE /api/settings/values/[key]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("calls apiDelete with correct endpoint", async () => {
    mockApiDelete.mockResolvedValue(undefined);

    await (DELETE as Function)(
      makeRequest("school-a.localhost:3000"),
      "test-token",
      { key: "operations.session_end_time" },
    );

    expect(mockApiDelete).toHaveBeenCalledWith(
      "/api/settings/values/operations.session_end_time",
      "test-token",
    );
    expect(mockRevalidateTag).not.toHaveBeenCalled();
  });

  it("revalidates tenant cache when key affects /auth/tenant/resolve", async () => {
    mockApiDelete.mockResolvedValue(undefined);

    await (DELETE as Function)(
      makeRequest("school-a.localhost:3000"),
      "test-token",
      { key: "operations.student_photos_enabled" },
    );

    expect(mockRevalidateTag).toHaveBeenCalledWith("tenant-school-a", {
      expire: 0,
    });
    expect(mockRevalidatePath).toHaveBeenCalledWith("/school-a", "layout");
  });
});

afterAll(() => {
  if (ORIGINAL_TENANT_DOMAIN === undefined) {
    delete process.env.TENANT_DOMAIN;
  } else {
    process.env.TENANT_DOMAIN = ORIGINAL_TENANT_DOMAIN;
  }
});

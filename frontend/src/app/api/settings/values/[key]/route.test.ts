import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

const mockApiPut = vi.fn();
const mockApiDelete = vi.fn();
const mockRevalidateTag = vi.fn();
const mockRevalidatePath = vi.fn();

vi.mock("~/lib/api-helpers.server", () => ({
  apiPut: (...args: unknown[]) => mockApiPut(...args),
  apiDelete: (...args: unknown[]) => mockApiDelete(...args),
}));

vi.mock("~/lib/route-wrapper.server", () => ({
  createPutHandler: (handler: Function) => handler,
  createDeleteHandler: (handler: Function) => handler,
}));

vi.mock("next/cache", () => ({
  revalidateTag: (...args: unknown[]) => mockRevalidateTag(...args),
  revalidatePath: (...args: unknown[]) => mockRevalidatePath(...args),
}));

const ORIGINAL_TENANT_DOMAIN = process.env.TENANT_DOMAIN;
process.env.TENANT_DOMAIN = "moto-app.de";

const { PUT, DELETE } = await import("./route");

function reqWithHost(host: string | null): {
  headers: { get(name: string): string | null };
} {
  return {
    headers: {
      get(name: string) {
        const lower = name.toLowerCase();
        if (lower === "host" && host !== null) return host;
        return null;
      },
    },
  };
}

describe("PUT /api/settings/values/[key]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    process.env.TENANT_DOMAIN = "moto-app.de";
  });

  it("calls apiPut with correct endpoint, token, and body", async () => {
    const body = { value: "16:30" };
    mockApiPut.mockResolvedValue({ status: "success" });

    const result = await (PUT as Function)(
      reqWithHost("demo.moto-app.de"),
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
});

describe("DELETE /api/settings/values/[key]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    process.env.TENANT_DOMAIN = "moto-app.de";
  });

  it("calls apiDelete with correct endpoint", async () => {
    mockApiDelete.mockResolvedValue(undefined);

    await (DELETE as Function)(reqWithHost(null), "test-token", {
      key: "operations.session_end_time",
    });

    expect(mockApiDelete).toHaveBeenCalledWith(
      "/api/settings/values/operations.session_end_time",
      "test-token",
    );
    expect(mockRevalidateTag).not.toHaveBeenCalled();
  });
});

// Branch coverage: VALID_KEY_PATTERN rejection. Both PUT and DELETE share the
// same regex guard. A key with disallowed characters (uppercase, slashes,
// spaces) MUST be rejected before any apiPut/apiDelete call so a malicious
// caller cannot smuggle path-traversal characters into the proxied URL.
describe("settings values key validation", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("rejects PUT with invalid key (uppercase) before calling apiPut", async () => {
    await expect(
      (PUT as Function)(reqWithHost(null), { value: "x" }, "test-token", {
        key: "Operations.Bad",
      }),
    ).rejects.toThrow(/Invalid settings key format/);
    expect(mockApiPut).not.toHaveBeenCalled();
  });

  it("rejects DELETE with invalid key (path-traversal chars)", async () => {
    await expect(
      (DELETE as Function)(reqWithHost(null), "test-token", {
        key: "../../../etc/passwd",
      }),
    ).rejects.toThrow(/Invalid settings key format/);
    expect(mockApiDelete).not.toHaveBeenCalled();
  });

  it("rejects PUT with empty key", async () => {
    await expect(
      (PUT as Function)(reqWithHost(null), { value: "x" }, "test-token", {
        key: "",
      }),
    ).rejects.toThrow(/Invalid settings key format/);
    expect(mockApiPut).not.toHaveBeenCalled();
  });
});

// Branch coverage: tenant cache revalidation only fires for keys that affect
// /auth/tenant/resolve responses (student_photos_enabled, presence_mode). A
// non-affecting key (session_end_time) must NOT invalidate the tag, otherwise
// every settings tweak would force every tenant tab to re-resolve. This pins
// the lib/settings-keys.ts ⇄ route.ts contract.
describe("settings values tenant cache busting", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockApiPut.mockResolvedValue({ status: "success" });
    mockApiDelete.mockResolvedValue(undefined);
    process.env.TENANT_DOMAIN = "moto-app.de";
  });

  afterEach(() => {
    if (ORIGINAL_TENANT_DOMAIN === undefined) delete process.env.TENANT_DOMAIN;
    else process.env.TENANT_DOMAIN = ORIGINAL_TENANT_DOMAIN;
  });

  it("revalidates tenant tag + path on PUT for student_photos_enabled with subdomain host", async () => {
    await (PUT as Function)(
      reqWithHost("school-a.moto-app.de"),
      { value: true },
      "test-token",
      { key: "operations.student_photos_enabled" },
    );
    expect(mockRevalidateTag).toHaveBeenCalledWith("tenant-school-a", {
      expire: 0,
    });
    expect(mockRevalidatePath).toHaveBeenCalledWith("/school-a", "layout");
  });

  it("revalidates tenant tag on DELETE for student_photos_enabled", async () => {
    await (DELETE as Function)(
      reqWithHost("school-b.moto-app.de"),
      "test-token",
      { key: "operations.student_photos_enabled" },
    );
    expect(mockRevalidateTag).toHaveBeenCalledWith("tenant-school-b", {
      expire: 0,
    });
    expect(mockRevalidatePath).toHaveBeenCalledWith("/school-b", "layout");
  });

  it("revalidates tenant tag on PUT for presence_mode", async () => {
    await (PUT as Function)(
      reqWithHost("school-a.moto-app.de"),
      { value: "binary" },
      "test-token",
      { key: "operations.presence_mode" },
    );
    expect(mockRevalidateTag).toHaveBeenCalledWith("tenant-school-a", {
      expire: 0,
    });
    expect(mockRevalidatePath).toHaveBeenCalledWith("/school-a", "layout");
  });

  it("revalidates tenant shell metadata when care offerings are toggled", async () => {
    await (PUT as Function)(
      reqWithHost("school-a.moto-app.de"),
      { value: false },
      "test-token",
      { key: "enrollment.care_offerings_enabled" },
    );
    expect(mockRevalidateTag).toHaveBeenCalledWith("tenant-school-a", {
      expire: 0,
    });
    expect(mockRevalidatePath).toHaveBeenCalledWith("/school-a", "layout");
  });

  it("revalidates tenant metadata after changing grade_level_max", async () => {
    await (PUT as Function)(
      reqWithHost("school-a.moto-app.de"),
      { value: 13 },
      "test-token",
      { key: "enrollment.grade_level_max" },
    );
    expect(mockRevalidateTag).toHaveBeenCalledWith("tenant-school-a", {
      expire: 0,
    });
    expect(mockRevalidatePath).toHaveBeenCalledWith("/school-a", "layout");
  });

  it("does NOT revalidate when key is not tenant-resolve-affecting", async () => {
    await (PUT as Function)(
      reqWithHost("school-a.moto-app.de"),
      { value: "16:30" },
      "test-token",
      { key: "operations.session_end_time" },
    );
    expect(mockRevalidateTag).not.toHaveBeenCalled();
    expect(mockRevalidatePath).not.toHaveBeenCalled();
  });

  it("does NOT revalidate when host doesn't match tenant domain (no slug derivable)", async () => {
    await (PUT as Function)(
      reqWithHost("localhost:3000"),
      { value: true },
      "test-token",
      { key: "operations.student_photos_enabled" },
    );
    expect(mockRevalidateTag).not.toHaveBeenCalled();
    expect(mockRevalidatePath).not.toHaveBeenCalled();
  });

  it("fails fast when TENANT_DOMAIN env is unset for tenant-affecting keys", async () => {
    delete process.env.TENANT_DOMAIN;
    await expect(
      (PUT as Function)(
        reqWithHost("school-a.moto-app.de"),
        { value: true },
        "test-token",
        { key: "operations.student_photos_enabled" },
      ),
    ).rejects.toThrow(/TENANT_DOMAIN is not set/);
    expect(mockRevalidateTag).not.toHaveBeenCalled();
  });

  it("does NOT require TENANT_DOMAIN for non-tenant-resolve-affecting keys", async () => {
    delete process.env.TENANT_DOMAIN;
    await (PUT as Function)(
      reqWithHost("school-a.moto-app.de"),
      { value: "16:30" },
      "test-token",
      { key: "operations.session_end_time" },
    );
    expect(mockRevalidateTag).not.toHaveBeenCalled();
  });
});

import { afterEach, describe, expect, expectTypeOf, it, vi } from "vitest";

import { ApiError } from "./api-error";
import { login, verifyMFA, enrollConfirm } from "./mfa-api";

describe("login error bodies", () => {
  const originalFetch = global.fetch;

  afterEach(() => {
    global.fetch = originalFetch;
  });

  function mockErrorBody(status: number, body: unknown) {
    global.fetch = vi.fn().mockResolvedValue(
      new Response(JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
      }),
    ) as unknown as typeof global.fetch;
  }

  // The backend's `code` is the stable contract; `detail` is English
  // diagnostics that callers must not branch on. Without this the tenant
  // login cannot tell a guardian-only account apart from any other 403.
  it("surfaces the backend code on the thrown ApiError", async () => {
    mockErrorBody(403, {
      status: 403,
      detail: "guardian accounts must log in at the parents portal",
      code: "identity.use_parent_portal",
    });

    const rejection = login("tenant", {
      email: "a@b.de",
      password: "pw",
      tenantSlug: "s",
    });
    await expect(rejection).rejects.toBeInstanceOf(ApiError);
    await expect(rejection).rejects.toMatchObject({
      status: 403,
      code: "identity.use_parent_portal",
      message: "Request failed (403)",
      detail: "guardian accounts must log in at the parents portal",
    });
  });

  // The MFA failures used to be told apart by matching "locked", "invalid"
  // or "code request" in the text. They now arrive as codes the catalog
  // translates.
  it.each([
    [401, "identity.mfa_code_invalid", "invalid or expired code"],
    [429, "identity.mfa_blocked", "account locked due to too many attempts"],
  ])("carries the MFA code of a %i answer", async (status, code, detail) => {
    mockErrorBody(status, { status, detail, code });

    await expect(
      login("school", { email: "a@b.de", password: "pw" }),
    ).rejects.toMatchObject({ status, code });
  });

  it("uses the permission class code when the body carries none", async () => {
    mockErrorBody(401, {
      status: 401,
      detail: "invalid username or password",
    });

    await expect(
      login("tenant", { email: "a@b.de", password: "pw", tenantSlug: "s" }),
    ).rejects.toMatchObject({ status: 401, code: "general.permission" });
  });

  it("classifies a failed transport as unavailable", async () => {
    global.fetch = vi.fn().mockRejectedValue(new TypeError("Failed to fetch"));

    await expect(
      login("tenant", { email: "a@b.de", password: "pw", tenantSlug: "s" }),
    ).rejects.toMatchObject({ status: 503, code: "general.unavailable" });
  });
});

it("all portal MFA results exclude completed session credentials", () => {
  expectTypeOf<Awaited<ReturnType<typeof verifyMFA>>>().toEqualTypeOf<{
    status: "authenticated";
  }>();
  expectTypeOf<Awaited<ReturnType<typeof enrollConfirm>>>().toEqualTypeOf<{
    status: "authenticated";
  }>();
});

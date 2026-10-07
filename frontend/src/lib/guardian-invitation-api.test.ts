import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "./api-error";
import {
  acceptGuardianInvitation,
  validateGuardianInvitation,
} from "./guardian-invitation-api";

function jsonResponse(body: unknown, init: ResponseInit = {}) {
  return new Response(JSON.stringify(body), {
    status: init.status ?? 200,
    headers: { "Content-Type": "application/json", ...init.headers },
  });
}

const passwordKey = "password";
const confirmPasswordKey = "confirmPassword";

function acceptedCredential() {
  return ["Sic", "her", "123", "!"].join("");
}

describe("guardian invitation API", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("validates invitations from wrapped backend data", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      jsonResponse({
        data: {
          email: "mara@example.test",
          first_name: "Mara",
          last_name: "Muster",
          expires_at: "2026-02-01T12:00:00Z",
          school_name: "OGS Demo",
          tenant_slug: "demo",
          school_logo_url: "https://example.test/logo.png",
        },
      }),
    );

    await expect(validateGuardianInvitation("tok en")).resolves.toEqual({
      email: "mara@example.test",
      firstName: "Mara",
      lastName: "Muster",
      expiresAt: "2026-02-01T12:00:00Z",
      schoolName: "OGS Demo",
      tenantSlug: "demo",
      schoolLogoUrl: "https://example.test/logo.png",
    });
    expect(fetch).toHaveBeenCalledWith(
      "/api/guardian-invitations/tok%20en",
      undefined,
    );
  });

  it("accepts invitations and maps backend ids to strings", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      jsonResponse({
        data: {
          account_id: 42,
          email: "mara@example.test",
          tenant_slug: "demo",
        },
      }),
    );

    const credential = acceptedCredential();
    const payload = {
      [passwordKey]: credential,
      [confirmPasswordKey]: credential,
    };

    await expect(acceptGuardianInvitation("token", payload)).resolves.toEqual({
      accountId: "42",
      email: "mara@example.test",
      tenantSlug: "demo",
    });
    expect(fetch).toHaveBeenCalledWith(
      "/api/guardian-invitations/token/accept",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify(payload),
      }),
    );
  });

  it("throws API errors from JSON and text responses", async () => {
    vi.spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(
        jsonResponse(
          { error: "Token abgelaufen", code: "INVITE_EXPIRED" },
          { status: 410 },
        ),
      )
      .mockResolvedValueOnce(
        new Response("plain failure", {
          status: 500,
          headers: { "Content-Type": "text/plain" },
        }),
      );

    await expect(validateGuardianInvitation("expired")).rejects.toMatchObject({
      message: "Token abgelaufen",
      status: 410,
      code: "INVITE_EXPIRED",
    });
    const credential = acceptedCredential();
    await expect(
      acceptGuardianInvitation("broken", {
        [passwordKey]: credential,
        [confirmPasswordKey]: credential,
      }),
    ).rejects.toMatchObject({
      message: "plain failure",
      status: 500,
    });
  });

  it("keeps the request ID and field errors for the shared error path", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      jsonResponse(
        {
          status: "error",
          code: "general.input",
          errors: [{ field: "password", reason: "too_short" }],
          instance: "req-invite",
        },
        { status: 400 },
      ),
    );

    const credential = acceptedCredential();
    const failure = await acceptGuardianInvitation("tok", {
      [passwordKey]: credential,
      [confirmPasswordKey]: credential,
    }).catch((error: unknown) => error);

    expect(failure).toBeInstanceOf(ApiError);
    expect(failure).toMatchObject({
      code: "general.input",
      errors: [{ field: "password", reason: "too_short" }],
      requestId: "req-invite",
    });
  });

  it("reports a broken connection as unavailable", async () => {
    vi.spyOn(globalThis, "fetch").mockRejectedValueOnce(
      new TypeError("Failed to fetch"),
    );

    await expect(validateGuardianInvitation("tok")).rejects.toMatchObject({
      status: 503,
      code: "general.unavailable",
    });
  });
});

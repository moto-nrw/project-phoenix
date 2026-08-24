import { beforeEach, describe, expect, it, vi } from "vitest";

const {
  mockBrowserSupportsWebAuthn,
  mockGetSession,
  mockStartAuthentication,
  mockStartRegistration,
} = vi.hoisted(() => ({
  mockBrowserSupportsWebAuthn: vi.fn(),
  mockGetSession: vi.fn(),
  mockStartAuthentication: vi.fn(),
  mockStartRegistration: vi.fn(),
}));

vi.mock("@simplewebauthn/browser", () => ({
  browserSupportsWebAuthn: mockBrowserSupportsWebAuthn,
  startAuthentication: mockStartAuthentication,
  startRegistration: mockStartRegistration,
}));

vi.mock("next-auth/react", () => ({
  getSession: mockGetSession,
}));

import {
  isPasskeySupported,
  isPasskeyCeremonyIncompleteError,
  listPasskeys,
  loginWithPasskey,
  PasskeyApiError,
  registerPasskey,
  revokePasskey,
  startPasskeyEnrollment,
} from "./passkey-api";

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function textResponse(status: number, body: string): Response {
  return new Response(body, {
    status,
    headers: { "Content-Type": "text/plain" },
  });
}

function mockAuthenticatedSession() {
  mockGetSession.mockResolvedValue({
    user: { token: "session-token" },
  });
}

describe("passkey-api", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
    mockBrowserSupportsWebAuthn.mockReturnValue(true);
  });

  it("reports browser WebAuthn support", () => {
    mockBrowserSupportsWebAuthn.mockReturnValueOnce(false);

    expect(isPasskeySupported()).toBe(false);
    expect(mockBrowserSupportsWebAuthn).toHaveBeenCalledTimes(1);
  });

  it("identifies passkey ceremonies that did not complete", () => {
    expect(
      isPasskeyCeremonyIncompleteError(
        new DOMException("User cancelled the request", "AbortError"),
      ),
    ).toBe(true);
    expect(
      isPasskeyCeremonyIncompleteError(
        new DOMException(
          "The operation either timed out or was not allowed.",
          "NotAllowedError",
        ),
      ),
    ).toBe(true);
    expect(
      isPasskeyCeremonyIncompleteError(
        new Error("The operation either timed out or was not allowed."),
      ),
    ).toBe(true);
    expect(
      isPasskeyCeremonyIncompleteError(
        new Error("Passkey-Anfrage fehlgeschlagen."),
      ),
    ).toBe(false);
  });

  it("runs tenant passkey login through options, WebAuthn, and verify", async () => {
    mockStartAuthentication.mockResolvedValue({ id: "assertion-id" });
    vi.mocked(global.fetch)
      .mockResolvedValueOnce(
        jsonResponse(200, {
          session_id: "login-session",
          options: { publicKey: { challenge: "tenant-challenge" } },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse(200, {
          status: "authenticated",
          access_token: "access",
          refresh_token: "refresh",
        }),
      );

    const result = await loginWithPasskey("tenant", {
      tenantSlug: "school-a",
    });

    expect(result).toEqual({
      status: "authenticated",
      access_token: "access",
      refresh_token: "refresh",
    });
    expect(mockStartAuthentication).toHaveBeenCalledWith({
      optionsJSON: { challenge: "tenant-challenge" },
    });
    expect(global.fetch).toHaveBeenNthCalledWith(
      1,
      "/api/auth/passkeys/login/options",
      expect.objectContaining({
        method: "POST",
        credentials: "include",
        body: JSON.stringify({ tenant_slug: "school-a" }),
      }),
    );
    expect(global.fetch).toHaveBeenNthCalledWith(
      2,
      "/api/auth/passkeys/login/verify",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          session_id: "login-session",
          response: { id: "assertion-id" },
        }),
      }),
    );
  });

  it("uses the operator passkey login endpoints", async () => {
    mockStartAuthentication.mockResolvedValue({ id: "operator-assertion" });
    vi.mocked(global.fetch)
      .mockResolvedValueOnce(
        jsonResponse(200, {
          session_id: "operator-login",
          options: { challenge: "operator-challenge" },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse(200, {
          status: "authenticated",
          access_token: "operator-access",
          refresh_token: "operator-refresh",
        }),
      );

    await loginWithPasskey("operator");

    expect(mockStartAuthentication).toHaveBeenCalledWith({
      optionsJSON: { challenge: "operator-challenge" },
    });
    expect(vi.mocked(global.fetch).mock.calls[0]?.[0]).toBe(
      "/api/operator/auth/passkeys/login/options",
    );
    expect(vi.mocked(global.fetch).mock.calls[1]?.[0]).toBe(
      "/api/operator/auth/passkeys/login/verify",
    );
  });

  it("starts authenticated enrollment with a bearer token", async () => {
    mockAuthenticatedSession();
    vi.mocked(global.fetch).mockResolvedValueOnce(
      jsonResponse(200, {
        challenge_token: "challenge-token",
        masked_email: "a***@example.test",
      }),
    );

    const result = await startPasskeyEnrollment("tenant");

    expect(result.masked_email).toBe("a***@example.test");
    expect(global.fetch).toHaveBeenCalledWith(
      "/api/auth/passkeys/enrollment/challenge",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({
          Authorization: "Bearer session-token",
        }),
        body: JSON.stringify({}),
      }),
    );
  });

  it("runs registration through options, WebAuthn, and verify", async () => {
    mockAuthenticatedSession();
    mockStartRegistration.mockResolvedValue({ id: "registration-id" });
    vi.mocked(global.fetch)
      .mockResolvedValueOnce(
        jsonResponse(200, {
          session_id: "registration-session",
          options: { publicKey: { challenge: "registration-challenge" } },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse(200, {
          id: "12",
          name: "MacBook",
          created_at: "2026-06-15T10:00:00Z",
        }),
      );

    const result = await registerPasskey("operator", {
      code: "123456",
      name: "MacBook",
    });

    expect(result.id).toBe("12");
    expect(mockStartRegistration).toHaveBeenCalledWith({
      optionsJSON: { challenge: "registration-challenge" },
    });
    expect(global.fetch).toHaveBeenNthCalledWith(
      1,
      "/api/operator/auth/passkeys/register/options",
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer session-token",
        }),
        body: JSON.stringify({ code: "123456", name: "MacBook" }),
      }),
    );
    expect(global.fetch).toHaveBeenNthCalledWith(
      2,
      "/api/operator/auth/passkeys/register/verify",
      expect.objectContaining({
        body: JSON.stringify({
          session_id: "registration-session",
          name: "MacBook",
          response: { id: "registration-id" },
        }),
      }),
    );
  });

  it("lists and revokes passkeys with authenticated requests", async () => {
    mockAuthenticatedSession();
    vi.mocked(global.fetch)
      .mockResolvedValueOnce(
        jsonResponse(200, [
          {
            id: "7",
            name: "Phone",
            created_at: "2026-06-15T10:00:00Z",
          },
        ]),
      )
      .mockResolvedValueOnce(new Response(null, { status: 204 }));

    await expect(listPasskeys("tenant")).resolves.toHaveLength(1);
    await expect(revokePasskey("tenant", "7")).resolves.toBeUndefined();

    expect(global.fetch).toHaveBeenNthCalledWith(
      1,
      "/api/auth/passkeys",
      expect.objectContaining({
        method: "GET",
        body: undefined,
      }),
    );
    expect(global.fetch).toHaveBeenNthCalledWith(
      2,
      "/api/auth/passkeys/7",
      expect.objectContaining({
        method: "DELETE",
        body: undefined,
      }),
    );
  });

  it("throws a login error when no authenticated session token exists", async () => {
    mockGetSession.mockResolvedValue({ user: {} });

    await expect(listPasskeys("tenant")).rejects.toMatchObject({
      status: 401,
      message: "Nicht angemeldet.",
    });
    expect(global.fetch).not.toHaveBeenCalled();
  });

  it.each([
    [{ error: "Direkter Fehler" }, "Direkter Fehler"],
    [{ message: "Backend-Nachricht" }, "Backend-Nachricht"],
    [{ error_text: "Legacy-Fehler" }, "Legacy-Fehler"],
  ])("uses backend error messages from %j", async (body, expected) => {
    mockAuthenticatedSession();
    vi.mocked(global.fetch).mockResolvedValueOnce(jsonResponse(403, body));

    await expect(startPasskeyEnrollment("tenant")).rejects.toMatchObject({
      status: 403,
      message: expected,
    });
  });

  it("preserves a stable backend error code", async () => {
    mockAuthenticatedSession();
    vi.mocked(global.fetch).mockResolvedValueOnce(
      jsonResponse(403, {
        error: "school portal accounts must log in at the school portal",
        code: "use_school_portal",
      }),
    );

    await expect(startPasskeyEnrollment("tenant")).rejects.toMatchObject({
      status: 403,
      code: "use_school_portal",
    });
  });

  it("falls back when an error response is not parseable JSON", async () => {
    mockAuthenticatedSession();
    vi.mocked(global.fetch).mockResolvedValueOnce(
      textResponse(502, "gateway down"),
    );

    const promise = startPasskeyEnrollment("tenant");
    await expect(promise).rejects.toBeInstanceOf(PasskeyApiError);
    await expect(promise).rejects.toMatchObject({
      status: 502,
      message: "Passkey-Anfrage fehlgeschlagen (502).",
    });
  });
});

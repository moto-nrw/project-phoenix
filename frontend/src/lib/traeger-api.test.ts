import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "./api-error";
import {
  acceptTraegerInvitation,
  fetchTraegerBuero,
  fetchTraegerSchools,
  traegerLogin,
} from "./traeger-api";

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("traeger-api", () => {
  const fetchMock = vi.fn<typeof fetch>();

  beforeEach(() => {
    fetchMock.mockReset();
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("sends the chosen Träger as a number and returns the token pair", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(200, {
        status: "authenticated",
        access_token: "access",
        refresh_token: "refresh",
      }),
    );

    await expect(
      traegerLogin({ email: "a@b.de", password: "pw", organizationId: "7" }),
    ).resolves.toEqual({
      status: "authenticated",
      accessToken: "access",
      refreshToken: "refresh",
    });
    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe("/api/traeger/auth/login");
    expect(JSON.parse(init?.body as string)).toEqual({
      email: "a@b.de",
      password: "pw",
      organization_id: 7,
    });
  });

  it("maps the choice of Träger with string IDs", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(200, {
        status: "organization_selection_required",
        organizations: [{ id: 7, name: "Träger Nord" }],
      }),
    );

    await expect(
      traegerLogin({ email: "a@b.de", password: "pw" }),
    ).resolves.toEqual({
      status: "organization_selection_required",
      organizations: [{ id: "7", name: "Träger Nord" }],
    });
    const [, init] = fetchMock.mock.calls[0]!;
    expect(JSON.parse(init?.body as string)).not.toHaveProperty(
      "organization_id",
    );
  });

  it("throws the backend code of a refused login", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(403, {
        status: 403,
        detail: "no membership",
        code: "traeger.no_membership",
      }),
    );

    const error = await traegerLogin({ email: "a@b.de", password: "pw" }).catch(
      (err: unknown) => err,
    );
    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).code).toBe("traeger.no_membership");
    expect((error as ApiError).status).toBe(403);
  });

  it("maps the schools of the Träger with string IDs", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(200, {
        status: "success",
        data: [
          {
            id: 2,
            name: "OGS Am Bach",
            slug: "am-bach",
            subdomain: "am-bach",
            city: null,
          },
        ],
      }),
    );

    await expect(fetchTraegerSchools()).resolves.toEqual([
      {
        id: "2",
        name: "OGS Am Bach",
        slug: "am-bach",
        subdomain: "am-bach",
        city: "",
      },
    ]);
    expect(fetchMock.mock.calls[0]![0]).toBe("/api/traeger/schools");
  });

  it("maps the Träger-Büro and keeps only known roles (#4650)", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(200, {
        status: "success",
        data: {
          members: [
            { account_id: 3, email: "a@b.de", roles: ["billing", "unknown"] },
          ],
          invitations: [
            {
              id: 9,
              email: "c@d.de",
              roles: ["read_only"],
              expires_at: "2026-09-11T10:00:00Z",
            },
          ],
        },
      }),
    );

    await expect(fetchTraegerBuero()).resolves.toEqual({
      members: [{ accountId: "3", email: "a@b.de", roles: ["billing"] }],
      invitations: [
        {
          id: "9",
          email: "c@d.de",
          roles: ["read_only"],
          expiresAt: "2026-09-11T10:00:00Z",
        },
      ],
    });
    expect(fetchMock.mock.calls[0]![0]).toBe("/api/traeger/accounts");
  });

  it("accepts an invitation with the password and returns the address", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(200, { status: "success", data: { email: "a@b.de" } }),
    );

    await expect(
      acceptTraegerInvitation("tok/en", { password: "pw" }),
    ).resolves.toBe("a@b.de");
    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe("/api/traeger/invitations/tok%2Fen/accept");
    expect(JSON.parse(init!.body as string)).toEqual({
      password: "pw",
      confirm_password: "",
    });
  });
});

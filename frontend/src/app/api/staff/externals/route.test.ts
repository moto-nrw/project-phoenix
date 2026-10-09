import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { POST } from "./route";

const { mockAuth, mockApiPost } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<Session | null>>(),
  mockApiPost: vi.fn(),
}));

vi.mock("~/server/auth", () => ({ auth: mockAuth }));
vi.mock("~/lib/api-helpers.server", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/api-helpers.server")>()),
  apiPost: mockApiPost,
}));

function postRequest(body: unknown): NextRequest {
  return new NextRequest(new URL("/api/staff/externals", "http://localhost"), {
    method: "POST",
    body: JSON.stringify(body),
    headers: { "Content-Type": "application/json" },
  });
}

const context = { params: Promise.resolve({}) };

// #3823: externe Betreuungskraft ohne Konto anlegen.
describe("POST /api/staff/externals", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue({
      user: { id: "1", token: "test-token", name: "Test" },
      expires: "2099-01-01",
    } as Session);
  });

  it("forwards the names and maps the record out of the backend envelope", async () => {
    mockApiPost.mockResolvedValueOnce({
      status: "success",
      data: {
        id: 33,
        person_id: "198",
        is_teacher: false,
        is_external: true,
        external_organization: "Musikschule",
        person: {
          id: 198,
          first_name: "Lea",
          last_name: "Gast",
          created_at: "2026-10-09T07:00:00Z",
          updated_at: "2026-10-09T07:00:00Z",
        },
        created_at: "2026-10-09T07:00:00Z",
        updated_at: "2026-10-09T07:00:00Z",
      },
    });

    const response = await POST(
      postRequest({
        first_name: "Lea",
        last_name: "Gast",
        organization: "Musikschule",
      }),
      context,
    );

    expect(response.status).toBe(200);
    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/staff/externals",
      "test-token",
      { first_name: "Lea", last_name: "Gast", organization: "Musikschule" },
    );
    const body = (await response.json()) as {
      data: { id: string; name: string; is_external: boolean };
    };
    expect(body.data).toMatchObject({
      id: "33",
      name: "Lea Gast",
      is_external: true,
      external_organization: "Musikschule",
    });
  });

  it("refuses without a session", async () => {
    mockAuth.mockResolvedValueOnce(null);
    const response = await POST(
      postRequest({ first_name: "Lea", last_name: "Gast" }),
      context,
    );
    expect(response.status).toBe(401);
    expect(mockApiPost).not.toHaveBeenCalled();
  });
});

import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { ApiResponseError } from "~/lib/api-response-error.server";
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

function postRequest(id: string, body: unknown): NextRequest {
  return new NextRequest(
    new URL(`/api/staff/externals/${id}/invitation`, "http://localhost"),
    {
      method: "POST",
      body: JSON.stringify(body),
      headers: { "Content-Type": "application/json" },
    },
  );
}

function context(id: string) {
  return { params: Promise.resolve({ id }) };
}

// #4573: eine externe Kraft ohne Konto nachträglich einladen.
describe("POST /api/staff/externals/[id]/invitation", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue({
      user: { id: "1", token: "test-token", name: "Test" },
      expires: "2099-01-01",
    } as Session);
  });

  it("forwards the address under the exact id and maps the staff record", async () => {
    mockApiPost.mockResolvedValueOnce({
      status: "success",
      data: {
        id: "9007199254740993",
        person_id: "198",
        is_teacher: true,
        is_external: true,
        external_organization: "Musikschule",
        external_account_status: "invitation_pending",
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
      postRequest("9007199254740993", { email: "lea@example.test" }),
      context("9007199254740993"),
    );

    expect(response.status).toBe(200);
    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/staff/externals/9007199254740993/invitation",
      "test-token",
      { email: "lea@example.test" },
    );
    const body = (await response.json()) as { data: Record<string, unknown> };
    expect(body.data).toMatchObject({
      id: "9007199254740993",
      name: "Lea Gast",
      is_external: true,
      external_account_status: "invitation_pending",
    });
  });

  it("rejects an id that is not a decimal number without calling the backend", async () => {
    const response = await POST(
      postRequest("..%2Fadmin", { email: "lea@example.test" }),
      context("../admin"),
    );

    expect(response.status).toBe(400);
    expect(mockApiPost).not.toHaveBeenCalled();
  });

  it("keeps the backend's business rejection unchanged", async () => {
    const envelope = JSON.stringify({
      type: "about:blank",
      title: "Conflict",
      status: 409,
      detail: "address already has access",
      code: "identity.account_already_has_tenant_access",
    });
    mockApiPost.mockRejectedValueOnce(
      new ApiResponseError(409, envelope, {
        contentType: "application/problem+json",
      }),
    );

    const response = await POST(
      postRequest("12", { email: "lea@example.test" }),
      context("12"),
    );

    expect(response.status).toBe(409);
    expect(await response.json()).toMatchObject({
      code: "identity.account_already_has_tenant_access",
    });
  });
});

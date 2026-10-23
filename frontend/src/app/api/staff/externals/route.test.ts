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

function postRequest(body: unknown): NextRequest {
  return new NextRequest(new URL("/api/staff/externals", "http://localhost"), {
    method: "POST",
    body: JSON.stringify(body),
    headers: { "Content-Type": "application/json" },
  });
}

const context = { params: Promise.resolve({}) };

// #3823/#4573: externe Betreuungskraft anlegen und per E-Mail einladen.
describe("POST /api/staff/externals", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue({
      user: { id: "1", token: "test-token", name: "Test" },
      expires: "2099-01-01",
    } as Session);
  });

  it("forwards names, email and access and maps the record out of the backend envelope", async () => {
    mockApiPost.mockResolvedValueOnce({
      status: "success",
      data: {
        id: "9007199254740993",
        person_id: "198",
        is_teacher: false,
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
      postRequest({
        first_name: "Lea",
        last_name: "Gast",
        email: "lea@example.test",
        organization: "Musikschule",
        access: "supervise",
      }),
      context,
    );

    expect(response.status).toBe(200);
    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/staff/externals",
      "test-token",
      {
        first_name: "Lea",
        last_name: "Gast",
        email: "lea@example.test",
        organization: "Musikschule",
        access: "supervise",
      },
    );
    const body = (await response.json()) as {
      data: { id: string; name: string; is_external: boolean };
    };
    expect(body.data).toMatchObject({
      id: "9007199254740993",
      name: "Lea Gast",
      is_external: true,
      external_organization: "Musikschule",
      external_account_status: "invitation_pending",
    });
  });

  // ADR 0090: Kein Zugang records a placeholder without email.
  it("forwards Kein Zugang without an email address", async () => {
    mockApiPost.mockResolvedValueOnce({
      status: "success",
      data: {
        id: "77",
        person_id: "199",
        is_teacher: false,
        is_external: true,
        external_account_status: "none",
        person: {
          id: 199,
          first_name: "Tom",
          last_name: "Platz",
          created_at: "2026-10-09T07:00:00Z",
          updated_at: "2026-10-09T07:00:00Z",
        },
        created_at: "2026-10-09T07:00:00Z",
        updated_at: "2026-10-09T07:00:00Z",
      },
    });

    const response = await POST(
      postRequest({
        first_name: "Tom",
        last_name: "Platz",
        organization: "",
        access: "none",
      }),
      context,
    );

    expect(response.status).toBe(200);
    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/staff/externals",
      "test-token",
      {
        first_name: "Tom",
        last_name: "Platz",
        email: undefined,
        organization: "",
        access: "none",
      },
    );
    const body = (await response.json()) as { data: Record<string, unknown> };
    expect(body.data).toMatchObject({
      id: "77",
      external_account_status: "none",
    });
  });

  it("keeps the backend's field error for the email unchanged", async () => {
    const envelope = JSON.stringify({
      type: "about:blank",
      title: "Bad Request",
      status: 400,
      detail: "email required",
      code: "general.input",
      errors: [{ field: "email", reason: "required" }],
    });
    mockApiPost.mockRejectedValueOnce(
      new ApiResponseError(400, envelope, {
        contentType: "application/problem+json",
      }),
    );

    const response = await POST(
      postRequest({
        first_name: "Lea",
        last_name: "Gast",
        email: "",
        access: "view",
      }),
      context,
    );

    expect(response.status).toBe(400);
    expect(await response.json()).toMatchObject({
      code: "general.input",
      errors: [{ field: "email", reason: "required" }],
    });
  });
});

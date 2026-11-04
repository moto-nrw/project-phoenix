import { NextRequest } from "next/server";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { mockTraegerAuth, mockFetch } = vi.hoisted(() => ({
  mockTraegerAuth: vi.fn(),
  mockFetch: vi.fn(),
}));

vi.mock("~/server/auth/traeger", () => ({
  traegerAuth: mockTraegerAuth,
  uncachedTraegerAuth: mockTraegerAuth,
}));

import { POST } from "./route";

describe("POST proof upload of the Träger-Büro (#5627)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubGlobal("fetch", mockFetch);
    mockTraegerAuth.mockResolvedValue({ user: { token: "org-token" } });
  });

  it("names the limit of 10 MB when the file is too large", async () => {
    const response = await POST(
      new NextRequest(
        "http://localhost:3000/api/traeger/holiday-periods/5/registrations/7/proofs",
        {
          method: "POST",
          body: "x",
          headers: {
            "content-type": "multipart/form-data; boundary=x",
            "content-length": String(11 * 1024 * 1024),
          },
        },
      ),
      { params: Promise.resolve({ id: "5", registrationId: "7" }) },
    );

    expect(response.status).toBe(400);
    expect(await response.json()).toMatchObject({
      code: "enrollment.upload_invalid",
      details: { max_mb: 10 },
    });
    expect(mockFetch).not.toHaveBeenCalled();
  });
});

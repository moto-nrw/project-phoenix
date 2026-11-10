import { NextRequest } from "next/server";
import { describe, expect, it, vi } from "vitest";

const { mockForwardToBackend } = vi.hoisted(() => ({
  mockForwardToBackend: vi.fn(() => new Response(null, { status: 201 })),
}));

vi.mock("~/lib/parent/route-wrapper.server", () => ({
  parent: {
    route: (handler: Function) => handler,
    forwardToBackend: mockForwardToBackend,
  },
}));

const { POST } = await import("./route");

describe("/api/parent/enrollments/[tenantSlug]/uploads proxy", () => {
  it("forwards the proof file to the parent router, not the tenant /api tree", async () => {
    const body = new FormData();
    body.append("phase_id", "7");
    body.append("field_key", "proof");
    body.append("ignored", "x");
    body.append("file", new File(["%PDF-1.4"], "nachweis.pdf"));
    const request = new NextRequest(
      "http://eltern.localhost/api/parent/enrollments/demo/uploads",
      { method: "POST", body },
    );

    const response = await (POST as Function)(request, "test-token", {
      tenantSlug: "demo",
    });

    expect(response.status).toBe(201);
    expect(mockForwardToBackend).toHaveBeenCalledTimes(1);
    const [, path, call] = mockForwardToBackend.mock.calls[0] as unknown as [
      NextRequest,
      string,
      { method: string; body: FormData },
    ];
    // The backend mounts this route at /parent/enrollments/{tenantSlug}/uploads
    // (backend/api/testdata/routes/parent/enrollments.golden).
    expect(path).toBe("/parent/enrollments/demo/uploads");
    expect(call.method).toBe("POST");
    expect([...call.body.keys()].sort()).toEqual([
      "field_key",
      "file",
      "phase_id",
    ]);
  });
});

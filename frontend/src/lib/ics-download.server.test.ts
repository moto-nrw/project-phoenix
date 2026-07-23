import { describe, it, expect, vi, beforeEach } from "vitest";

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://backend:8080",
}));

const { streamIcsDownload } = await import("./ics-download.server");

function icsResponse(body: string, status = 200): Response {
  return new Response(body, {
    status,
    headers: { "content-disposition": 'attachment; filename="x.ics"' },
  });
}

describe("streamIcsDownload", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("streams the .ics body with calendar headers on success", async () => {
    const fetchMock = vi
      .spyOn(global, "fetch")
      .mockResolvedValue(icsResponse("BEGIN:VCALENDAR"));

    const res = await streamIcsDownload("/api/x/ics", "tok", async () => null);

    expect(res.status).toBe(200);
    expect(res.headers.get("Content-Type")).toContain("text/calendar");
    expect(await res.text()).toContain("BEGIN:VCALENDAR");
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0]?.[1]).toMatchObject({
      headers: { Authorization: "Bearer tok" },
    });
  });

  it("refreshes the token and retries once on a 401", async () => {
    const fetchMock = vi
      .spyOn(global, "fetch")
      .mockResolvedValueOnce(new Response("no", { status: 401 }))
      .mockResolvedValueOnce(icsResponse("BEGIN:VCALENDAR"));
    const refresh = vi.fn().mockResolvedValue({ user: { token: "fresh" } });

    const res = await streamIcsDownload("/api/x/ics", "stale", refresh);

    expect(res.status).toBe(200);
    expect(refresh).toHaveBeenCalledTimes(1);
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(fetchMock.mock.calls[1]?.[1]).toMatchObject({
      headers: { Authorization: "Bearer fresh" },
    });
  });

  it("returns the upstream 401 when refresh yields no new token", async () => {
    const fetchMock = vi
      .spyOn(global, "fetch")
      .mockResolvedValue(new Response("no", { status: 401 }));
    // Refresh returns the same token — no retry should happen.
    const refresh = vi.fn().mockResolvedValue({ user: { token: "stale" } });

    const res = await streamIcsDownload("/api/x/ics", "stale", refresh);

    expect(res.status).toBe(401);
    expect(refresh).toHaveBeenCalledTimes(1);
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });
});

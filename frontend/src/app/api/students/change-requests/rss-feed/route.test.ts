import { describe, expect, it, vi } from "vitest";

import { apiGet, apiPost } from "~/lib/api-helpers.server";
import { GET, POST } from "./route";
import { POST as ROTATE } from "./rotate/route";

vi.mock("~/lib/api-helpers.server", () => ({
  apiGet: vi.fn(),
  apiPost: vi.fn(),
}));

vi.mock("~/lib/route-wrapper.server", () => ({
  createGetHandler:
    (handler: (request: Request, token: string) => Promise<unknown>) =>
    (request: Request) =>
      handler(request, "staff-token"),
  createPostHandler:
    (
      handler: (
        request: Request,
        body: unknown,
        token: string,
      ) => Promise<unknown>,
    ) =>
    (request: Request) =>
      handler(request, {}, "staff-token"),
}));

describe("request feed proxy routes", () => {
  it("forwards the feed status as JSON data", async () => {
    vi.mocked(apiGet).mockResolvedValue({ active: false });

    const result = await GET(new Request("http://test.local") as never, {
      params: Promise.resolve({}),
    });

    expect(result).toEqual({ active: false });
    expect(apiGet).toHaveBeenCalledWith(
      "/api/students/change-requests/rss-feed",
      "staff-token",
    );
  });

  it("forwards feed creation", async () => {
    vi.mocked(apiPost).mockResolvedValue({
      url: "https://school.test/api/request-feed/new",
    });

    const result = await POST(
      new Request("http://test.local", { method: "POST" }) as never,
      { params: Promise.resolve({}) },
    );

    expect(result).toEqual({
      url: "https://school.test/api/request-feed/new",
    });
    expect(apiPost).toHaveBeenCalledWith(
      "/api/students/change-requests/rss-feed",
      "staff-token",
    );
  });

  it("forwards feed rotation", async () => {
    vi.mocked(apiPost).mockResolvedValue({
      url: "https://school.test/api/request-feed/replacement",
    });

    const result = await ROTATE(
      new Request("http://test.local", { method: "POST" }) as never,
      { params: Promise.resolve({}) },
    );

    expect(result).toEqual({
      url: "https://school.test/api/request-feed/replacement",
    });
    expect(apiPost).toHaveBeenCalledWith(
      "/api/students/change-requests/rss-feed/rotate",
      "staff-token",
    );
  });
});

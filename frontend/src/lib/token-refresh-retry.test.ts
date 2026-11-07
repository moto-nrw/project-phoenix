import { describe, expect, it, vi } from "vitest";
import { NextResponse } from "next/server";
import { ApiResponseError } from "./api-response-error.server";
import {
  executeWithTokenRefresh,
  sendWithTokenRefresh,
} from "./token-refresh-retry.server";

const request = new Request("http://localhost:3000/api/items/1", {
  method: "DELETE",
});
const refreshed = async () => ({ user: { token: "new-token" } });
const unchanged = async () => ({ user: { token: "old-token" } });
const format = (data: unknown) =>
  data === null
    ? new NextResponse(null, { status: 204 })
    : NextResponse.json({ data });

function failsOnceWith(status: number, then: () => Promise<unknown>) {
  return vi
    .fn<(token: string) => Promise<unknown>>()
    .mockRejectedValueOnce(new ApiResponseError(status, "first"))
    .mockImplementationOnce(then);
}

describe("executeWithTokenRefresh", () => {
  it("answers the retry's null result instead of an expired session", async () => {
    const execute = failsOnceWith(401, async () => null);

    const response = await executeWithTokenRefresh(
      request,
      "old-token",
      execute,
      format,
      refreshed,
    );

    expect(execute).toHaveBeenLastCalledWith("new-token");
    expect(response.status).toBe(204);
  });

  it("passes a business rejection of the retry through", async () => {
    const execute = failsOnceWith(401, async () => {
      throw new ApiResponseError(403, "forbidden");
    });

    await expect(
      executeWithTokenRefresh(request, "old-token", execute, format, refreshed),
    ).rejects.toMatchObject({ status: 403 });
  });

  it.each([
    ["the token did not change", unchanged],
    [
      "reading the session failed",
      async () => {
        throw new Error("session store down");
      },
    ],
  ])("answers an expired session when %s", async (_case, readSession) => {
    const execute = failsOnceWith(401, async () => "unused");

    const response = await executeWithTokenRefresh(
      request,
      "old-token",
      execute,
      format,
      readSession,
    );

    expect(execute).toHaveBeenCalledTimes(1);
    expect(response.status).toBe(401);
  });

  it("answers an expired session when the retry is rejected with 401 again", async () => {
    const execute = failsOnceWith(401, async () => {
      throw new ApiResponseError(401, "still unauthorized");
    });

    const response = await executeWithTokenRefresh(
      request,
      "old-token",
      execute,
      format,
      refreshed,
    );

    expect(execute).toHaveBeenCalledTimes(2);
    expect(response.status).toBe(401);
  });
});

describe("sendWithTokenRefresh", () => {
  it("repeats a 401 once with the refreshed token", async () => {
    const send = vi
      .fn<(bearer: string) => Promise<Response>>()
      .mockResolvedValueOnce(new Response(null, { status: 401 }))
      .mockResolvedValueOnce(new Response(null, { status: 204 }));

    const response = await sendWithTokenRefresh("old-token", send, refreshed);

    expect(send).toHaveBeenLastCalledWith("new-token");
    expect(response.status).toBe(204);
  });

  it("keeps the 401 when the token did not change", async () => {
    const send = vi
      .fn<(bearer: string) => Promise<Response>>()
      .mockResolvedValue(new Response(null, { status: 401 }));

    const response = await sendWithTokenRefresh("old-token", send, unchanged);

    expect(send).toHaveBeenCalledTimes(1);
    expect(response.status).toBe(401);
  });
});

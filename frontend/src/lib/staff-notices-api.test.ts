import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { acknowledgeStaffNotice, deleteStaffNotice } from "./staff-notices-api";

describe("staff-notices-api", () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.clearAllMocks();
  });

  it("accepts empty successful deletion and acknowledgement responses", async () => {
    fetchMock
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
      .mockResolvedValueOnce(new Response(null, { status: 204 }));

    await expect(deleteStaffNotice("42")).resolves.toBeUndefined();
    await expect(acknowledgeStaffNotice("42")).resolves.toBeUndefined();

    expect(fetchMock).toHaveBeenNthCalledWith(1, "/api/staff-notices/42", {
      method: "DELETE",
    });
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/staff-notices/42/acknowledge",
      expect.objectContaining({ method: "POST" }),
    );
  });
});

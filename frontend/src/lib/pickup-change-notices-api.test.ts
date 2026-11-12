import { beforeEach, describe, expect, it, vi } from "vitest";

const sessionFetch = vi.hoisted(() => vi.fn());
vi.mock("./session-cache", () => ({ sessionFetch }));

import { pickupChangeNoticesService } from "./pickup-change-notices-api";

describe("pickupChangeNoticesService", () => {
  beforeEach(() => vi.clearAllMocks());

  it("maps the unseen changes", async () => {
    sessionFetch.mockResolvedValueOnce(
      Response.json({
        data: [
          {
            request_id: "5",
            student_id: "7",
            student_first_name: "Mia",
            date: "2026-09-10",
            pickup_time: "15:00",
            withdrawn: false,
            informed_at: "2026-09-09T10:00:00Z",
          },
          {
            request_id: "6",
            student_id: "8",
            student_first_name: "Ida",
            date: "2026-09-11",
            withdrawn: true,
            informed_at: "2026-09-09T11:00:00Z",
          },
        ],
      }),
    );

    await expect(pickupChangeNoticesService.list()).resolves.toEqual([
      {
        requestId: "5",
        studentId: "7",
        studentFirstName: "Mia",
        date: "2026-09-10",
        pickupTime: "15:00",
        withdrawn: false,
        informedAt: "2026-09-09T10:00:00Z",
      },
      {
        requestId: "6",
        studentId: "8",
        studentFirstName: "Ida",
        date: "2026-09-11",
        pickupTime: "",
        withdrawn: true,
        informedAt: "2026-09-09T11:00:00Z",
      },
    ]);
    expect(sessionFetch).toHaveBeenCalledWith(
      "/api/students/pickup-change-notices",
      expect.anything(),
    );
  });

  it("sends the observed requests and exact timestamps", async () => {
    sessionFetch.mockResolvedValueOnce(Response.json({ data: [] }));
    const informedAt = "2026-09-09T10:00:00.123456Z";
    await expect(
      pickupChangeNoticesService.markRead("42", [
        { requestId: "9007199254740993", informedAt },
      ]),
    ).resolves.toEqual([]);

    expect(sessionFetch).toHaveBeenCalledWith(
      "/api/students/42/pickup-change-notices/read",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          notices: [
            { request_id: "9007199254740993", informed_at: informedAt },
          ],
        }),
      }),
    );
  });
});

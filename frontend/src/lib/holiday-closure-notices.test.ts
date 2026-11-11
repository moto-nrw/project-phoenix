import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  fetchHolidayClosureNotices,
  notifyHolidayClosureNotice,
} from "./holiday-closure-notices";
import {
  deleteTraegerClosingDay,
  fetchPendingHolidayClosures,
  notifyTraegerHolidayClosureNotice,
  previewTraegerClosingDayHolidayBookings,
} from "./traeger-closing-days";

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

const backendNotice = {
  id: 7,
  school_id: 3,
  closing_day_id: 11,
  reason: "Teamtag",
  previous: { start_date: "2026-10-05", end_date: "2026-10-05" },
  current: null,
  revision: 4,
  token: "impact-preview",
  status: "suggested",
  changed: false,
  can_notify: true,
  families: 2,
  confirmed_at: null,
  updated_at: "2026-09-08T10:00:00Z",
};

describe("holiday closure notices (ADR 0196)", () => {
  let fetchMock: ReturnType<typeof vi.fn>;
  beforeEach(() => {
    fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
  });
  afterEach(() => vi.unstubAllGlobals());

  it("maps the school's notices with string IDs and nullable ranges", async () => {
    fetchMock.mockResolvedValueOnce(json({ data: [backendNotice] }));

    await expect(fetchHolidayClosureNotices()).resolves.toEqual([
      {
        id: "7",
        schoolId: "3",
        schoolName: "",
        closingDayId: "11",
        reason: "Teamtag",
        previous: { startDate: "2026-10-05", endDate: "2026-10-05" },
        current: null,
        revision: 4,
        token: "impact-preview",
        status: "suggested",
        changed: false,
        canNotify: true,
        families: 2,
        confirmedAt: null,
        updatedAt: "2026-09-08T10:00:00Z",
      },
    ]);
    expect(fetchMock.mock.calls[0]![0]).toBe("/api/holiday-closure-notices");
  });

  it("sends the revision the school and the Büro saw", async () => {
    fetchMock.mockImplementation(() =>
      Promise.resolve(json({ data: backendNotice })),
    );

    await notifyHolidayClosureNotice("7", 4, "impact-preview");
    await notifyTraegerHolidayClosureNotice("7", 4, "impact-preview");

    expect(fetchMock.mock.calls.map(([url]) => url)).toEqual([
      "/api/holiday-closure-notices/7/notify",
      "/api/traeger/holiday-closures/pending/7/notify",
    ]);
    for (const [, init] of fetchMock.mock.calls) {
      expect(init).toMatchObject({ method: "POST" });
      expect(JSON.parse((init as RequestInit).body as string)).toEqual({
        revision: 4,
        token: "impact-preview",
      });
    }
  });

  it("maps the Büro's notices with their weeks and reopened days", async () => {
    fetchMock.mockResolvedValueOnce(
      json({
        data: {
          weeks: [],
          children: 1,
          families: 1,
          token: "t",
          keeps_contribution: true,
          notices: [
            {
              ...backendNotice,
              children: 1,
              weeks: [
                {
                  week_id: 5,
                  period_name: "Herbstferien",
                  starts_on: "2026-10-05",
                  ends_on: "2026-10-09",
                  host_school_id: 3,
                  closed_days: null,
                  reopened_days: [{ date: "2026-10-05", reason: "Teamtag" }],
                  children: 1,
                  waitlisted: 0,
                  families: 1,
                },
              ],
            },
          ],
        },
      }),
    );

    const pending = await fetchPendingHolidayClosures();
    expect(pending.notices[0]).toMatchObject({
      id: "7",
      children: 1,
      weeks: [
        {
          weekId: "5",
          hostSchoolId: "3",
          closedDays: [],
          reopenedDays: [{ date: "2026-10-05", reason: "Teamtag" }],
        },
      ],
    });
  });

  it("previews a deletion with the stored closing day and deletes with its token (#6457)", async () => {
    fetchMock
      .mockResolvedValueOnce(json({ data: { weeks: [], token: "d1" } }))
      .mockResolvedValueOnce(json({ data: { notified_families: 2 } }));
    const day = {
      id: "9",
      startDate: "2026-10-05",
      endDate: "2026-10-06",
      reason: "Teamtag",
      schoolIds: ["3"],
    };

    await previewTraegerClosingDayHolidayBookings(
      { ...day, reason: "" },
      { closingDay: day, delete: true },
    );
    await expect(deleteTraegerClosingDay("9", "d1")).resolves.toBe(2);

    expect(
      JSON.parse((fetchMock.mock.calls[0]![1] as RequestInit).body as string),
    ).toMatchObject({
      closing_day_id: 9,
      previous: {
        school_ids: [3],
        start_date: "2026-10-05",
        end_date: "2026-10-06",
      },
      delete: true,
    });
    expect(fetchMock.mock.calls[1]![0]).toBe(
      "/api/traeger/closing-days/9?holiday_bookings_token=d1",
    );
  });
});

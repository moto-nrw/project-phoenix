import { afterEach, describe, expect, it, vi } from "vitest";
import {
  describeAdjustment,
  formatWeekdayAdverbs,
  mapRosterImpact,
  removeStudentTemplateDays,
} from "./roster-impact";

describe("mapRosterImpact", () => {
  it("maps both lists and defaults missing weekday arrays", () => {
    expect(
      mapRosterImpact({
        effective_from: "2027-05-31",
        adjusted: [
          { template_id: "7", name: "Kurzbetreuung", removed_weekdays: [5] },
        ],
        not_adjusted: [
          {
            template_id: "9",
            name: "RDST 1b",
            weekdays: [5],
            via_target: true,
          },
        ],
      }),
    ).toEqual({
      effectiveFrom: "2027-05-31",
      adjusted: [
        {
          templateId: "7",
          name: "Kurzbetreuung",
          removedWeekdays: [5],
          addedWeekdays: [],
        },
      ],
      notAdjusted: [
        { templateId: "9", name: "RDST 1b", weekdays: [5], viaTarget: true },
      ],
    });
  });

  it("returns null when the response carries no impact", () => {
    expect(mapRosterImpact(undefined)).toBeNull();
  });
});

describe("weekday texts", () => {
  it("joins weekdays in week order", () => {
    expect(formatWeekdayAdverbs([5])).toBe("freitags");
    expect(formatWeekdayAdverbs([5, 1])).toBe("montags und freitags");
    expect(formatWeekdayAdverbs([3, 1, 5])).toBe(
      "montags, mittwochs und freitags",
    );
  });

  it("names removed and added weekdays of an adjusted Regeltermin", () => {
    expect(
      describeAdjustment({
        templateId: "1",
        name: "RDST",
        removedWeekdays: [5],
        addedWeekdays: [2],
      }),
    ).toBe("freitags entfernt, dienstags ergänzt");
  });
});

describe("removeStudentTemplateDays", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("posts the weekdays and the Stichtag to the template", async () => {
    const fetchMock = vi.fn(
      async () => new Response(JSON.stringify({ data: {} }), { status: 200 }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await removeStudentTemplateDays({
      templateId: "9",
      studentId: "42",
      weekdays: [5],
      effectiveFrom: "2027-05-31",
    });

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/timetable/templates/9/students/42/remove-days",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ weekdays: [5], effective_from: "2027-05-31" }),
      }),
    );
  });

  it("throws the backend error", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(
        async () =>
          new Response(
            JSON.stringify({
              status: 403,
              code: "general.permission",
              detail: "no",
            }),
            { status: 403 },
          ),
      ),
    );

    await expect(
      removeStudentTemplateDays({
        templateId: "9",
        studentId: "42",
        weekdays: [5],
        effectiveFrom: "2027-05-31",
      }),
    ).rejects.toMatchObject({ status: 403 });
  });
});

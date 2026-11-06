import { describe, it, expect, vi, beforeEach } from "vitest";
import { ApiError } from "./api-error";
import {
  ALL_COMPANION_WEEKDAYS,
  COMPANION_WEEKDAYS,
  companionDisplayName,
  fetchStudentCompanions,
  formatCompanionWeekdays,
} from "./student-companion-api";

const { mockSessionFetch } = vi.hoisted(() => ({
  mockSessionFetch:
    vi.fn<(url: string, init?: RequestInit) => Promise<Response>>(),
}));

vi.mock("./session-cache", () => ({ sessionFetch: mockSessionFetch }));

describe("student-companion-api", () => {
  beforeEach(() => {
    mockSessionFetch.mockReset();
  });

  describe("COMPANION_WEEKDAYS", () => {
    it("exposes the 5 schooldays Monday-Friday", () => {
      expect(COMPANION_WEEKDAYS).toHaveLength(5);
      expect(COMPANION_WEEKDAYS[0]).toEqual({
        value: "mon",
        label: "Montag",
        short: "Mo",
      });
      expect(COMPANION_WEEKDAYS.at(-1)).toEqual({
        value: "fri",
        label: "Freitag",
        short: "Fr",
      });
    });

    it("ALL_COMPANION_WEEKDAYS lists the keys in weekday order", () => {
      expect(ALL_COMPANION_WEEKDAYS).toEqual([
        "mon",
        "tue",
        "wed",
        "thu",
        "fri",
      ]);
    });
  });

  describe("fetchStudentCompanions", () => {
    it("fetches the companion list and maps ids to strings", async () => {
      mockSessionFetch.mockResolvedValueOnce(
        Response.json({
          success: true,
          data: [
            {
              // The backend serializes the int64 id as a JSON number …
              companion_student_id: 7,
              first_name: "Lina",
              last_name: "Klein",
              weekdays: ["mon", "tue"],
            },
            { companion_student_id: 8 },
          ],
        }),
      );

      const result = await fetchStudentCompanions("42");

      expect(mockSessionFetch).toHaveBeenCalledWith(
        "/api/students/42/companions",
        expect.objectContaining({ method: "GET" }),
      );
      // … and the client maps it to a string like every other backend id.
      expect(result).toEqual([
        {
          companion_student_id: "7",
          first_name: "Lina",
          last_name: "Klein",
          weekdays: ["mon", "tue"],
        },
        {
          companion_student_id: "8",
          first_name: undefined,
          last_name: undefined,
          weekdays: [],
        },
      ]);
    });

    it("returns an empty array for a null list or a 204", async () => {
      mockSessionFetch
        .mockResolvedValueOnce(Response.json({ success: true, data: null }))
        .mockResolvedValueOnce(new Response(null, { status: 204 }));

      await expect(fetchStudentCompanions("42")).resolves.toEqual([]);
      await expect(fetchStudentCompanions("42")).resolves.toEqual([]);
    });
  });

  describe("error handling", () => {
    it("keeps the backend diagnostic out of the message", async () => {
      mockSessionFetch.mockResolvedValueOnce(
        Response.json(
          {
            status: 400,
            detail: "Ein Kind kann nicht mit sich selbst laufen.",
          },
          { status: 400 },
        ),
      );

      const error = await fetchStudentCompanions("42").catch(
        (err: unknown) => err,
      );
      expect(error).toMatchObject({
        status: 400,
        detail: "Ein Kind kann nicht mit sich selbst laufen.",
      });
      expect((error as Error).message).not.toContain("selbst");
    });

    it("throws an ApiError with code and request ID for the shared error path", async () => {
      mockSessionFetch.mockResolvedValueOnce(
        Response.json(
          {
            detail: "unavailable",
            code: "general.unavailable",
            instance: "req-companions",
          },
          { status: 503 },
        ),
      );

      const error = await fetchStudentCompanions("42").catch(
        (err: unknown) => err,
      );
      expect(error).toBeInstanceOf(ApiError);
      expect(error).toMatchObject({
        status: 503,
        code: "general.unavailable",
        requestId: "req-companions",
      });
    });

    it("classifies a body that is not JSON by its status", async () => {
      mockSessionFetch.mockResolvedValueOnce(
        new Response("server exploded", { status: 500 }),
      );

      await expect(fetchStudentCompanions("42")).rejects.toMatchObject({
        status: 500,
        code: "general.server",
      });
    });
  });

  describe("companionDisplayName", () => {
    it("joins first and last name", () => {
      expect(
        companionDisplayName({
          companion_student_id: "7",
          first_name: "Lina",
          last_name: "Klein",
          weekdays: [],
        }),
      ).toBe("Lina Klein");
    });

    it("uses whichever name part exists", () => {
      expect(
        companionDisplayName({
          companion_student_id: "7",
          first_name: "Lina",
          weekdays: [],
        }),
      ).toBe("Lina");
    });

    it("falls back to the id when no name is present", () => {
      expect(
        companionDisplayName({
          companion_student_id: "7",
          weekdays: [],
        }),
      ).toBe("Kind #7");
    });

    it("falls back to the id when the names are blank", () => {
      expect(
        companionDisplayName({
          companion_student_id: "9",
          first_name: "",
          last_name: "  ",
          weekdays: [],
        }),
      ).toBe("Kind #9");
    });
  });

  describe("formatCompanionWeekdays", () => {
    it("returns 'kein Tag' for an empty selection", () => {
      expect(formatCompanionWeekdays([])).toBe("kein Tag");
    });

    it("collapses the full week", () => {
      expect(formatCompanionWeekdays([...ALL_COMPANION_WEEKDAYS])).toBe(
        "Mo bis Fr",
      );
    });

    it("lists single days in weekday order regardless of input order", () => {
      expect(formatCompanionWeekdays(["wed", "mon"])).toBe("Mo, Mi");
    });

    it("renders one day on its own", () => {
      expect(formatCompanionWeekdays(["thu"])).toBe("Do");
    });
  });
});

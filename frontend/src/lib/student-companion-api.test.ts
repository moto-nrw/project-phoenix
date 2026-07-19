import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  ALL_COMPANION_WEEKDAYS,
  COMPANION_WEEKDAYS,
  companionDisplayName,
  fetchStudentCompanions,
  formatCompanionWeekdays,
  updateStudentCompanions,
} from "./student-companion-api";
import type { StudentCompanionInput } from "./student-companion-api";

const { mockGetSession } = vi.hoisted(() => ({
  mockGetSession: vi.fn(),
}));

vi.mock("next-auth/react", () => ({
  getSession: mockGetSession,
}));

function mockFetchResponse(
  data: unknown,
  options: { ok?: boolean; status?: number; text?: string } = {},
): Response {
  const ok = options.ok ?? true;
  const status = options.status ?? 200;
  return {
    ok,
    status,
    json: () => Promise.resolve(data),
    text: () => Promise.resolve(options.text ?? ""),
  } as unknown as Response;
}

describe("student-companion-api", () => {
  const fetchSpy = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    globalThis.fetch = fetchSpy as unknown as typeof fetch;
    mockGetSession.mockResolvedValue({
      user: { token: "test-token" },
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
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

  describe("auth headers", () => {
    it("sends a bearer token when the session has one", async () => {
      fetchSpy.mockResolvedValueOnce(mockFetchResponse({ data: [] }));

      await fetchStudentCompanions("42");

      const headers = (fetchSpy.mock.calls[0]![1] as RequestInit)
        .headers as Record<string, string>;
      expect(headers.Authorization).toBe("Bearer test-token");
      expect(headers["Content-Type"]).toBe("application/json");
    });

    it("omits Authorization when session has no token", async () => {
      mockGetSession.mockResolvedValueOnce({ user: {} });
      fetchSpy.mockResolvedValueOnce(mockFetchResponse({ data: [] }));

      await fetchStudentCompanions("42");

      const headers = (fetchSpy.mock.calls[0]![1] as RequestInit)
        .headers as Record<string, string>;
      expect(headers.Authorization).toBeUndefined();
      expect(headers["Content-Type"]).toBe("application/json");
    });

    it("omits Authorization when session is null", async () => {
      mockGetSession.mockResolvedValueOnce(null);
      fetchSpy.mockResolvedValueOnce(mockFetchResponse({ data: [] }));

      await fetchStudentCompanions("42");

      const headers = (fetchSpy.mock.calls[0]![1] as RequestInit)
        .headers as Record<string, string>;
      expect(headers.Authorization).toBeUndefined();
    });
  });

  describe("fetchStudentCompanions", () => {
    it("fetches and unwraps the companion list", async () => {
      fetchSpy.mockResolvedValueOnce(
        mockFetchResponse({
          data: [
            {
              companion_student_id: 7,
              first_name: "Lina",
              last_name: "Klein",
              weekdays: ["mon", "tue"],
            },
          ],
        }),
      );

      const result = await fetchStudentCompanions("42");

      expect(fetchSpy).toHaveBeenCalledWith(
        "/api/students/42/companions",
        expect.objectContaining({
          method: "GET",
          credentials: "include",
        }),
      );
      expect(result).toHaveLength(1);
      expect(result[0]!.companion_student_id).toBe(7);
      expect(result[0]!.weekdays).toEqual(["mon", "tue"]);
    });

    it("handles unwrapped (no .data wrapper) payloads", async () => {
      const payload = [{ companion_student_id: 3, weekdays: ["fri"] as const }];
      fetchSpy.mockResolvedValueOnce(mockFetchResponse(payload));

      const result = await fetchStudentCompanions("42");

      expect(result).toEqual(payload);
    });

    it("returns an empty array when the payload is null", async () => {
      fetchSpy.mockResolvedValueOnce(mockFetchResponse({ data: null }));

      await expect(fetchStudentCompanions("42")).resolves.toEqual([]);
    });

    it("returns an empty array on 204", async () => {
      fetchSpy.mockResolvedValueOnce(
        mockFetchResponse(null, { ok: true, status: 204 }),
      );

      await expect(fetchStudentCompanions("42")).resolves.toEqual([]);
    });
  });

  describe("error handling", () => {
    it("surfaces the German error field from the backend envelope", async () => {
      fetchSpy.mockResolvedValueOnce(
        mockFetchResponse(null, {
          ok: false,
          status: 400,
          text: JSON.stringify({
            status: "Invalid request",
            error: "Ein Kind kann nicht mit sich selbst laufen.",
          }),
        }),
      );

      await expect(fetchStudentCompanions("42")).rejects.toThrow(
        "Ein Kind kann nicht mit sich selbst laufen.",
      );
    });

    it("falls back to the status field when error is missing", async () => {
      fetchSpy.mockResolvedValueOnce(
        mockFetchResponse(null, {
          ok: false,
          status: 404,
          text: JSON.stringify({ status: "Kind nicht gefunden" }),
        }),
      );

      await expect(fetchStudentCompanions("42")).rejects.toThrow(
        "Kind nicht gefunden",
      );
    });

    it("uses the raw body when it is not JSON", async () => {
      fetchSpy.mockResolvedValueOnce(
        mockFetchResponse(null, {
          ok: false,
          status: 500,
          text: "server exploded",
        }),
      );

      await expect(fetchStudentCompanions("42")).rejects.toThrow(
        "server exploded",
      );
    });

    it("falls back to the status code when there is no body", async () => {
      fetchSpy.mockResolvedValueOnce(
        mockFetchResponse(null, { ok: false, status: 403, text: "" }),
      );

      await expect(fetchStudentCompanions("42")).rejects.toThrow(
        "Anfrage fehlgeschlagen (403)",
      );
    });

    it("falls back to the status code when reading the body fails", async () => {
      fetchSpy.mockResolvedValueOnce({
        ok: false,
        status: 502,
        text: () => Promise.reject(new Error("stream error")),
      } as unknown as Response);

      await expect(fetchStudentCompanions("42")).rejects.toThrow(
        "Anfrage fehlgeschlagen (502)",
      );
    });
  });

  describe("updateStudentCompanions", () => {
    it("sends PUT with the companions body", async () => {
      fetchSpy.mockResolvedValueOnce(mockFetchResponse({ data: [] }));

      const companions: StudentCompanionInput[] = [
        { companion_student_id: 7, weekdays: ["mon", "fri"] },
      ];
      await updateStudentCompanions("42", companions);

      expect(fetchSpy).toHaveBeenCalledWith(
        "/api/students/42/companions",
        expect.objectContaining({
          method: "PUT",
          credentials: "include",
          body: JSON.stringify({ companions }),
        }),
      );
    });

    it("sends an empty list to clear every link", async () => {
      fetchSpy.mockResolvedValueOnce(mockFetchResponse({ data: [] }));

      const result = await updateStudentCompanions("42", []);

      const body = (fetchSpy.mock.calls[0]![1] as RequestInit).body as string;
      expect(JSON.parse(body)).toEqual({ companions: [] });
      expect(result).toEqual([]);
    });

    it("returns the saved list from the response", async () => {
      fetchSpy.mockResolvedValueOnce(
        mockFetchResponse({
          data: [
            {
              companion_student_id: 7,
              first_name: "Lina",
              last_name: "Klein",
              weekdays: ["mon"],
            },
          ],
        }),
      );

      const result = await updateStudentCompanions("42", [
        { companion_student_id: 7, weekdays: ["mon"] },
      ]);

      expect(result[0]!.first_name).toBe("Lina");
    });

    it("throws the backend validation message", async () => {
      fetchSpy.mockResolvedValueOnce(
        mockFetchResponse(null, {
          ok: false,
          status: 400,
          text: JSON.stringify({
            error: "Bitte mindestens einen Wochentag auswählen.",
          }),
        }),
      );

      await expect(
        updateStudentCompanions("42", [
          { companion_student_id: 7, weekdays: [] },
        ]),
      ).rejects.toThrow("Bitte mindestens einen Wochentag auswählen.");
    });
  });

  describe("companionDisplayName", () => {
    it("joins first and last name", () => {
      expect(
        companionDisplayName({
          companion_student_id: 7,
          first_name: "Lina",
          last_name: "Klein",
          weekdays: [],
        }),
      ).toBe("Lina Klein");
    });

    it("uses whichever name part exists", () => {
      expect(
        companionDisplayName({
          companion_student_id: 7,
          first_name: "Lina",
          weekdays: [],
        }),
      ).toBe("Lina");
    });

    it("falls back to the id when no name is present", () => {
      expect(
        companionDisplayName({
          companion_student_id: 7,
          weekdays: [],
        }),
      ).toBe("Kind #7");
    });

    it("falls back to the id when the names are blank", () => {
      expect(
        companionDisplayName({
          companion_student_id: 9,
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

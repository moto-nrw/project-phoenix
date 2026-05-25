import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  createStudentStatusDays,
  deleteStudentStatusDay,
  fetchStudentStatusDays,
} from "./student-status-days-api";

const backendDay = {
  id: 7,
  student_id: 42,
  date: "2026-05-26",
  status: "excused" as const,
  label: "Entschuldigt",
  reported_at: "2026-05-25T08:00:00Z",
  cleared_at: null,
  source: "planned",
  created_at: "2026-05-25T08:00:00Z",
  updated_at: "2026-05-25T08:00:00Z",
};

describe("student-status-days-api", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("maps fetched backend rows to frontend ids", async () => {
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(
        Response.json({ status: "success", data: [backendDay] }),
      );

    const days = await fetchStudentStatusDays("42", "2026-05-25", "2026-05-29");

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/students/42/status-days?from=2026-05-25&to=2026-05-29",
    );
    expect(days).toEqual([
      expect.objectContaining({
        id: "7",
        student_id: "42",
        status: "excused",
      }),
    ]);
  });

  it("posts status and dates when creating planned days", async () => {
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(
        Response.json({ status: "success", data: [backendDay] }),
      );

    const days = await createStudentStatusDays("42", "sick", [
      "2026-05-26",
      "2026-05-27",
    ]);

    expect(fetchMock).toHaveBeenCalledWith("/api/students/42/status-days", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        status: "sick",
        dates: ["2026-05-26", "2026-05-27"],
      }),
    });
    expect(days[0]?.id).toBe("7");
  });

  it("deletes a planned status day", async () => {
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(null, { status: 204 }));

    await deleteStudentStatusDay("42", "7");

    expect(fetchMock).toHaveBeenCalledWith("/api/students/42/status-days/7", {
      method: "DELETE",
    });
  });

  it("throws backend errors from successful HTTP responses", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      Response.json({ status: "error", error: "Bereits geplant" }),
    );

    await expect(
      fetchStudentStatusDays("42", "2026-05-25", "2026-05-29"),
    ).rejects.toThrow("Bereits geplant");
  });

  it("throws fallback messages for failed HTTP responses", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      new Response("nope", { status: 500 }),
    );

    await expect(createStudentStatusDays("42", "sick", [])).rejects.toThrow(
      "Geplante Einträge konnten nicht gespeichert werden",
    );
  });

  it("throws fallback messages when fetch or delete fail", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      new Response("nope", { status: 500 }),
    );

    await expect(
      fetchStudentStatusDays("42", "2026-05-25", "2026-05-29"),
    ).rejects.toThrow("Geplante Einträge konnten nicht geladen werden");

    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      new Response("nope", { status: 500 }),
    );

    await expect(deleteStudentStatusDay("42", "7")).rejects.toThrow(
      "Geplanter Eintrag konnte nicht gelöscht werden",
    );
  });
});

import { beforeEach, describe, expect, it, vi } from "vitest";

const sessionFetch = vi.hoisted(() => vi.fn());
vi.mock("./session-cache", () => ({ sessionFetch }));

import { ApiError } from "./api-error";
import { studentNotesService } from "./student-notes-api";

describe("studentNotesService errors", () => {
  beforeEach(() => vi.clearAllMocks());

  it("uses a German message when loading notes fails", async () => {
    sessionFetch.mockResolvedValueOnce(
      new Response(null, { status: 500, statusText: "Internal Server Error" }),
    );

    const error = await studentNotesService.list("42").catch((e) => e);
    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({
      message: "Notizen konnten nicht geladen werden.",
      code: "general.server",
    });
  });

  it("does not expose a backend validation error when saving a note", async () => {
    sessionFetch.mockResolvedValueOnce(
      Response.json(
        {
          error: "invalid student note",
          code: "general.input",
          errors: [{ field: "body", reason: "too long" }],
        },
        { status: 400 },
      ),
    );

    await expect(
      studentNotesService.create("42", {
        kind: "journal",
        visibility: "all_staff",
        body: "Kurzes Gespräch.",
        subjectDate: "2026-09-09",
      }),
    ).rejects.toMatchObject({
      message: "Notiz konnte nicht gespeichert werden.",
      code: "general.input",
      errors: [{ field: "body", reason: "too long" }],
    });
  });
});

describe("studentNotesService activities", () => {
  beforeEach(() => vi.clearAllMocks());

  it("reads the child's activities for the note reference", async () => {
    sessionFetch.mockResolvedValueOnce(
      Response.json({
        data: {
          activities: [
            { id: "12", name: "Fußball-AG" },
            { id: "15", name: "Schach" },
          ],
        },
      }),
    );

    await expect(studentNotesService.listActivities("42")).resolves.toEqual([
      { id: "12", name: "Fußball-AG" },
      { id: "15", name: "Schach" },
    ]);
    expect(sessionFetch).toHaveBeenCalledWith("/api/students/42/note-subjects");
  });

  it("uses a German message when the activities cannot be read", async () => {
    sessionFetch.mockResolvedValueOnce(
      new Response(null, { status: 500, statusText: "Internal Server Error" }),
    );

    await expect(
      studentNotesService.listActivities("42"),
    ).rejects.toMatchObject({
      message: "Angebote konnten nicht geladen werden.",
    });
  });

  it("sends the chosen activity as the note's reference", async () => {
    sessionFetch.mockResolvedValueOnce(Response.json({ data: {} }));

    await studentNotesService.create("42", {
      kind: "journal",
      visibility: "group_leads",
      body: "Hat beim Training gefehlt.",
      subjectDate: "2026-09-09",
      activityGroupId: "12",
    });

    const [, init] = sessionFetch.mock.calls[0] as [string, RequestInit];
    expect(JSON.parse(init.body as string)).toMatchObject({
      activity_group_id: "12",
      education_group_id: null,
      subject_date: "2026-09-09",
    });
  });
});

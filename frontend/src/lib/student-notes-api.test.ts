import { beforeEach, describe, expect, it, vi } from "vitest";

const sessionFetch = vi.hoisted(() => vi.fn());
vi.mock("./session-cache", () => ({ sessionFetch }));

import { studentNotesService } from "./student-notes-api";

describe("studentNotesService errors", () => {
  beforeEach(() => vi.clearAllMocks());

  it("uses a German message when loading notes fails", async () => {
    sessionFetch.mockResolvedValueOnce(
      new Response(null, { status: 500, statusText: "Internal Server Error" }),
    );

    await expect(studentNotesService.list("42")).rejects.toThrow(
      "Notizen konnten nicht geladen werden.",
    );
  });

  it("does not expose a backend validation error when saving a note", async () => {
    sessionFetch.mockResolvedValueOnce(
      Response.json({ error: "invalid student note" }, { status: 400 }),
    );

    await expect(
      studentNotesService.create("42", {
        kind: "journal",
        visibility: "all_staff",
        body: "Kurzes Gespräch.",
        subjectDate: "2026-09-09",
      }),
    ).rejects.toThrow("Notiz konnte nicht gespeichert werden.");
  });
});

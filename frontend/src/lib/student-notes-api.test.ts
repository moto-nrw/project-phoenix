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

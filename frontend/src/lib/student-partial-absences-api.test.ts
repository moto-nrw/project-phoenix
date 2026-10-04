import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "./api-error";
import {
  deleteStudentPartialAbsence,
  fetchStudentPartialAbsences,
  saveStudentPartialAbsence,
} from "./student-partial-absences-api";

const backendRow = {
  id: 9,
  student_id: 42,
  date: "2026-05-27",
  from_time: "13:30",
  reason: "Arzttermin",
  pickup_time: "13:30",
  created_by: 5,
  created_at: "2026-05-20T08:00:00Z",
  updated_at: "2026-05-20T08:00:00Z",
};

describe("student-partial-absences-api", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("maps backend IDs and fields when listing", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(
        new Response(JSON.stringify({ status: "success", data: [backendRow] })),
      );
    vi.stubGlobal("fetch", fetchMock);

    const result = await fetchStudentPartialAbsences(
      "42",
      "2026-05-01",
      "2026-05-31",
    );

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/students/42/partial-absences?from=2026-05-01&to=2026-05-31",
    );
    expect(result[0]).toMatchObject({
      id: "9",
      studentId: "42",
      fromTime: "13:30",
      createdBy: "5",
    });
  });

  it("uses POST for create and PUT for edit", async () => {
    const fetchMock = vi
      .fn()
      .mockImplementation(() =>
        Promise.resolve(
          new Response(JSON.stringify({ status: "success", data: backendRow })),
        ),
      );
    vi.stubGlobal("fetch", fetchMock);

    await saveStudentPartialAbsence(
      "42",
      null,
      "2026-05-27",
      "13:30",
      "Arzttermin",
    );
    await saveStudentPartialAbsence("42", "9", "2026-05-27", "14:00");

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/students/42/partial-absences",
      expect.objectContaining({ method: "POST" }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/students/42/partial-absences/9",
      expect.objectContaining({ method: "PUT" }),
    );
  });

  it("deletes through the dedicated endpoint", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null));
    vi.stubGlobal("fetch", fetchMock);

    await deleteStudentPartialAbsence("42", "9");

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/students/42/partial-absences/9",
      { method: "DELETE" },
    );
  });

  it("throws an ApiError with code and field errors when saving fails", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        Response.json(
          {
            status: "error",
            error: "from_time invalid",
            code: "general.input",
            errors: [{ field: "from_time", reason: "invalid" }],
          },
          { status: 400 },
        ),
      ),
    );

    const error = await saveStudentPartialAbsence(
      "42",
      null,
      "2026-05-27",
      "25:00",
    ).catch((caught: unknown) => caught);

    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).code).toBe("general.input");
    expect((error as ApiError).errors).toEqual([
      { field: "from_time", reason: "invalid" },
    ]);
  });

  it("classifies a non-JSON failure by its status", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(new Response("<html>", { status: 502 })),
    );

    const error = await fetchStudentPartialAbsences(
      "42",
      "2026-05-25",
      "2026-05-29",
    ).catch((caught: unknown) => caught);

    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).code).toBe("general.unavailable");
  });
});

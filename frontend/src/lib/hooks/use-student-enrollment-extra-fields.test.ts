import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useStudentEnrollmentExtraFields } from "./use-student-enrollment-extra-fields";

const { fetchStudentEnrollmentExtraFieldsMock, loggerWarnMock } = vi.hoisted(
  () => ({
    fetchStudentEnrollmentExtraFieldsMock: vi.fn(),
    loggerWarnMock: vi.fn(),
  }),
);

vi.mock("~/lib/student-api", () => ({
  fetchStudentEnrollmentExtraFields: fetchStudentEnrollmentExtraFieldsMock,
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({ warn: loggerWarnMock }),
}));

const groups = [
  {
    request_id: "77",
    phase_name: "Anmeldung 2026",
    submitted_at: "2026-06-01T12:00:00Z",
    fields: [
      {
        key: "can_swim",
        label: "Kann Schwimmen?",
        type: "boolean",
        value: true,
      },
    ],
  },
];

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

describe("useStudentEnrollmentExtraFields", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    fetchStudentEnrollmentExtraFieldsMock.mockResolvedValue([]);
  });

  it("loads enrollment extra field groups for full-access students", async () => {
    fetchStudentEnrollmentExtraFieldsMock.mockResolvedValueOnce(groups);

    const { result } = renderHook(() =>
      useStudentEnrollmentExtraFields("123", true),
    );

    expect(result.current.loading).toBe(true);

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(fetchStudentEnrollmentExtraFieldsMock).toHaveBeenCalledWith("123");
    expect(result.current.groups).toEqual(groups);
    expect(result.current.hasError).toBe(false);
  });

  it("clears state and skips loading without full access", async () => {
    const { result } = renderHook(() =>
      useStudentEnrollmentExtraFields("123", false),
    );

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(fetchStudentEnrollmentExtraFieldsMock).not.toHaveBeenCalled();
    expect(result.current.groups).toEqual([]);
    expect(result.current.hasError).toBe(false);
  });

  it("sets error state and clears groups when loading fails", async () => {
    fetchStudentEnrollmentExtraFieldsMock.mockRejectedValueOnce(
      new Error("server unavailable"),
    );

    const { result } = renderHook(() =>
      useStudentEnrollmentExtraFields("123", true),
    );

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.groups).toEqual([]);
    expect(result.current.hasError).toBe(true);
    expect(loggerWarnMock).toHaveBeenCalledWith(
      "student_enrollment_extra_fields_load_failed",
      expect.objectContaining({
        student_id: "123",
        error: "server unavailable",
      }),
    );
  });

  it("clears previous groups before loading another student", async () => {
    fetchStudentEnrollmentExtraFieldsMock.mockResolvedValueOnce(groups);
    fetchStudentEnrollmentExtraFieldsMock.mockImplementationOnce(
      () => new Promise(() => undefined),
    );

    const { result, rerender } = renderHook(
      ({ studentId }) => useStudentEnrollmentExtraFields(studentId, true),
      { initialProps: { studentId: "123" } },
    );

    await waitFor(() => expect(result.current.groups).toEqual(groups));

    rerender({ studentId: "456" });

    expect(result.current.groups).toEqual([]);
    expect(result.current.loading).toBe(true);
  });

  it("ignores successful responses after unmount", async () => {
    const request = deferred<typeof groups>();
    fetchStudentEnrollmentExtraFieldsMock.mockReturnValueOnce(request.promise);

    const { unmount } = renderHook(() =>
      useStudentEnrollmentExtraFields("123", true),
    );

    unmount();
    request.resolve(groups);

    await expect(request.promise).resolves.toEqual(groups);
    expect(loggerWarnMock).not.toHaveBeenCalled();
  });

  it("ignores failed responses after unmount", async () => {
    const request = deferred<typeof groups>();
    fetchStudentEnrollmentExtraFieldsMock.mockReturnValueOnce(request.promise);

    const { unmount } = renderHook(() =>
      useStudentEnrollmentExtraFields("123", true),
    );

    unmount();
    request.reject(new Error("late failure"));

    await expect(request.promise).rejects.toThrow("late failure");
    expect(loggerWarnMock).not.toHaveBeenCalled();
  });
});

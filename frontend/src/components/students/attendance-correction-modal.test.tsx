import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { AttendanceCorrectionModal } from "./attendance-correction-modal";

const { mockGetCachedSession } = vi.hoisted(() => ({
  mockGetCachedSession: vi.fn(),
}));

vi.mock("~/lib/session-cache", () => ({
  getCachedSession: mockGetCachedSession,
}));

describe("AttendanceCorrectionModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetCachedSession.mockResolvedValue({ user: { token: "test-token" } });
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({ data: { corrections: [] } }),
      }),
    );
  });

  it("clears the substatus when expected attendance is selected", async () => {
    render(
      <AttendanceCorrectionModal
        isOpen
        onClose={vi.fn()}
        studentId="student-1"
        slot={{
          instanceId: "instance-1",
          title: "Bastelstunde",
          date: "2026-09-07",
          startTime: "14:00",
          endTime: "15:00",
          status: "present",
          substatus: "late",
          note: null,
        }}
        onCorrected={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("combobox", { name: "Anwesenheit" }));
    fireEvent.click(screen.getByRole("option", { name: "Erwartet" }));
    fireEvent.change(screen.getByLabelText(/Grund der Korrektur/), {
      target: { value: "Status korrigieren" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/timetable/instances/instance-1/students/student-1/correction",
        expect.objectContaining({ method: "POST" }),
      );
    });

    const correctionCall = vi
      .mocked(global.fetch)
      .mock.calls.find(([, options]) => options?.method === "POST");
    expect(correctionCall).toBeDefined();
    expect(JSON.parse(String(correctionCall?.[1]?.body))).toEqual({
      reason: "Status korrigieren",
      status: "expected",
      substatus: null,
    });
  });
});

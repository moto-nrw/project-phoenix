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

  it("clears and disables substatus when expected attendance is selected", async () => {
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
    expect(screen.getByRole("combobox", { name: "Hinweis" })).toBeDisabled();
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

  it("sends an explicit substatus clear when attendance becomes expected", async () => {
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
          substatus: null,
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

  const slot = {
    instanceId: "instance-1",
    title: "Bastelstunde",
    date: "2026-09-07",
    startTime: "14:00",
    endTime: "15:00",
    status: "present",
    substatus: null,
    note: null,
  };

  it("asks for a reason at the field before sending", async () => {
    render(
      <AttendanceCorrectionModal
        isOpen
        onClose={vi.fn()}
        studentId="student-1"
        slot={slot}
        onCorrected={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(
        "Bitte geben Sie einen Grund für die Korrektur an.",
      ),
    ).toBeInTheDocument();
    const reason = screen.getByLabelText(/Grund der Korrektur/);
    await waitFor(() => expect(reason).toHaveFocus());
    expect(reason).toHaveAttribute("aria-invalid", "true");
    expect(
      vi
        .mocked(global.fetch)
        .mock.calls.some(([, options]) => options?.method === "POST"),
    ).toBe(false);
  });

  it("shows the catalog text for a refused correction and keeps the dialog open", async () => {
    vi.mocked(global.fetch).mockImplementation(async (_url, options) =>
      options?.method === "POST"
        ? Response.json(
            {
              status: "error",
              error: "instance not completed",
              code: "general.business_rejection",
            },
            { status: 409 },
          )
        : Response.json({ data: { corrections: [] } }),
    );
    const onClose = vi.fn();
    render(
      <AttendanceCorrectionModal
        isOpen
        onClose={onClose}
        studentId="student-1"
        slot={slot}
        onCorrected={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("combobox", { name: "Anwesenheit" }));
    fireEvent.click(screen.getByRole("option", { name: "Abwesend" }));
    fireEvent.change(screen.getByLabelText(/Grund der Korrektur/), {
      target: { value: "Kind war krank" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(
        "Die Korrektur konnte nicht geändert werden. Bitte prüfen Sie den aktuellen Stand.",
      ),
    ).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
  });
});

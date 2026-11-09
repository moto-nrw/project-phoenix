import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import type { StaffHistorySession } from "~/lib/staff-api";
import { setTestClock } from "~/test/clock";
import { catalogText } from "~/test/error-catalog-text";

import { AdminSessionEditModal } from "./admin-session-edit-modal";

vi.mock("~/components/ui/modal", () => ({
  Modal: ({
    isOpen,
    title,
    children,
    footer,
  }: {
    isOpen: boolean;
    title: string;
    children: React.ReactNode;
    footer?: React.ReactNode;
  }) =>
    isOpen ? (
      <div role="dialog" aria-label={title}>
        {children}
        {footer}
      </div>
    ) : null,
}));

const mocks = vi.hoisted(() => ({
  createSession: vi.fn(),
  updateSession: vi.fn(),
}));

vi.mock("~/lib/staff-api", () => ({
  staffSessionService: {
    createSession: mocks.createSession,
    updateSession: mocks.updateSession,
  },
}));

function renderBackfill() {
  const onClose = vi.fn();
  const onSaved = vi.fn();
  render(
    <AdminSessionEditModal
      isOpen
      mode="nachtragen"
      staffId="7"
      date={new Date(2026, 8, 7)}
      session={null}
      onClose={onClose}
      onSaved={onSaved}
    />,
  );
  fireEvent.change(
    screen.getByPlaceholderText(
      "z. B. Mitarbeiter hatte vergessen einzustempeln",
    ),
    { target: { value: "Vergessen" } },
  );
  fireEvent.click(screen.getByRole("button", { name: "Eintrag anlegen" }));
  return { onClose, onSaved };
}

describe("AdminSessionEditModal", () => {
  beforeEach(() => {
    mocks.createSession.mockReset();
    mocks.updateSession.mockReset();
  });

  it("explains an overlapping block in the dialog and keeps it open", async () => {
    mocks.createSession.mockRejectedValueOnce(
      new ApiError("work session overlaps an existing block", 409, {
        code: "workforce.work_session_overlap",
        errors: [{ field: "check_in_time", reason: "overlap" }],
      }),
    );
    const { onClose, onSaved } = renderBackfill();

    expect(
      await screen.findByText(
        catalogText("workforce.work_session_overlap", "die Arbeitszeit"),
      ),
    ).toBeInTheDocument();
    await waitFor(() =>
      expect(
        document.querySelector('input[name="check_in_time"]'),
      ).toHaveAttribute("aria-invalid", "true"),
    );
    expect(onClose).not.toHaveBeenCalled();
    expect(onSaved).not.toHaveBeenCalled();
  });

  it("offers a retry for a server error", async () => {
    mocks.createSession
      .mockRejectedValueOnce(
        new ApiError("boom", 500, { code: "general.server", instance: "r1" }),
      )
      .mockResolvedValueOnce(undefined);
    const { onClose } = renderBackfill();

    await screen.findByText(catalogText("general.server", "die Arbeitszeit"));
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(mocks.createSession).toHaveBeenCalledTimes(2);
  });

  describe("edit across a daylight saving switch (#6293)", () => {
    // 20.10.2026 is summer time: 08:00 Berlin = 06:00 UTC. The stored end
    // carries seconds, which a resent prefill would cut off.
    const summerSession: StaffHistorySession = {
      id: "42",
      date: "2026-10-20",
      status: "present",
      net_minutes: 480,
      check_in_time: "2026-10-20T06:00:00Z",
      check_out_time: "2026-10-20T14:30:37Z",
      break_minutes: 30,
    };

    function renderEdit(session: StaffHistorySession) {
      render(
        <AdminSessionEditModal
          isOpen
          mode="edit"
          staffId="7"
          date={new Date(`${session.date}T00:00:00`)}
          session={session}
          onClose={vi.fn()}
          onSaved={vi.fn()}
        />,
      );
      fireEvent.change(
        screen.getByPlaceholderText(
          "z. B. Mitarbeiter hatte vergessen einzustempeln",
        ),
        { target: { value: "Zeitkorrektur" } },
      );
    }

    const checkInField = () =>
      document.querySelector<HTMLInputElement>('input[name="check_in_time"]')!;
    const checkOutField = () =>
      document.querySelector<HTMLInputElement>('input[name="check_out_time"]')!;

    it("sends no times when only the reason changes", async () => {
      setTestClock("2026-11-09T12:00:00+01:00");
      mocks.updateSession.mockResolvedValue(undefined);
      renderEdit(summerSession);
      expect(checkInField()).toHaveValue("08:00");
      expect(checkOutField()).toHaveValue("16:30");

      fireEvent.click(
        screen.getByRole("button", { name: "Änderungen speichern" }),
      );

      await waitFor(() => expect(mocks.updateSession).toHaveBeenCalledTimes(1));
      const payload = mocks.updateSession.mock.calls[0]![2];
      expect(payload.notes).toBe("Zeitkorrektur");
      expect(payload.check_in_time).toBeUndefined();
      expect(payload.check_out_time).toBeUndefined();
    });

    it("sends only the changed start, as Berlin time of the session day", async () => {
      setTestClock("2026-11-09T12:00:00+01:00");
      mocks.updateSession.mockResolvedValue(undefined);
      renderEdit(summerSession);
      fireEvent.change(checkInField(), { target: { value: "07:30" } });

      fireEvent.click(
        screen.getByRole("button", { name: "Änderungen speichern" }),
      );

      await waitFor(() => expect(mocks.updateSession).toHaveBeenCalledTimes(1));
      const payload = mocks.updateSession.mock.calls[0]![2];
      expect(payload.check_in_time).toBe("2026-10-20T05:30:00.000Z");
      expect(payload.check_out_time).toBeUndefined();
    });

    it("saves a reason for a block stamped inside the repeated hour", async () => {
      setTestClock("2026-11-09T12:00:00+01:00");
      mocks.updateSession.mockResolvedValue(undefined);
      // 02:30 CET, the second 02:30 of 25.10.2026
      renderEdit({
        ...summerSession,
        date: "2026-10-25",
        check_in_time: "2026-10-25T01:30:00Z",
        check_out_time: "2026-10-25T09:00:00Z",
      });
      expect(checkInField()).toHaveValue("02:30");

      fireEvent.click(
        screen.getByRole("button", { name: "Änderungen speichern" }),
      );

      await waitFor(() => expect(mocks.updateSession).toHaveBeenCalledTimes(1));
      const payload = mocks.updateSession.mock.calls[0]![2];
      expect(payload.check_in_time).toBeUndefined();
      expect(payload.check_out_time).toBeUndefined();
    });

    // Every reason the save button is blocked for stands at its field.
    it.each([
      [
        "a cleared stored end",
        "check_out_time",
        "",
        "Ein gespeichertes Ende lässt sich nicht löschen. Bitte eine Uhrzeit eintragen.",
      ],
      ["an empty start", "check_in_time", "", "Bitte eine Uhrzeit eintragen."],
      [
        "an end before the start",
        "check_out_time",
        "07:00",
        "Das Ende muss nach dem Beginn liegen.",
      ],
    ])("explains %s at the field", (_label, field, value, message) => {
      setTestClock("2026-11-09T12:00:00+01:00");
      renderEdit(summerSession);
      const input = document.querySelector<HTMLInputElement>(
        `input[name="${field}"]`,
      )!;
      fireEvent.change(input, { target: { value } });

      expect(screen.getByText(message)).toBeInTheDocument();
      expect(input).toHaveAttribute("aria-invalid", "true");
      const save = screen.getByRole("button", { name: "Änderungen speichern" });
      expect(save).toBeDisabled();
      fireEvent.click(save);
      expect(mocks.updateSession).not.toHaveBeenCalled();
    });

    it("blocks a time the switch day repeats", () => {
      setTestClock("2026-11-09T12:00:00+01:00");
      renderEdit({
        ...summerSession,
        date: "2026-10-25",
        check_in_time: "2026-10-25T07:00:00Z",
        check_out_time: "2026-10-25T15:00:00Z",
      });
      fireEvent.change(checkInField(), { target: { value: "02:30" } });

      expect(
        screen.getByText(
          "Diese Uhrzeit kommt an diesem Tag in Berlin zweimal vor. Bitte eine andere Uhrzeit wählen.",
        ),
      ).toBeInTheDocument();
      expect(checkInField()).toHaveAttribute("aria-invalid", "true");
      const save = screen.getByRole("button", { name: "Änderungen speichern" });
      expect(save).toBeDisabled();
      fireEvent.click(save);
      expect(mocks.updateSession).not.toHaveBeenCalled();
    });
  });
});

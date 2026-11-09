import { afterEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";

import type { StaffHistorySession } from "~/lib/staff-api";
import { setTestClock } from "~/test/clock";

import { AdminSessionEditModal } from "./admin-session-edit-modal";

// Runs in `school-date-dom`: only forked processes apply a runtime TZ change.
// The admin works from a browser outside Germany; the stamps stay school
// times (#6293).
afterEach(() => vi.unstubAllEnvs());

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

// 20.10.2026 is summer time: 08:00 Berlin = 06:00 UTC.
const session: StaffHistorySession = {
  id: "42",
  date: "2026-10-20",
  status: "present",
  net_minutes: 480,
  check_in_time: "2026-10-20T06:00:00Z",
  check_out_time: "2026-10-20T14:30:00Z",
  break_minutes: 30,
};

function renderModal(mode: "edit" | "nachtragen") {
  render(
    <AdminSessionEditModal
      isOpen
      mode={mode}
      staffId="7"
      date={new Date(2026, 9, 20)}
      session={mode === "edit" ? session : null}
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

describe("AdminSessionEditModal in a browser outside Germany (#6293)", () => {
  function useUtcBrowser() {
    vi.stubEnv("TZ", "UTC");
    setTestClock("2026-11-09T12:00:00+01:00");
    // The zone change took effect, or the cases below prove nothing.
    expect(new Date("2026-10-20T06:00:00Z").getHours()).toBe(6);
  }

  it("prefills the stored stamps as Berlin time", () => {
    useUtcBrowser();
    renderModal("edit");

    expect(checkInField()).toHaveValue("08:00");
    expect(document.querySelector('input[name="check_out_time"]')).toHaveValue(
      "16:30",
    );
  });

  it("saves a changed start as Berlin time of the session day", async () => {
    useUtcBrowser();
    mocks.updateSession.mockResolvedValue(undefined);
    renderModal("edit");
    fireEvent.change(checkInField(), { target: { value: "07:30" } });

    fireEvent.click(
      screen.getByRole("button", { name: "Änderungen speichern" }),
    );

    await waitFor(() => expect(mocks.updateSession).toHaveBeenCalledTimes(1));
    expect(mocks.updateSession.mock.calls[0]![2].check_in_time).toBe(
      "2026-10-20T05:30:00.000Z",
    );
  });

  it("creates a backfilled block in Berlin time", async () => {
    useUtcBrowser();
    mocks.createSession.mockResolvedValue(undefined);
    renderModal("nachtragen");

    fireEvent.click(screen.getByRole("button", { name: "Eintrag anlegen" }));

    await waitFor(() => expect(mocks.createSession).toHaveBeenCalledTimes(1));
    const payload = mocks.createSession.mock.calls[0]![1];
    expect(payload.check_in_time).toBe("2026-10-20T06:00:00.000Z");
    expect(payload.check_out_time).toBe("2026-10-20T14:00:00.000Z");
    expect(payload.date).toBe("2026-10-20T00:00:00.000Z");
  });
});

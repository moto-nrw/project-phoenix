import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { setTestClock } from "~/test/clock";
import { todayISO, parseISODate, toISODate } from "~/lib/date-helpers";

import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";
import { suppressConsole } from "~/test/helpers/console";

const mocks = vi.hoisted(() => ({
  push: vi.fn(),
  createAbsence: vi.fn(),
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: mocks.push }),
}));

vi.mock("~/lib/staff-api", () => ({
  staffAbsenceService: {
    createAbsence: mocks.createAbsence,
  },
}));

// The kit DatePicker opens a react-day-picker overlay; swap it for a plain
// native date input so the value can be driven deterministically in tests.
vi.mock("~/components/ui/date-picker", () => ({
  DatePicker: ({
    value,
    onChange,
    placeholder,
  }: {
    value: Date | null;
    onChange: (date: Date | null) => void;
    placeholder?: string;
  }) => {
    const fmt = (d: Date) => {
      const y = d.getFullYear();
      const m = String(d.getMonth() + 1).padStart(2, "0");
      const day = String(d.getDate()).padStart(2, "0");
      return `${y}-${m}-${day}`;
    };
    return (
      <input
        type="date"
        aria-label={placeholder}
        value={value ? fmt(value) : ""}
        onChange={(e) =>
          onChange(
            e.target.value ? new Date(`${e.target.value}T00:00:00`) : null,
          )
        }
      />
    );
  },
}));

import { SickReportModal } from "./sick-report-modal";

const staff = { id: "7", firstName: "Ada", lastName: "Lovelace" };

const createdRow = {
  id: 42,
  staff_id: 7,
  absence_type: "sick",
  date_start: "2026-07-20",
  date_end: "2026-07-22",
  half_day: false,
  note: "Grippe",
  status: "reported",
};

function renderModal(
  overrides: Partial<{
    onClose: () => void;
    onCreated: (a: unknown) => void;
  }> = {},
) {
  const onClose = overrides.onClose ?? vi.fn();
  const onCreated = overrides.onCreated ?? vi.fn();
  render(
    <SickReportModal
      isOpen
      staff={staff}
      onClose={onClose}
      onCreated={onCreated}
    />,
  );
  return { onClose, onCreated };
}

describe("SickReportModal", () => {
  suppressConsole("error");

  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it.each([
    ["UTC", "2026-01-15T23:30:00Z", "2026-01-16", "2026-01-15"],
    ["UTC", "2026-07-15T22:30:00Z", "2026-07-16", "2026-07-15"],
    ["UTC", "2026-03-28T23:30:00Z", "2026-03-29", "2026-03-28"],
    ["UTC", "2026-03-29T22:30:00Z", "2026-03-30", "2026-03-29"],
    ["UTC", "2026-10-24T22:30:00Z", "2026-10-25", "2026-10-24"],
    ["UTC", "2026-10-25T23:30:00Z", "2026-10-26", "2026-10-25"],
    ["Asia/Tokyo", "2026-01-15T15:30:00Z", "2026-01-15", "2026-01-16"],
  ])(
    "submits the Berlin school day in %s at %s",
    async (zone, instant, day, localDay) => {
      vi.stubEnv("TZ", zone);
      setTestClock(instant);
      // Prove that the browser day really differs, including in the test worker.
      expect(todayISO()).toBe(localDay);
      mocks.createAbsence.mockResolvedValue(createdRow);
      const props = { staff, onClose: vi.fn(), onCreated: vi.fn() };
      const { rerender } = render(<SickReportModal {...props} isOpen />);

      for (const input of screen.getAllByLabelText("Datum auswählen")) {
        expect(input).toHaveValue(day);
      }
      fireEvent.click(screen.getByRole("button", { name: "Krank melden" }));
      await screen.findByRole("button", { name: "Zur Vertretung" });
      expect(mocks.createAbsence).toHaveBeenLastCalledWith(
        "7",
        expect.objectContaining({
          date_start: day,
          date_end: day,
        }),
      );

      rerender(<SickReportModal {...props} isOpen={false} />);
      setTestClock(new Date(new Date(instant).getTime() + 26 * 60 * 60 * 1000));
      const nextDate = parseISODate(day);
      nextDate.setDate(nextDate.getDate() + 1);
      const nextDay = toISODate(nextDate);
      rerender(<SickReportModal {...props} isOpen />);
      for (const input of screen.getAllByLabelText("Datum auswählen")) {
        expect(input).toHaveValue(nextDay);
      }
      fireEvent.click(screen.getByRole("button", { name: "Krank melden" }));
      await screen.findByRole("button", { name: "Zur Vertretung" });
      expect(mocks.createAbsence).toHaveBeenLastCalledWith(
        "7",
        expect.objectContaining({
          date_start: nextDay,
          date_end: nextDay,
        }),
      );
    },
  );

  it("preserves an explicit initial date across reopening in a non-Berlin browser", async () => {
    vi.stubEnv("TZ", "UTC");
    setTestClock("2026-01-15T23:30:00Z");
    mocks.createAbsence.mockResolvedValue(createdRow);
    const props = {
      staff,
      initialDate: "2026-01-10",
      onClose: vi.fn(),
      onCreated: vi.fn(),
    };
    const { rerender } = render(<SickReportModal {...props} isOpen />);
    rerender(<SickReportModal {...props} isOpen={false} />);
    setTestClock("2026-01-16T23:30:00Z");
    rerender(<SickReportModal {...props} isOpen />);
    fireEvent.click(screen.getByRole("checkbox", { name: "Halber Tag" }));
    fireEvent.click(screen.getByRole("button", { name: "Krank melden" }));
    await screen.findByRole("button", { name: "Zur Vertretung" });
    expect(mocks.createAbsence).toHaveBeenCalledWith(
      "7",
      expect.objectContaining({
        date_start: "2026-01-10",
        date_end: "2026-01-10",
        half_day: true,
      }),
    );
  });

  it("submits a sick absence with the entered values", async () => {
    mocks.createAbsence.mockResolvedValue(createdRow);
    const { onCreated } = renderModal();

    const [start, end] = screen.getAllByLabelText("Datum auswählen");
    fireEvent.change(start!, { target: { value: "2026-07-20" } });
    fireEvent.change(end!, { target: { value: "2026-07-22" } });
    fireEvent.change(screen.getByLabelText("Notiz (optional)"), {
      target: { value: "  Grippe  " },
    });

    fireEvent.click(screen.getByRole("button", { name: "Krank melden" }));

    await waitFor(() =>
      expect(mocks.createAbsence).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({
          absence_type: "sick",
          date_start: "2026-07-20",
          date_end: "2026-07-22",
          note: "Grippe",
        }),
      ),
    );
    expect(onCreated).toHaveBeenCalledWith(createdRow);
    // Success state reveals the jump to the Vertretungsplan.
    expect(
      await screen.findByRole("button", { name: "Zur Vertretung" }),
    ).toBeInTheDocument();
  });

  it("navigates to the Vertretung day view and closes on 'Zur Vertretung'", async () => {
    mocks.createAbsence.mockResolvedValue(createdRow);
    const { onClose } = renderModal();

    const [start, end] = screen.getAllByLabelText("Datum auswählen");
    fireEvent.change(start!, { target: { value: "2026-07-20" } });
    fireEvent.change(end!, { target: { value: "2026-07-20" } });
    fireEvent.click(screen.getByRole("button", { name: "Krank melden" }));

    const jump = await screen.findByRole("button", { name: "Zur Vertretung" });
    fireEvent.click(jump);

    expect(mocks.push).toHaveBeenCalledWith("/vertretung?d=2026-07-20");
    expect(onClose).toHaveBeenCalled();
  });

  it("clamps the end date to the start when start moves past it", () => {
    renderModal();

    const [start, end] = screen.getAllByLabelText("Datum auswählen");
    fireEvent.change(start!, { target: { value: "2026-07-10" } });
    fireEvent.change(end!, { target: { value: "2026-07-15" } });
    // Move the start beyond the end — the end must follow.
    fireEvent.change(start!, { target: { value: "2026-07-20" } });

    expect(end).toHaveValue("2026-07-20");
  });

  it("limits half-day reports to one date and explains that plans stay unchanged", async () => {
    mocks.createAbsence.mockResolvedValue({
      ...createdRow,
      date_end: "2026-07-20",
      half_day: true,
    });
    renderModal();

    fireEvent.click(screen.getByRole("checkbox", { name: "Halber Tag" }));
    const [date] = screen.getAllByLabelText("Datum auswählen");
    expect(screen.getAllByLabelText("Datum auswählen")).toHaveLength(1);
    fireEvent.change(date!, { target: { value: "2026-07-20" } });

    fireEvent.click(screen.getByRole("button", { name: "Krank melden" }));

    await waitFor(() =>
      expect(mocks.createAbsence).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({
          absence_type: "sick",
          date_start: "2026-07-20",
          date_end: "2026-07-20",
          half_day: true,
        }),
      ),
    );
    expect(
      await screen.findByText(
        /Dienst- und Betreuungsplan wurden nicht automatisch geändert/,
      ),
    ).toBeInTheDocument();
  });

  it("shows the refusal in the dialog and does not report success", async () => {
    mocks.createAbsence.mockRejectedValue(
      new ApiError("absence overlaps", 409, {
        code: "workforce.absence_overlap",
      }),
    );
    const { onCreated } = renderModal();

    fireEvent.click(screen.getByRole("button", { name: "Krank melden" }));

    const dialog = await screen.findByRole("dialog");
    expect(
      await within(dialog).findByText(
        catalogText("workforce.absence_overlap", "die Krankmeldung"),
      ),
    ).toBeInTheDocument();
    // The backend's own sentence never reaches the screen.
    expect(screen.queryByText("absence overlaps")).not.toBeInTheDocument();
    expect(onCreated).not.toHaveBeenCalled();
    expect(
      screen.queryByRole("button", { name: "Zur Vertretung" }),
    ).not.toBeInTheDocument();
  });
});

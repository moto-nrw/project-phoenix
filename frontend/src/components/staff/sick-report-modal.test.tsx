import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { suppressConsole } from "~/test/helpers/console";

const mocks = vi.hoisted(() => ({
  push: vi.fn(),
  createAbsence: vi.fn(),
  getCompTimePreview: vi.fn(),
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: mocks.push }),
}));

vi.mock("~/lib/staff-api", () => ({
  staffAbsenceService: {
    createAbsence: mocks.createAbsence,
    getCompTimePreview: mocks.getCompTimePreview,
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
    absenceType: "sick" | "comp_time";
  }> = {},
) {
  const onClose = overrides.onClose ?? vi.fn();
  const onCreated = overrides.onCreated ?? vi.fn();
  render(
    <SickReportModal
      isOpen
      staff={staff}
      absenceType={overrides.absenceType}
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
    // Default Saldo-Vorschau (#2873): enough Plus-Stunden, no confirmation
    // required. Individual tests override for the overdraft path.
    mocks.getCompTimePreview.mockResolvedValue({
      currentBalanceMinutes: 600,
      deductionMinutes: 480,
      realizedDeductionMinutes: 0,
      futureCommitmentMinutes: 0,
      projectedBalanceMinutes: 120,
    });
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

  it("shows the backend error and does not report success", async () => {
    mocks.createAbsence.mockRejectedValue(new Error("overlapping absence"));
    const { onCreated } = renderModal();

    fireEvent.click(screen.getByRole("button", { name: "Krank melden" }));

    expect(await screen.findByText("overlapping absence")).toBeInTheDocument();
    expect(onCreated).not.toHaveBeenCalled();
    expect(
      screen.queryByRole("button", { name: "Zur Vertretung" }),
    ).not.toBeInTheDocument();
  });

  it("creates manager-controlled comp time and explains half-day accounting", async () => {
    mocks.createAbsence.mockResolvedValue({
      ...createdRow,
      absence_type: "comp_time",
      half_day: true,
    });
    renderModal({ absenceType: "comp_time" });

    expect(
      screen.getByText(/andere Hälfte als Arbeitszeit erfasst/),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("checkbox", { name: "Halber Tag" }));
    // Der Haken lädt die Saldo-Vorschau neu; solange sie lädt, ist der
    // Submit gesperrt (#2885).
    const submit = screen.getByRole("button", {
      name: "Freizeitausgleich eintragen",
    });
    await waitFor(() => expect(submit).toBeEnabled());
    fireEvent.click(submit);

    await waitFor(() =>
      expect(mocks.createAbsence).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({
          absence_type: "comp_time",
          half_day: true,
        }),
      ),
    );
    expect(
      await screen.findByText(/keine Sollzeit gutgeschrieben/),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Zur Vertretung" }),
    ).not.toBeInTheDocument();
  });

  it("shows the Saldo-Vorschau for comp time (#2873)", async () => {
    renderModal({ absenceType: "comp_time" });

    expect(await screen.findByText("Stundenkonto aktuell")).toBeInTheDocument();
    expect(screen.getByText("Abzug für diesen Eintrag")).toBeInTheDocument();
    expect(screen.getByText("Stundenkonto danach")).toBeInTheDocument();
    // Positive projection: no warning, no confirmation checkbox.
    expect(
      screen.queryByText(/fällt damit unter null/),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("checkbox", {
        name: "Ich trage den Freizeitausgleich trotzdem ein.",
      }),
    ).not.toBeInTheDocument();
  });

  it("requires an explicit confirmation when the projection is negative (#2873)", async () => {
    mocks.getCompTimePreview.mockResolvedValue({
      currentBalanceMinutes: 120,
      deductionMinutes: 480,
      realizedDeductionMinutes: 0,
      futureCommitmentMinutes: 240,
      projectedBalanceMinutes: -600,
    });
    mocks.createAbsence.mockResolvedValue({
      ...createdRow,
      absence_type: "comp_time",
    });
    renderModal({ absenceType: "comp_time" });

    expect(
      await screen.findByText(/fällt damit unter null/),
    ).toBeInTheDocument();
    // The already planned future Freizeitausgleich shows up as its own row.
    expect(
      screen.getByText("Bereits geplanter Freizeitausgleich"),
    ).toBeInTheDocument();

    const submit = screen.getByRole("button", {
      name: "Freizeitausgleich eintragen",
    });
    expect(submit).toBeDisabled();

    fireEvent.click(
      screen.getByRole("checkbox", {
        name: "Ich trage den Freizeitausgleich trotzdem ein.",
      }),
    );
    expect(submit).toBeEnabled();

    fireEvent.click(submit);
    await waitFor(() =>
      expect(mocks.createAbsence).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({ absence_type: "comp_time" }),
      ),
    );
  });

  it("still allows submitting when the Saldo-Vorschau fails to load (#2873)", async () => {
    mocks.getCompTimePreview.mockRejectedValue(new Error("preview down"));
    mocks.createAbsence.mockResolvedValue({
      ...createdRow,
      absence_type: "comp_time",
    });
    renderModal({ absenceType: "comp_time" });

    const submit = screen.getByRole("button", {
      name: "Freizeitausgleich eintragen",
    });
    await waitFor(() => expect(submit).toBeEnabled());
    fireEvent.click(submit);
    await waitFor(() => expect(mocks.createAbsence).toHaveBeenCalled());
  });

  it("blocks the submit while a changed range's Vorschau reloads (#2885)", async () => {
    renderModal({ absenceType: "comp_time" });

    expect(await screen.findByText("Stundenkonto danach")).toBeInTheDocument();
    const submit = screen.getByRole("button", {
      name: "Freizeitausgleich eintragen",
    });
    expect(submit).toBeEnabled();

    // The reload hangs: the stale positive projection must disappear and the
    // submit stay blocked — otherwise an overdrafting range could be booked
    // without the explicit confirmation.
    let resolvePreview!: (value: unknown) => void;
    mocks.getCompTimePreview.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolvePreview = resolve;
        }),
    );
    const [startInput] = screen.getAllByLabelText("Datum auswählen");
    fireEvent.change(startInput!, { target: { value: "2026-07-21" } });

    expect(
      await screen.findByText(/Stundenkonto wird berechnet/),
    ).toBeInTheDocument();
    expect(screen.queryByText("Stundenkonto danach")).not.toBeInTheDocument();
    expect(submit).toBeDisabled();

    resolvePreview({
      currentBalanceMinutes: 600,
      deductionMinutes: 960,
      realizedDeductionMinutes: 0,
      futureCommitmentMinutes: 0,
      projectedBalanceMinutes: -360,
    });
    expect(
      await screen.findByText(/fällt damit unter null/),
    ).toBeInTheDocument();
    expect(submit).toBeDisabled();
  });
});

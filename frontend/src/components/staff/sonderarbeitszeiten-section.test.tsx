import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { StaffTargetOverride } from "~/lib/staff-target-overrides-api";
import { setTestClock } from "~/test/clock";
import { SonderarbeitszeitenSection } from "./sonderarbeitszeiten-section";

const mocks = vi.hoisted(() => ({
  rows: [] as StaffTargetOverride[],
  create: vi.fn(),
  remove: vi.fn(),
  mutateList: vi.fn(),
  mutate: vi.fn(),
  toastSuccess: vi.fn(),
}));

vi.mock("swr", () => ({
  useSWRConfig: () => ({ mutate: mocks.mutate }),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: () => ({
    data: mocks.rows,
    error: undefined,
    isLoading: false,
    mutate: mocks.mutateList,
  }),
}));

vi.mock("~/lib/staff-target-overrides-api", () => ({
  staffTargetOverrideService: {
    list: vi.fn(),
    create: mocks.create,
    delete: mocks.remove,
  },
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ success: mocks.toastSuccess, error: vi.fn() }),
}));

vi.mock("~/components/ui/date-picker", async (importOriginal) => {
  const { isoDatePickerMock } = await import("~/test/mocks/date-picker");
  return { ...(await importOriginal<object>()), ...isoDatePickerMock() };
});

const holidayCare: StaffTargetOverride = {
  id: "7",
  staffId: "42",
  startDate: "2026-10-19",
  endDate: "2026-10-23",
  dailyMinutes: 510,
};

function openCreate() {
  render(<SonderarbeitszeitenSection staffId="42" canEdit />);
  fireEvent.click(
    screen.getAllByRole("button", { name: "Sonderarbeitszeit anlegen" })[0]!,
  );
  expect(
    screen.getByRole("dialog", { name: "Sonderarbeitszeit anlegen" }),
  ).toBeInTheDocument();
}

function fillRange(hours: string) {
  fireEvent.change(screen.getByLabelText("Erster Tag"), {
    target: { value: "2026-10-19" },
  });
  fireEvent.change(screen.getByLabelText("Letzter Tag"), {
    target: { value: "2026-10-23" },
  });
  fireEvent.change(screen.getByLabelText("Stunden pro Tag"), {
    target: { value: hours },
  });
}

describe("SonderarbeitszeitenSection", () => {
  beforeEach(() => {
    setTestClock("2026-10-01T10:00:00+02:00");
    mocks.rows = [holidayCare];
    mocks.create.mockReset();
    mocks.remove.mockReset();
    mocks.mutateList.mockReset();
    mocks.mutate.mockReset();
    mocks.toastSuccess.mockReset();
  });

  it("lists the range with its daily hours as decimal hours", () => {
    render(<SonderarbeitszeitenSection staffId="42" canEdit />);

    expect(
      screen.getByRole("heading", { name: "Sonderarbeitszeiten" }),
    ).toBeInTheDocument();
    expect(screen.getByText("19.10.2026 – 23.10.2026")).toBeInTheDocument();
    expect(screen.getByText("8,5 Std.")).toBeInTheDocument();
    expect(
      screen.getByText(/auch an Schließtagen\. Feiertage bleiben frei\./),
    ).toBeInTheDocument();
  });

  it("offers only deleting on a row, no editing", () => {
    render(<SonderarbeitszeitenSection staffId="42" canEdit />);
    fireEvent.click(
      screen.getByRole("button", {
        name: "Aktionen für die Sonderarbeitszeit ab 19.10.2026",
      }),
    );

    expect(
      screen.getAllByRole("menuitem").map((item) => item.textContent),
    ).toEqual(["Löschen"]);
  });

  it("hides every write action without the edit permission", () => {
    render(<SonderarbeitszeitenSection staffId="42" canEdit={false} />);

    expect(
      screen.queryByRole("button", { name: "Sonderarbeitszeit anlegen" }),
    ).toBeNull();
    expect(
      screen.queryByRole("button", {
        name: /Aktionen für die Sonderarbeitszeit/,
      }),
    ).toBeNull();
  });

  it("creates 8,25 hours as 495 minutes and refreshes the Soll caches", async () => {
    mocks.rows = [];
    mocks.create.mockResolvedValue({ ...holidayCare, dailyMinutes: 495 });
    openCreate();
    fillRange("8,25");
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(mocks.create).toHaveBeenCalledWith("42", {
        startDate: "2026-10-19",
        endDate: "2026-10-23",
        dailyMinutes: 495,
      }),
    );
    expect(mocks.mutateList).toHaveBeenCalled();
    expect(mocks.toastSuccess).toHaveBeenCalledWith(
      "Sonderarbeitszeit angelegt.",
    );
    const invalidate = mocks.mutate.mock.calls[0]?.[0] as (
      key: unknown,
    ) => boolean;
    expect(
      invalidate("tenant:staff-schedule-targets-42-2026-10-01-2026-10-31"),
    ).toBe(true);
    expect(invalidate("tenant:staff-month-summary-42-2026-10")).toBe(true);
  });

  it("accepts 0 hours, rejects more than 12", async () => {
    mocks.create.mockResolvedValue({ ...holidayCare, dailyMinutes: 0 });
    openCreate();
    fillRange("13");
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    expect(
      await screen.findByText(
        "Bitte 0 bis 12 Stunden eingeben, zum Beispiel 8,5.",
      ),
    ).toBeInTheDocument();
    expect(mocks.create).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText("Stunden pro Tag"), {
      target: { value: "0" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await waitFor(() =>
      expect(mocks.create).toHaveBeenCalledWith(
        "42",
        expect.objectContaining({ dailyMinutes: 0 }),
      ),
    );
  });

  it("shows the server's reason when a range overlaps another one", async () => {
    mocks.create.mockRejectedValue(
      new Error(
        "Für diesen Zeitraum gibt es schon eine Sonderarbeitszeit (19.10.2026 bis 23.10.2026). Löschen Sie diese zuerst oder wählen Sie andere Tage.",
      ),
    );
    openCreate();
    fillRange("8,5");
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(/gibt es schon eine Sonderarbeitszeit/),
    ).toBeInTheDocument();
    expect(mocks.mutate).not.toHaveBeenCalled();
  });

  it("deletes after the two-step confirmation and confirms with a toast", async () => {
    mocks.remove.mockResolvedValue(undefined);
    render(<SonderarbeitszeitenSection staffId="42" canEdit />);
    fireEvent.click(
      screen.getByRole("button", {
        name: "Aktionen für die Sonderarbeitszeit ab 19.10.2026",
      }),
    );
    fireEvent.click(screen.getByRole("menuitem", { name: "Löschen" }));
    fireEvent.click(screen.getByRole("button", { name: "Ja, löschen" }));
    fireEvent.click(screen.getByRole("button", { name: "Endgültig löschen" }));

    await waitFor(() => expect(mocks.remove).toHaveBeenCalledWith("42", "7"));
    await waitFor(() =>
      expect(mocks.toastSuccess).toHaveBeenCalledWith(
        "Sonderarbeitszeit gelöscht.",
      ),
    );
    expect(mocks.mutateList).toHaveBeenCalled();
  });

  it("names the section once, not again above the table", () => {
    render(<SonderarbeitszeitenSection staffId="42" canEdit />);

    expect(screen.getAllByText("Sonderarbeitszeiten")).toHaveLength(1);
  });

  it("offers the next step in the empty state", () => {
    mocks.rows = [];
    render(<SonderarbeitszeitenSection staffId="42" canEdit />);

    expect(
      screen.getByText("Keine Sonderarbeitszeiten eingetragen."),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Legen Sie dafür eine Sonderarbeitszeit an\./),
    ).toBeInTheDocument();
    expect(
      screen.getAllByRole("button", { name: "Sonderarbeitszeit anlegen" }),
    ).toHaveLength(2);
  });

  it("shows a single day as one date", () => {
    mocks.rows = [
      { ...holidayCare, startDate: "2026-10-19", endDate: "2026-10-19" },
    ];
    render(<SonderarbeitszeitenSection staffId="42" canEdit />);

    expect(screen.getByText("19.10.2026")).toBeInTheDocument();
  });

  it("puts each validation error at its field", async () => {
    mocks.rows = [];
    openCreate();
    fireEvent.change(screen.getByLabelText("Stunden pro Tag"), {
      target: { value: "abc" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText("Bitte den ersten Tag wählen."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Bitte den letzten Tag wählen."),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Stunden pro Tag")).toHaveAttribute(
      "aria-invalid",
      "true",
    );
    expect(
      screen.getByText("Bitte die markierten Felder prüfen."),
    ).toBeInTheDocument();
    expect(mocks.create).not.toHaveBeenCalled();
  });
});

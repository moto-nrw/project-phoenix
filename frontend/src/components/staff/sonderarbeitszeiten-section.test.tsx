import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import type { StaffTargetOverride } from "~/lib/staff-target-overrides-api";
import { setTestClock } from "~/test/clock";
import { catalogText } from "~/test/error-catalog-text";
import { SonderarbeitszeitenSection } from "./sonderarbeitszeiten-section";

const mocks = vi.hoisted(() => ({
  rows: [] as StaffTargetOverride[],
  create: vi.fn(),
  remove: vi.fn(),
  mutateList: vi.fn(),
  mutate: vi.fn(),
  toastSuccess: vi.fn(),
  loadError: undefined as unknown,
  endDefaultMonth: undefined as string | undefined,
}));

vi.mock("swr", () => ({
  useSWRConfig: () => ({ mutate: mocks.mutate }),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: () => ({
    data: mocks.loadError ? undefined : mocks.rows,
    error: mocks.loadError,
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

vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({ success: mocks.toastSuccess, error: vi.fn() }),
}));

vi.mock("~/components/ui/date-picker", async () => {
  const { datePickerModuleMock } = await import("~/test/mocks/date-picker");
  const stubs = datePickerModuleMock();
  const Stub = stubs.ISODatePicker;
  return {
    ...stubs,
    // Records the month the "Letzter Tag" calendar opens in.
    ISODatePicker: (props: Parameters<typeof Stub>[0]) => {
      if (props.id === "target-override-end") {
        mocks.endDefaultMonth = props.defaultMonth;
      }
      return Stub(props);
    },
  };
});

const holidayCare: StaffTargetOverride = {
  id: "7",
  staffId: "42",
  startDate: "2026-10-19",
  endDate: "2026-10-23",
  dailyMinutes: 510,
  weekdayMinutes: null,
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
    mocks.loadError = undefined;
  });

  it("shows a failed load in place, not as an empty list", async () => {
    mocks.loadError = new ApiError("boom", 500, {
      code: "general.server",
      instance: "req-overrides",
    });
    render(<SonderarbeitszeitenSection staffId="42" canEdit />);

    expect(
      await screen.findByText(
        catalogText("general.server", "die Liste der Sonderarbeitszeiten"),
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Keine Sonderarbeitszeiten eingetragen."),
    ).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(mocks.mutateList).toHaveBeenCalled();
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
      "Die Sonderarbeitszeit ist gespeichert.",
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
      await screen.findByText("Bitte 0 bis 12 Stunden eingeben."),
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

  it("explains a refused overlapping range in the dialog", async () => {
    mocks.create.mockRejectedValue(
      new ApiError("target override overlaps", 409, {
        code: "general.business_rejection",
      }),
    );
    openCreate();
    fillRange("8,5");
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(
        catalogText("general.business_rejection", "die Sonderarbeitszeit"),
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("dialog", { name: "Sonderarbeitszeit anlegen" }),
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
        "Die Sonderarbeitszeit ist gelöscht.",
      ),
    );
    expect(mocks.mutateList).toHaveBeenCalled();
  });

  it("keeps a refused delete in the dialog", async () => {
    mocks.remove.mockRejectedValue(
      new ApiError("forbidden", 403, { code: "general.permission" }),
    );
    render(<SonderarbeitszeitenSection staffId="42" canEdit />);
    fireEvent.click(
      screen.getByRole("button", {
        name: "Aktionen für die Sonderarbeitszeit ab 19.10.2026",
      }),
    );
    fireEvent.click(screen.getByRole("menuitem", { name: "Löschen" }));
    fireEvent.click(screen.getByRole("button", { name: "Ja, löschen" }));
    fireEvent.click(screen.getByRole("button", { name: "Endgültig löschen" }));

    expect(
      await screen.findByText(
        catalogText("general.permission", "die Sonderarbeitszeit"),
      ),
    ).toBeInTheDocument();
    expect(mocks.toastSuccess).not.toHaveBeenCalled();
  });

  it("lists hours per weekday with equal days joined (#3745)", () => {
    mocks.rows = [
      {
        ...holidayCare,
        dailyMinutes: null,
        weekdayMinutes: [210, 30, 30, 30, 30],
      },
      {
        ...holidayCare,
        id: "8",
        startDate: "2026-10-26",
        endDate: "2026-10-30",
        dailyMinutes: null,
        weekdayMinutes: [240, 240, 0, 240, 120],
      },
    ];
    render(<SonderarbeitszeitenSection staffId="42" canEdit />);

    expect(
      screen.getByText("Mo 3,5 Std. · Di–Fr je 0,5 Std."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Mo–Di je 4 Std. · Mi 0 Std. · Do 4 Std. · Fr 2 Std."),
    ).toBeInTheDocument();
  });

  it("creates hours per weekday, starting from the hours already typed (#3745)", async () => {
    mocks.rows = [];
    mocks.create.mockResolvedValue({
      ...holidayCare,
      dailyMinutes: null,
      weekdayMinutes: [210, 30, 30, 30, 30],
    });
    openCreate();
    fillRange("0,5");
    fireEvent.click(screen.getByRole("button", { name: "Je Wochentag" }));

    expect(screen.queryByLabelText("Stunden pro Tag")).toBeNull();
    for (const day of [
      "Montag",
      "Dienstag",
      "Mittwoch",
      "Donnerstag",
      "Freitag",
    ]) {
      expect(screen.getByLabelText(`Stunden am ${day}`)).toHaveValue("0,5");
    }
    fireEvent.change(screen.getByLabelText("Stunden am Montag"), {
      target: { value: "3,5" },
    });
    expect(
      screen.getByText(/Zusammen 5,5 Stunden pro Woche\./),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(mocks.create).toHaveBeenCalledWith("42", {
        startDate: "2026-10-19",
        endDate: "2026-10-23",
        weekdayMinutes: [210, 30, 30, 30, 30],
      }),
    );
  });

  it("rejects a weekday without valid hours", async () => {
    mocks.rows = [];
    openCreate();
    fillRange("1");
    fireEvent.click(screen.getByRole("button", { name: "Je Wochentag" }));
    fireEvent.change(screen.getByLabelText("Stunden am Mittwoch"), {
      target: { value: "13" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText("Bitte für jeden Tag 0 bis 12 Stunden eingeben."),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Stunden am Mittwoch")).toHaveAttribute(
      "aria-invalid",
      "true",
    );
    expect(screen.getByLabelText("Stunden am Montag")).not.toHaveAttribute(
      "aria-invalid",
    );
    expect(mocks.create).not.toHaveBeenCalled();
  });

  it("keeps equal weekday hours when switching back to the same hours every day", () => {
    mocks.rows = [];
    openCreate();
    fireEvent.click(screen.getByRole("button", { name: "Je Wochentag" }));
    for (const day of [
      "Montag",
      "Dienstag",
      "Mittwoch",
      "Donnerstag",
      "Freitag",
    ]) {
      fireEvent.change(screen.getByLabelText(`Stunden am ${day}`), {
        target: { value: "8" },
      });
    }
    fireEvent.click(screen.getByRole("button", { name: "Jeden Tag gleich" }));

    expect(screen.getByLabelText("Stunden pro Tag")).toHaveValue("8");
  });

  it("keeps equal weekday hours with different decimal spellings", () => {
    mocks.rows = [];
    openCreate();
    fireEvent.click(screen.getByRole("button", { name: "Je Wochentag" }));
    const values = ["1", "1,0", "1.00", "1", "1"];
    for (const [index, day] of [
      "Montag",
      "Dienstag",
      "Mittwoch",
      "Donnerstag",
      "Freitag",
    ].entries()) {
      fireEvent.change(screen.getByLabelText(`Stunden am ${day}`), {
        target: { value: values[index] },
      });
    }
    fireEvent.click(screen.getByRole("button", { name: "Jeden Tag gleich" }));

    expect(screen.getByLabelText("Stunden pro Tag")).toHaveValue("1");
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
      screen.getByText("Es gilt das Arbeitszeitmodell."),
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
    // Same red frame as the date fields.
    expect(screen.getByLabelText("Stunden pro Tag")).toHaveClass(
      "ring-moto-red",
    );
    expect(
      screen.getByText("Bitte prüfen Sie die markierten Felder."),
    ).toBeInTheDocument();
    expect(mocks.create).not.toHaveBeenCalled();
  });

  it("opens the last day in the first day's month without filling it in", () => {
    mocks.rows = [];
    openCreate();
    expect(mocks.endDefaultMonth).toBeUndefined();

    fireEvent.change(screen.getByLabelText("Erster Tag"), {
      target: { value: "2026-12-21" },
    });

    expect(mocks.endDefaultMonth).toBe("2026-12-21");
    expect(screen.getByLabelText("Letzter Tag")).toHaveValue("");
  });
});

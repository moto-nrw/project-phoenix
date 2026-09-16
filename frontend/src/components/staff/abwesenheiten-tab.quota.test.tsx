import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { StaffVacationQuotaSummary } from "~/lib/staff-api";

// Both hooks return stable identities on purpose: the tab's reload callback
// depends on the toast object, so a fresh object per render would re-run the
// load effect forever and keep the tab in its loading state.
const stable = vi.hoisted(() => ({
  toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() },
  mutateMatching: vi.fn().mockResolvedValue(undefined),
  swrMutate: vi.fn().mockResolvedValue(undefined),
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => stable.toast,
}));

vi.mock("swr", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  useSWRConfig: () => ({ mutate: stable.swrMutate }),
}));

vi.mock("~/lib/swr", () => ({
  useTenantMutateMatching: () => stable.mutateMatching,
}));

const mocks = vi.hoisted(() => ({
  getVacationQuota: vi.fn(),
  getAbsences: vi.fn(),
  setVacationQuota: vi.fn(),
}));

vi.mock("~/lib/staff-api", () => ({
  staffAbsenceService: {
    getVacationQuota: mocks.getVacationQuota,
    getAbsences: mocks.getAbsences,
    setVacationQuota: mocks.setVacationQuota,
    setVacationOpening: vi.fn(),
    deleteVacationOpening: vi.fn(),
    approve: vi.fn(),
    deleteAbsence: vi.fn(),
  },
}));

import { AbwesenheitenTab } from "./abwesenheiten-tab";

const year = new Date().getFullYear();

function quota(
  overrides: Partial<StaffVacationQuotaSummary> = {},
): StaffVacationQuotaSummary {
  return {
    staff_id: 4,
    year,
    entitled_days: 30,
    carryover_days: 2,
    taken_before_days: 0,
    taken_days: 0,
    reserved_days: 0,
    remaining_days: 32,
    ...overrides,
  };
}

const EDIT_BUTTON = { name: "Anspruch ändern: Urlaub" };

async function openQuotaEditor() {
  fireEvent.click(await screen.findByRole("button", EDIT_BUTTON));
  return {
    entitled: screen.getByLabelText("Jahresanspruch (Tage)"),
    carryover: screen.getByLabelText("Übertrag aus Vorjahr (Tage)"),
  };
}

// Der Urlaubsanspruch wird am Objekt bearbeitet (BAUARTEN-SPEC Bauart 2 Regeln
// 3 bis 5, #3119): „Anspruch ändern“ an der Urlaubskarte schaltet sie in den
// Bearbeiten-Zustand (#3256), „Speichern“ steht unten in `EditActions`, Fehler stehen
// im Alert oben und am Feld. Es gibt kein Modal mehr.
describe("AbwesenheitenTab Urlaubsanspruch bearbeiten", () => {
  beforeEach(() => {
    mocks.getVacationQuota.mockReset();
    mocks.getAbsences.mockReset();
    mocks.setVacationQuota.mockReset();
    stable.toast.success.mockReset();
    stable.toast.error.mockReset();
    mocks.getVacationQuota.mockResolvedValue(quota());
    mocks.getAbsences.mockResolvedValue([]);
  });

  it("switches the section into the edit state and saves both fields", async () => {
    mocks.setVacationQuota.mockResolvedValue(undefined);
    render(
      <AbwesenheitenTab
        staffId="4"
        canEdit
        canEditQuota
        canManageSickReports
      />,
    );

    expect(await screen.findByText("Übrig")).toBeInTheDocument();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    const fields = await openQuotaEditor();
    // Im Bearbeiten-Zustand weichen die Zahlen der Karte dem Formular, der
    // Einstieg ist damit im Formular nicht mehr da.
    expect(screen.queryByRole("button", EDIT_BUTTON)).not.toBeInTheDocument();
    expect(screen.queryByText("Übrig")).not.toBeInTheDocument();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(fields.entitled).toHaveValue(30);
    expect(fields.carryover).toHaveValue(2);

    fireEvent.change(fields.entitled, { target: { value: "31.5" } });
    fireEvent.change(fields.carryover, { target: { value: "0" } });
    fireEvent.change(screen.getByLabelText("Begründung"), {
      target: { value: "Stellenumfang erhöht" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(mocks.setVacationQuota).toHaveBeenCalledWith("4", {
        year,
        entitled_days: 31.5,
        carryover_days: 0,
        reason: "Stellenumfang erhöht",
      });
    });
    expect(stable.toast.success).toHaveBeenCalledWith(
      "Urlaubsanspruch gespeichert.",
    );
    // Zurück in der Anzeige mit neu geladenen Zahlen.
    expect(await screen.findByRole("button", EDIT_BUTTON)).toBeInTheDocument();
    expect(mocks.getVacationQuota).toHaveBeenCalledTimes(2);
  });

  it("reports out-of-range values in the alert and at the field without saving", async () => {
    render(
      <AbwesenheitenTab
        staffId="4"
        canEdit
        canEditQuota
        canManageSickReports
      />,
    );
    const fields = await openQuotaEditor();

    fireEvent.change(fields.entitled, { target: { value: "367" } });
    fireEvent.change(fields.carryover, { target: { value: "" } });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    const alerts = screen.getAllByRole("alert").map((a) => a.textContent);
    expect(alerts).toContain("Bitte prüfen Sie die markierten Felder.");
    expect(
      alerts.filter(
        (a) => a === "Bitte eine Zahl zwischen 0 und 366 eingeben.",
      ),
    ).toHaveLength(2);
    // #3256: ohne Begründung wird nichts gespeichert.
    expect(alerts).toContain(
      "Bitte kurz sagen, warum sich der Anspruch ändert.",
    );
    expect(fields.entitled).toHaveAttribute("aria-invalid", "true");
    expect(fields.carryover).toHaveAttribute("aria-invalid", "true");
    expect(mocks.setVacationQuota).not.toHaveBeenCalled();
    expect(stable.toast.error).not.toHaveBeenCalled();
  });

  it("shows a failed save in the alert, not as a toast, and stays in the edit state", async () => {
    mocks.setVacationQuota.mockRejectedValue(new Error("Keine Berechtigung."));
    render(
      <AbwesenheitenTab
        staffId="4"
        canEdit
        canEditQuota
        canManageSickReports
      />,
    );
    const fields = await openQuotaEditor();

    fireEvent.change(fields.entitled, { target: { value: "28" } });
    fireEvent.change(screen.getByLabelText("Begründung"), {
      target: { value: "Teilzeit" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Keine Berechtigung.",
    );
    expect(screen.getByLabelText("Jahresanspruch (Tage)")).toHaveValue(28);
    expect(stable.toast.error).not.toHaveBeenCalled();
    expect(mocks.getVacationQuota).toHaveBeenCalledTimes(1);
  });

  it("discards the draft on cancel", async () => {
    render(
      <AbwesenheitenTab
        staffId="4"
        canEdit
        canEditQuota
        canManageSickReports
      />,
    );
    const fields = await openQuotaEditor();

    fireEvent.change(fields.entitled, { target: { value: "5" } });
    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));

    expect(
      screen.queryByLabelText("Jahresanspruch (Tage)"),
    ).not.toBeInTheDocument();
    expect(mocks.setVacationQuota).not.toHaveBeenCalled();
    // Die Karte zeigt weiter den gespeicherten Stand (30 + 2).
    expect(screen.getByText("30 + 2 aus dem Vorjahr")).toBeInTheDocument();

    const reopened = await openQuotaEditor();
    expect(reopened.entitled).toHaveValue(30);
  });

  it("hides the entry point without time_tracking:manage", async () => {
    render(
      <AbwesenheitenTab
        staffId="4"
        canEdit
        canEditQuota={false}
        canManageSickReports
      />,
    );

    expect(await screen.findByText("Übrig")).toBeInTheDocument();
    expect(screen.queryByRole("button", EDIT_BUTTON)).not.toBeInTheDocument();
  });
});

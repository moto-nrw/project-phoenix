import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type {
  BulkSubstitutionResponse,
  EnrichedInstance,
  WeeklyInstancesResponse,
} from "~/lib/timetable-types";

// Der Kit-Datepicker wird durch native Inputs ersetzt: die Tests hier prüfen
// die Geschäftsregeln (welche Tage gesendet werden, wann der Save blockt),
// nicht das Kalender-Overlay.
vi.mock("~/components/ui/date-picker", async (importOriginal) => {
  const { isoDatePickerMock } = await import("~/test/mocks/date-picker");
  return { ...(await importOriginal<object>()), ...isoDatePickerMock() };
});

const {
  mockGetWeek,
  mockApplyBulk,
  mockToastSuccess,
  mockToastError,
  mockUseTenantMutateMatching,
  mockClearPreviewCache,
  mockSwrState,
} = vi.hoisted(() => {
  const mockClearPreviewCache = vi.fn(() => Promise.resolve());
  return {
    mockGetWeek: vi.fn(),
    mockApplyBulk: vi.fn(),
    mockToastSuccess: vi.fn(),
    mockToastError: vi.fn(),
    mockClearPreviewCache,
    mockUseTenantMutateMatching: vi.fn(() => mockClearPreviewCache),
    // Steuerbarer Lade-/Fehlerzustand des Vorschau-Fetches. data bleibt dabei
    // absichtlich gefüllt — das spiegelt keepPreviousData: beim
    // Zeitraumwechsel stehen die Tage des ALTEN Zeitraums noch in data,
    // während der neue lädt.
    mockSwrState: {
      isLoading: false,
      error: undefined as Error | undefined,
    },
  };
});

vi.mock("~/lib/timetable-api", () => ({
  timetableService: {
    getWeek: mockGetWeek,
    applyBulkSubstitution: mockApplyBulk,
  },
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: mockToastSuccess,
    error: mockToastError,
  }),
}));

// useSWRAuth wird auf einen direkten Fetcher-Durchlauf reduziert: key null →
// kein Abruf; sonst synchron aufgelöste Daten aus mockGetWeek.
vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null) => {
    if (key === null) {
      return { data: undefined, isLoading: false, error: undefined };
    }
    return {
      data: mockGetWeek() as WeeklyInstancesResponse,
      isLoading: mockSwrState.isLoading,
      error: mockSwrState.error,
    };
  },
  useTenantMutateMatching: mockUseTenantMutateMatching,
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({ error: vi.fn(), info: vi.fn(), warn: vi.fn() }),
}));

// Der Mock macht das closeDisabled-Prop als data-Attribut sichtbar; dass
// FormModal damit wirklich alle Schließwege blockiert, prüft der eigene
// FormModal-Test.
vi.mock("~/components/ui/form-modal", () => ({
  FormModal: ({
    isOpen,
    children,
    footer,
    closeDisabled,
  }: {
    isOpen: boolean;
    children: ReactNode;
    footer?: ReactNode;
    closeDisabled?: boolean;
  }) =>
    isOpen ? (
      <div
        data-testid="form-modal"
        data-close-disabled={closeDisabled ? "true" : "false"}
      >
        {children}
        {footer}
      </div>
    ) : null,
}));

import { BulkSubstitutionModal } from "./bulk-substitution-modal";
import { berlinTodayISO, parseISODate } from "~/lib/date-helpers";

const STAFF_OPTIONS = [
  { id: "11", name: "Anna Alt" },
  { id: "12", name: "Bernd Neu" },
  { id: "13", name: "Carla Klar" },
];

/**
 * ISO-Datum `offset` Tage nach heute — hält die Fixtures zukunftsfest.
 * Berlin-verankert wie der "heute"-Anker der Komponente, damit die Tests um
 * Mitternacht in Nicht-Berlin-Zeitzonen nicht kippen.
 */
function futureISO(offset: number): string {
  const d = parseISODate(berlinTodayISO());
  d.setDate(d.getDate() + offset);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

function makeInstance(overrides: Partial<EnrichedInstance>): EnrichedInstance {
  return {
    id: "1",
    date: futureISO(1),
    startTime: "14:00",
    endTime: "15:00",
    title: "Malen-AG",
    status: "planned",
    isSpontaneous: false,
    isLive: false,
    activityType: "activity",
    roomId: "1",
    roomName: "Atelier",
    staff: [
      { staffId: "11", isPrimary: true, isAbsent: false, isSubstitute: false },
    ],
    studentIds: [],
    students: [],
    staffCount: 1,
    absentStaffCount: 0,
    expectedStudentsCount: 0,
    notScheduledStudentsCount: 0,
    presentStudentsCount: 0,
    ...overrides,
  } as unknown as EnrichedInstance;
}

function weekResponse(instances: EnrichedInstance[]): WeeklyInstancesResponse {
  return { instances } as unknown as WeeklyInstancesResponse;
}

function bulkResponse(
  overrides: Partial<BulkSubstitutionResponse> = {},
): BulkSubstitutionResponse {
  return {
    days: [
      { date: futureISO(1), affectedInstances: [], warningCount: 0 },
      { date: futureISO(2), affectedInstances: [], warningCount: 0 },
    ],
    totalAffected: 3,
    warningCount: 0,
    ...overrides,
  };
}

function renderModal() {
  const onClose = vi.fn();
  const onSaved = vi.fn();
  render(
    <BulkSubstitutionModal
      isOpen
      onClose={onClose}
      staffOptions={STAFF_OPTIONS}
      staffLoadError={false}
      onSaved={onSaved}
    />,
  );
  return { onClose, onSaved };
}

function pickOption(triggerName: string, optionName: string) {
  fireEvent.click(screen.getByRole("combobox", { name: triggerName }));
  fireEvent.click(screen.getByRole("option", { name: optionName }));
}

function setRange(fromISO: string, toISO: string) {
  fireEvent.change(screen.getByLabelText("Von"), {
    target: { value: fromISO },
  });
  fireEvent.change(screen.getByLabelText("Bis"), { target: { value: toISO } });
}

describe("BulkSubstitutionModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetWeek.mockReturnValue(weekResponse([]));
    mockClearPreviewCache.mockResolvedValue(undefined);
    mockSwrState.isLoading = false;
    mockSwrState.error = undefined;
  });

  it("zeigt ohne gewählte Person keine Terminvorschau und blockt den Save", () => {
    renderModal();
    expect(screen.queryByText(/Betroffene Termine/)).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Speichern" })).toBeDisabled();
  });

  it("gruppiert die Termine der Person nach Tagen und ignoriert fremde/abgesagte Termine", () => {
    mockGetWeek.mockReturnValue(
      weekResponse([
        makeInstance({ id: "1", date: futureISO(1), title: "Malen-AG" }),
        makeInstance({
          id: "2",
          date: futureISO(1),
          startTime: "09:00",
          title: "Randstunde",
        }),
        makeInstance({ id: "3", date: futureISO(2), title: "Lernzeit 2" }),
        // Fremder Termin (andere Person) und abgesagter Termin bleiben draußen.
        makeInstance({
          id: "4",
          date: futureISO(2),
          title: "Fremd",
          staff: [
            {
              staffId: "12",
              isPrimary: true,
              isAbsent: false,
              isSubstitute: false,
            },
          ],
        }),
        makeInstance({
          id: "5",
          date: futureISO(2),
          title: "Abgesagt",
          status: "cancelled",
        }),
      ]),
    );
    renderModal();
    pickOption("Abwesende Person", "Anna Alt");
    setRange(futureISO(1), futureISO(3));

    expect(screen.getByText(/09:00 Randstunde/)).toBeInTheDocument();
    expect(screen.getByText(/Lernzeit 2/)).toBeInTheDocument();
    expect(screen.queryByText(/Fremd/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Abgesagt/)).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Für 2 Tag(e) speichern" }),
    ).toBeInTheDocument();
  });

  it("sendet nur die ausgewählten Tage mit Ersatzperson und Grund", async () => {
    mockGetWeek.mockReturnValue(
      weekResponse([
        makeInstance({ id: "1", date: futureISO(1) }),
        makeInstance({ id: "2", date: futureISO(2) }),
        makeInstance({ id: "3", date: futureISO(3) }),
      ]),
    );
    mockApplyBulk.mockResolvedValue(bulkResponse());
    const { onClose, onSaved } = renderModal();

    pickOption("Abwesende Person", "Anna Alt");
    setRange(futureISO(1), futureISO(3));
    pickOption("Ersatzperson", "Bernd Neu");
    fireEvent.change(screen.getByLabelText("Grund (optional)"), {
      target: { value: "Krankheit" },
    });
    // Zweiten Tag abwählen.
    const checkboxes = screen.getAllByRole("checkbox");
    fireEvent.click(checkboxes[1]!);

    fireEvent.click(
      screen.getByRole("button", { name: "Für 2 Tag(e) speichern" }),
    );

    await waitFor(() => expect(mockApplyBulk).toHaveBeenCalledOnce());
    expect(mockApplyBulk).toHaveBeenCalledWith({
      absentStaffId: "11",
      substituteStaffId: "12",
      dates: [futureISO(1), futureISO(3)],
      reason: "Krankheit",
    });
    await waitFor(() => expect(onSaved).toHaveBeenCalledOnce());
    expect(onClose).toHaveBeenCalledOnce();
    expect(mockToastSuccess).toHaveBeenCalledWith(
      expect.stringContaining("Vertretung eingetragen"),
    );
  });

  it("meldet ohne Ersatzperson eine Abwesenheit", async () => {
    mockGetWeek.mockReturnValue(
      weekResponse([makeInstance({ id: "1", date: futureISO(1) })]),
    );
    mockApplyBulk.mockResolvedValue(
      bulkResponse({
        days: [{ date: futureISO(1), affectedInstances: [], warningCount: 0 }],
        totalAffected: 1,
      }),
    );
    renderModal();

    pickOption("Abwesende Person", "Anna Alt");
    setRange(futureISO(1), futureISO(1));
    fireEvent.click(
      screen.getByRole("button", { name: "Für 1 Tag(e) speichern" }),
    );

    await waitFor(() => expect(mockApplyBulk).toHaveBeenCalledOnce());
    expect(mockApplyBulk).toHaveBeenCalledWith({
      absentStaffId: "11",
      substituteStaffId: undefined,
      dates: [futureISO(1)],
      reason: undefined,
    });
    expect(mockToastSuccess).toHaveBeenCalledWith(
      expect.stringContaining("Abwesenheit eingetragen"),
    );
  });

  it("hält das Formular bei einem Save-Fehler offen (alles-oder-nichts)", async () => {
    mockGetWeek.mockReturnValue(
      weekResponse([makeInstance({ id: "1", date: futureISO(1) })]),
    );
    mockApplyBulk.mockRejectedValue(
      new Error("die Ersatzperson ist am 18.08.2026 selbst abwesend"),
    );
    const { onClose, onSaved } = renderModal();

    pickOption("Abwesende Person", "Anna Alt");
    setRange(futureISO(1), futureISO(1));
    fireEvent.click(
      screen.getByRole("button", { name: "Für 1 Tag(e) speichern" }),
    );

    await waitFor(() => expect(mockToastError).toHaveBeenCalledOnce());
    expect(mockToastError).toHaveBeenCalledWith(
      "die Ersatzperson ist am 18.08.2026 selbst abwesend",
    );
    expect(onClose).not.toHaveBeenCalled();
    expect(onSaved).not.toHaveBeenCalled();
  });

  it("lehnt einen zu langen Vorschau-Zeitraum ab (mehr als 56 Kalendertage)", () => {
    renderModal();
    pickOption("Abwesende Person", "Anna Alt");
    setRange(futureISO(1), futureISO(60));

    expect(screen.getByText(/höchstens 56 Tage/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Speichern" })).toBeDisabled();
  });

  it("erlaubt einen Zeitraum über 31 Kalendertage, begrenzt aber die GEWÄHLTEN Tage auf 31", () => {
    // 32 Termintage in einem 40-Tage-Fenster: der Zeitraum ist gültig
    // (≤ 56 Tage Vorschau), aber die Auswahl überschreitet das Save-Limit.
    mockGetWeek.mockReturnValue(
      weekResponse(
        Array.from({ length: 32 }, (_, i) =>
          makeInstance({ id: String(i + 1), date: futureISO(i + 1) }),
        ),
      ),
    );
    renderModal();
    pickOption("Abwesende Person", "Anna Alt");
    setRange(futureISO(1), futureISO(40));

    expect(
      screen.getByText(/Höchstens 31 Tage pro Speichern/),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Für 32 Tag(e) speichern" }),
    ).toBeDisabled();

    // Einen Tag abwählen → 31 gewählte Tage sind wieder speicherbar.
    fireEvent.click(screen.getAllByRole("checkbox")[0]!);
    expect(
      screen.queryByText(/Höchstens 31 Tage pro Speichern/),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Für 31 Tag(e) speichern" }),
    ).toBeEnabled();
  });

  it("blockiert das Schließen des Modals, solange der Save läuft", async () => {
    mockGetWeek.mockReturnValue(
      weekResponse([makeInstance({ id: "1", date: futureISO(1) })]),
    );
    let resolveSave: (value: BulkSubstitutionResponse) => void = () => {};
    mockApplyBulk.mockImplementation(
      () =>
        new Promise<BulkSubstitutionResponse>((resolve) => {
          resolveSave = resolve;
        }),
    );
    renderModal();

    pickOption("Abwesende Person", "Anna Alt");
    setRange(futureISO(1), futureISO(1));
    expect(screen.getByTestId("form-modal")).toHaveAttribute(
      "data-close-disabled",
      "false",
    );

    fireEvent.click(
      screen.getByRole("button", { name: "Für 1 Tag(e) speichern" }),
    );
    expect(screen.getByTestId("form-modal")).toHaveAttribute(
      "data-close-disabled",
      "true",
    );
    expect(screen.getByRole("button", { name: "Abbrechen" })).toBeDisabled();

    resolveSave(
      bulkResponse({
        days: [{ date: futureISO(1), affectedInstances: [], warningCount: 0 }],
        totalAffected: 1,
      }),
    );
    await waitFor(() => expect(mockToastSuccess).toHaveBeenCalledOnce());
  });

  it("markiert Tage, an denen die Person bereits überall abwesend gemeldet ist", () => {
    mockGetWeek.mockReturnValue(
      weekResponse([
        makeInstance({
          id: "1",
          date: futureISO(1),
          staff: [
            {
              staffId: "11",
              isPrimary: true,
              isAbsent: true,
              isSubstitute: false,
            },
          ],
        }),
      ]),
    );
    renderModal();
    pickOption("Abwesende Person", "Anna Alt");
    setRange(futureISO(1), futureISO(1));

    expect(screen.getByText("bereits abwesend gemeldet")).toBeInTheDocument();
  });

  it("blockt den Save, solange die Vorschau des aktuellen Zeitraums noch lädt", () => {
    // keepPreviousData: beim Zeitraumwechsel stehen die Tage des ALTEN
    // Zeitraums noch in data, während der neue lädt — ein schneller Save
    // würde sonst Tage außerhalb des neu gewählten Zeitraums senden.
    mockGetWeek.mockReturnValue(
      weekResponse([makeInstance({ id: "1", date: futureISO(1) })]),
    );
    mockSwrState.isLoading = true;
    renderModal();
    pickOption("Abwesende Person", "Anna Alt");
    setRange(futureISO(1), futureISO(1));

    expect(
      screen.getByRole("button", { name: "Für 1 Tag(e) speichern" }),
    ).toBeDisabled();
  });

  it("blockt den Save, wenn die Vorschau nicht geladen werden konnte", () => {
    mockGetWeek.mockReturnValue(
      weekResponse([makeInstance({ id: "1", date: futureISO(1) })]),
    );
    mockSwrState.error = new Error("Netzwerkfehler");
    renderModal();
    pickOption("Abwesende Person", "Anna Alt");
    setRange(futureISO(1), futureISO(1));

    expect(
      screen.getByText(/Termine konnten nicht geladen werden/),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Für 1 Tag(e) speichern" }),
    ).toBeDisabled();
  });

  it("leert den Vorschau-Cache nach einem erfolgreichen Save", async () => {
    mockGetWeek.mockReturnValue(
      weekResponse([makeInstance({ id: "1", date: futureISO(1) })]),
    );
    mockApplyBulk.mockResolvedValue(
      bulkResponse({
        days: [{ date: futureISO(1), affectedInstances: [], warningCount: 0 }],
        totalAffected: 1,
      }),
    );
    renderModal();

    pickOption("Abwesende Person", "Anna Alt");
    setRange(futureISO(1), futureISO(1));
    fireEvent.click(
      screen.getByRole("button", { name: "Für 1 Tag(e) speichern" }),
    );

    // Geleert (nicht revalidiert): mit keepPreviousData würde ein erneutes
    // Öffnen desselben Zeitraums sonst die Auswahl von VOR dem Save sofort
    // wieder speicherbar anzeigen.
    await waitFor(() =>
      expect(mockClearPreviewCache).toHaveBeenCalledWith({ clear: true }),
    );
    expect(mockUseTenantMutateMatching).toHaveBeenCalledWith([
      "timetable-sammel-vertretung-",
    ]);
  });

  it("leert den Vorschau-Cache bei einem Save-Fehler nicht", async () => {
    mockGetWeek.mockReturnValue(
      weekResponse([makeInstance({ id: "1", date: futureISO(1) })]),
    );
    mockApplyBulk.mockRejectedValue(new Error("Speichern fehlgeschlagen"));
    renderModal();

    pickOption("Abwesende Person", "Anna Alt");
    setRange(futureISO(1), futureISO(1));
    fireEvent.click(
      screen.getByRole("button", { name: "Für 1 Tag(e) speichern" }),
    );

    await waitFor(() => expect(mockToastError).toHaveBeenCalledOnce());
    expect(mockClearPreviewCache).not.toHaveBeenCalled();
  });

  it("heute (Berliner Kalendertag) als Standardzeitraum vorbelegt", () => {
    renderModal();
    expect(screen.getByLabelText("Von")).toHaveValue(berlinTodayISO());
    expect(screen.getByLabelText("Bis")).toHaveValue(berlinTodayISO());
  });
});

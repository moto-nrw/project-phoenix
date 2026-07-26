import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SubstitutionSlideOver } from "./substitution-slide-over";
import { useSWRAuth } from "~/lib/swr";
import type {
  ApplyDeviationsInput,
  DeviationHistoryEvent,
  EnrichedInstance,
  InstanceStaffSummary,
} from "~/lib/timetable-types";

type ApplyFn = (input: ApplyDeviationsInput) => Promise<boolean>;

function applyMock(result = true) {
  return vi.fn<ApplyFn>(async () => result);
}

// The editor's Verlauf reiter fetches the change log through useSWRAuth; the
// Bearbeiten reiter never touches it (Radix unmounts the inactive tab). Mocking
// the hook + api client keeps both paths hermetic.
vi.mock("~/lib/swr", () => ({
  useSWRAuth: vi.fn(),
}));

vi.mock("~/lib/timetable-api", () => ({
  timetableService: {
    getDeviationHistory: vi.fn(),
  },
}));

const mockUseSWRAuth = vi.mocked(useSWRAuth);

function mockHistory(events: DeviationHistoryEvent[]) {
  mockUseSWRAuth.mockReturnValue({
    data: { events },
    isLoading: false,
    error: undefined,
    mutate: vi.fn(),
    isValidating: false,
  } as unknown as ReturnType<typeof useSWRAuth>);
}

// A far-future date keeps `isPast` false regardless of the wall clock, so the
// editing controls render without fake timers.
const FUTURE_DATE = "2099-09-21";

function plannedPerson(
  overrides: Partial<InstanceStaffSummary> = {},
): InstanceStaffSummary {
  return {
    staffId: "11",
    isPrimary: true,
    isAbsent: false,
    isSubstitute: false,
    ...overrides,
  };
}

function makeInstance(
  overrides: Partial<EnrichedInstance> = {},
): EnrichedInstance {
  return {
    id: "42",
    date: FUTURE_DATE,
    startTime: "14:00",
    endTime: "15:00",
    title: "Malen-AG",
    status: "planned",
    isSpontaneous: false,
    isLive: false,
    activityGroupId: "7",
    activityType: "activity",
    roomId: "1",
    roomName: "Atelier",
    staff: [plannedPerson()],
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

const STAFF_NAMES = new Map([
  ["11", "Anna Alt"],
  ["12", "Bernd Neu"],
  ["13", "Carla Klar"],
]);

const STAFF_OPTIONS = [
  { id: "12", name: "Bernd Neu" },
  { id: "13", name: "Carla Klar" },
];

interface RenderOptions {
  instance?: EnrichedInstance;
  dayAbsentStaffIds?: ReadonlySet<string>;
  canManage?: boolean;
  onApply?: ApplyFn;
  onClose?: () => void;
  initialTab?: "bearbeiten" | "verlauf";
}

function renderEditor(opts: RenderOptions = {}) {
  const onApply = opts.onApply ?? applyMock(true);
  const onClose = opts.onClose ?? vi.fn<() => void>();
  render(
    <SubstitutionSlideOver
      instance={opts.instance ?? makeInstance()}
      staffOptions={STAFF_OPTIONS}
      staffNames={STAFF_NAMES}
      dayAbsentStaffIds={opts.dayAbsentStaffIds ?? new Set()}
      canManage={opts.canManage ?? true}
      onClose={onClose}
      onApply={onApply}
      initialTab={opts.initialTab}
    />,
  );
  return { onApply, onClose };
}

function markAbsent() {
  fireEvent.click(screen.getByRole("button", { name: /Abwesend/ }));
}

function pickSubstitute(triggerName: string, optionName: string) {
  fireEvent.click(screen.getByRole("combobox", { name: triggerName }));
  fireEvent.click(screen.getByRole("option", { name: optionName }));
}

describe("SubstitutionSlideOver", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockHistory([]);
  });

  describe("Radio-Zweige (Absage-Exklusivität)", () => {
    it("blendet die Besetzungs-Kontrollen aus, sobald 'Block absagen' gewählt ist, und wieder ein", () => {
      renderEditor();

      // Zweig A (Default): Personenliste sichtbar, kein Absage-Grund.
      expect(screen.getByText("Anna Alt")).toBeInTheDocument();
      expect(
        screen.getByRole("radio", { name: /Besetzung bearbeiten/ }),
      ).toBeChecked();

      // Zweig B: Personenliste weg, Absage-Grund da.
      fireEvent.click(screen.getByRole("radio", { name: /Block absagen/ }));
      expect(screen.queryByText("Anna Alt")).not.toBeInTheDocument();
      expect(screen.getByText(/Der Termin wird abgesagt/)).toBeInTheDocument();

      // Zurück zu Zweig A: Personenliste wieder da, Absage-Grund weg.
      fireEvent.click(
        screen.getByRole("radio", { name: /Besetzung bearbeiten/ }),
      );
      expect(screen.getByText("Anna Alt")).toBeInTheDocument();
      expect(
        screen.queryByText(/Der Termin wird abgesagt/),
      ).not.toBeInTheDocument();
    });
  });

  describe("Ein-Request-Payload", () => {
    it("überträgt Abwesenheit, Ersatz und Grund in EINEM onApply-Aufruf", async () => {
      const onApply = applyMock(true);
      renderEditor({ onApply });

      markAbsent();
      fireEvent.click(screen.getByRole("button", { name: /Grund hinzufügen/ }));
      fireEvent.change(screen.getByPlaceholderText("Grund (optional)"), {
        target: { value: "krank" },
      });
      pickSubstitute("Vertretung für Anna Alt", "Bernd Neu");

      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

      await waitFor(() => expect(onApply).toHaveBeenCalledTimes(1));
      expect(onApply).toHaveBeenCalledWith({
        substitutions: [
          {
            absentStaffId: "11",
            substituteStaffId: "12",
            reason: "krank",
          },
        ],
      });
    });

    it("hält das Formular offen und behält die Eingaben, wenn onApply false liefert", async () => {
      const onApply = applyMock(false);
      const onClose = vi.fn<() => void>();
      renderEditor({ onApply, onClose });

      markAbsent();
      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

      await waitFor(() => expect(onApply).toHaveBeenCalledTimes(1));
      expect(onClose).not.toHaveBeenCalled();
      // Panel bleibt offen: die Bearbeiten-Steuerung ist weiter da und die
      // Person weiter als abwesend markiert.
      expect(
        screen.getByRole("button", { name: "Speichern" }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("combobox", { name: "Vertretung für Anna Alt" }),
      ).toBeInTheDocument();
    });
  });

  describe("Ersatz-Ausschluss", () => {
    it("bietet tagesweit abwesende Personen in keiner Ersatz-Auswahl an", () => {
      renderEditor({ dayAbsentStaffIds: new Set(["12"]) });

      markAbsent();
      fireEvent.click(
        screen.getByRole("combobox", { name: "Vertretung für Anna Alt" }),
      );

      expect(
        screen.getByRole("option", { name: "Carla Klar" }),
      ).toBeInTheDocument();
      expect(
        screen.queryByRole("option", { name: "Bernd Neu" }),
      ).not.toBeInTheDocument();
    });
  });

  describe("Bewusst unbesetzt", () => {
    it("ist nur bei projizierter Unterbesetzung aktivierbar", () => {
      renderEditor();

      const checkbox = screen.getByRole("checkbox", {
        name: /Bewusst unbesetzt/,
      });
      // Voll besetzt (Anna anwesend) → deaktiviert.
      expect(checkbox).toBeDisabled();

      // Anna abwesend, kein Ersatz → unterbesetzt → aktivierbar.
      markAbsent();
      expect(
        screen.getByRole("checkbox", { name: /Bewusst unbesetzt/ }),
      ).toBeEnabled();
    });
  });

  describe("Verlaufs-Reiter", () => {
    it("ist über den Reiter erreichbar und startet mit initialTab='verlauf' dort", () => {
      mockHistory([
        makeEvent({
          eventType: "absence",
          subjectStaffId: "11",
          subjectStaffName: "Anna Alt",
        }),
      ]);

      // Direkt auf dem Verlaufs-Reiter geöffnet.
      renderEditor({ initialTab: "verlauf" });
      expect(screen.getByText("Abwesenheit eingetragen")).toBeInTheDocument();
      // Der Bearbeiten-Reiter ist inaktiv (Radix hängt ihn aus).
      expect(screen.queryByText("Personal")).not.toBeInTheDocument();
    });

    it("beschreibt einen Personal-Move mit Quelle und Ziel (#1884)", () => {
      mockHistory([
        makeEvent({
          eventType: "staff_moved",
          subjectStaffId: "11",
          subjectStaffName: "Anna Alt",
          oldValue: { from_instance_id: 189, from_title: "Schulhof" },
          newValue: { to_instance_id: 190, to_title: "Mensa" },
        }),
      ]);
      renderEditor({ initialTab: "verlauf" });
      expect(screen.getByText("Person verschoben")).toBeInTheDocument();
      expect(
        screen.getByText(
          "Anna Alt wurde von „Schulhof“ nach „Mensa“ verschoben.",
        ),
      ).toBeInTheDocument();
    });

    it("beschreibt eine Pool-Zuweisung ohne Quellblock (#1884)", () => {
      mockHistory([
        makeEvent({
          eventType: "staff_moved",
          subjectStaffId: "12",
          subjectStaffName: "Bernd Neu",
          newValue: { to_instance_id: 190, to_title: "Mensa" },
        }),
      ]);
      renderEditor({ initialTab: "verlauf" });
      expect(
        screen.getByText(
          "Bernd Neu wurde „Mensa“ aus dem Personalpool zugewiesen.",
        ),
      ).toBeInTheDocument();
    });

    it("wechselt per Reiterklick vom Bearbeiten- zum Verlaufs-Reiter", () => {
      mockHistory([
        makeEvent({ eventType: "absence", subjectStaffName: "Anna Alt" }),
      ]);
      renderEditor();

      // Startet auf Bearbeiten.
      expect(screen.getByText("Personal")).toBeInTheDocument();

      // Radix-Tabs aktivieren via mouseDown, nicht click.
      fireEvent.mouseDown(screen.getByRole("tab", { name: "Verlauf" }), {
        button: 0,
      });
      expect(screen.getByText("Abwesenheit eingetragen")).toBeInTheDocument();
    });

    it("springt auf Bearbeiten zurück, wenn initialTab ohne Blockwechsel kippt", () => {
      mockHistory([
        makeEvent({ eventType: "absence", subjectStaffName: "Anna Alt" }),
      ]);
      const props = {
        instance: makeInstance(),
        staffOptions: STAFF_OPTIONS,
        staffNames: STAFF_NAMES,
        dayAbsentStaffIds: new Set<string>(),
        canManage: true,
        onClose: vi.fn<() => void>(),
        onApply: applyMock(true),
      };
      const { rerender } = render(
        <SubstitutionSlideOver {...props} initialTab="bearbeiten" />,
      );

      // Nutzer öffnet den Verlauf; der Elternteil spiegelt das in die URL
      // und reicht initialTab="verlauf" zurück (onTabChange-Pfad).
      fireEvent.mouseDown(screen.getByRole("tab", { name: "Verlauf" }), {
        button: 0,
      });
      rerender(<SubstitutionSlideOver {...props} initialTab="verlauf" />);
      expect(screen.getByText("Abwesenheit eingetragen")).toBeInTheDocument();

      // Erneuter Klick auf denselben Block: openEditor räumt verlauf aus der
      // URL, instance.id bleibt gleich — nur initialTab kippt zurück. Der
      // Editor muss im Bearbeiten-Reiter landen, nicht im Verlauf kleben.
      rerender(<SubstitutionSlideOver {...props} initialTab="bearbeiten" />);
      expect(screen.getByText("Personal")).toBeInTheDocument();
      expect(
        screen.queryByText("Abwesenheit eingetragen"),
      ).not.toBeInTheDocument();
    });

    it("zeigt Ereigniszeilen mit Beschreibung, Grund und Akteur", () => {
      mockHistory([
        makeEvent({
          eventType: "substitution",
          subjectStaffId: "11",
          subjectStaffName: "Anna Alt",
          relatedStaffId: "12",
          relatedStaffName: "Bernd Neu",
          actorName: "Clara Chef",
          reason: "krank",
        }),
      ]);
      renderEditor({ initialTab: "verlauf" });

      expect(screen.getByText("Vertretung zugewiesen")).toBeInTheDocument();
      expect(
        screen.getByText("Bernd Neu vertritt Anna Alt."),
      ).toBeInTheDocument();
      expect(screen.getByText(/Begründung: krank/)).toBeInTheDocument();
      expect(screen.getByText(/Clara Chef/)).toBeInTheDocument();
    });

    it("fällt auf 'Unbekanntes Konto' zurück, wenn der Akteur fehlt", () => {
      mockHistory([makeEvent({ subjectStaffName: "Anna Alt" })]);
      renderEditor({ initialTab: "verlauf" });

      expect(screen.getByText(/Unbekanntes Konto/)).toBeInTheDocument();
    });

    it("zeigt den Leerzustand, wenn nichts protokolliert ist", () => {
      mockHistory([]);
      renderEditor({ initialTab: "verlauf" });

      expect(
        screen.getByText(/sind noch keine Änderungen protokolliert/),
      ).toBeInTheDocument();
    });

    describe("Kontext-Chip (Slot-Anker)", () => {
      it("nennt im Block-Scope Position, Wochentag und Startzeit; im Tages-Scope das Datum", () => {
        mockHistory([makeEvent({ eventType: "absence" })]);
        // Montag 2026-07-13 → "montags".
        renderEditor({
          instance: makeInstance({ date: "2026-07-13", title: "Mensa" }),
          initialTab: "verlauf",
        });

        expect(
          screen.getByText("Diese Position: Mensa, montags 14:00"),
        ).toBeInTheDocument();

        // Scope-Umschaltung auf "Ganzer Tag" → Datum statt Position.
        fireEvent.mouseDown(screen.getByRole("tab", { name: "Ganzer Tag" }), {
          button: 0,
        });
        expect(screen.getByText("13.07.2026")).toBeInTheDocument();
        expect(
          screen.queryByText("Diese Position: Mensa, montags 14:00"),
        ).not.toBeInTheDocument();
      });

      it("blendet die Scope-Tabs für spontane Blöcke ohne Slot-Anker aus", () => {
        mockHistory([makeEvent({ eventType: "absence" })]);
        renderEditor({
          instance: makeInstance({
            date: "2026-07-13",
            activityGroupId: undefined,
          }),
          initialTab: "verlauf",
        });

        expect(
          screen.queryByRole("tab", { name: "Dieser Block" }),
        ).not.toBeInTheDocument();
        // Nur der Tages-Scope existiert → Datum-Chip.
        expect(screen.getByText("13.07.2026")).toBeInTheDocument();
      });
    });

    describe("Vorher/Nachher-Hinweis (oldValue/newValue)", () => {
      it("rendert bei einer Vertretung die aufgelöste Ersatzperson", () => {
        mockHistory([
          makeEvent({
            eventType: "substitution",
            newValue: { is_absent: true, substitute_staff_id: 12 },
          }),
        ]);
        renderEditor({ initialTab: "verlauf" });

        expect(
          screen.getByText("Vorher: keine Vertretung → Nachher: Bernd Neu"),
        ).toBeInTheDocument();
      });

      it("fällt auf 'Unbekannte Person' zurück, wenn die Ersatzperson nicht auflösbar ist", () => {
        mockHistory([
          makeEvent({
            eventType: "substitution",
            newValue: { is_absent: true, substitute_staff_id: 99 },
          }),
        ]);
        renderEditor({ initialTab: "verlauf" });

        expect(
          screen.getByText(
            "Vorher: keine Vertretung → Nachher: Unbekannte Person",
          ),
        ).toBeInTheDocument();
      });

      it("rendert bei einer Absage den Statuswechsel", () => {
        mockHistory([
          makeEvent({
            eventType: "cancellation",
            oldValue: { status: "planned" },
            newValue: { status: "cancelled" },
          }),
        ]);
        renderEditor({ initialTab: "verlauf" });

        expect(
          screen.getByText("Status: Geplant → abgesagt"),
        ).toBeInTheDocument();
      });

      it("zeigt keinen Vorher/Nachher-Hinweis für Ereignistypen ohne verständliches Paar", () => {
        mockHistory([
          makeEvent({
            eventType: "absence",
            subjectStaffName: "Anna Alt",
            oldValue: { is_absent: false },
            newValue: { is_absent: true },
          }),
        ]);
        renderEditor({ initialTab: "verlauf" });

        expect(screen.queryByText(/Vorher:/)).not.toBeInTheDocument();
        expect(screen.queryByText(/Status:/)).not.toBeInTheDocument();
      });
    });
  });
});

function makeEvent(
  overrides: Partial<DeviationHistoryEvent>,
): DeviationHistoryEvent {
  return {
    id: "1",
    occurrenceDate: "2026-07-13",
    startTime: "14:00",
    eventType: "absence",
    occurredAt: "2026-07-12T09:30:00+02:00",
    ...overrides,
  };
}

import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";
import type { RosterImpact } from "~/lib/roster-impact";
import { CarePlanImpact } from "./care-plan-impact";

const mocks = vi.hoisted(() => ({
  useSession: vi.fn(),
  removeStudentTemplateDays: vi.fn(),
  resetStudentPickupToOffering: vi.fn(),
  berlinToday: { current: "2026-09-09" },
}));

vi.mock("next-auth/react", () => ({ useSession: mocks.useSession }));
vi.mock("~/lib/roster-impact", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/roster-impact")>()),
  removeStudentTemplateDays: mocks.removeStudentTemplateDays,
}));
vi.mock("~/lib/pickup-schedule-api", () => ({
  resetStudentPickupToOffering: mocks.resetStudentPickupToOffering,
}));
vi.mock("~/lib/hooks/use-berlin-today", () => ({
  useBerlinToday: () => mocks.berlinToday.current,
}));

const impact: RosterImpact = {
  effectiveFrom: "2027-05-31",
  pickupOverrides: [],
  adjusted: [
    {
      templateId: "7",
      name: "Kurzbetreuung",
      removedWeekdays: [5],
      addedWeekdays: [],
    },
  ],
  notAdjusted: [
    {
      templateId: "9",
      name: "RDST 1b",
      weekdays: [5],
      viaTarget: false,
      until: null,
    },
  ],
};

// Klassen-Regeltermin: Entfernen hält nicht, der Weg ist „Mit Angebot verbinden“.
const classImpact: RosterImpact = {
  effectiveFrom: "2027-05-31",
  pickupOverrides: [],
  adjusted: [],
  notAdjusted: [
    {
      templateId: "11",
      name: "Klasse 2a",
      weekdays: [3],
      viaTarget: true,
      until: null,
    },
  ],
};

// Zwei Zielgruppen-Regeltermine und eine Hand-Liste: der Hinweis steht einmal.
const twoClassesImpact: RosterImpact = {
  effectiveFrom: "2027-05-31",
  pickupOverrides: [],
  adjusted: [],
  notAdjusted: [
    {
      templateId: "9",
      name: "RDST 1b",
      weekdays: [5],
      viaTarget: false,
      until: null,
    },
    {
      templateId: "11",
      name: "Klasse 2a",
      weekdays: [3],
      viaTarget: true,
      until: null,
    },
    {
      templateId: "12",
      name: "Jahrgang 1",
      weekdays: [3],
      viaTarget: true,
      until: null,
    },
  ],
};

const SHARED_HINT =
  "Regeltermine für eine Klasse, einen Jahrgang oder eine Gruppe planen alle Kinder darin ein. Bei der nächsten Planung steht das Kind wieder im Plan.";

// Ohne zweites Argument rendert das Kind "42"; ein ausdrückliches
// `undefined` bleibt erhalten und rendert ohne Kind-ID.
function renderImpact(
  value: RosterImpact = impact,
  ...rest: [studentId?: string | undefined]
) {
  const studentId = rest.length > 0 ? rest[0] : "42";
  return render(
    <ToastProvider>
      <CarePlanImpact impact={value} studentId={studentId} />
    </ToastProvider>,
  );
}

// Schule am Berg, November 2026: Montag und Donnerstag hatten 14:30 Uhr von
// Hand, die neue Buchung setzt 16:00 Uhr.
const pickupImpact: RosterImpact = {
  effectiveFrom: "2026-10-30",
  adjusted: [],
  notAdjusted: [],
  pickupOverrides: [
    {
      weekday: 1,
      date: "2026-11-02",
      storedTime: "14:30",
      bookedTime: "16:00",
      resetNow: true,
    },
    {
      weekday: 4,
      date: "2026-11-05",
      storedTime: "14:30",
      bookedTime: "16:00",
      resetNow: true,
    },
  ],
};

function grantPermissions(permissions: string[]) {
  mocks.useSession.mockReturnValue({
    data: { user: { id: "1", token: "t", permissions } },
    status: "authenticated",
  });
}

function grantPlanEditing(granted: boolean) {
  mocks.useSession.mockReturnValue({
    data: {
      user: {
        id: "1",
        token: "t",
        permissions: granted ? ["schedules:manage"] : [],
      },
    },
    status: "authenticated",
  });
}

describe("CarePlanImpact", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.berlinToday.current = "2026-09-09";
    grantPlanEditing(true);
  });

  it("lists adjusted and hand-maintained Regeltermine with their weekdays", () => {
    renderImpact();

    expect(
      screen.getByRole("region", {
        name: "Auswirkung auf den Betreuungsplan ab 31.05.2027",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { level: 5, name: "Automatisch angepasst" }),
    ).toBeInTheDocument();
    expect(screen.getByText(/freitags entfernt/)).toBeInTheDocument();
    expect(
      screen.getByText("Nicht angepasst, weil von Hand gepflegt"),
    ).toBeInTheDocument();
    expect(screen.getByText(/Kind bleibt eingeplant/)).toBeInTheDocument();
    expect(
      screen.getByRole("link", {
        name: "Im Betreuungsplan ansehen: „RDST 1b“",
      }),
    ).toHaveAttribute(
      "href",
      expect.stringContaining("/betreuungsplan?view=serien&d=2027-05-31"),
    );
  });

  it("removes the child only after the two-step confirmation and marks the entry", async () => {
    mocks.removeStudentTemplateDays.mockResolvedValue(undefined);
    renderImpact();

    fireEvent.click(
      screen.getByRole("button", { name: "Dort auch entfernen: „RDST 1b“" }),
    );
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText(/RDST 1b/)).toBeInTheDocument();
    expect(mocks.removeStudentTemplateDays).not.toHaveBeenCalled();

    fireEvent.click(within(dialog).getByRole("button", { name: "Entfernen" }));
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Jetzt entfernen" }),
    );

    await waitFor(() =>
      expect(mocks.removeStudentTemplateDays).toHaveBeenCalledWith({
        templateId: "9",
        studentId: "42",
        weekdays: [5],
        effectiveFrom: "2027-05-31",
        effectiveUntil: null,
      }),
    );
    expect(await screen.findByText("Entfernt")).toBeInTheDocument();
    expect(screen.getByText(/Kind entfernt\./)).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /^Dort auch entfernen/ }),
    ).not.toBeInTheDocument();
  });

  it("removes only until another booking carries the day again (#5497)", async () => {
    mocks.removeStudentTemplateDays.mockResolvedValue(undefined);
    renderImpact({
      effectiveFrom: "2027-05-31",
      pickupOverrides: [],
      adjusted: [],
      notAdjusted: [
        {
          templateId: "9",
          name: "RDST 1b",
          weekdays: [5],
          viaTarget: false,
          until: "2027-06-14",
        },
      ],
    });

    expect(screen.getByText(/freitags bis 13\.06\.2027:/)).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Dort auch entfernen: „RDST 1b“" }),
    );
    const dialog = await screen.findByRole("dialog");
    expect(
      within(dialog).getByText(/vom 31\.05\.2027 bis 13\.06\.2027 freitags/),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByText(
        /Ab 14\.06\.2027 passt eine andere Buchung wieder\./,
      ),
    ).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: "Entfernen" }));
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Jetzt entfernen" }),
    );
    await waitFor(() =>
      expect(mocks.removeStudentTemplateDays).toHaveBeenCalledWith({
        templateId: "9",
        studentId: "42",
        weekdays: [5],
        effectiveFrom: "2027-05-31",
        effectiveUntil: "2027-06-14",
      }),
    );
  });

  it("does not offer removal when the reported gap ends tomorrow", () => {
    renderImpact({
      effectiveFrom: "2026-09-09",
      pickupOverrides: [],
      adjusted: [],
      notAdjusted: [
        {
          templateId: "9",
          name: "RDST 1b",
          weekdays: [5],
          viaTarget: false,
          until: "2026-09-10",
        },
      ],
    });

    // Anker: Die geladene Zeile bleibt sichtbar, nur ihre nicht ausführbare
    // Aktion fehlt.
    expect(screen.getByText(/freitags bis 09\.09\.2026:/)).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Dort auch entfernen: „RDST 1b“" }),
    ).not.toBeInTheDocument();
  });

  it("updates the removal boundary after Berlin midnight", () => {
    const boundedImpact: RosterImpact = {
      effectiveFrom: "2026-09-09",
      pickupOverrides: [],
      adjusted: [],
      notAdjusted: [
        {
          templateId: "9",
          name: "RDST 1b",
          weekdays: [5],
          viaTarget: false,
          until: "2026-09-11",
        },
      ],
    };
    const { rerender } = renderImpact(boundedImpact);

    expect(
      screen.getByRole("button", {
        name: "Dort auch entfernen: „RDST 1b“",
      }),
    ).toBeInTheDocument();

    mocks.berlinToday.current = "2026-09-10";
    rerender(
      <ToastProvider>
        <CarePlanImpact impact={boundedImpact} studentId="42" />
      </ToastProvider>,
    );

    expect(
      screen.queryByRole("button", {
        name: "Dort auch entfernen: „RDST 1b“",
      }),
    ).not.toBeInTheDocument();
  });

  it("says who may remove the child when the person cannot edit the plan", () => {
    grantPlanEditing(false);
    renderImpact();

    expect(
      screen.getByText(
        "Ändern kann nur, wer den Betreuungsplan bearbeiten darf.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /^Dort auch entfernen/ }),
    ).not.toBeInTheDocument();
  });

  it("leads a class Regeltermin to „Mit Angebot verbinden“ instead of removing", () => {
    renderImpact(classImpact);

    expect(
      screen.getByText(
        "Regeltermine für eine Klasse, einen Jahrgang oder eine Gruppe planen alle Kinder darin ein. Bei der nächsten Planung steht das Kind wieder im Plan. Verbinden Sie den Regeltermin mit dem Angebot.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Mit Angebot verbinden: „Klasse 2a“" }),
    ).toHaveAttribute(
      "href",
      expect.stringContaining(
        "/betreuungsplan?view=serien&d=2027-05-31&verbinden=11",
      ),
    );
    expect(
      screen.queryByRole("button", { name: /^Dort auch entfernen/ }),
    ).not.toBeInTheDocument();
  });

  it("states the class hint once above the list, not in a row", () => {
    renderImpact(twoClassesImpact);

    expect(
      screen.getAllByText(
        `${SHARED_HINT} Verbinden Sie den Regeltermin mit dem Angebot.`,
      ),
    ).toHaveLength(1);
    const rows = screen.getAllByRole("listitem");
    expect(rows).toHaveLength(3);
    const [manualRow, classRow, yearRow] = rows as [
      HTMLElement,
      HTMLElement,
      HTMLElement,
    ];
    // Anker: jede Zeile ist gerendert und trägt ihre eigene Aktion.
    expect(
      within(manualRow).getByRole("button", {
        name: "Dort auch entfernen: „RDST 1b“",
      }),
    ).toBeInTheDocument();
    expect(
      within(classRow).getByRole("link", {
        name: "Mit Angebot verbinden: „Klasse 2a“",
      }),
    ).toBeInTheDocument();
    expect(
      within(yearRow).getByRole("link", {
        name: "Mit Angebot verbinden: „Jahrgang 1“",
      }),
    ).toBeInTheDocument();
    // Weder Hinweis noch eine Marke in einer Zeile (Anker: Hinweis oben, Aktionen).
    for (const row of rows) {
      expect(within(row).queryByText(/Bei der nächsten Planung/)).toBeNull();
      expect(
        within(row).queryByText(/Klasse, Jahrgang oder Gruppe/),
      ).toBeNull();
    }
  });

  it("drops the call to connect from the shared hint without plan rights", () => {
    grantPlanEditing(false);
    renderImpact(twoClassesImpact);

    // Anker: der gemeinsame Hinweis steht, nur ohne den letzten Satz.
    expect(screen.getAllByText(SHARED_HINT)).toHaveLength(1);
    expect(screen.queryByText(/Verbinden Sie/)).not.toBeInTheDocument();
    // Anker: alle drei Zeilen stehen, sie tragen weder Hinweis noch Marke.
    const rows = screen.getAllByRole("listitem");
    expect(rows).toHaveLength(3);
    for (const row of rows) {
      expect(
        within(row).getByRole("link", { name: /^Im Betreuungsplan ansehen/ }),
      ).toBeInTheDocument();
      expect(within(row).queryByText(/Bei der nächsten Planung/)).toBeNull();
      expect(
        within(row).queryByText(/Klasse, Jahrgang oder Gruppe/),
      ).toBeNull();
    }
    expect(
      screen.queryByRole("link", { name: /^Mit Angebot verbinden/ }),
    ).not.toBeInTheDocument();
  });

  it("offers the connection without a child id, but not without plan rights", () => {
    const { unmount } = renderImpact(classImpact, undefined);
    expect(
      screen.getByRole("link", { name: "Mit Angebot verbinden: „Klasse 2a“" }),
    ).toHaveAttribute(
      "href",
      expect.stringContaining(
        "/betreuungsplan?view=serien&d=2027-05-31&verbinden=11",
      ),
    );
    unmount();

    grantPlanEditing(false);
    renderImpact(classImpact, undefined);
    expect(
      screen.getByText(
        "Ändern kann nur, wer den Betreuungsplan bearbeiten darf.",
      ),
    ).toBeInTheDocument();
    // Ohne Recht fordert der Hinweis nicht zu einem Schritt auf, den die
    // Person nicht gehen kann.
    expect(
      screen.getByText(
        "Regeltermine für eine Klasse, einen Jahrgang oder eine Gruppe planen alle Kinder darin ein. Bei der nächsten Planung steht das Kind wieder im Plan.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: /^Mit Angebot verbinden/ }),
    ).not.toBeInTheDocument();
  });

  it("offers no removal from a hand-maintained Regeltermin without a child id", () => {
    renderImpact(impact, undefined);

    // Anker: der Eintrag der Hand-Liste steht mit seinem Planlink.
    expect(
      screen.getByRole("link", {
        name: "Im Betreuungsplan ansehen: „RDST 1b“",
      }),
    ).toBeInTheDocument();
    // Gleiche Abfrage wie im Entfernen-Test mit Kind-ID und Recht.
    expect(
      screen.queryByRole("button", { name: "Dort auch entfernen: „RDST 1b“" }),
    ).not.toBeInTheDocument();
  });

  it("names the child in the section when a page shows several children", () => {
    render(
      <ToastProvider>
        <CarePlanImpact impact={impact} studentId="42" childName="Mia" />
      </ToastProvider>,
    );

    expect(
      screen.getByRole("region", {
        name: "Auswirkung auf den Betreuungsplan für Mia ab 31.05.2027",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", {
        level: 4,
        name: "Auswirkung auf den Betreuungsplan für Mia ab 31.05.2027",
      }),
    ).toBeInTheDocument();
  });

  it("confirms briefly when nothing in the plan changed", () => {
    renderImpact({
      effectiveFrom: "2027-05-31",
      pickupOverrides: [],
      adjusted: [],
      notAdjusted: [],
    });

    expect(
      screen.getByText(
        "Im Betreuungsplan musste kein Regeltermin angepasst werden.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Im Betreuungsplan musste kein Regeltermin angepasst werden.",
      ),
    ).toBeVisible();
  });

  describe("entered pickup times that hide the booking", () => {
    // Die Änderung gilt schon (Stichtag 30.10., heute Freitag 06.11.); der
    // Fall mit Stichtag in der Zukunft steht in eigenen Tests.
    beforeEach(() => {
      mocks.berlinToday.current = "2026-11-06";
    });

    it("lists each weekday with the entered and the booked time", () => {
      grantPermissions(["users:update"]);
      renderImpact(pickupImpact);

      const section = screen.getByRole("region", {
        name: /Auswirkung auf den Betreuungsplan/,
      });
      expect(
        within(section).getByText("Kein Regeltermin musste angepasst werden."),
      ).toBeInTheDocument();
      expect(
        within(section).getByText("Abholzeit weicht von der Buchung ab"),
      ).toBeInTheDocument();
      expect(section).toHaveTextContent(
        "Montags: eingetragen 14:30 Uhr, gebucht 16:00 Uhr.",
      );
      expect(section).toHaveTextContent(
        "Donnerstags: eingetragen 14:30 Uhr, gebucht 16:00 Uhr.",
      );
    });

    it("takes the booked time for every weekday with one click", async () => {
      grantPermissions(["users:update"]);
      mocks.resetStudentPickupToOffering.mockResolvedValue({});
      renderImpact(pickupImpact);

      fireEvent.click(
        screen.getByRole("button", { name: "Gebuchte Zeiten übernehmen" }),
      );

      await waitFor(() =>
        expect(
          screen.getByText("Jetzt gelten die gebuchten Abholzeiten."),
        ).toBeInTheDocument(),
      );
      expect(mocks.resetStudentPickupToOffering.mock.calls).toEqual([
        ["42", 1, "2026-11-02"],
        ["42", 4, "2026-11-05"],
      ]);
      expect(screen.getAllByText("Übernommen")).toHaveLength(2);
      expect(screen.getByText(/Montags/).closest("li")).toHaveTextContent(
        "Montags: jetzt 16:00 Uhr wie gebucht.",
      );
      expect(
        screen.queryByRole("button", { name: "Gebuchte Zeiten übernehmen" }),
      ).not.toBeInTheDocument();
    });

    it("keeps the weekdays that failed open and says so", async () => {
      grantPermissions(["users:update"]);
      mocks.resetStudentPickupToOffering
        .mockResolvedValueOnce({})
        .mockRejectedValueOnce(new Error("boom"));
      renderImpact(pickupImpact);

      fireEvent.click(
        screen.getByRole("button", { name: "Gebuchte Zeiten übernehmen" }),
      );

      await waitFor(() =>
        expect(screen.getByRole("alert")).toBeInTheDocument(),
      );
      expect(screen.getAllByText("Übernommen")).toHaveLength(1);
      expect(screen.getByText(/Donnerstags/).closest("li")).toHaveTextContent(
        "Donnerstags: eingetragen 14:30 Uhr, gebucht 16:00 Uhr.",
      );
      expect(
        screen.getByRole("button", { name: "Gebuchte Zeiten übernehmen" }),
      ).toBeEnabled();
    });

    it("says who may change the time when the person may not", () => {
      grantPermissions(["schedules:manage"]);
      renderImpact(pickupImpact);

      expect(
        screen.getByText("Ändern kann nur, wer Kinder bearbeiten darf."),
      ).toBeInTheDocument();
      expect(
        screen.getByText("Abholzeit weicht von der Buchung ab"),
      ).toBeInTheDocument();
      expect(
        screen.queryByRole("button", { name: "Gebuchte Zeiten übernehmen" }),
      ).not.toBeInTheDocument();
    });

    // Eine Handzeit ohne Datum ist ab heute weg. Gilt sie bis zum Stichtag
    // noch und weicht sie von der bisherigen Buchung ab, bleibt sie stehen.
    it("resets only the weekdays that may change today", async () => {
      grantPermissions(["users:update"]);
      mocks.resetStudentPickupToOffering.mockResolvedValue({});
      renderImpact({
        ...pickupImpact,
        pickupOverrides: [
          pickupImpact.pickupOverrides[0]!,
          {
            weekday: 4,
            date: "2026-11-12",
            storedTime: "15:15",
            bookedTime: "16:00",
            resetNow: false,
          },
        ],
      });

      expect(screen.getByText("Donnerstags").closest("li")).toHaveTextContent(
        "Donnerstags: eingetragen 15:15 Uhr, ab 12.11.2026 gebucht 16:00 Uhr.",
      );
      expect(
        screen.getByText(
          "Donnerstags gilt die eingetragene Zeit bis zum genannten Tag weiter. Ab diesem Tag setzen Sie sie unter „Betreuungszeiten“ zurück.",
        ),
      ).toBeInTheDocument();

      fireEvent.click(
        screen.getByRole("button", { name: "Gebuchte Zeiten übernehmen" }),
      );

      await waitFor(() =>
        expect(screen.getAllByText("Übernommen")).toHaveLength(1),
      );
      expect(mocks.resetStudentPickupToOffering.mock.calls).toEqual([
        ["42", 1, "2026-11-02"],
      ]);
      expect(screen.getByText("Donnerstags").closest("li")).toHaveTextContent(
        "ab 12.11.2026 gebucht 16:00 Uhr.",
      );
    });

    // Bis zum Stichtag ergibt die Buchung schon die eingetragene Zeit; die
    // gebuchte neue Zeit gilt erst ab dem Stichtag. Das Datum bleibt stehen,
    // vor und nach dem Übernehmen, und die Meldung nennt es auch.
    it("keeps a future change's date before and after taking it over", async () => {
      mocks.berlinToday.current = "2026-10-28";
      grantPermissions(["users:update"]);
      mocks.resetStudentPickupToOffering.mockResolvedValue({});
      renderImpact(pickupImpact);

      expect(screen.getByText(/Montags/).closest("li")).toHaveTextContent(
        "Montags: eingetragen 14:30 Uhr, ab 02.11.2026 gebucht 16:00 Uhr.",
      );

      fireEvent.click(
        screen.getByRole("button", { name: "Gebuchte Zeiten übernehmen" }),
      );

      await waitFor(() =>
        expect(
          screen.getByText("Ab 02.11.2026 gelten die gebuchten Abholzeiten."),
        ).toBeInTheDocument(),
      );
      expect(screen.getByText(/Montags/).closest("li")).toHaveTextContent(
        "Montags: ab 02.11.2026 16:00 Uhr wie gebucht.",
      );
      expect(screen.getByText(/Donnerstags/).closest("li")).toHaveTextContent(
        "Donnerstags: ab 05.11.2026 16:00 Uhr wie gebucht.",
      );
      expect(screen.queryByText(/jetzt 16:00 Uhr/)).not.toBeInTheDocument();
    });

    it("offers no reset while every weekday still applies until the change", () => {
      grantPermissions(["users:update"]);
      renderImpact({
        ...pickupImpact,
        pickupOverrides: pickupImpact.pickupOverrides.map((override) => ({
          ...override,
          resetNow: false,
        })),
      });

      expect(
        screen.getByText("Abholzeit weicht von der Buchung ab"),
      ).toBeInTheDocument();
      expect(
        screen.queryByRole("button", { name: "Gebuchte Zeiten übernehmen" }),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByText("Ändern kann nur, wer Kinder bearbeiten darf."),
      ).not.toBeInTheDocument();
    });

    it("restores taken-over weekdays and reports each new one", async () => {
      grantPermissions(["users:update"]);
      mocks.resetStudentPickupToOffering.mockResolvedValue({});
      const onProgress = vi.fn();
      render(
        <ToastProvider>
          <CarePlanImpact
            impact={pickupImpact}
            studentId="42"
            progress={{ removed: [], resetWeekdays: [1] }}
            onProgress={onProgress}
          />
        </ToastProvider>,
      );

      expect(screen.getByText(/Montags/).closest("li")).toHaveTextContent(
        "Montags: jetzt 16:00 Uhr wie gebucht.",
      );

      fireEvent.click(
        screen.getByRole("button", { name: "Gebuchte Zeiten übernehmen" }),
      );

      await waitFor(() =>
        expect(onProgress).toHaveBeenLastCalledWith({
          removed: [],
          resetWeekdays: [1, 4],
        }),
      );
      expect(mocks.resetStudentPickupToOffering.mock.calls).toEqual([
        ["42", 4, "2026-11-05"],
      ]);
    });
  });
});

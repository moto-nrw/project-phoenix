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
}));

vi.mock("next-auth/react", () => ({ useSession: mocks.useSession }));
vi.mock("~/lib/roster-impact", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/roster-impact")>()),
  removeStudentTemplateDays: mocks.removeStudentTemplateDays,
}));

const impact: RosterImpact = {
  effectiveFrom: "2027-05-31",
  adjusted: [
    {
      templateId: "7",
      name: "Kurzbetreuung",
      removedWeekdays: [5],
      addedWeekdays: [],
    },
  ],
  notAdjusted: [
    { templateId: "9", name: "RDST 1b", weekdays: [5], viaTarget: false },
  ],
};

// Klassen-Regeltermin: Entfernen hält nicht, der Weg ist „Mit Angebot verbinden“.
const classImpact: RosterImpact = {
  effectiveFrom: "2027-05-31",
  adjusted: [],
  notAdjusted: [
    { templateId: "11", name: "Klasse 2a", weekdays: [3], viaTarget: true },
  ],
};

// Zwei Zielgruppen-Regeltermine und eine Hand-Liste: der Hinweis steht einmal.
const twoClassesImpact: RosterImpact = {
  effectiveFrom: "2027-05-31",
  adjusted: [],
  notAdjusted: [
    { templateId: "9", name: "RDST 1b", weekdays: [5], viaTarget: false },
    { templateId: "11", name: "Klasse 2a", weekdays: [3], viaTarget: true },
    { templateId: "12", name: "Jahrgang 1", weekdays: [3], viaTarget: true },
  ],
};

const SHARED_HINT =
  "Regeltermine mit „Klasse, Jahrgang oder Gruppe“ planen alle Kinder darin ein. Bei der nächsten Planung steht das Kind wieder im Plan.";

function renderImpact(
  value: RosterImpact = impact,
  studentId: string | undefined = "42",
) {
  return render(
    <ToastProvider>
      <CarePlanImpact impact={value} studentId={studentId} />
    </ToastProvider>,
  );
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
      }),
    );
    expect(await screen.findByText("Entfernt")).toBeInTheDocument();
    expect(screen.getByText(/Kind entfernt\./)).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /^Dort auch entfernen/ }),
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
        "Regeltermine mit „Klasse, Jahrgang oder Gruppe“ planen alle Kinder darin ein. Bei der nächsten Planung steht das Kind wieder im Plan. Verbinden Sie den Regeltermin mit dem Angebot.",
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

  it("states the class hint once above the list and marks each class row", () => {
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
    // Die Marke verbindet die Zeile mit dem Hinweis; die Hand-Liste trägt sie nicht.
    expect(
      within(classRow).getByText("Klasse, Jahrgang oder Gruppe"),
    ).toBeInTheDocument();
    expect(
      within(yearRow).getByText("Klasse, Jahrgang oder Gruppe"),
    ).toBeInTheDocument();
    // Anker: die Hand-Liste ist gerendert und bietet ihr Entfernen an.
    expect(
      within(manualRow).getByRole("button", {
        name: "Dort auch entfernen: „RDST 1b“",
      }),
    ).toBeInTheDocument();
    expect(
      within(manualRow).queryByText("Klasse, Jahrgang oder Gruppe"),
    ).not.toBeInTheDocument();
    // Anker: der Hinweis oben ist gerendert (erste Prüfung); in keiner Zeile.
    for (const row of rows) {
      expect(within(row).queryByText(/Bei der nächsten Planung/)).toBeNull();
    }
    expect(
      screen.getByRole("link", { name: "Mit Angebot verbinden: „Jahrgang 1“" }),
    ).toBeInTheDocument();
  });

  it("drops the call to connect from the shared hint without plan rights", () => {
    grantPlanEditing(false);
    renderImpact(twoClassesImpact);

    // Anker: der gemeinsame Hinweis steht, nur ohne den letzten Satz.
    expect(screen.getAllByText(SHARED_HINT)).toHaveLength(1);
    expect(screen.queryByText(/Verbinden Sie/)).not.toBeInTheDocument();
    expect(screen.getAllByText("Klasse, Jahrgang oder Gruppe")).toHaveLength(2);
    expect(
      screen.queryByRole("link", { name: /^Mit Angebot verbinden/ }),
    ).not.toBeInTheDocument();
  });

  it("offers the connection without a child id, but not without plan rights", () => {
    const { unmount } = renderImpact(classImpact, undefined);
    expect(
      screen.getByRole("link", { name: "Mit Angebot verbinden: „Klasse 2a“" }),
    ).toBeInTheDocument();
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
        "Regeltermine mit „Klasse, Jahrgang oder Gruppe“ planen alle Kinder darin ein. Bei der nächsten Planung steht das Kind wieder im Plan.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: /^Mit Angebot verbinden/ }),
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
});

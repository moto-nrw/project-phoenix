import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import type { SchoolSetupState } from "~/lib/school-setup-api";

const push = vi.fn();
const replace = vi.fn(() => Promise.resolve());
const refresh = vi.fn(() => Promise.resolve());
let shellUser: { id: string } | null = { id: "73" };
let setupState: SchoolSetupState | null = null;
let pathname = "/dashboard";

vi.mock("next/navigation", () => ({
  usePathname: () => pathname,
}));

vi.mock("~/lib/shell-auth-context", () => ({
  useShellAuthSafe: () =>
    shellUser ? { user: shellUser, isPreview: false } : undefined,
}));

vi.mock("~/lib/tenant-context", () => ({
  useNFCEnabled: () => false,
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push }),
}));

vi.mock("./use-school-setup", () => ({
  useSchoolSetup: () => ({
    state: setupState,
    accountID: "73",
    refresh,
    replace,
  }),
}));

const setSettingValue = vi.fn((_key: string, _value: unknown) =>
  Promise.resolve<string | null>(null),
);
vi.mock("~/lib/settings-api", () => ({
  setSettingValue: (key: string, value: unknown) => setSettingValue(key, value),
}));

const api = vi.hoisted(() => ({
  confirmSchoolSetupBasics: vi.fn(),
  setSchoolSetupStepSkipped: vi.fn(),
  completeSchoolSetup: vi.fn(),
  setSchoolSetupDismissed: vi.fn(),
}));
vi.mock("~/lib/school-setup-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/school-setup-api")>()),
  ...api,
}));

import { SchoolSetupWizard } from "./school-setup-wizard";

function newSchool(): SchoolSetupState {
  return {
    completed: false,
    dismissed: false,
    basics: {
      presenceMode: "detailed",
      groupMode: "fixed_groups",
      timetableEnabled: true,
      parentAppUsed: null,
    },
    steps: [
      { key: "basics", applies: true, done: false, skipped: false },
      { key: "team", applies: true, done: false, skipped: false },
      { key: "rooms", applies: true, done: false, skipped: false },
      { key: "groups", applies: true, done: false, skipped: false },
      { key: "students", applies: true, done: false, skipped: false },
      { key: "guardians", applies: true, done: false, skipped: false },
    ],
  };
}

/** Ein sichtbares Tour-Ziel; jsdom misst sonst alles mit Größe 0. */
function visibleTarget(tourId: string): HTMLButtonElement {
  const element = document.createElement("button");
  element.dataset.setupTour = tourId;
  element.getClientRects = () => [{}] as unknown as DOMRectList;
  element.getBoundingClientRect = () =>
    ({
      top: 10,
      left: 10,
      width: 120,
      height: 40,
      bottom: 50,
      right: 130,
    }) as DOMRect;
  document.body.appendChild(element);
  return element;
}

/** Eine sichtbare Fläche, gemessen wie ein Tour-Ziel. */
function measured<T extends HTMLElement>(element: T): T {
  element.getClientRects = () => [{}] as unknown as DOMRectList;
  element.getBoundingClientRect = () =>
    ({
      top: 10,
      left: 10,
      width: 300,
      height: 200,
      bottom: 210,
      right: 310,
    }) as DOMRect;
  document.body.appendChild(element);
  return element;
}

/** Eine Karte der Import-Seite, erkannt an einem Feld mit dieser id. */
function visibleSection(fieldID: string): HTMLElement {
  const section = measured(document.createElement("section"));
  const field = document.createElement("div");
  field.id = fieldID;
  section.appendChild(field);
  return section;
}

/**
 * Das Fenster „Neues Kind“: ein Dialog mit Schließen-Knopf und dem Formular,
 * dessen erster Block die Angaben trägt.
 */
function studentFormWindow(): HTMLElement {
  const overlay = measured(document.createElement("div"));
  overlay.setAttribute("role", "dialog");
  const close = document.createElement("button");
  close.dataset.overlayClose = "";
  close.addEventListener("click", () => overlay.remove());
  overlay.appendChild(close);
  const form = document.createElement("form");
  overlay.appendChild(form);
  const body = document.createElement("div");
  measured(body);
  form.appendChild(body);
  const submit = visibleTarget("student-submit");
  form.appendChild(submit);
  return overlay;
}

/** Die Karte „Was soll der Import tun?“ mit ihrer Auswahl. */
function visibleModeCard(): HTMLElement {
  const section = measured(document.createElement("section"));
  const control = document.createElement("div");
  control.setAttribute("aria-label", "Import-Modus");
  section.appendChild(control);
  return section;
}

/** Die Karte „Datei hochladen“ mit ihrem versteckten Dateifeld. */
function visibleUploadCard(): HTMLElement {
  const card = measured(document.createElement("div"));
  const input = document.createElement("input");
  input.type = "file";
  card.appendChild(input);
  return card;
}

/**
 * Die Karte „Neue Einladung“ wie das Fenster sie öffnet: Kopf und Formular,
 * darin unten der Senden-Knopf.
 */
function inviteForm(): HTMLButtonElement {
  const card = document.createElement("div");
  card.getClientRects = () => [{}] as unknown as DOMRectList;
  card.getBoundingClientRect = () =>
    ({
      top: 10,
      left: 10,
      width: 300,
      height: 300,
      bottom: 310,
      right: 310,
    }) as DOMRect;
  const form = document.createElement("form");
  card.appendChild(form);
  document.body.appendChild(card);
  const submit = visibleTarget("invite-submit");
  submit.getBoundingClientRect = () =>
    ({
      top: 260,
      left: 10,
      width: 300,
      height: 40,
      bottom: 300,
      right: 310,
    }) as DOMRect;
  form.appendChild(submit);
  return submit;
}

/** Erst wenn die Stelle gefunden ist, liegt die Hervorhebung auf ihr. */
async function waitForHighlight() {
  await waitFor(() =>
    expect(document.querySelector(".ring-moto-green")).not.toBeNull(),
  );
}

function afterBasics(): SchoolSetupState {
  const state = newSchool();
  state.steps[0] = { key: "basics", applies: true, done: true, skipped: false };
  return state;
}

/** Team, Räume und Gruppen sind erledigt; als Nächstes kommen die Kinder. */
function studentsNext(): SchoolSetupState {
  const state = afterBasics();
  for (const index of [1, 2, 3]) {
    const step = state.steps[index];
    if (step) state.steps[index] = { ...step, done: true };
  }
  return state;
}

describe("SchoolSetupWizard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    sessionStorage.clear();
    document.body.innerHTML = "";
    shellUser = { id: "73" };
    setupState = newSchool();
    pathname = "/dashboard";
  });

  it("opens the basics once per sign-in, afterwards only the beacon", () => {
    const { unmount } = render(<SchoolSetupWizard />);
    expect(screen.getByText("So arbeitet Ihre OGS")).toBeInTheDocument();
    unmount();

    render(<SchoolSetupWizard />);
    expect(screen.queryByText("So arbeitet Ihre OGS")).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Erste Schritte öffnen, 6 offen" }),
    ).toBeInTheDocument();
  });

  it("stays hidden for a finished school, a hidden wizard and without an account", () => {
    setupState = { ...newSchool(), completed: true };
    const { rerender } = render(<SchoolSetupWizard />);
    expect(screen.queryByText(/Erste Schritte/)).not.toBeInTheDocument();

    setupState = { ...newSchool(), dismissed: true };
    rerender(<SchoolSetupWizard />);
    expect(screen.queryByText(/Erste Schritte/)).not.toBeInTheDocument();

    setupState = newSchool();
    shellUser = null;
    rerender(<SchoolSetupWizard />);
    expect(screen.queryByText(/Erste Schritte/)).not.toBeInTheDocument();
  });

  it("explains every basic question behind a question mark", () => {
    render(<SchoolSetupWizard />);
    expect(
      screen.getByLabelText("Was heißt das? Arbeiten Sie mit festen Gruppen?"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Jedes Kind gehört zu einer Gruppe mit eigener Leitung/),
    ).toBeInTheDocument();
  });

  it("saves the answers and continues in the checklist", async () => {
    api.confirmSchoolSetupBasics.mockImplementation(async () => {
      setupState = afterBasics();
      return setupState;
    });
    render(<SchoolSetupWizard />);

    fireEvent.click(
      screen.getByRole("button", { name: "Anwesend oder abwesend" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Offene Betreuung" }));
    const parentQuestion = screen.getByRole("group", {
      name: /Sollen Eltern die Eltern-App nutzen\?/,
    });
    fireEvent.click(
      Array.from(parentQuestion.querySelectorAll("button")).find(
        (button) => button.textContent === "Nein",
      )!,
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Speichern und weiter" }),
    );

    await waitFor(() =>
      expect(api.confirmSchoolSetupBasics).toHaveBeenCalledWith(
        "binary",
        false,
      ),
    );
    expect(setSettingValue).toHaveBeenCalledWith(
      "operations.group_mode",
      "open_care",
    );
    expect(setSettingValue).toHaveBeenCalledWith("timetable.enabled", true);
    expect(
      await screen.findByRole("button", {
        name: /^Erste Person ins Team einladen/,
      }),
    ).toHaveAttribute("aria-expanded", "true");
  });

  it("asks for the parent app answer before saving", () => {
    render(<SchoolSetupWizard />);
    fireEvent.click(
      screen.getByRole("button", { name: "Speichern und weiter" }),
    );
    expect(
      screen.getByText(
        "Bitte sagen Sie noch, ob Eltern die Eltern-App nutzen sollen.",
      ),
    ).toBeInTheDocument();
    expect(api.confirmSchoolSetupBasics).not.toHaveBeenCalled();
  });

  it("shows the checklist with progress and the next open step expanded", () => {
    setupState = afterBasics();
    render(<SchoolSetupWizard />);

    const checklist = screen.getByRole("region", {
      name: "Erste Schritte mit moto",
    });
    expect(checklist).toHaveTextContent("1 von 6 erledigt");
    expect(
      screen.getByRole("progressbar", { name: "1 von 6 Schritten erledigt" }),
    ).toHaveAttribute("aria-valuenow", "1");
    expect(
      screen.getByRole("button", { name: /^Erste Person ins Team einladen/ }),
    ).toHaveAttribute("aria-expanded", "true");
    expect(
      screen.getByRole("button", { name: /^Ersten Raum anlegen/ }),
    ).toHaveAttribute("aria-expanded", "false");
    expect(
      screen.getByRole("button", { name: "Zeig es mir" }),
    ).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Checkliste einklappen" }),
    );
    expect(
      screen.getByRole("button", { name: "Erste Schritte öffnen, 5 offen" }),
    ).toBeInTheDocument();
  });

  it("hides the checklist for good only after a confirmation", async () => {
    setupState = afterBasics();
    api.setSchoolSetupDismissed.mockResolvedValue({
      ...afterBasics(),
      dismissed: true,
    });
    render(<SchoolSetupWizard />);
    expect(
      screen.getByText(
        /Die Anleitungen zu allen Schritten finden Sie jederzeit/,
      ),
    ).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Nicht mehr anzeigen" }),
    );
    expect(api.setSchoolSetupDismissed).not.toHaveBeenCalled();
    expect(
      screen.getByText(/Sie können sie nicht wieder einblenden/),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));
    expect(api.setSchoolSetupDismissed).not.toHaveBeenCalled();

    fireEvent.click(
      screen.getByRole("button", { name: "Nicht mehr anzeigen" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Ausblenden" }));
    await waitFor(() =>
      expect(api.setSchoolSetupDismissed).toHaveBeenCalledWith(true),
    );
  });

  it("offers the parent tour only once there is a child", () => {
    setupState = afterBasics();
    const { unmount } = render(<SchoolSetupWizard />);

    fireEvent.click(
      screen.getByRole("button", { name: /^Erste Eltern einladen/ }),
    );
    expect(
      screen.getByText("Dafür braucht es zuerst mindestens ein Kind in moto."),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Zeig es mir" }),
    ).not.toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Zu „Erstes Kind anlegen“" }),
    );
    expect(
      screen.getByRole("button", { name: /^Erstes Kind anlegen/ }),
    ).toHaveAttribute("aria-expanded", "true");
    unmount();

    // Mit einem Kind gibt es die Tour. Neue Sitzung, damit die Liste aufgeht.
    sessionStorage.clear();
    setupState = afterBasics();
    setupState.steps[4] = {
      key: "students",
      applies: true,
      done: true,
      skipped: false,
    };
    render(<SchoolSetupWizard />);
    fireEvent.click(
      screen.getByRole("button", { name: /^Erste Eltern einladen/ }),
    );
    expect(
      screen.queryByText(
        "Dafür braucht es zuerst mindestens ein Kind in moto.",
      ),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Zeig es mir" }),
    ).toBeInTheDocument();
  });

  it("shows how to enter a parent, and skips that when one is already there", async () => {
    setupState = afterBasics();
    setupState.steps[4] = {
      key: "students",
      applies: true,
      done: true,
      skipped: false,
    };
    pathname = "/database/students";
    const { rerender } = render(<SchoolSetupWizard />);
    fireEvent.click(
      screen.getByRole("button", { name: /^Erste Eltern einladen/ }),
    );

    const row = visibleTarget("row");
    row.setAttribute("href", "/students/5?from=%2Fdatabase%2Fstudents");
    const tab = visibleTarget("tab");
    tab.setAttribute("role", "tab");
    tab.dataset.tabValue = "erziehungsberechtigte";

    fireEvent.click(screen.getByRole("button", { name: "Zeig es mir" }));
    // Die Zeile ist ein <button> mit href-Attribut; der Selektor fragt nach <a>.
    const link = document.createElement("a");
    link.href = "/students/5?from=%2Fdatabase%2Fstudents";
    link.getClientRects = row.getClientRects;
    link.getBoundingClientRect = row.getBoundingClientRect;
    document.body.appendChild(link);

    expect(
      await screen.findByRole("dialog", { name: "Kind öffnen" }),
    ).toBeInTheDocument();
    await waitForHighlight();
    fireEvent.click(link);
    // Erst auf der Kindakte geht es weiter, nicht auf der noch sichtbaren Liste.
    await new Promise((resolve) => setTimeout(resolve, 600));
    expect(
      screen.queryByRole("dialog", { name: "Erziehungsberechtigte" }),
    ).not.toBeInTheDocument();
    pathname = "/students/5";
    rerender(<SchoolSetupWizard />);
    expect(
      await screen.findByRole("dialog", { name: "Erziehungsberechtigte" }),
    ).toBeInTheDocument();

    // Eine Person mit E-Mail-Adresse steht schon da: Eintragen entfällt.
    const menuButton = visibleTarget("guardian-menu");
    await waitForHighlight();
    fireEvent.click(tab);
    expect(
      await screen.findByRole("dialog", { name: "Menü öffnen" }),
    ).toBeInTheDocument();

    // Menü auf, „Einladen“ gewählt: Die Tour endet, und der Stand wird
    // mehrmals nachgeladen, weil die Einladung dann noch unterwegs ist.
    const menu = visibleTarget("menu");
    menu.setAttribute("role", "menu");
    await waitForHighlight();
    fireEvent.click(menuButton);
    expect(
      await screen.findByRole("dialog", { name: "Einladen" }),
    ).toBeInTheDocument();
    await waitForHighlight();
    refresh.mockClear();
    fireEvent.click(menu);
    await waitFor(() => expect(refresh).toHaveBeenCalledTimes(2), {
      timeout: 2000,
    });
  });

  it("lets the person open any step of the checklist", () => {
    setupState = afterBasics();
    render(<SchoolSetupWizard />);

    fireEvent.click(
      screen.getByRole("button", { name: /^Erste Gruppe anlegen/ }),
    );

    expect(
      screen.getByRole("button", { name: /^Erste Gruppe anlegen/ }),
    ).toHaveAttribute("aria-expanded", "true");
    expect(
      screen.getByText(/Gruppenleitung kann nur werden/),
    ).toBeInTheDocument();
  });

  it("skips a step and moves on", async () => {
    setupState = afterBasics();
    api.setSchoolSetupStepSkipped.mockImplementation(async () => {
      const next = afterBasics();
      next.steps[1] = {
        key: "team",
        applies: true,
        done: false,
        skipped: true,
      };
      setupState = next;
      return next;
    });
    render(<SchoolSetupWizard />);

    fireEvent.click(screen.getByRole("button", { name: "Überspringen" }));

    await waitFor(() =>
      expect(api.setSchoolSetupStepSkipped).toHaveBeenCalledWith("team", true),
    );
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: /^Ersten Raum anlegen/ }),
      ).toHaveAttribute("aria-expanded", "true"),
    );
  });

  it("starts the tour in the sidebar and follows the clicks to the page", async () => {
    setupState = afterBasics();
    const { rerender } = render(<SchoolSetupWizard />);
    const database = visibleTarget("nav-database");

    fireEvent.click(screen.getByRole("button", { name: "Zeig es mir" }));

    const bubble = await screen.findByRole("dialog", {
      name: "Datenverwaltung öffnen",
    });
    await waitForHighlight();
    // „Verwaltung“ ist schon offen (Datenverwaltung sichtbar): Station 1 entfällt.
    expect(bubble).toHaveTextContent("Station 2 von 6");
    expect(push).not.toHaveBeenCalled();
    expect(
      screen.queryByRole("button", { name: "Weiter" }),
    ).not.toBeInTheDocument();

    fireEvent.click(database);
    const personal = visibleTarget("nav-/database/personal");
    expect(
      await screen.findByRole("dialog", { name: "Personal öffnen" }),
    ).toBeInTheDocument();
    await waitForHighlight();

    fireEvent.click(personal);
    pathname = "/database/personal";
    rerender(<SchoolSetupWizard />);
    visibleTarget("create");
    expect(
      await screen.findByRole("dialog", { name: "Personal einladen" }),
    ).toBeInTheDocument();
  });

  it("does not skip a sidebar station whose entry sits in a folded area", async () => {
    setupState = afterBasics();
    render(<SchoolSetupWizard />);
    visibleTarget("nav-group-verwaltung");
    // Eingeklappt: Die Bereiche bleiben im Baum, sind aber inert.
    const folded = document.createElement("div");
    folded.setAttribute("inert", "");
    document.body.appendChild(folded);
    folded.appendChild(visibleTarget("nav-database"));

    fireEvent.click(screen.getByRole("button", { name: "Zeig es mir" }));

    const bubble = await screen.findByRole("dialog", {
      name: "Verwaltung öffnen",
    });
    await waitForHighlight();
    expect(bubble).toHaveTextContent("Station 1 von 6");
  });

  it("starts on the page itself when the person is already there", async () => {
    setupState = afterBasics();
    pathname = "/database/personal";
    render(<SchoolSetupWizard />);
    const create = visibleTarget("create");

    fireEvent.click(screen.getByRole("button", { name: "Zeig es mir" }));
    expect(
      await screen.findByRole("dialog", { name: "Personal einladen" }),
    ).toHaveTextContent("Station 4 von 6");
    await waitForHighlight();

    // Der Klick öffnet das Einladungsfenster: das ganze Formular ist markiert.
    inviteForm();
    fireEvent.click(create);
    const form = await screen.findByRole("dialog", {
      name: "Angaben eintragen",
    });
    expect(form).toHaveTextContent("An die E-Mail-Adresse schickt moto");
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));
    expect(
      await screen.findByRole("dialog", { name: "Einladung senden" }),
    ).toHaveTextContent("Station 6 von 6");

    fireEvent.click(screen.getByRole("button", { name: "Zurück" }));
    expect(
      await screen.findByRole("dialog", { name: "Angaben eintragen" }),
    ).toBeInTheDocument();
  });

  it("goes back into the sidebar to show the way again, without jumping forward", async () => {
    setupState = afterBasics();
    pathname = "/database/personal";
    render(<SchoolSetupWizard />);
    visibleTarget("create");
    visibleTarget("nav-database");
    visibleTarget("nav-/database/personal");

    fireEvent.click(screen.getByRole("button", { name: "Zeig es mir" }));
    expect(
      await screen.findByRole("dialog", { name: "Personal einladen" }),
    ).toBeInTheDocument();
    await waitForHighlight();

    fireEvent.click(screen.getByRole("button", { name: "Zurück" }));
    expect(
      await screen.findByRole("dialog", { name: "Personal öffnen" }),
    ).toHaveTextContent("Station 3 von 6");

    // Obwohl „Personal“ schon sichtbar ist, bleibt „Datenverwaltung öffnen“ stehen.
    fireEvent.click(screen.getByRole("button", { name: "Zurück" }));
    const database = await screen.findByRole("dialog", {
      name: "Datenverwaltung öffnen",
    });
    await new Promise((resolve) => setTimeout(resolve, 450));
    expect(database).toBeInTheDocument();
    expect(database).toHaveTextContent("Station 2 von 6");
  });

  it("tells the person to click the spot when there is no Weiter", async () => {
    setupState = afterBasics();
    pathname = "/database/personal";
    render(<SchoolSetupWizard />);
    const create = visibleTarget("create");
    inviteForm();

    fireEvent.click(screen.getByRole("button", { name: "Zeig es mir" }));
    const clickStop = await screen.findByRole("dialog", {
      name: "Personal einladen",
    });
    await waitForHighlight();
    expect(clickStop).toHaveTextContent(
      "Klicken Sie auf die grün umrandete Stelle.",
    );
    expect(
      document.querySelector(".motion-safe\\:animate-pulse"),
    ).not.toBeNull();

    fireEvent.click(create);
    const nextStop = await screen.findByRole("dialog", {
      name: "Angaben eintragen",
    });
    expect(nextStop).not.toHaveTextContent("grün umrandete Stelle");
    expect(screen.getByRole("button", { name: "Weiter" })).toBeInTheDocument();
  });

  it("waits for the new page instead of highlighting a button of the old one", async () => {
    setupState = afterBasics();
    pathname = "/database/groups";
    const { rerender } = render(<SchoolSetupWizard />);
    visibleTarget("nav-database");
    const personal = visibleTarget("nav-/database/personal");
    // Die alte Seite (Gruppen) hat auch einen „+“-Knopf mit demselben Merkmal.
    visibleTarget("create");

    fireEvent.click(screen.getByRole("button", { name: "Zeig es mir" }));
    expect(
      await screen.findByRole("dialog", { name: "Personal öffnen" }),
    ).toBeInTheDocument();
    await waitForHighlight();
    fireEvent.click(personal);

    await new Promise((resolve) => setTimeout(resolve, 600));
    expect(
      screen.queryByRole("dialog", { name: "Personal einladen" }),
    ).not.toBeInTheDocument();

    pathname = "/database/personal";
    rerender(<SchoolSetupWizard />);
    expect(
      await screen.findByRole("dialog", { name: "Personal einladen" }),
    ).toBeInTheDocument();
  });

  it("looks again when the highlighted spot is replaced", async () => {
    setupState = afterBasics();
    pathname = "/database/personal";
    render(<SchoolSetupWizard />);
    const first = visibleTarget("create");

    fireEvent.click(screen.getByRole("button", { name: "Zeig es mir" }));
    expect(
      await screen.findByRole("dialog", { name: "Personal einladen" }),
    ).toBeInTheDocument();
    await waitForHighlight();

    // Die Seite rendert neu: der Knopf wird ersetzt.
    first.remove();
    await waitFor(() =>
      expect(
        screen.queryByRole("dialog", { name: "Personal einladen" }),
      ).not.toBeInTheDocument(),
    );
    const second = visibleTarget("create");
    expect(
      await screen.findByRole("dialog", { name: "Personal einladen" }),
    ).toBeInTheDocument();

    // Der Klick auf den neuen Knopf führt weiter.
    inviteForm();
    fireEvent.click(second);
    expect(
      await screen.findByRole("dialog", { name: "Angaben eintragen" }),
    ).toBeInTheDocument();
  });

  it("closes the form window when the person goes back to its button", async () => {
    setupState = studentsNext();
    pathname = "/database/students";
    render(<SchoolSetupWizard />);
    visibleTarget("student-import");
    const create = visibleTarget("create");

    fireEvent.click(screen.getByRole("button", { name: "Zeig es mir" }));
    await screen.findByRole("dialog", { name: "Viele Kinder auf einmal" });
    await waitForHighlight();
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));
    await screen.findByRole("dialog", { name: "Ein Kind anlegen" });
    await waitForHighlight();

    // „+ Kinder“ öffnet das Fenster „Neues Kind“ über dem Knopf.
    const window = studentFormWindow();
    fireEvent.click(create);
    await screen.findByRole("dialog", { name: "Angaben eintragen" });
    await waitForHighlight();

    fireEvent.click(screen.getByRole("button", { name: "Zurück" }));
    expect(window).not.toBeInTheDocument();
    expect(
      await screen.findByRole("dialog", { name: "Ein Kind anlegen" }),
    ).toBeInTheDocument();
  });

  it("continues with the import when the person chooses Importieren", async () => {
    setupState = studentsNext();
    pathname = "/database/students";
    const { rerender } = render(<SchoolSetupWizard />);
    const importLink = visibleTarget("student-import");

    fireEvent.click(screen.getByRole("button", { name: "Zeig es mir" }));
    expect(
      await screen.findByRole("dialog", { name: "Viele Kinder auf einmal" }),
    ).toBeInTheDocument();
    await waitForHighlight();

    // Die Person klickt „Importieren“: Die Import-Seite öffnet sich.
    fireEvent.click(importLink);
    importLink.remove();
    pathname = "/database/students/import";
    rerender(<SchoolSetupWizard />);
    visibleSection("format-select");

    const template = await screen.findByRole("dialog", {
      name: "Vorlage herunterladen",
    });
    expect(template).toHaveTextContent("Station 1 von 4");
    // Der Zweig beginnt neu: kein Weg zurück auf die Kinderliste.
    expect(
      screen.queryByRole("button", { name: "Zurück" }),
    ).not.toBeInTheDocument();
    // Der Wechsel ist kein Verlassen der Tour.
    await new Promise((resolve) => setTimeout(resolve, 1000));
    expect(screen.queryByText(/Die Tour ist beendet/)).not.toBeInTheDocument();
  });

  it("moves on to the import button once the preview is there", async () => {
    setupState = studentsNext();
    pathname = "/database/students/import";
    render(<SchoolSetupWizard />);
    visibleSection("format-select");
    visibleModeCard();
    const upload = visibleUploadCard();

    // Wer schon beim Import ist, beginnt dort.
    fireEvent.click(screen.getByRole("button", { name: "Zeig es mir" }));
    expect(
      await screen.findByRole("dialog", { name: "Vorlage herunterladen" }),
    ).toBeInTheDocument();
    await waitForHighlight();
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));
    await screen.findByRole("dialog", { name: "Neue Kinder anlegen" });
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));
    expect(
      await screen.findByRole("dialog", { name: "Datei hochladen" }),
    ).toBeInTheDocument();
    expect(upload).toBeInTheDocument();

    // Die Datei ist hochgeladen: Vorschau und Import-Knopf erscheinen.
    visibleTarget("student-import-submit");
    expect(
      await screen.findByRole("dialog", { name: "Prüfen und importieren" }),
    ).toHaveTextContent("Station 4 von 4");
  });

  it("ends the tour when the person opens another page", async () => {
    setupState = afterBasics();
    pathname = "/database/personal";
    const { rerender } = render(<SchoolSetupWizard />);
    visibleTarget("create");

    fireEvent.click(screen.getByRole("button", { name: "Zeig es mir" }));
    expect(
      await screen.findByRole("dialog", { name: "Personal einladen" }),
    ).toBeInTheDocument();

    pathname = "/dashboard";
    rerender(<SchoolSetupWizard />);

    expect(
      await screen.findByText(
        /Die Tour ist beendet, weil Sie eine andere Seite/,
      ),
    ).toBeInTheDocument();
    // Neu starten geht mit demselben Knopf.
    fireEvent.click(screen.getByRole("button", { name: "Zeig es mir" }));
    expect(screen.queryByText(/Die Tour ist beendet/)).not.toBeInTheDocument();
  });

  it("shows no bubble while it is still looking for the next spot", async () => {
    setupState = afterBasics();
    pathname = "/database/personal";
    render(<SchoolSetupWizard />);
    const create = visibleTarget("create");

    fireEvent.click(screen.getByRole("button", { name: "Zeig es mir" }));
    await waitForHighlight();
    fireEvent.click(create);

    // Das Formular ist noch nicht da: keine Sprechblase in der Bildschirmmitte.
    await waitFor(() =>
      expect(
        screen.queryByRole("dialog", { name: "Personal einladen" }),
      ).not.toBeInTheDocument(),
    );
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    inviteForm();
    expect(
      await screen.findByRole("dialog", { name: "Angaben eintragen" }),
    ).toBeInTheDocument();
  });

  it("opens the page itself when there is no sidebar", async () => {
    setupState = afterBasics();
    render(<SchoolSetupWizard />);

    fireEvent.click(screen.getByRole("button", { name: "Zeig es mir" }));

    await waitFor(
      () => expect(push).toHaveBeenCalledWith("/database/personal"),
      {
        timeout: 3000,
      },
    );
  });

  it("closes with the cards and completes the school", async () => {
    setupState = {
      ...newSchool(),
      steps: newSchool().steps.map((step) => ({ ...step, done: true })),
    };
    api.completeSchoolSetup.mockResolvedValue({
      ...setupState,
      completed: true,
    });
    render(<SchoolSetupWizard />);

    expect(screen.getByText("Herzlichen Glückwunsch!")).toBeInTheDocument();
    expect(screen.getByText(/Die ersten Einträge stehen/)).toBeInTheDocument();
    expect(
      screen.getByRole("region", { name: "Jetzt die übrigen Daten anlegen" }),
    ).toHaveTextContent("Alle Kinder übernehmen");
    const after = screen.getByRole("region", { name: "Danach" });
    expect(after).toHaveTextContent("Anmeldungen");
    expect(after).toHaveTextContent("zur Betreuung anmelden");
    // „Danach“ führt auf die Oberthemen der Hilfe, nicht auf einzelne Artikel.
    const link = after.querySelector(
      'a[href^="/help/gruppe/anmeldeverwaltung?"]',
    );
    expect(link).not.toBeNull();
    expect(link?.getAttribute("href")).toContain("role=lead");
    fireEvent.click(screen.getByRole("button", { name: "Abschließen" }));

    await waitFor(() => expect(api.completeSchoolSetup).toHaveBeenCalled());
  });

  it("explains a refused presence mode switch", async () => {
    const { SchoolSetupError } = await import("~/lib/school-setup-api");
    api.confirmSchoolSetupBasics.mockRejectedValue(new SchoolSetupError(409));
    setupState = {
      ...newSchool(),
      basics: { ...newSchool().basics, parentAppUsed: true },
    };
    render(<SchoolSetupWizard />);

    fireEvent.click(
      screen.getByRole("button", { name: "Speichern und weiter" }),
    );

    expect(
      await screen.findByText(
        "Heute sind schon Kinder angemeldet. Die Anwesenheitsart können Sie dann nur über das moto-Team ändern.",
      ),
    ).toBeInTheDocument();
  });
});

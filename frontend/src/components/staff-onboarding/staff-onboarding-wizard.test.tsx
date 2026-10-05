import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import type { StaffOnboardingState } from "~/lib/staff-onboarding-api";

const push = vi.fn();
const replace = vi.fn(() => Promise.resolve());
let shellUser: { id: string } | null = { id: "74" };
let preview = false;
let onboarding: StaffOnboardingState | null = null;
let pathname = "/dashboard";
let permissions: string[] = ["calendar:own"];
let groups: { id: string; is_personal?: boolean }[] = [{ id: "1" }];
let presenceMode: "detailed" | "binary" = "detailed";
let openCare = false;
let webAttendance = true;
let tenantSlug = "school-one";

vi.mock("next/navigation", () => ({
  usePathname: () => pathname,
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: "74", permissions } },
    status: "authenticated",
  }),
}));

vi.mock("~/lib/auth-utils", () => ({
  hasPermission: (_session: unknown, permission: string) =>
    permissions.includes(permission),
}));

vi.mock("~/lib/shell-auth-context", () => ({
  useShellAuthSafe: () =>
    shellUser ? { user: shellUser, isPreview: preview } : undefined,
}));

vi.mock("~/lib/tenant-context", () => ({
  useNFCEnabled: () => false,
  usePresenceMode: () => presenceMode,
  useOpenCareGroupMode: () => openCare,
  useAttendanceWebEnabled: () => webAttendance,
  useTenantSlugSafe: () => tenantSlug,
}));

vi.mock("~/lib/supervision-context", () => ({
  useOptionalSupervision: () => ({ groups }),
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push }),
}));

vi.mock("./use-staff-onboarding", () => ({
  useStaffOnboarding: () => ({
    state: onboarding,
    accountID: "74",
    replace,
  }),
}));

const api = vi.hoisted(() => ({
  setStaffOnboardingStepState: vi.fn(),
  setStaffOnboardingDismissed: vi.fn(),
}));
vi.mock("~/lib/staff-onboarding-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/staff-onboarding-api")>()),
  ...api,
}));

import { StaffOnboardingWizard } from "./staff-onboarding-wizard";

function newPerson(): StaffOnboardingState {
  return {
    dismissed: false,
    schoolReady: true,
    doneSteps: [],
    skippedSteps: [],
  };
}

/** Ein sichtbares Tour-Ziel; jsdom misst sonst alles mit Größe 0. */
function visible<T extends HTMLElement>(element: T): T {
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

function stepTitles(): string[] {
  const region = screen.getByRole("region", {
    name: "Erste Schritte mit moto",
  });
  return Array.from(region.querySelectorAll("ol > li button[aria-expanded]"))
    .map((button) => button.querySelector("span > span")?.textContent ?? "")
    .filter(Boolean);
}

describe("StaffOnboardingWizard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    sessionStorage.clear();
    document.body.innerHTML = "";
    shellUser = { id: "74" };
    preview = false;
    onboarding = newPerson();
    pathname = "/dashboard";
    permissions = ["calendar:own"];
    groups = [{ id: "1" }];
    presenceMode = "detailed";
    openCare = false;
    webAttendance = true;
    tenantSlug = "school-one";
  });

  it("opens the checklist once per school and sign-in with every step", () => {
    const { unmount } = render(<StaffOnboardingWizard />);

    expect(stepTitles()).toEqual([
      "Meine Gruppe ansehen",
      "Ein Kind finden und Angaben ansehen",
      "Ein Kind an- und abmelden",
      "Meine Termine ansehen",
      "Eine Aufsicht starten",
      "Arbeitszeit erfassen",
    ]);
    expect(screen.getByText("0 von 6 erledigt")).toBeInTheDocument();
    unmount();

    const sameSchool = render(<StaffOnboardingWizard />);
    expect(
      screen.getByRole("button", { name: "Erste Schritte öffnen, 6 offen" }),
    ).toBeInTheDocument();
    sameSchool.unmount();

    tenantSlug = "school-two";
    render(<StaffOnboardingWizard />);
    expect(screen.getByText("0 von 6 erledigt")).toBeInTheDocument();
  });

  it("leaves out steps the school or the person does not have", () => {
    openCare = true;
    webAttendance = false;
    presenceMode = "binary";
    permissions = [];
    render(<StaffOnboardingWizard />);

    expect(stepTitles()).toEqual([
      "Ein Kind finden und Angaben ansehen",
      "Arbeitszeit erfassen",
    ]);
  });

  it("shows the group step only to a person with an own group", () => {
    groups = [{ id: "9", is_personal: false }];
    render(<StaffOnboardingWizard />);

    expect(stepTitles()).not.toContain("Meine Gruppe ansehen");
  });

  it("stays hidden until the school is ready, once hidden, and in a preview", () => {
    onboarding = { ...newPerson(), schoolReady: false };
    const { unmount } = render(<StaffOnboardingWizard />);
    expect(screen.queryByText("Erste Schritte")).toBeNull();
    unmount();

    onboarding = { ...newPerson(), dismissed: true };
    const second = render(<StaffOnboardingWizard />);
    expect(screen.queryByText("Erste Schritte")).toBeNull();
    second.unmount();

    onboarding = newPerson();
    preview = true;
    render(<StaffOnboardingWizard />);
    expect(screen.queryByText("Erste Schritte")).toBeNull();
  });

  it("marks a step done once the person reaches the end of its tour", async () => {
    pathname = "/time-tracking";
    const done = { ...newPerson(), doneSteps: ["work_time" as const] };
    api.setStaffOnboardingStepState.mockResolvedValue(done);
    onboarding = {
      ...newPerson(),
      doneSteps: ["groups", "students", "attendance", "calendar"],
      skippedSteps: ["supervision"],
    };
    render(<StaffOnboardingWizard />);
    const clock = document.createElement("div");
    clock.dataset.setupTour = "time-clock";
    visible(clock);

    fireEvent.click(screen.getByRole("button", { name: "Zeig es mir" }));
    const bubble = await screen.findByRole("dialog", { name: "Stempeluhr" });
    expect(bubble).toHaveTextContent("Station 4 von 4");
    expect(api.setStaffOnboardingStepState).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Fertig" }));

    await waitFor(() =>
      expect(api.setStaffOnboardingStepState).toHaveBeenCalledWith(
        "work_time",
        "done",
      ),
    );
    expect(replace).toHaveBeenCalledWith(done);
  });

  it("walks through the tabs of the child record and skips a missing one", async () => {
    pathname = "/students/search";
    onboarding = { ...newPerson(), doneSteps: ["groups"] };
    const { rerender } = render(<StaffOnboardingWizard />);
    const search = document.createElement("input");
    search.placeholder = "Name suchen…";
    visible(search);
    const card = document.createElement("button");
    card.dataset.setupTour = "student-card";
    visible(card);

    fireEvent.click(screen.getByRole("button", { name: "Zeig es mir" }));
    await screen.findByRole("dialog", { name: "Kind suchen" });
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));
    expect(
      await screen.findByRole("dialog", { name: "Die Karte eines Kindes" }),
    ).toHaveTextContent("wann es kommt und geht");

    // Die Karte öffnet die Kindakte mit ihren Reitern.
    fireEvent.click(card);
    card.remove();
    search.remove();
    // Der offene Reiter mit seiner ersten Karte, auf die die Tour zeigt.
    const panel = document.createElement("div");
    panel.setAttribute("role", "tabpanel");
    panel.dataset.state = "active";
    document.body.appendChild(panel);
    const firstCard = document.createElement("section");
    panel.appendChild(visible(firstCard));
    const tablist = document.createElement("div");
    tablist.setAttribute("role", "tablist");
    visible(tablist);
    const tab = (value: string) => {
      const element = document.createElement("button");
      element.setAttribute("role", "tab");
      element.dataset.tabValue = value;
      tablist.appendChild(visible(element));
      return element;
    };
    const stammdaten = tab("stammdaten");
    // Kein Reiter „Nachrichten“: Die Person sieht ihn nicht.
    tab("erziehungsberechtigte");
    pathname = "/students/42";
    rerender(<StaffOnboardingWizard />);

    await screen.findByRole("dialog", { name: "Angaben des Kindes" });
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));

    await screen.findByRole("dialog", { name: "Stammdaten" });
    fireEvent.click(stammdaten);
    expect(
      await screen.findByText(/Gesundheitsinformationen, Notizen/),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));

    // „Nachrichten“ fehlt und entfällt still; es geht bei den
    // Erziehungsberechtigten weiter.
    expect(
      await screen.findByRole(
        "dialog",
        { name: "Erziehungsberechtigte" },
        { timeout: 4000 },
      ),
    ).toHaveTextContent("Wählen Sie „Erziehungsberechtigte“.");
    expect(screen.queryByText(/nicht zu sehen/)).toBeNull();
  });

  it("goes back from the child record to the card on the search page", async () => {
    pathname = "/students/search";
    onboarding = { ...newPerson(), doneSteps: ["groups"] };
    const { rerender } = render(<StaffOnboardingWizard />);
    const search = document.createElement("input");
    search.placeholder = "Name suchen…";
    visible(search);
    const card = document.createElement("button");
    card.dataset.setupTour = "student-card";
    visible(card);

    fireEvent.click(screen.getByRole("button", { name: "Zeig es mir" }));
    await screen.findByRole("dialog", { name: "Kind suchen" });
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));
    await screen.findByRole("dialog", { name: "Die Karte eines Kindes" });
    fireEvent.click(card);
    const tablist = document.createElement("div");
    tablist.setAttribute("role", "tablist");
    visible(tablist);
    pathname = "/students/42";
    rerender(<StaffOnboardingWizard />);
    await screen.findByRole("dialog", { name: "Angaben des Kindes" });

    fireEvent.click(screen.getByRole("button", { name: "Zurück" }));

    // „Zurück“ öffnet die Kindersuche wieder, statt auf ihr zu warten.
    expect(push).toHaveBeenCalledWith("/students/search");
    pathname = "/students/search";
    rerender(<StaffOnboardingWizard />);
    expect(
      await screen.findByRole("dialog", { name: "Die Karte eines Kindes" }),
    ).toBeInTheDocument();
  });

  it("tells a person who is already clocked in what the clock shows", async () => {
    pathname = "/time-tracking";
    onboarding = {
      ...newPerson(),
      doneSteps: ["groups", "students", "attendance", "calendar"],
      skippedSteps: ["supervision"],
    };
    render(<StaffOnboardingWizard />);
    const clock = document.createElement("div");
    clock.dataset.setupTour = "time-clock";
    visible(clock);
    const clockOut = document.createElement("button");
    clockOut.setAttribute("aria-label", "Ausstempeln");
    clock.appendChild(visible(clockOut));

    fireEvent.click(screen.getByRole("button", { name: "Zeig es mir" }));

    const bubble = await screen.findByRole("dialog", { name: "Stempeluhr" });
    expect(bubble).toHaveTextContent("Sie sind schon eingestempelt.");
    expect(bubble).not.toHaveTextContent("dann „Einstempeln“");
  });

  it("shows the way through the bottom bar on a phone", async () => {
    onboarding = { ...newPerson(), doneSteps: ["groups"] };
    render(<StaffOnboardingWizard />);
    // Handy: keine Seitenleiste, aber die untere Leiste mit „Suchen“.
    const searchTab = document.createElement("a");
    searchTab.dataset.setupTour = "mobile-nav-/students/search";
    visible(searchTab);
    const more = document.createElement("button");
    more.dataset.setupTour = "nav-more";
    visible(more);

    fireEvent.click(screen.getByRole("button", { name: "Zeig es mir" }));

    const bubble = await screen.findByRole("dialog", {
      name: "Alle Kinder öffnen",
    });
    expect(bubble).toHaveTextContent("Wählen Sie unten „Suchen“.");
    expect(bubble).toHaveTextContent("Station 2 von");
  });

  it("opens Mehr first on a phone when the page is not in the bottom bar", async () => {
    onboarding = {
      ...newPerson(),
      doneSteps: ["groups", "students", "attendance"],
    };
    render(<StaffOnboardingWizard />);
    const more = visible(document.createElement("button"));
    more.dataset.setupTour = "nav-more";

    fireEvent.click(screen.getByRole("button", { name: "Zeig es mir" }));
    expect(
      await screen.findByRole("dialog", { name: "Mehr öffnen" }),
    ).toHaveTextContent("Wählen Sie unten „Mehr“.");

    // „Mehr“ öffnet das Menü mit „Mein Kalender“.
    const entry = document.createElement("a");
    entry.dataset.setupTour = "nav-/calendar";
    visible(entry);
    fireEvent.click(more);
    expect(
      await screen.findByRole("dialog", { name: "Mein Kalender öffnen" }),
    ).toHaveTextContent("Wählen Sie „Mein Kalender“.");
  });

  it("updates the clock text once the page has loaded the clocked-in state", async () => {
    pathname = "/time-tracking";
    onboarding = {
      ...newPerson(),
      doneSteps: ["groups", "students", "attendance", "calendar"],
      skippedSteps: ["supervision"],
    };
    render(<StaffOnboardingWizard />);
    const clock = document.createElement("div");
    clock.dataset.setupTour = "time-clock";
    visible(clock);

    fireEvent.click(screen.getByRole("button", { name: "Zeig es mir" }));
    const bubble = await screen.findByRole("dialog", { name: "Stempeluhr" });
    expect(bubble).toHaveTextContent("dann „Einstempeln“");

    // Der Stand kommt nach: Die Person ist schon eingestempelt.
    const clockOut = document.createElement("button");
    clockOut.setAttribute("aria-label", "Ausstempeln");
    clock.appendChild(visible(clockOut));

    await waitFor(() =>
      expect(
        screen.getByRole("dialog", { name: "Stempeluhr" }),
      ).toHaveTextContent("Sie sind schon eingestempelt."),
    );
  });

  it("explains the empty supervision page without pointing at Als Nächstes", async () => {
    pathname = "/active-supervisions";
    onboarding = {
      ...newPerson(),
      doneSteps: ["groups", "students", "attendance", "calendar"],
    };
    render(<StaffOnboardingWizard />);
    const empty = document.createElement("div");
    empty.dataset.setupTour = "supervision-empty";
    visible(empty);

    fireEvent.click(screen.getByRole("button", { name: "Zeig es mir" }));

    const bubble = await screen.findByRole("dialog", {
      name: "Aufsicht übernehmen",
    });
    expect(bubble).toHaveTextContent("Gerade beaufsichtigen Sie keinen Raum");
    expect(bubble).not.toHaveTextContent("Beaufsichtigen");
  });

  it("finishes the calendar tour in an empty week", async () => {
    pathname = "/calendar";
    api.setStaffOnboardingStepState.mockResolvedValue({
      ...newPerson(),
      doneSteps: ["calendar"],
    });
    onboarding = {
      ...newPerson(),
      doneSteps: ["groups", "students", "attendance"],
    };
    render(<StaffOnboardingWizard />);
    const views = document.createElement("div");
    views.setAttribute("aria-label", "Ansicht wählen");
    visible(views);
    // Leere Woche: statt des Rasters nur der Leerzustand in der Fläche.
    const body = document.createElement("div");
    body.className = "moto-tenant-body";
    visible(body);

    fireEvent.click(screen.getByRole("button", { name: "Zeig es mir" }));
    await screen.findByRole("dialog", { name: "Tag, Woche oder Monat" });
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));
    expect(
      await screen.findByRole("dialog", { name: "Ihre Termine" }),
    ).toHaveTextContent("bleibt die Fläche leer");
    fireEvent.click(screen.getByRole("button", { name: "Fertig" }));

    await waitFor(() =>
      expect(api.setStaffOnboardingStepState).toHaveBeenCalledWith(
        "calendar",
        "done",
      ),
    );
  });

  it("offers a finished tour again", () => {
    onboarding = { ...newPerson(), doneSteps: ["students"] };
    render(<StaffOnboardingWizard />);

    fireEvent.click(
      screen.getByRole("button", {
        name: /Ein Kind finden und Angaben ansehen/,
      }),
    );

    expect(
      screen.getByRole("button", { name: "Noch einmal zeigen" }),
    ).toBeInTheDocument();
  });

  it("skips a step and takes the skip back", async () => {
    api.setStaffOnboardingStepState.mockResolvedValue({
      ...newPerson(),
      skippedSteps: ["groups"],
    });
    const first = render(<StaffOnboardingWizard />);

    fireEvent.click(screen.getByRole("button", { name: "Überspringen" }));
    await waitFor(() =>
      expect(api.setStaffOnboardingStepState).toHaveBeenCalledWith(
        "groups",
        "skipped",
      ),
    );

    first.unmount();
    sessionStorage.clear();
    onboarding = { ...newPerson(), skippedSteps: ["groups"] };
    render(<StaffOnboardingWizard />);
    fireEvent.click(
      screen.getByRole("button", { name: /Meine Gruppe ansehen/ }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Doch nicht überspringen" }),
    );
    await waitFor(() =>
      expect(api.setStaffOnboardingStepState).toHaveBeenLastCalledWith(
        "groups",
        "open",
      ),
    );
  });

  it("closes with further guides once every step is finished", async () => {
    onboarding = {
      ...newPerson(),
      doneSteps: ["groups", "students", "attendance", "calendar", "work_time"],
      skippedSteps: ["supervision"],
    };
    api.setStaffOnboardingDismissed.mockResolvedValue({
      ...onboarding,
      dismissed: true,
    });
    render(<StaffOnboardingWizard />);

    expect(screen.getByText("Geschafft!")).toBeInTheDocument();
    const more = screen.getByRole("region", { name: "Auch nützlich" });
    expect(more).toHaveTextContent("moto aufs Handy holen");
    expect(more.querySelector('a[href*="role=caregiver"]')).not.toBeNull();

    // Fest in der Fußzeile, damit er auf dem Handy nicht unter den Karten
    // aus dem sichtbaren Bereich rutscht.
    const finish = screen.getByRole("button", { name: "Abschließen" });
    expect(finish.closest("footer")).not.toBeNull();
    expect(
      screen.queryByRole("button", { name: "Nicht mehr anzeigen" }),
    ).toBeNull();
    fireEvent.click(finish);
    await waitFor(() =>
      expect(api.setStaffOnboardingDismissed).toHaveBeenCalledWith(true),
    );
  });

  it("hides the checklist for good only after a confirmation", async () => {
    api.setStaffOnboardingDismissed.mockResolvedValue({
      ...newPerson(),
      dismissed: true,
    });
    render(<StaffOnboardingWizard />);

    fireEvent.click(
      screen.getByRole("button", { name: "Nicht mehr anzeigen" }),
    );
    expect(api.setStaffOnboardingDismissed).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Ausblenden" }));

    await waitFor(() =>
      expect(api.setStaffOnboardingDismissed).toHaveBeenCalledWith(true),
    );
  });

  it("says so when a step could not be saved", async () => {
    const { StaffOnboardingError } = await import("~/lib/staff-onboarding-api");
    api.setStaffOnboardingStepState.mockRejectedValue(
      new StaffOnboardingError(500),
    );
    render(<StaffOnboardingWizard />);

    fireEvent.click(screen.getByRole("button", { name: "Überspringen" }));

    expect(
      await screen.findByText(
        "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.",
      ),
    ).toBeInTheDocument();
    expect(replace).not.toHaveBeenCalled();
  });
});

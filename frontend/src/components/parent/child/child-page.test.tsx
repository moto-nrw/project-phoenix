import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NextIntlClientProvider } from "next-intl";
import { useEffect } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import deMessages from "~/i18n/messages/de.json";
import { getChildToday, listMyChildren } from "~/lib/parent-api";
import { ChildPage } from "./child-page";

let mockSearchParams = new URLSearchParams();
const mockMasterDataMount = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), prefetch: vi.fn() }),
  useSearchParams: () => mockSearchParams,
}));

vi.mock("~/lib/parent-url", () => ({
  parentPath: (path: string) => path,
}));

vi.mock("~/lib/parent-api", () => ({
  listMyChildren: vi.fn(),
  getChildToday: vi.fn().mockResolvedValue({ at_ogs: null, state: "unknown" }),
  UNKNOWN_CHILD_TODAY: { at_ogs: null, state: "unknown" },
}));

// Die schweren Abschnitte laden eigene Daten; hier zaehlt nur, DASS sie in der
// richtigen Reihenfolge stehen.
vi.mock("~/components/parent/child/booked-care-section", () => ({
  BookedCareSection: () => <div data-testid="section-care">Betreuung</div>,
}));
vi.mock("~/components/parent/child-master-data", () => ({
  ChildMasterDataView: ({
    childName,
    area,
  }: {
    childName: string;
    area: "details" | "departure" | "contact";
  }) => {
    useEffect(() => {
      mockMasterDataMount();
    }, []);
    return (
      <div data-testid={`section-${area}`}>
        {area === "details" && `Angaben zu ${childName}`}
        {area === "departure" && `Heimweg von ${childName}`}
        {area === "contact" && "Ihre Kontaktdaten"}
      </div>
    );
  },
}));
vi.mock("~/components/parent/guardians-panel", () => ({
  default: () => <div data-testid="section-people">Eltern</div>,
}));
// Der Betreuungszustand ist pro Test austauschbar: die Erreichbarkeit des
// Abhol-Dialogs haengt davon ab, was die Schule freigeschaltet hat und was an
// Antraegen offen ist.
const careMock = vi.hoisted(() => ({
  value: undefined as unknown as ReturnType<typeof buildCare>,
}));

vi.mock("~/components/parent/child-care", () => ({
  useChildCare: () => careMock.value,
  SickNoteModal: () => <div data-testid="sick-modal" />,
  PickupTimeModal: () => <div data-testid="pickup-modal" />,
}));

function buildCare(
  overrides: {
    features?: Record<string, unknown>;
    pickupChangeRequests?: { id: string; date: string; status: string }[];
  } = {},
) {
  return {
    loading: false,
    features: {
      sick_note_enabled: true,
      sick_requires_approval: false,
      pickup_change_enabled: true,
      pickup_manage_allowed: true,
      notes_enabled: true,
      related_accounts_invite_enabled: false,
      related_accounts_remove_enabled: false,
      excused_requires_approval: false,
      ...overrides.features,
    },
    careExceptions: [],
    careExceptionsLoaded: true,
    pickupChangeRequests: overrides.pickupChangeRequests ?? [],
    pickupChangeRequestsLoaded: true,
    todayPickup: { kind: "time", time: "15:00", changed: false },
    sickDays: [],
    excusedRequests: [],
    reportSick: vi.fn(),
    refresh: vi.fn(),
    saveCareException: vi.fn(),
    removeCareException: vi.fn(),
  };
}

const mockedChildren = vi.mocked(listMyChildren);
const mockedToday = vi.mocked(getChildToday);

const felix = {
  student_id: "42",
  tenant_id: "7",
  first_name: "Felix",
  last_name: "Schneider",
  school_class: "1a",
  status: "active",
  school_name: "Schule am Berg",
  school_slug: "berg",
} as unknown as Awaited<ReturnType<typeof listMyChildren>>[number];

const mia = { ...felix, student_id: "43", first_name: "Mia" };

function renderPage(studentId?: string) {
  return render(
    <NextIntlClientProvider locale="de" messages={deMessages}>
      <ChildPage studentId={studentId} />
    </NextIntlClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mockSearchParams = new URLSearchParams();
  careMock.value = buildCare();
  mockedChildren.mockResolvedValue([felix]);
  mockedToday.mockResolvedValue({ at_ogs: null, state: "unknown" });
});

describe("ChildPage", () => {
  // Ein Kind heisst kein Umschalter. "tablist" meint auf dieser Seite nur die
  // Inhalts-Reiter, die es immer gibt.
  it("zeigt bei einem Kind weder Liste noch Umschalter", async () => {
    renderPage();
    expect(
      await screen.findByRole("heading", { level: 1, name: "Felix Schneider" }),
    ).toBeInTheDocument();
    expect(screen.queryByTestId("child-selection")).not.toBeInTheDocument();
  });

  // Der Name steht als Seitentitel; die Tageskarte darunter wiederholt ihn nicht.
  it("nennt das Kind genau einmal", async () => {
    renderPage();
    await screen.findByRole("heading", { level: 1, name: "Felix Schneider" });
    expect(screen.getAllByText("Felix Schneider")).toHaveLength(1);
  });

  it("zeigt bei mehreren Kindern zuerst eine eigene Auswahlseite", async () => {
    mockedChildren.mockResolvedValue([felix, mia]);
    mockedToday.mockImplementation(async (studentId) =>
      studentId === "42"
        ? { at_ogs: true, state: "present", since: "10:23" }
        : { at_ogs: false, state: "no_care" },
    );
    renderPage();
    expect(
      await screen.findByRole("heading", { level: 1, name: "Meine Kinder" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Kinder im Überblick")).toBeInTheDocument();
    expect(screen.getByTestId("child-selection")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /Felix Schneider/ }),
    ).toHaveAttribute("href", "/parents/children/42");
    expect(screen.getByRole("link", { name: /Mia Schneider/ })).toHaveAttribute(
      "href",
      "/parents/children/43",
    );
    expect(
      screen.queryByTestId("child-day-state-icon"),
    ).not.toBeInTheDocument();
    expect(screen.queryByRole("tablist")).not.toBeInTheDocument();
    expect(screen.queryByText("Elternportal")).not.toBeInTheDocument();
    expect(screen.queryByText("FS")).not.toBeInTheDocument();
    expect(screen.queryByText("MS")).not.toBeInTheDocument();
    expect(
      document.querySelectorAll('[data-moto-duotone-tone="greenVivid"]'),
    ).toHaveLength(2);
    expect(screen.getByText("In der OGS")).toBeInTheDocument();
    expect(screen.getByText("Nicht in der OGS")).toBeInTheDocument();
  });

  it("zeigt im Profil nur das aktive Kind als Seitentitel", async () => {
    mockedChildren.mockResolvedValue([felix, mia]);
    renderPage("42");
    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: "Felix Schneider",
      }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Mia Schneider")).not.toBeInTheDocument();
    expect(screen.queryByTestId("child-selection")).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Meine Kinder" })).toHaveAttribute(
      "href",
      "/parents/children",
    );
    expect(screen.getByText("1a · Schule am Berg")).toBeInTheDocument();
    expect(screen.getByTestId("child-profile-icon")).toBeInTheDocument();
    expect(screen.queryByText("OGS")).not.toBeInTheDocument();
    expect(screen.queryByText("PARENTCHILD.OGSLABEL")).not.toBeInTheDocument();
  });

  it("gliedert die Inhalte mit drei verstaendlichen Reitern auf derselben Seite", async () => {
    mockedChildren.mockResolvedValue([felix, mia]);
    renderPage("42");
    await screen.findByRole("heading", { level: 1, name: "Felix Schneider" });

    expect(
      screen.getByRole("tablist", { name: "Bereiche zum Kind" }),
    ).toBeInTheDocument();
    for (const label of [
      "Betreuung & Wochenplan",
      "Angaben zum Kind",
      "Kontakte & Abholung",
    ]) {
      expect(screen.getByRole("tab", { name: label })).toBeInTheDocument();
    }
    expect(
      screen.getByRole("tab", { name: "Betreuung & Wochenplan" }),
    ).toHaveAttribute("aria-selected", "true");
    for (const [name, visibleLabel] of [
      ["Betreuung & Wochenplan", "Betreuung"],
      ["Angaben zum Kind", "Angaben"],
      ["Kontakte & Abholung", "Kontakte"],
    ] as const) {
      expect(screen.getByRole("tab", { name })).toHaveTextContent(
        visibleLabel ?? "",
      );
    }
  });

  it("zeigt Heute und standardmaessig die Betreuung", async () => {
    let resolveToday!: (
      value: Awaited<ReturnType<typeof getChildToday>>,
    ) => void;
    mockedToday.mockReturnValue(
      new Promise((resolve) => {
        resolveToday = resolve;
      }),
    );
    renderPage();
    await screen.findByRole("heading", { level: 1, name: "Felix Schneider" });

    expect(
      screen.queryByTestId("child-day-state-icon"),
    ).not.toBeInTheDocument();
    resolveToday({ at_ogs: null, state: "unknown" });
    expect(
      await screen.findByTestId("child-day-state-icon"),
    ).toBeInTheDocument();
    expect(screen.getByText("Abholung")).toBeInTheDocument();
    expect(screen.getByText("Abholung heute um 15:00 Uhr")).toBeInTheDocument();

    expect(screen.getByTestId("section-care")).toBeInTheDocument();
    expect(screen.getByTestId("section-departure")).toBeInTheDocument();
    expect(
      screen.getByTestId("section-departure").closest('[role="tabpanel"]'),
    ).toHaveAttribute("data-state", "active");
    expect(
      screen.getByTestId("section-details").closest('[role="tabpanel"]'),
    ).toHaveAttribute("data-state", "inactive");
    expect(
      screen.getByTestId("section-people").closest('[role="tabpanel"]'),
    ).toHaveAttribute("data-state", "inactive");
  });

  it("wechselt Bereiche lokal und behaelt den Tagesstatus sichtbar", async () => {
    const user = userEvent.setup();
    renderPage("42");
    await screen.findByRole("heading", { level: 1, name: "Felix Schneider" });

    await user.click(screen.getByRole("tab", { name: "Angaben zum Kind" }));
    expect(screen.getByTestId("section-details")).toBeInTheDocument();
    expect(
      screen.getByTestId("section-care").closest('[role="tabpanel"]'),
    ).toHaveAttribute("data-state", "inactive");
    expect(screen.getByTestId("child-day-state-icon")).toBeInTheDocument();

    await user.click(screen.getByRole("tab", { name: "Kontakte & Abholung" }));
    expect(screen.queryByTestId("section-contact")).not.toBeInTheDocument();
    expect(screen.getByTestId("section-people")).toBeInTheDocument();
    expect(screen.getByTestId("child-day-state-icon")).toBeInTheDocument();
  });

  it("laedt Heimweg und Angaben beim erneuten Tabwechsel nicht neu", async () => {
    const user = userEvent.setup();
    renderPage("42");
    await screen.findByRole("heading", { level: 1, name: "Felix Schneider" });

    await user.click(screen.getByRole("tab", { name: "Angaben zum Kind" }));
    await user.click(screen.getByRole("tab", { name: "Kontakte & Abholung" }));
    await user.click(screen.getByRole("tab", { name: "Angaben zum Kind" }));

    expect(mockMasterDataMount).toHaveBeenCalledTimes(2);
  });

  it("nennt die geplante Abholung im Klartext", async () => {
    renderPage();
    expect(
      await screen.findByText("Abholung heute um 15:00 Uhr"),
    ).toBeInTheDocument();
  });

  it("kennt weder Betreuungszeiten noch AGs", async () => {
    renderPage();
    await screen.findByRole("heading", { level: 1, name: "Felix Schneider" });
    expect(screen.queryByText(/Betreuungszeiten/)).not.toBeInTheDocument();
    expect(screen.queryByText(/AGs/)).not.toBeInTheDocument();
  });

  it("bietet keine Eingabefelder im Kinderbereich ausserhalb der Datenabschnitte", async () => {
    renderPage();
    await screen.findByRole("heading", { level: 1, name: "Felix Schneider" });
    await waitFor(() =>
      expect(document.querySelectorAll("input")).toHaveLength(0),
    );
  });

  // Schaltet die OGS das Aendern der Abholzeit ab, waehrend ein Antrag noch
  // offen ist, bleibt der Dialog der einzige Ort zum Zuruecknehmen. Er muss
  // erreichbar bleiben, sonst sitzt der Antrag fest.
  it("oeffnet den Abhol-Dialog bei offenem Antrag trotz abgeschalteter Funktion", async () => {
    careMock.value = buildCare({
      features: { pickup_change_enabled: false },
      pickupChangeRequests: [
        { id: "9", date: "2026-09-01", status: "pending" },
      ],
    });
    mockSearchParams = new URLSearchParams("action=pickup");

    renderPage();

    expect(await screen.findByTestId("pickup-modal")).toBeInTheDocument();
  });

  // Ohne offenen Antrag und ohne eigene Ausnahme gibt es nichts zu verwalten:
  // dann bleibt der Dialog zu, sonst verspricht er eine Aenderung, die das
  // Backend ablehnen wuerde.
  it("oeffnet den Abhol-Dialog ohne offenen Antrag nicht, wenn die Funktion aus ist", async () => {
    careMock.value = buildCare({ features: { pickup_change_enabled: false } });
    mockSearchParams = new URLSearchParams("action=pickup");

    renderPage();
    await screen.findByRole("heading", { level: 1, name: "Felix Schneider" });

    expect(screen.queryByTestId("pickup-modal")).not.toBeInTheDocument();
  });
});

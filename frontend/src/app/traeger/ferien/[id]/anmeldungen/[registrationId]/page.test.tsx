import { SWRConfig } from "~/lib/swr/testing";
vi.unmock("swr");
import { render as rtlRender, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { useTenantSlugSafe } from "~/lib/tenant-context";
import {
  HOLIDAY_CONTRIBUTION,
  HOLIDAY_PERIOD,
  HOLIDAY_REGISTRATION,
  HOLIDAY_REGISTRATION_HEALTH,
  HOLIDAY_SCHOOLS,
} from "~/test/fixtures/holiday-registrations";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  useParams: () => ({ id: "5", registrationId: "7" }),
}));
vi.mock("~/lib/traeger-api", () => ({
  fetchHolidayPeriod: () => Promise.resolve(HOLIDAY_PERIOD),
  fetchTraegerSchools: () => Promise.resolve(HOLIDAY_SCHOOLS),
}));
const { mockRegistration, mockHealth, sessionPermissions } = vi.hoisted(() => ({
  mockRegistration: vi.fn(),
  mockHealth: vi.fn(),
  sessionPermissions: { current: [] as string[] },
}));
vi.mock("~/lib/traeger-holiday-registrations-api", async (importOriginal) => ({
  ...(await importOriginal<
    typeof import("~/lib/traeger-holiday-registrations-api")
  >()),
  fetchHolidayRegistration: () => mockRegistration(),
  fetchHolidayRegistrationHealth: () => mockHealth(),
}));
vi.mock("~/lib/traeger-contributions-api", () => ({
  fetchHolidayContribution: () => Promise.resolve(HOLIDAY_CONTRIBUTION),
}));

import TraegerHolidayRegistrationPage from "./page";

describe("TraegerHolidayRegistrationPage", () => {
  beforeEach(() => {
    mockRegistration.mockResolvedValue(HOLIDAY_REGISTRATION);
    mockHealth.mockReset();
    mockHealth.mockResolvedValue(HOLIDAY_REGISTRATION_HEALTH);
    sessionPermissions.current = [];
    vi.stubEnv("NEXT_PUBLIC_TRAEGER_HOSTNAME", "traeger.localhost:3000");
    // The Träger portal has no school: no TenantProvider above the page.
    vi.mocked(useTenantSlugSafe).mockReturnValue(null);
  });
  afterEach(() => {
    vi.unstubAllEnvs();
    vi.mocked(useTenantSlugSafe).mockReturnValue("test-tenant");
  });

  it("names the parents in the title and keeps the number in the status line (#5146)", async () => {
    render(
      <ToastProvider>
        <TraegerHolidayRegistrationPage />
      </ToastProvider>,
    );

    // The registration ID is the database key; the title names the parents.
    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: "Anmeldung von Eva Muster",
      }),
    ).toBeInTheDocument();
    // Export and duplicate hint refer to „Anmeldung Nr. 7“, so the number
    // stays visible next to the status.
    expect(
      await screen.findByText(/Nr\. 7 · Winterferien 2027 · Eingegangen am/),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /Zurück zu den Anmeldungen/ }),
    ).toHaveAttribute("href", "/traeger/ferien/5/anmeldungen");
  });
  it("opens the uploaded proofs of benefits from their own card (#5238)", async () => {
    mockRegistration.mockResolvedValue({
      ...HOLIDAY_REGISTRATION,
      benefitsDeclared: true,
      benefitsProofs: [
        {
          id: "91",
          filename: "Bescheid Bürgergeld.pdf",
          sizeBytes: 250_000,
          contentType: "application/pdf",
          uploadedAt: "2026-09-03T08:00:00Z",
        },
      ],
    });
    render(
      <ToastProvider>
        <TraegerHolidayRegistrationPage />
      </ToastProvider>,
    );
    const heading = await screen.findByRole("heading", {
      name: "Nachweis für den Leistungsbezug",
    });
    const card = within(heading.closest("section")!);
    expect(card.getByText("Bescheid Bürgergeld.pdf")).toBeInTheDocument();
    expect(card.getByText("244 KB")).toBeInTheDocument();
    const open = card.getByRole("link", {
      name: "Bescheid Bürgergeld.pdf",
    });
    expect(open).toHaveAttribute(
      "href",
      "/api/traeger/holiday-periods/5/registrations/7/proofs/91?inline=1",
    );
    expect(open).toHaveAttribute("target", "_blank");
    expect(open).toHaveAttribute("rel", "noopener");
    expect(open).toHaveAttribute("title", "In neuem Tab öffnen");
  });

  it("asks the office to collect a proof the parents did not upload (#5238)", async () => {
    mockRegistration.mockResolvedValue({
      ...HOLIDAY_REGISTRATION,
      benefitsDeclared: true,
    });
    render(
      <ToastProvider>
        <TraegerHolidayRegistrationPage />
      </ToastProvider>,
    );
    const heading = await screen.findByRole("heading", {
      name: "Nachweis für den Leistungsbezug",
    });
    expect(
      within(heading.closest("section")!).getByText(
        /Die Eltern haben keinen Nachweis hochgeladen/,
      ),
    ).toBeInTheDocument();
  });

  it("shows no proof card without the statement on benefits (#5238)", async () => {
    render(
      <ToastProvider>
        <TraegerHolidayRegistrationPage />
      </ToastProvider>,
    );
    expect(
      await screen.findByRole("heading", { name: "Eltern" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", {
        name: "Nachweis für den Leistungsbezug",
      }),
    ).not.toBeInTheDocument();
  });

  it("shows the second guardian only in its own contact card", async () => {
    mockRegistration.mockResolvedValue({
      ...HOLIDAY_REGISTRATION,
      secondGuardian: {
        firstName: "Tom",
        lastName: "Anders",
        email: "tom@example.org",
        phone: "0151 9999999",
      },
    });
    render(
      <ToastProvider>
        <TraegerHolidayRegistrationPage />
      </ToastProvider>,
    );
    const heading = await screen.findByRole("heading", {
      name: "Zweite sorgeberechtigte Person",
    });
    const second = within(heading.closest("section")!);
    expect(second.getByText("Tom Anders")).toBeInTheDocument();
    expect(second.getByText("tom@example.org")).toBeInTheDocument();
    expect(second.getByText("0151 9999999")).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Anmeldung von Eva Muster" }),
    ).toBeInTheDocument();
  });

  it("reads the health information only with the right „Gesundheitsangaben sehen“ (#4722)", async () => {
    render(
      <ToastProvider>
        <TraegerHolidayRegistrationPage />
      </ToastProvider>,
    );
    // Anker: die Anmeldung ist geladen und die Karten sagen, wer die Angaben sieht.
    expect(
      (
        await screen.findAllByText(
          "Gesundheitsangaben sieht nur die Rolle Verwaltung.",
        )
      ).length,
    ).toBeGreaterThan(0);
    expect(mockHealth).not.toHaveBeenCalled();
    expect(screen.queryByText("Erdnüsse")).not.toBeInTheDocument();
  });

  it("shows the health information to the right (#4722)", async () => {
    sessionPermissions.current = [
      "traeger_enrollment:manage",
      "traeger_health:read",
    ];
    render(
      <ToastProvider>
        <TraegerHolidayRegistrationPage />
      </ToastProvider>,
    );
    // Anker: die Gesundheitsangaben sind geladen.
    expect(await screen.findByText("Erdnüsse")).toBeInTheDocument();
    expect(mockHealth).toHaveBeenCalledTimes(1);
    expect(
      screen.queryByText("Gesundheitsangaben sieht nur die Rolle Verwaltung."),
    ).not.toBeInTheDocument();
  });

  it("shows the answers with the labels of the form version the parents answered (#5198)", async () => {
    mockRegistration.mockResolvedValue({
      ...HOLIDAY_REGISTRATION,
      form: {
        version: 2,
        fields: [
          {
            key: "quelle",
            label: "Wie haben Sie von den Ferien erfahren?",
            type: "text",
            sort_order: 0,
          },
          {
            key: "tetanus",
            label: "Tetanus-Impfschutz",
            type: "select",
            applies_to_child: true,
            health: true,
            sort_order: 1,
            options: [{ label: "weiß nicht", value: "weiss_nicht" }],
          },
          {
            key: "ausfluege",
            label: "Darf an Ausflügen teilnehmen",
            type: "boolean",
            applies_to_child: true,
            sort_order: 2,
          },
        ],
      },
      answers: { quelle: "Elternbrief" },
      childAnswers: { "9": { tetanus: "weiss_nicht", ausfluege: false } },
    });
    render(
      <ToastProvider>
        <TraegerHolidayRegistrationPage />
      </ToastProvider>,
    );

    expect(
      await screen.findByText(
        "Antworten auf die Fragen des Formulars, Version 2. Antworten zu einem Kind stehen beim Kind.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Wie haben Sie von den Ferien erfahren?"),
    ).toBeInTheDocument();
    expect(screen.getByText("Elternbrief")).toBeInTheDocument();
    // A health answer carries its marker; the label comes from version 2.
    expect(
      screen.getByText("Tetanus-Impfschutz (Gesundheit)"),
    ).toBeInTheDocument();
    expect(screen.getByText("weiß nicht")).toBeInTheDocument();
    expect(
      screen.getByText("Darf an Ausflügen teilnehmen"),
    ).toBeInTheDocument();
    expect(screen.getByText("Nein")).toBeInTheDocument();
    // Ben answered nothing: only Lia's card lists answers.
    expect(screen.getAllByText("Weitere Angaben")).toHaveLength(1);
    expect(screen.getByText("Weitere Angaben zur Familie")).toBeInTheDocument();
  });
  it("shows pinned legal acknowledgements with Berlin time and German PDF actions", async () => {
    mockRegistration.mockResolvedValue({
      ...HOLIDAY_REGISTRATION,
      form: {
        version: 2,
        fields: [],
        legalBlocks: [
          {
            key: "hinweis",
            kind: "terms",
            source: "custom",
            enabled: true,
            required: true,
            title: "Damals gültiger Ferienbrief",
            label: "Gelesen",
            text: "Der **historische Ferienbrief** bleibt verbindlich.",
            sort_order: 0,
            attachments: [{ label: "Ferienbrief", document_url: "81" }],
          },
          {
            key: "foto",
            kind: "consent",
            source: "custom",
            enabled: true,
            required: false,
            title: "Fotos",
            label: "Fotos erlaubt",
            text: "",
            sort_order: 1,
          },
          {
            key: "info",
            kind: "notice",
            source: "custom",
            enabled: true,
            required: false,
            title: "Trinkflaschen",
            label: "Bitte eine Trinkflasche mitgeben.",
            text: "",
            sort_order: 2,
            attachments: [{ label: "Packliste", document_url: "82" }],
          },
        ],
      },
      legalAcceptances: { hinweis: "2026-11-03T10:12:00Z" },
    });
    render(
      <ToastProvider>
        <TraegerHolidayRegistrationPage />
      </ToastProvider>,
    );
    const title = await screen.findByRole("heading", {
      name: "Hinweise und Einwilligungen",
    });
    const evidence = within(title.closest("section")!);
    expect(
      evidence.getByText("Damals gültiger Ferienbrief"),
    ).toBeInTheDocument();
    expect(evidence.getByText("Gelesen")).toBeInTheDocument();
    expect(evidence.getByText("historische Ferienbrief")).toHaveProperty(
      "tagName",
      "STRONG",
    );
    expect(
      evidence.getByText("Zugestimmt am 03.11.2026, 11:12 Uhr"),
    ).toBeInTheDocument();
    expect(evidence.getAllByText("Nicht zugestimmt")).toHaveLength(1);
    // Anker: the information-only notice and its PDF remain visible.
    const notice = within(
      evidence.getByRole("heading", { name: "Trinkflaschen" }).parentElement!,
    );
    expect(
      notice.getByRole("button", { name: "Packliste öffnen" }),
    ).toBeInTheDocument();
    expect(notice.queryByText("Nicht zugestimmt")).not.toBeInTheDocument();
    expect(
      evidence.getByRole("button", { name: "Ferienbrief öffnen" }),
    ).toHaveTextContent("Öffnen");
    expect(
      evidence.getByRole("button", { name: "Ferienbrief herunterladen" }),
    ).toHaveTextContent("Herunterladen");
  });
});

function render(ui: React.ReactElement) {
  return rtlRender(
    <SWRConfig value={{ provider: () => new Map() }}>{ui}</SWRConfig>,
  );
}

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: {
      user: {
        id: "A",
        scope: "org",
        orgId: 7,
        permissions: sessionPermissions.current,
      },
    },
    status: "authenticated",
  }),
}));

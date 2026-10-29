import { render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { useTenantSlugSafe } from "~/lib/tenant-context";
import {
  HOLIDAY_CONTRIBUTION,
  HOLIDAY_PERIOD,
  HOLIDAY_REGISTRATION,
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
const { mockRegistration } = vi.hoisted(() => ({ mockRegistration: vi.fn() }));
vi.mock("~/lib/traeger-holiday-registrations-api", () => ({
  fetchHolidayRegistration: () => mockRegistration(),
}));
vi.mock("~/lib/traeger-contributions-api", () => ({
  fetchHolidayContribution: () => Promise.resolve(HOLIDAY_CONTRIBUTION),
}));

import TraegerHolidayRegistrationPage from "./page";

describe("TraegerHolidayRegistrationPage", () => {
  beforeEach(() => {
    mockRegistration.mockResolvedValue(HOLIDAY_REGISTRATION);
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
});

import { render, screen } from "@testing-library/react";
import type { ReactElement } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type { HolidayGuestSheet } from "~/lib/holiday-host-api";
import { SWRConfig } from "~/lib/swr/testing";
import { useTenantSlugSafe } from "~/lib/tenant-context";
import { HolidayGuestSection } from "./holiday-guest-section";

vi.unmock("swr");

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    status: "authenticated",
    data: { user: { id: "1", token: "test-token" } },
  }),
}));

const { sheetMock, clock } = vi.hoisted(() => ({
  sheetMock: vi.fn(),
  clock: { today: "2026-09-09" },
}));

vi.mock("~/lib/hooks/use-berlin-today", () => ({
  useBerlinToday: () => clock.today,
}));

vi.mock("~/lib/holiday-host-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/holiday-host-api")>()),
  fetchHolidayGuestSheet: sheetMock,
}));

vi.mock("./care-times-section", () => ({
  StudentCareTimesSection: ({
    initialRange,
  }: {
    initialRange?: { from: string; to: string };
  }) => (
    <div data-testid="care-times">
      {initialRange ? `${initialRange.from}/${initialRange.to}` : "Monat"}
    </div>
  ),
}));

function renderSection(ui: ReactElement) {
  return render(
    <ToastProvider>
      <SWRConfig value={{ provider: () => new Map(), dedupingInterval: 0 }}>
        {ui}
      </SWRConfig>
    </ToastProvider>,
  );
}

const sheet: HolidayGuestSheet = {
  studentId: "91",
  startsOn: "2026-09-14",
  endsOn: "2026-09-18",
  timeModelName: "bis 13 Uhr",
  timeModelStartsAt: "08:00",
  timeModelEndsAt: "13:00",
  homeSchoolId: null,
  guestSchoolName: "Grundschule Melle",
  schoolName: "Grundschule Melle",
  emergencyContacts: [
    { name: "Eva Muster", phone: "0151 2345678", relationship: "Eltern" },
    { name: "Oma Muster", phone: "05402 1234", relationship: "Großmutter" },
  ],
  pickupContacts: [{ name: "Opa Muster", phone: "", relationship: "" }],
  answers: [],
  health: null,
  readOnly: false,
};

// Die Testuhr steht auf Mittwoch, 09.09.2026.
describe("Soll und Ist des Feriengasts (#5994)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    clock.today = "2026-09-09";
    vi.mocked(useTenantSlugSafe).mockReturnValue("test-tenant");
  });

  it("zeigt die vergangene Ferienwoche", async () => {
    sheetMock.mockResolvedValue({
      ...sheet,
      startsOn: "2026-08-24",
      endsOn: "2026-08-28",
    });
    renderSection(<HolidayGuestSection studentId="91" canRead />);
    expect(await screen.findByTestId("care-times")).toHaveTextContent(
      "2026-08-24/2026-08-28",
    );
  });

  it("endet in der laufenden Ferienwoche heute", async () => {
    sheetMock.mockResolvedValue({
      ...sheet,
      startsOn: "2026-09-07",
      endsOn: "2026-09-11",
    });
    renderSection(<HolidayGuestSection studentId="91" canRead />);
    expect(await screen.findByTestId("care-times")).toHaveTextContent(
      "2026-09-07/2026-09-09",
    );
  });

  it("lädt vor der Ferienwoche keinen Vergleich", async () => {
    sheetMock.mockResolvedValue(sheet);
    renderSection(<HolidayGuestSection studentId="91" canRead />);
    expect(
      await screen.findByText("14.09.2026 bis 18.09.2026"),
    ).toBeInTheDocument();
    expect(screen.queryByTestId("care-times")).not.toBeInTheDocument();
  });

  it("beginnt den Vergleich, wenn bei offener Seite die Woche anfängt", async () => {
    clock.today = "2026-09-13";
    sheetMock.mockResolvedValue(sheet);
    const { rerender } = renderSection(
      <HolidayGuestSection studentId="91" canRead />,
    );
    expect(
      await screen.findByText("14.09.2026 bis 18.09.2026"),
    ).toBeInTheDocument();
    expect(screen.queryByTestId("care-times")).not.toBeInTheDocument();

    clock.today = "2026-09-14";
    rerender(
      <ToastProvider>
        <SWRConfig value={{ provider: () => new Map(), dedupingInterval: 0 }}>
          <HolidayGuestSection studentId="91" canRead />
        </SWRConfig>
      </ToastProvider>,
    );
    expect(await screen.findByTestId("care-times")).toHaveTextContent(
      "2026-09-14/2026-09-14",
    );
  });

  it("zeigt ohne Leserecht keinen Vergleich", () => {
    renderSection(<HolidayGuestSection studentId="91" canRead={false} />);
    expect(screen.getByText("Feriengast")).toBeInTheDocument();
    expect(screen.queryByTestId("care-times")).not.toBeInTheDocument();
    expect(sheetMock).not.toHaveBeenCalled();
  });
});

describe("HolidayGuestSection", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useTenantSlugSafe).mockReturnValue("test-tenant");
  });

  it("shows only care, emergency contacts and pickup persons", async () => {
    sheetMock.mockResolvedValue(sheet);

    renderSection(<HolidayGuestSection studentId="91" canRead />);

    expect(
      await screen.findByText("bis 13 Uhr (08:00 bis 13:00 Uhr)"),
    ).toBeInTheDocument();
    expect(screen.getByText("14.09.2026 bis 18.09.2026")).toBeInTheDocument();
    expect(screen.getByText("Gast aus Grundschule Melle")).toBeInTheDocument();
    expect(screen.getByText("Eva Muster (Eltern)")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "0151 2345678" })).toHaveAttribute(
      "href",
      "tel:01512345678",
    );
    expect(screen.getByText("Oma Muster (Großmutter)")).toBeInTheDocument();
    expect(
      screen.getByText("Außer den Eltern dürfen abholen:"),
    ).toBeInTheDocument();
    expect(screen.getByText("Opa Muster")).toBeInTheDocument();
    expect(sheetMock).toHaveBeenCalledWith("91");
  });

  it("names the home school of a child of another school of the Träger (#5630)", async () => {
    sheetMock.mockResolvedValue({
      ...sheet,
      homeSchoolId: "7",
      guestSchoolName: "",
      schoolName: "OGS Am Park",
    } satisfies HolidayGuestSheet);

    renderSection(<HolidayGuestSection studentId="91" canRead />);

    expect(await screen.findByText("Gast aus OGS Am Park")).toBeInTheDocument();
  });

  it("says Feriengast for a child without any school (#5630)", async () => {
    sheetMock.mockResolvedValue({
      ...sheet,
      guestSchoolName: "",
      schoolName: "",
    } satisfies HolidayGuestSheet);

    renderSection(<HolidayGuestSection studentId="91" canRead />);

    const school = await screen.findByText("Schule");
    expect(school.parentElement).toHaveTextContent("Feriengast");
  });

  it("shows the answers the Träger shares under their own heading (#5202)", async () => {
    sheetMock.mockResolvedValue({
      ...sheet,
      answers: [
        {
          key: "darf_allein_nach_hause",
          label: "Darf allein nach Hause gehen",
          type: "boolean",
          value: true,
        },
        {
          key: "darf_an_ausfluegen_teilnehmen",
          label: "Darf an Ausflügen teilnehmen",
          type: "boolean",
          value: null,
        },
      ],
    } satisfies HolidayGuestSheet);

    renderSection(<HolidayGuestSection studentId="91" canRead />);

    expect(
      await screen.findByText("Angaben aus der Ferienanmeldung"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Darf allein nach Hause gehen"),
    ).toBeInTheDocument();
    expect(screen.getByText("Ja")).toBeInTheDocument();
    expect(
      screen.getByText("Darf an Ausflügen teilnehmen"),
    ).toBeInTheDocument();
    expect(screen.getByText("–")).toBeInTheDocument();
  });

  it("shows the health information the parents agreed to share (#4722)", async () => {
    sheetMock.mockResolvedValue({
      ...sheet,
      health: {
        allergies: "",
        medication: "Asthmaspray",
        healthNotes: "Trägt eine Brille",
        answers: [],
      },
    });
    renderSection(<HolidayGuestSection studentId="91" canRead />);

    expect(await screen.findByText("Asthmaspray")).toBeInTheDocument();
    expect(screen.getByText("Gesundheit")).toBeInTheDocument();
    expect(screen.getByText("Medikamente")).toBeInTheDocument();
    expect(screen.getByText("Trägt eine Brille")).toBeInTheDocument();
  });

  it("keeps the sheet of before without shared answers", async () => {
    sheetMock.mockResolvedValue(sheet);

    renderSection(<HolidayGuestSection studentId="91" canRead />);

    // Anker: das Blatt ist geladen.
    expect(await screen.findByText("Eva Muster (Eltern)")).toBeInTheDocument();
    expect(
      screen.queryByText("Angaben aus der Ferienanmeldung"),
    ).not.toBeInTheDocument();
  });

  it("says the week is over during the read-only days", async () => {
    sheetMock.mockResolvedValue({
      ...sheet,
      readOnly: true,
      pickupContacts: [],
    });

    renderSection(<HolidayGuestSection studentId="91" canRead />);

    expect(
      await screen.findByText(
        "Die Ferienwoche ist vorbei. Die Angaben bleiben noch 7 Tage zum Nachlesen und werden dann gelöscht.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Nur die Eltern dürfen das Kind abholen."),
    ).toBeInTheDocument();
  });

  it("reads nothing without the right to read the children's details", () => {
    renderSection(<HolidayGuestSection studentId="91" canRead={false} />);

    expect(
      screen.getByText(
        "Notfall, Abholung und Gesundheit sieht, wer die Angaben der Kinder lesen darf. Bitte fragen Sie Ihre Leitung.",
      ),
    ).toBeInTheDocument();
    expect(sheetMock).not.toHaveBeenCalled();
  });

  it("hides the cached contacts once a reload refuses the sheet", async () => {
    const cache = new Map();
    const renderCached = () =>
      render(
        <ToastProvider>
          <SWRConfig value={{ provider: () => cache, dedupingInterval: 0 }}>
            <HolidayGuestSection studentId="91" canRead />
          </SWRConfig>
        </ToastProvider>,
      );
    sheetMock.mockResolvedValueOnce(sheet);
    const first = renderCached();
    expect(await screen.findByText("Eva Muster (Eltern)")).toBeInTheDocument();
    first.unmount();

    sheetMock.mockRejectedValue(
      new ApiError("holiday guest not found", 404, {
        code: "enrollment.holiday_guest_not_found",
      }),
    );
    renderCached();

    // Anker: der Ladefehler steht da.
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(screen.queryByText("Eva Muster (Eltern)")).not.toBeInTheDocument();
  });
});

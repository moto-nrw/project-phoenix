import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { SWRConfig } from "~/lib/swr/testing";
import { useTenantSlugSafe } from "~/lib/tenant-context";
import { HolidayStudentRegistrationSection } from "./holiday-student-registration-section";

vi.unmock("swr");

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    status: "authenticated",
    data: { user: { id: "1", token: "test-token" } },
  }),
}));

const { healthMock, registrationMock } = vi.hoisted(() => ({
  healthMock: vi.fn(),
  registrationMock: vi.fn(),
}));

vi.mock("~/lib/holiday-host-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/holiday-host-api")>()),
  fetchHolidayStudentHealth: healthMock,
  fetchHolidayStudentRegistrations: registrationMock,
}));

function renderSection() {
  return render(
    <ToastProvider>
      <SWRConfig value={{ provider: () => new Map(), dedupingInterval: 0 }}>
        <HolidayStudentRegistrationSection
          studentId="7"
          schoolHealthInfo="Laktose (Schule)"
        />
      </SWRConfig>
    </ToastProvider>,
  );
}

describe("HolidayStudentRegistrationSection", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useTenantSlugSafe).mockReturnValue("test-tenant");
    healthMock.mockResolvedValue([]);
    registrationMock.mockResolvedValue([]);
  });

  it("shows the health information of the holiday registration beside the child's own (#4722)", async () => {
    healthMock.mockResolvedValue([
      {
        weekId: "51",
        periodName: "Sommerferien",
        startsOn: "2026-09-14",
        endsOn: "2026-09-18",
        health: {
          allergies: "Erdnüsse",
          medication: "",
          healthNotes: "",
          answers: [
            {
              key: "tetanus_impfschutz",
              label: "Tetanus-Impfschutz",
              type: "select",
              value: "Ja",
            },
          ],
        },
      },
    ]);
    renderSection();

    expect(await screen.findByText("Erdnüsse")).toBeInTheDocument();
    expect(
      screen.getByText("Angaben aus der Ferienanmeldung"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Ferienwoche 14.09.2026 bis 18.09.2026"),
    ).toBeInTheDocument();
    expect(screen.getByText("Tetanus-Impfschutz")).toBeInTheDocument();
    // The school's own entry stands beside it, so a difference shows.
    expect(screen.getByText("Laktose (Schule)")).toBeInTheDocument();
    expect(healthMock).toHaveBeenCalledWith("7");
  });

  const registration = {
    weekId: "51",
    periodName: "Weihnachtsferien",
    startsOn: "2027-01-04",
    endsOn: "2027-01-08",
    days: [],
    timeModelName: "bis 16 Uhr",
    timeModelStartsAt: "08:00",
    timeModelEndsAt: "16:00",
    emergencyContacts: [
      { name: "Eva Muster", phone: "0151 2345678", relationship: "Eltern" },
    ],
    pickupContacts: [
      { name: "Opa Muster", phone: "", relationship: "Großvater" },
    ],
  };

  it("shows care, emergency contacts and pickup persons of the registration (#6678)", async () => {
    registrationMock.mockResolvedValue([registration]);
    renderSection();

    expect(
      await screen.findByText("Ferienwoche 04.01.2027 bis 08.01.2027"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("bis 16 Uhr (08:00 bis 16:00 Uhr)"),
    ).toBeInTheDocument();
    expect(screen.getByText("ganze Woche")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Der Check-in erwartet das Kind an den gebuchten Tagen.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Eva Muster (Eltern)")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "0151 2345678" })).toHaveAttribute(
      "href",
      "tel:01512345678",
    );
    expect(screen.getByText("Opa Muster (Großvater)")).toBeInTheDocument();
    expect(
      screen.getByText("Außer den Eltern dürfen abholen:"),
    ).toBeInTheDocument();
    // Without health information the school's own entry is not repeated.
    expect(screen.queryByText("In moto eingetragen")).not.toBeInTheDocument();
    expect(registrationMock).toHaveBeenCalledWith("7");
  });

  it("shows nothing for a child without a holiday registration", async () => {
    render(
      <ToastProvider>
        <SWRConfig value={{ provider: () => new Map(), dedupingInterval: 0 }}>
          <HolidayStudentRegistrationSection
            studentId="7"
            schoolHealthInfo={null}
          />
          <p>Stammdaten</p>
        </SWRConfig>
      </ToastProvider>,
    );

    await vi.waitFor(() => expect(registrationMock).toHaveBeenCalledWith("7"));
    await vi.waitFor(() => expect(healthMock).toHaveBeenCalledWith("7"));
    // Anker: the rest of the child file stands, only this section is absent.
    expect(screen.getByText("Stammdaten")).toBeInTheDocument();
    expect(
      screen.queryByText("Angaben aus der Ferienanmeldung"),
    ).not.toBeInTheDocument();
  });
});

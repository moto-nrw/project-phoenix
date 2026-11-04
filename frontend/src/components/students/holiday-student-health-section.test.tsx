import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { SWRConfig } from "~/lib/swr/testing";
import { useTenantSlugSafe } from "~/lib/tenant-context";
import { HolidayStudentHealthSection } from "./holiday-student-health-section";

vi.unmock("swr");

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    status: "authenticated",
    data: { user: { id: "1", token: "test-token" } },
  }),
}));

const { healthMock } = vi.hoisted(() => ({ healthMock: vi.fn() }));

vi.mock("~/lib/holiday-host-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/holiday-host-api")>()),
  fetchHolidayStudentHealth: healthMock,
}));

function renderSection() {
  return render(
    <ToastProvider>
      <SWRConfig value={{ provider: () => new Map(), dedupingInterval: 0 }}>
        <HolidayStudentHealthSection
          studentId="7"
          schoolHealthInfo="Laktose (Schule)"
        />
      </SWRConfig>
    </ToastProvider>,
  );
}

describe("HolidayStudentHealthSection", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useTenantSlugSafe).mockReturnValue("test-tenant");
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
});

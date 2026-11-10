import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { ToastProvider } from "~/contexts/ToastContext";
import type { HolidayWeekPlaces } from "~/lib/traeger-holiday-registrations-api";
import { HOLIDAY_PERIOD } from "~/test/fixtures/holiday-registrations";

const { fetchPlaces } = vi.hoisted(() => ({ fetchPlaces: vi.fn() }));
vi.mock("next/navigation", () => ({ useParams: () => ({ id: "5" }) }));
vi.mock("~/components/traeger/use-traeger-resources", () => ({
  useTraegerHolidayPeriod: () => ({ data: HOLIDAY_PERIOD }),
}));
vi.mock("~/components/traeger/use-traeger-schools", () => ({
  useTraegerSchools: () => ({ schools: [] }),
}));
vi.mock("~/lib/traeger-holiday-registrations-api", async (original) => ({
  ...(await original<
    typeof import("~/lib/traeger-holiday-registrations-api")
  >()),
  fetchHolidayWeekPlaces: () => fetchPlaces(),
}));
vi.mock("~/components/ui/tenant-page", () => ({
  TenantPage: ({
    stats,
    children,
  }: {
    stats: ReactNode;
    children: ReactNode;
  }) => (
    <div>
      {stats}
      {children}
    </div>
  ),
}));
vi.mock("~/lib/traeger-url", () => ({ traegerPath: (path: string) => path }));

import HolidayPlacesPage from "./page";

describe("holiday places summary", () => {
  it("counts a child waiting in three weeks only once", async () => {
    const weeks: HolidayWeekPlaces[] = ["51", "52", "53"].map((weekId) => ({
      weekId,
      startsOn: "2027-07-05",
      endsOn: "2027-07-09",
      hostSchoolId: "31",
      capacity: 0,
      placed: 0,
      waitlist: [
        {
          childId: "21",
          childFirstName: "Tom",
          childLastName: "Platz",
          registrationId: "14",
          registrationStatus: "received",
          submittedAt: "2026-11-09T13:12:00Z",
          timeModelName: "bis 13 Uhr",
          position: 1,
        },
      ],
    }));
    fetchPlaces.mockResolvedValue(weeks);
    render(
      <ToastProvider>
        <HolidayPlacesPage />
      </ToastProvider>,
    );
    expect(
      await screen.findByText("1 Kind wartet auf einen Platz"),
    ).toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: /Tom Platz/ })).toHaveLength(3);
  });
});

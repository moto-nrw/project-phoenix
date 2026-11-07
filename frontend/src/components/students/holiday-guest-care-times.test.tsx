import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "~/lib/api-error";
import type { HolidayGuestSheet } from "~/lib/holiday-host-api";
import { SWRConfig } from "~/lib/swr/testing";
import { useTenantSlugSafe } from "~/lib/tenant-context";
import { HolidayGuestCareTimes } from "./holiday-guest-care-times";

vi.unmock("swr");

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    status: "authenticated",
    data: { user: { id: "1", token: "test-token" } },
  }),
}));

const { sheetMock } = vi.hoisted(() => ({ sheetMock: vi.fn() }));

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

function week(startsOn: string, endsOn: string): HolidayGuestSheet {
  return { startsOn, endsOn } as HolidayGuestSheet;
}

function renderGuest() {
  return render(
    <SWRConfig value={{ provider: () => new Map(), dedupingInterval: 0 }}>
      <HolidayGuestCareTimes studentId="91" />
    </SWRConfig>,
  );
}

// Die Testuhr steht auf Mittwoch, 09.09.2026.
describe("HolidayGuestCareTimes (#5994)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useTenantSlugSafe).mockReturnValue("test-tenant");
  });

  it("beginnt mit der vergangenen Ferienwoche", async () => {
    sheetMock.mockResolvedValue(week("2026-08-24", "2026-08-28"));
    renderGuest();
    expect(await screen.findByTestId("care-times")).toHaveTextContent(
      "2026-08-24/2026-08-28",
    );
  });

  it("endet in der laufenden Ferienwoche heute", async () => {
    sheetMock.mockResolvedValue(week("2026-09-07", "2026-09-11"));
    renderGuest();
    expect(await screen.findByTestId("care-times")).toHaveTextContent(
      "2026-09-07/2026-09-09",
    );
  });

  it("bleibt beim laufenden Monat vor der Woche und ohne Bogen", async () => {
    sheetMock.mockResolvedValueOnce(week("2026-09-14", "2026-09-18"));
    const { unmount } = renderGuest();
    expect(await screen.findByTestId("care-times")).toHaveTextContent("Monat");
    unmount();

    sheetMock.mockRejectedValueOnce(
      new ApiError("holiday guest not found", 404, {
        code: "enrollment.holiday_guest_not_found",
      }),
    );
    renderGuest();
    expect(await screen.findByTestId("care-times")).toHaveTextContent("Monat");
  });
});

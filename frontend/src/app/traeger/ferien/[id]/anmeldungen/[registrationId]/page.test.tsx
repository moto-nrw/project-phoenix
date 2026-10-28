import { render, screen } from "@testing-library/react";
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
vi.mock("~/lib/traeger-holiday-registrations-api", () => ({
  fetchHolidayRegistration: () => Promise.resolve(HOLIDAY_REGISTRATION),
}));
vi.mock("~/lib/traeger-contributions-api", () => ({
  fetchHolidayContribution: () => Promise.resolve(HOLIDAY_CONTRIBUTION),
}));

import TraegerHolidayRegistrationPage from "./page";

describe("TraegerHolidayRegistrationPage", () => {
  beforeEach(() => {
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
});

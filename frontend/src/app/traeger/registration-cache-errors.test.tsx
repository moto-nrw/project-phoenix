import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { useTenantMutateMatching } from "~/lib/swr";
import { traegerKeys } from "~/lib/swr/portal-keys";
import { SWRConfig } from "~/lib/swr/testing";
import { catalogText } from "~/test/error-catalog-text";
import {
  HOLIDAY_CONTRIBUTION,
  HOLIDAY_PERIOD,
  HOLIDAY_REGISTRATION,
  HOLIDAY_REGISTRATION_LIST,
  HOLIDAY_SCHOOLS,
} from "~/test/fixtures/holiday-registrations";
import RegistrationsPage from "./ferien/[id]/anmeldungen/page";
import RegistrationPage from "./ferien/[id]/anmeldungen/[registrationId]/page";

vi.unmock("swr");
const { period, schools } = vi.hoisted(() => ({
  period: vi.fn(),
  schools: vi.fn(),
}));
vi.mock("~/lib/traeger-api", () => ({
  fetchHolidayPeriod: period,
  fetchTraegerSchools: schools,
}));
vi.mock("~/lib/traeger-holiday-registrations-api", async (original) => ({
  ...(await original<
    typeof import("~/lib/traeger-holiday-registrations-api")
  >()),
  fetchHolidayRegistrations: () => Promise.resolve(HOLIDAY_REGISTRATION_LIST),
  fetchHolidayRegistration: () => Promise.resolve(HOLIDAY_REGISTRATION),
}));
vi.mock("~/lib/traeger-contributions-api", () => ({
  fetchHolidayContribution: () => Promise.resolve(HOLIDAY_CONTRIBUTION),
}));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: "A", scope: "org", orgId: 7 } },
    status: "authenticated",
  }),
}));
vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => null,
  useTenantRoutingModeSafe: () => null,
}));
vi.mock("next/navigation", () => ({
  useParams: () => ({ id: HOLIDAY_PERIOD.id, registrationId: "7" }),
  useRouter: () => ({ push: vi.fn() }),
}));

function Refresh() {
  const refresh = useTenantMutateMatching([
    traegerKeys.period(HOLIDAY_PERIOD.id),
    traegerKeys.schools(),
  ]);
  return (
    <button onClick={() => void refresh().catch(() => undefined)}>
      Refresh metadata
    </button>
  );
}

beforeEach(() => {
  vi.stubEnv("NEXT_PUBLIC_TRAEGER_HOSTNAME", "traeger.localhost:3000");
  period.mockReset().mockResolvedValue(HOLIDAY_PERIOD);
  schools.mockReset().mockResolvedValue(HOLIDAY_SCHOOLS);
});
afterEach(() => vi.unstubAllEnvs());

describe.each([
  {
    name: "list",
    Page: RegistrationsPage,
    content: () => screen.queryByRole("table"),
  },
  {
    name: "detail",
    Page: RegistrationPage,
    content: () => screen.queryByRole("heading", { name: "Eltern" }),
  },
])("registration $name metadata errors", ({ Page, content }) => {
  it.each([503, 401, 403, 404])(
    "preserves content only for recoverable errors (schools %s, period 503)",
    async (status) => {
      render(
        <SWRConfig value={{ provider: () => new Map() }}>
          <ToastProvider>
            <Refresh />
            <Page />
          </ToastProvider>
        </SWRConfig>,
      );
      // Anker: the actual registration content has loaded before refreshing.
      await waitFor(() => expect(content()).toBeInTheDocument());
      period.mockRejectedValue(new ApiError("unavailable", 503));
      schools.mockRejectedValue(new ApiError("schools failed", status));
      fireEvent.click(screen.getByRole("button", { name: "Refresh metadata" }));
      expect(
        await screen.findByText(
          catalogText("general.unavailable", "den Ferienzeitraum"),
        ),
      ).toBeInTheDocument();
      await waitFor(() => expect(schools).toHaveBeenCalledTimes(2));
      if (status === 503) expect(content()).toBeInTheDocument();
      else await waitFor(() => expect(content()).not.toBeInTheDocument());
    },
  );
});

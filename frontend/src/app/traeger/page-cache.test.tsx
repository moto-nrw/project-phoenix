import { act, fireEvent, render, screen } from "@testing-library/react";
import type { HolidayPeriod } from "~/lib/traeger-api";
import type { Session } from "next-auth";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { SWRConfig } from "~/lib/swr/testing";
import { SWRScopeBoundary } from "~/lib/swr/scope-boundary";
import {
  HOLIDAY_PERIOD,
  HOLIDAY_SCHOOLS,
} from "~/test/fixtures/holiday-registrations";
import { useTenantMutateMatching } from "~/lib/swr";
import { traegerKeys } from "~/lib/swr/portal-keys";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";
import { resumeSWRSession } from "~/lib/swr/scope";
import PeriodsPage from "./ferien/page";
import PeriodPage from "./ferien/[id]/page";

vi.unmock("swr");
const { api, identity, announce } = vi.hoisted(() => ({
  api: {
    fetchHolidayPeriods: vi.fn(),
    fetchHolidayPeriod: vi.fn(),
    fetchTraegerSchools: vi.fn(),
  },
  identity: { session: null as Session | null, id: "" },
  announce: vi.fn(),
}));
vi.mock("~/lib/traeger-api", () => api);
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: identity.session,
    status: identity.session ? "authenticated" : "unauthenticated",
  }),
}));
vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => null,
  useTenantRoutingModeSafe: () => null,
}));
vi.mock("~/lib/session-cache", () => ({ announceSessionChange: announce }));
vi.mock("next/navigation", () => ({
  useParams: () => ({ id: identity.id }),
  useRouter: () => ({ push: vi.fn() }),
}));
vi.mock("~/lib/traeger-url", () => ({ traegerPath: (path: string) => path }));
vi.mock("~/components/traeger/holiday-period-form", () => ({
  HolidayPeriodForm: ({
    onSaved,
    period,
  }: {
    onSaved: (next: HolidayPeriod) => void;
    period: HolidayPeriod;
  }) => (
    <>
      <p>Confirmed form</p>
      <input aria-label="Draft text" defaultValue={period.name} />
      <button onClick={() => onSaved({ ...period, name: "Saved period" })}>
        Save answer
      </button>
    </>
  ),
}));
vi.mock("~/components/traeger/holiday-period-publish-card", () => ({
  HolidayPeriodPublishCard: ({
    onPublished,
    period,
  }: {
    onPublished: (next: HolidayPeriod) => void;
    period: HolidayPeriod;
  }) => (
    <>
      <p>Confirmed actions</p>
      <button
        onClick={() => onPublished({ ...period, name: "Published period" })}
      >
        Publish answer
      </button>
    </>
  ),
}));

function signIn(id = "A", orgId = 7) {
  identity.session = {
    user: { id, scope: "org", orgId },
    expires: "2099-01-01",
  } as Session;
}
function setup() {
  const cache = new Map();
  function Wrapper({ children }: Readonly<{ children: React.ReactNode }>) {
    return (
      <SWRConfig value={{ provider: () => cache }}>
        <ToastProvider>
          <SWRScopeBoundary />
          {children}
        </ToastProvider>
      </SWRConfig>
    );
  }
  return { wrapper: Wrapper };
}
function Refresh() {
  const mutate = useTenantMutateMatching([traegerKeys.period(identity.id)]);
  return (
    <button
      onClick={() => {
        void mutate().catch(() => undefined);
      }}
    >
      Refresh metadata
    </button>
  );
}
function Route({ detail }: Readonly<{ detail: boolean }>) {
  return detail ? (
    <>
      <Refresh />
      <PeriodPage />
    </>
  ) : (
    <PeriodsPage />
  );
}
beforeEach(() => {
  signIn();
  identity.id = HOLIDAY_PERIOD.id;
  api.fetchHolidayPeriods.mockReset().mockResolvedValue([HOLIDAY_PERIOD]);
  api.fetchHolidayPeriod.mockReset().mockResolvedValue(HOLIDAY_PERIOD);
  api.fetchTraegerSchools.mockReset().mockResolvedValue(HOLIDAY_SCHOOLS);
});

describe("Träger list-to-detail cache", () => {
  it("shows only the list name immediately while detail and schools start in parallel", async () => {
    const view = render(<Route detail={false} />, setup());
    expect(
      (await screen.findAllByText(HOLIDAY_PERIOD.name))[0],
    ).toBeInTheDocument();
    api.fetchHolidayPeriod.mockImplementation(
      () => new Promise(() => undefined),
    );
    api.fetchTraegerSchools.mockImplementation(
      () => new Promise(() => undefined),
    );
    view.rerender(<Route detail />);
    // Anker: the preview is the loaded list identity, not the generic loading title.
    expect(
      screen.getByRole("heading", { name: HOLIDAY_PERIOD.name }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Confirmed form")).not.toBeInTheDocument();
    expect(screen.queryByText("Confirmed actions")).not.toBeInTheDocument();
    expect(api.fetchHolidayPeriod).toHaveBeenCalledTimes(1);
    expect(api.fetchTraegerSchools).toHaveBeenCalledTimes(1);
  });

  it("shows body and actions only after the actual detail answer, and changes no calendar dates", async () => {
    let answer!: (value: typeof HOLIDAY_PERIOD) => void;
    api.fetchHolidayPeriod.mockImplementation(
      () =>
        new Promise((resolve) => {
          answer = resolve;
        }),
    );
    const view = render(<Route detail={false} />, setup());
    expect(
      (await screen.findAllByText(HOLIDAY_PERIOD.name))[0],
    ).toBeInTheDocument();
    view.rerender(<Route detail />);
    await act(async () => {
      answer(HOLIDAY_PERIOD);
    });
    expect(await screen.findByText("Confirmed form")).toBeInTheDocument();
    expect(screen.getByText("Confirmed actions")).toBeInTheDocument();
    expect(api.fetchHolidayPeriod).toHaveBeenCalledWith(HOLIDAY_PERIOD.id);
  });

  it("keeps the unsaved draft mounted when a background refresh fails", async () => {
    render(<Route detail />, setup());
    expect(await screen.findByText("Confirmed form")).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Draft text"), {
      target: { value: "Unsaved text" },
    });
    api.fetchHolidayPeriod.mockRejectedValue(
      new ApiError("unavailable", 503, { code: "general.unavailable" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Refresh metadata" }));
    expect(
      await screen.findByText(
        catalogText("general.unavailable", "den Ferienzeitraum"),
      ),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Draft text")).toHaveValue("Unsaved text");
  });

  it("still replaces the form when the server refuses the cached detail", async () => {
    render(<Route detail />, setup());
    expect(await screen.findByText("Confirmed form")).toBeInTheDocument();
    api.fetchHolidayPeriod.mockRejectedValue(
      new ApiError("forbidden", 403, { code: "general.permission" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Refresh metadata" }));
    // Anker: the refusal of a previously loaded detail is shown.
    expect(
      await screen.findByText(
        catalogText("general.permission", "den Ferienzeitraum"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByLabelText("Draft text")).not.toBeInTheDocument();
  });

  it.each(["Save", "Publish"])(
    "keeps the %s answer and invalidates the list before returning",
    async (action) => {
      const view = render(<Route detail={false} />, setup());
      expect(
        (await screen.findAllByText(HOLIDAY_PERIOD.name))[0],
      ).toBeInTheDocument();
      view.rerender(<Route detail />);
      expect(await screen.findByText("Confirmed form")).toBeInTheDocument();
      const name = action === "Save" ? "Saved period" : "Published period";
      api.fetchHolidayPeriods.mockResolvedValue([{ ...HOLIDAY_PERIOD, name }]);
      fireEvent.click(screen.getByRole("button", { name: `${action} answer` }));
      expect(screen.getByRole("heading", { name })).toBeInTheDocument();
      view.rerender(<Route detail={false} />);
      expect((await screen.findAllByText(name))[0]).toBeInTheDocument();
      expect(api.fetchHolidayPeriods).toHaveBeenCalledTimes(2);
    },
  );

  it.each([
    "account",
    "organization",
    "logout",
    "other-tab-account",
    "other-tab-logout",
  ])("withdraws the preview on %s", async (change) => {
    const view = render(<Route detail={false} />, setup());
    expect(
      (await screen.findAllByText(HOLIDAY_PERIOD.name))[0],
    ).toBeInTheDocument();
    api.fetchHolidayPeriod.mockImplementation(
      () => new Promise(() => undefined),
    );
    view.rerender(<Route detail />);
    expect(
      screen.getByRole("heading", { name: HOLIDAY_PERIOD.name }),
    ).toBeInTheDocument();
    if (change.startsWith("other-tab")) {
      act(() => {
        globalThis.dispatchEvent(
          new StorageEvent("storage", {
            key: "moto:personal-storage-owner",
            newValue: change.endsWith("logout") ? null : "7:B",
          }),
        );
      });
    } else {
      if (change === "logout") identity.session = null;
      else
        signIn(
          change === "account" ? "B" : "A",
          change === "organization" ? 8 : 7,
        );
      view.rerender(<Route detail />);
    }
    // Anker: the same detail is mounted, now in its generic loading state.
    expect(
      screen.getByRole("heading", { name: "Ferienzeitraum" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: HOLIDAY_PERIOD.name }),
    ).not.toBeInTheDocument();
    if (change.startsWith("other-tab")) expect(announce).toHaveBeenCalled();
  });
});

afterEach(() => resumeSWRSession());

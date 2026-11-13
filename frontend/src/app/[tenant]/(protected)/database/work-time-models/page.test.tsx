import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import WorkTimeModelsPage from "./page";
import { MODULE_KEYS } from "~/lib/modules";
import type { TenantInfo } from "~/lib/tenant-api";
import { useBookedModules, useTenantSafe } from "~/lib/tenant-context";

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: null, status: "unauthenticated" }),
}));
vi.mock("~/lib/swr/hooks", () => ({ useDiscardRefusedEntity: () => vi.fn() }));
vi.mock("~/hooks/useUpdateUrlParams", () => ({
  useUpdateUrlParams: () => vi.fn(),
}));

let currentSearch = new URLSearchParams();
vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), back: vi.fn() }),
  usePathname: () => "/test-tenant/database/work-time-models",
  useSearchParams: () => currentSearch,
}));

vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useApiErrorDisplay: () => ({ show: vi.fn() }),
  useToast: () => ({
    success: vi.fn(),
    error: vi.fn(),
    warning: vi.fn(),
    info: vi.fn(),
    remove: vi.fn(),
  }),
}));

vi.mock("~/components/ui/hooks/useIsMobile", () => ({
  useIsMobile: () => false,
}));

const workTimeModelService = vi.hoisted(() => ({
  listWithUsage: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  archive: vi.fn(),
  restore: vi.fn(),
  remove: vi.fn(),
}));
vi.mock("~/lib/staff-api", () => ({ workTimeModelService }));

type SWRState = { data: unknown; error: unknown; isLoading: boolean };
const swrState = vi.hoisted(() => ({
  current: {} as Record<string, SWRState>,
  keys: [] as (string | null)[],
}));
vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null) => {
    swrState.keys.push(key);
    return (
      (key === null ? undefined : swrState.current[key]) ?? {
        data: undefined,
        error: undefined,
        isLoading: false,
      }
    );
  },
  useTenantMutate: () => vi.fn(() => Promise.resolve()),
  useInvalidateMatching: () => vi.fn(() => Promise.resolve()),
}));

const FULL_TIME = {
  id: "7",
  name: "Vollzeit",
  rotationLength: 1,
  rotationAnchorDate: "2026-09-07",
  entries: [0, 1, 2, 3, 4].map((dayOfWeek) => ({
    weekIndex: 0,
    dayOfWeek,
    targetMinutes: 480,
  })),
  weeklyTotals: [2400],
  archivedAt: null,
  assignedStaffCount: 2,
};
const OLD = {
  ...FULL_TIME,
  id: "8",
  name: "Altvertrag",
  archivedAt: "2026-09-01T08:00:00Z",
  assignedStaffCount: 0,
};

beforeEach(() => {
  vi.mocked(useTenantSafe).mockReturnValue({
    tenantSlug: "demo",
    tenant: { tenantId: 1, slug: "demo" } as TenantInfo,
    routingMode: "subdomain",
  });
  vi.mocked(useBookedModules).mockReturnValue(MODULE_KEYS);
  currentSearch = new URLSearchParams("eintrag=7");
  swrState.keys = [];
  swrState.current = {
    "database-work-time-models": {
      data: [FULL_TIME, OLD],
      error: undefined,
      isLoading: false,
    },
  };
  workTimeModelService.update.mockResolvedValue(FULL_TIME);
  vi.stubGlobal(
    "matchMedia",
    vi.fn((query: string) => ({
      matches: false,
      media: query,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

describe("Arbeitszeitmodelle (#6891)", () => {
  it("loads nothing when Zeiterfassung is not booked", () => {
    vi.mocked(useBookedModules).mockReturnValue(
      MODULE_KEYS.filter((key) => key !== "zeiterfassung"),
    );
    render(<WorkTimeModelsPage />);

    expect(screen.getByText("Zeiterfassung ist nicht gebucht")).toBeVisible();
    expect(swrState.keys).toEqual([]);
  });

  it("lists hours, rotation and users, and keeps archived templates apart", () => {
    render(<WorkTimeModelsPage />);

    expect(
      screen.getAllByText("40 Std. pro Woche · 1 Woche · 2 Personen").length,
    ).toBeGreaterThan(0);
    expect(screen.getByText("Nicht mehr in der Auswahl")).toBeInTheDocument();
    expect(
      screen.getByText("1 Vorlage · 1 nicht mehr angeboten"),
    ).toBeInTheDocument();
  });

  it("says before saving whom a change reaches", () => {
    render(<WorkTimeModelsPage />);

    expect(
      screen.getByText(
        "2 Personen nutzen diese Vorlage. Wenn Sie speichern, gilt das neue Soll ab heute für alle 2.",
      ),
    ).toBeInTheDocument();
  });

  it("saves edited hours with the kept anchor", async () => {
    render(<WorkTimeModelsPage />);

    fireEvent.change(screen.getByLabelText("Dezimalstunden Freitag"), {
      target: { value: "4" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(workTimeModelService.update).toHaveBeenCalledWith("7", {
        name: "Vollzeit",
        rotationLength: 1,
        rotationAnchorDate: "2026-09-07",
        entries: [
          ...[0, 1, 2, 3].map((dayOfWeek) => ({
            weekIndex: 0,
            dayOfWeek,
            targetMinutes: 480,
          })),
          { weekIndex: 0, dayOfWeek: 4, targetMinutes: 240 },
        ],
      }),
    );
  });

  it("refuses a rotation week without hours", async () => {
    render(<WorkTimeModelsPage />);

    fireEvent.click(screen.getByRole("combobox", { name: "Rotation" }));
    fireEvent.click(
      screen.getByRole("option", { name: "A/B-Wochen (2 Wochen)" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findAllByText(
        "Woche B hat noch keine Stunden. Bitte tragen Sie mindestens einen Tag ein.",
      ),
    ).not.toHaveLength(0);
    expect(workTimeModelService.update).not.toHaveBeenCalled();
  });
});

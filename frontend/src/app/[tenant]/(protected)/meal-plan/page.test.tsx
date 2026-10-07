import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

import MealPlanPage from "./page";

const { getMealPlanWeek } = vi.hoisted(() => ({
  getMealPlanWeek: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), back: vi.fn() }),
  usePathname: () => "/schule/meal-plan",
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: "1", roles: ["admin"] } },
    status: "authenticated",
  }),
}));

vi.mock("~/lib/auth-utils", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/auth-utils")>()),
  isAdmin: () => true,
  hasPermission: () => true,
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
}));

vi.mock("~/lib/hooks/use-navigation-guard", () => ({
  useNavigationGuard: () => ({
    pendingHref: null,
    confirmNavigation: vi.fn(),
    cancelNavigation: vi.fn(),
  }),
}));

vi.mock("~/lib/hooks/use-settings-schema", () => ({
  useSettingsSchema: () => ({ data: undefined }),
}));

vi.mock("~/lib/hooks/use-berlin-today", () => ({
  useBerlinToday: () => "2026-10-06",
}));

vi.mock("~/lib/meal-plan-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/meal-plan-api")>()),
  getMealPlanWeek,
}));

function renderPage() {
  return render(
    <ToastProvider>
      <MealPlanPage />
    </ToastProvider>,
  );
}

describe("Essensplan: Statuszeile bei Ladefehler (#2517)", () => {
  beforeEach(() => {
    getMealPlanWeek.mockReset();
  });

  it("shows the catalog load error and no planned-days count", async () => {
    getMealPlanWeek.mockRejectedValue(
      new ApiError("week exploded", 503, { code: "general.unavailable" }),
    );

    renderPage();

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Woche im Essensplan"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Tagen geplant/)).not.toBeInTheDocument();
    // Kalenderwoche und Zeitraum hängen nicht an den Daten und bleiben.
    expect(screen.getByText(/KW 41/)).toBeInTheDocument();
  });

  it("counts the planned days of a loaded week", async () => {
    getMealPlanWeek.mockResolvedValue([]);

    renderPage();

    expect(
      await screen.findByText(/0 von 5 Tagen geplant/),
    ).toBeInTheDocument();
  });
});

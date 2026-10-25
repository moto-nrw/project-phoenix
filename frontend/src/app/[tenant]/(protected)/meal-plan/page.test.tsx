import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

import MealPlanPage from "./page";

const { getMealPlanWeek, getMealOrders, setDay, rights, features } = vi.hoisted(
  () => ({
    features: { mealPlanEnabled: true },
    rights: { admin: true, permissions: [] as string[] },
    getMealPlanWeek: vi.fn(),
    getMealOrders: vi.fn(),
    setDay: vi.fn(),
  }),
);

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
  isAdmin: () => rights.admin,
  hasPermission: (_session: unknown, permission: string) =>
    rights.permissions.includes(permission),
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

vi.mock("~/lib/hooks/use-berlin-today", () => ({
  useBerlinToday: () => "2026-10-06",
}));

vi.mock("~/lib/meal-plan-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/meal-plan-api")>()),
  getMealPlanWeek,
  getMealOrders,
  setDay,
}));

vi.mock("~/lib/tenant-context", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/tenant-context")>()),
  useMealPlanEnabled: () => features.mealPlanEnabled,
  useMealRegistrationEnabled: () => true,
}));
vi.mock("~/components/meal-plan/meal-participant-list", () => ({
  MealParticipantList: ({
    canRegister,
    canExport,
    onRegistrationChanged,
  }: {
    canRegister: boolean;
    canExport: boolean;
    onRegistrationChanged: () => void;
  }) => (
    <div
      data-testid="participants"
      data-register={String(canRegister)}
      data-export={String(canExport)}
    >
      <button onClick={onRegistrationChanged}>Save registration</button>
    </div>
  ),
}));

function renderPage() {
  return render(
    <ToastProvider>
      <MealPlanPage />
    </ToastProvider>,
  );
}

describe("Speiseplan: Statuszeile bei Ladefehler (#2517)", () => {
  beforeEach(() => {
    getMealPlanWeek.mockReset();
    getMealOrders.mockReset();
    getMealOrders.mockResolvedValue({
      rhythm: "daily",
      detail: "total",
      days: [],
    });
    rights.admin = true;
    rights.permissions = [];
  });

  it("reloads the order overview after saving a registration", async () => {
    getMealPlanWeek.mockResolvedValue([]);
    renderPage();
    await waitFor(() => expect(getMealOrders).toHaveBeenCalledTimes(1));
    fireEvent.click(
      await screen.findByRole("button", { name: "Save registration" }),
    );
    await waitFor(() => expect(getMealOrders).toHaveBeenCalledTimes(2));
  });

  it("shows the catalog load error and no planned-days count", async () => {
    getMealPlanWeek.mockRejectedValue(
      new ApiError("week exploded", 503, { code: "general.unavailable" }),
    );

    renderPage();

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Woche im Speiseplan"),
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

// Der Wochenwechsel steuert Wochenplan und Bestellung und steht darum als
// Bedienband in der Kopfkarte (Bauart 3 Regel 1, TENANT-PAGE-SPEC Regel 3).
describe("Speiseplan: Wochenwechsel im Seitenkopf", () => {
  beforeEach(() => {
    getMealPlanWeek.mockReset();
    getMealPlanWeek.mockResolvedValue([]);
    getMealOrders.mockReset();
    getMealOrders.mockResolvedValue({
      rhythm: "daily",
      detail: "total",
      days: [],
    });
    rights.admin = true;
    rights.permissions = [];
    features.mealPlanEnabled = true;
  });

  it("names the week once in the head bar, not again in the status line", async () => {
    renderPage();

    // Die Statuszeile trägt nur noch die geplanten Tage.
    expect(
      await screen.findByText("0 von 5 Tagen geplant"),
    ).toBeInTheDocument();
    expect(screen.getByText("KW 41 · 05.10.–09.10.2026")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Diese Woche" })).toBeDisabled();
  });

  it("switches plan and order together and returns to this week", async () => {
    renderPage();
    await screen.findByText("0 von 5 Tagen geplant");

    fireEvent.click(screen.getByRole("button", { name: "Nächste Woche" }));

    await waitFor(() =>
      expect(getMealPlanWeek).toHaveBeenLastCalledWith("2026-10-12"),
    );
    await waitFor(() =>
      expect(getMealOrders).toHaveBeenLastCalledWith(
        "2026-10-12",
        "2026-10-16",
      ),
    );
    expect(
      await screen.findByText("KW 42 · 12.10.–16.10.2026"),
    ).toBeInTheDocument();

    fireEvent.click(await screen.findByRole("button", { name: "Diese Woche" }));
    await waitFor(() =>
      expect(getMealPlanWeek).toHaveBeenLastCalledWith("2026-10-05"),
    );
  });

  it("orders the cards: plan, order, caterer, day list last", async () => {
    renderPage();

    const plan = await screen.findByRole("heading", { name: "Wochenplan" });
    const order = await screen.findByRole("heading", { name: "Bestellung" });
    const caterer = await screen.findByRole("heading", { name: "Caterer" });
    const list = await screen.findByTestId("participants");
    const follows = (a: Element, b: Element) =>
      Boolean(a.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING);
    expect(follows(plan, order)).toBe(true);
    expect(follows(order, caterer)).toBe(true);
    expect(follows(caterer, list)).toBe(true);
  });
});

describe("meal registration permissions", () => {
  it.each([
    [["meals:register"], "true", "false"],
    [["meals:orders"], "false", "true"],
  ] as const)(
    "keeps registration independent of export rights (%j)",
    async (permissions, register, exporting) => {
      rights.admin = false;
      rights.permissions = [...permissions];
      getMealPlanWeek.mockResolvedValue([]);
      renderPage();
      const list = await screen.findByTestId("participants");
      expect(list).toHaveAttribute("data-register", register);
      expect(list).toHaveAttribute("data-export", exporting);
    },
  );
});

describe("meal plan feature gate", () => {
  beforeEach(() => {
    getMealPlanWeek.mockReset();
    getMealPlanWeek.mockResolvedValue([]);
    rights.admin = true;
    rights.permissions = [];
    features.mealPlanEnabled = true;
  });

  it("waits for meal planning to be enabled before loading the week", async () => {
    features.mealPlanEnabled = false;
    const { rerender } = renderPage();

    expect(screen.getByText("Speiseplan")).toBeInTheDocument();
    expect(getMealPlanWeek).not.toHaveBeenCalled();

    features.mealPlanEnabled = true;
    rerender(
      <ToastProvider>
        <MealPlanPage />
      </ToastProvider>,
    );

    expect(
      await screen.findByText(/0 von 5 Tagen geplant/),
    ).toBeInTheDocument();
    expect(getMealPlanWeek).toHaveBeenCalledOnce();
    expect(getMealPlanWeek).toHaveBeenCalledWith("2026-10-05");
  });
});

describe("Speiseplan: Allergene je Gericht (#4777)", () => {
  beforeEach(() => {
    getMealPlanWeek.mockReset();
    setDay.mockReset();
    setDay.mockResolvedValue(undefined);
  });

  it("shows the stored marks and saves the chosen allergens", async () => {
    getMealPlanWeek.mockResolvedValue([
      {
        date: "2026-10-05",
        position: 0,
        dish: "Fischstäbchen",
        note: null,
        allergens: ["gluten", "fish"],
      },
    ]);

    renderPage();

    // Every dish card carries its own choice; the Monday dish shows its
    // letters, and its name spells the allergens out for screen readers.
    const trigger = await screen.findByRole("button", {
      name: "Allergene: Glutenhaltiges Getreide, Fisch",
    });
    expect(trigger).toHaveTextContent("A, D");
    expect(
      screen.getAllByRole("button", { name: "Allergene wählen" }),
    ).toHaveLength(4);
    // The legend under the plan explains the letters used this week.
    const legend = screen.getByRole("list", { name: "Allergene" });
    expect(legend).toHaveTextContent("A Glutenhaltiges Getreide");
    expect(legend).toHaveTextContent("D Fisch");
    expect(legend).not.toHaveTextContent("Milch");

    fireEvent.click(trigger);
    fireEvent.click(screen.getByLabelText("Milch (auch Laktose) (G)"));
    expect(trigger).toHaveTextContent("A, D, G");
    expect(legend).toHaveTextContent("G Milch (auch Laktose)");
    expect(trigger).toHaveAccessibleName(
      "Allergene: Glutenhaltiges Getreide, Fisch, Milch (auch Laktose)",
    );

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => expect(setDay).toHaveBeenCalledTimes(1));
    expect(setDay).toHaveBeenCalledWith("2026-10-05", [
      {
        dish: "Fischstäbchen",
        note: null,
        allergens: ["gluten", "fish", "milk"],
      },
    ]);
  });
});

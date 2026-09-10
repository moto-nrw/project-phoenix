import "@testing-library/jest-dom/vitest";
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import ShiftTypesPage from "./page";

const mockUpdateUrlParams = vi.hoisted(() => vi.fn());
vi.mock("~/hooks/useUpdateUrlParams", () => ({
  useUpdateUrlParams: () => mockUpdateUrlParams,
}));

let currentSearch = new URLSearchParams();
vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), back: vi.fn() }),
  usePathname: () => "/test-tenant/database/shift-types",
  useSearchParams: () => currentSearch,
}));

vi.mock("~/contexts/ToastContext", () => ({
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

const shiftTypeService = vi.hoisted(() => ({
  getShiftTypes: vi.fn(),
  createShiftType: vi.fn(),
  updateShiftType: vi.fn(),
  deleteShiftType: vi.fn(),
  createDefaults: vi.fn(),
}));
vi.mock("~/lib/shift-type-api", () => ({ shiftTypeService }));

const categoryService = vi.hoisted(() => ({ getManagedCategories: vi.fn() }));
vi.mock("~/lib/category-api", () => ({ categoryService }));

const mockTenantMutate = vi.hoisted(() => vi.fn(() => Promise.resolve()));
type SWRState = { data: unknown; error: unknown; isLoading: boolean };
const swrState = vi.hoisted(() => ({
  current: {} as Record<string, SWRState>,
}));
vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string) =>
    swrState.current[key] ?? {
      data: undefined,
      error: undefined,
      isLoading: false,
    },
  useTenantMutate: () => mockTenantMutate,
}));

const SHIFT_TYPES = [
  {
    id: "10",
    name: "Betreuung",
    color: "#83CD2D",
    description: "Zeit am Kind",
    isActive: true,
  },
];

const CATEGORIES = [
  { id: "1", name: "Essen", shiftTypeId: "10" },
  { id: "2", name: "Lernzeit" },
];

function setCategories(state: Partial<SWRState>) {
  swrState.current["database-shift-type-categories"] = {
    data: undefined,
    error: undefined,
    isLoading: false,
    ...state,
  };
}

beforeEach(() => {
  currentSearch = new URLSearchParams("eintrag=10");
  swrState.current = {
    "database-shift-types": {
      data: SHIFT_TYPES,
      error: undefined,
      isLoading: false,
    },
  };
  setCategories({ data: CATEGORIES });
  shiftTypeService.updateShiftType.mockResolvedValue(SHIFT_TYPES[0]);
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

function save() {
  fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
}

// Die Kategorie-Zuordnung ist die empfindliche Regel, die mit der Fläche
// umgezogen ist (#1837): eine leere Liste würde bestehende Zuordnungen
// löschen, also darf `category_ids` nur mitgehen, wenn die Kategorien
// wirklich geladen sind.
describe("Schichtarten — Kategorie-Zuordnung (#1837, #3114)", () => {
  it("preselects the categories currently mapped to the shift type", () => {
    render(<ShiftTypesPage />);

    expect(screen.getByText("Essen")).toBeInTheDocument();
    expect(screen.queryByText("Lernzeit")).not.toBeInTheDocument();
  });

  it("sends the mapping once the categories are loaded", async () => {
    render(<ShiftTypesPage />);

    save();

    await waitFor(() =>
      expect(shiftTypeService.updateShiftType).toHaveBeenCalledWith(
        "10",
        expect.objectContaining({ categoryIds: ["1"] }),
      ),
    );
  });

  it("keeps an assigned archived category in the mapping", async () => {
    setCategories({
      data: [
        ...CATEGORIES,
        {
          id: "3",
          name: "Frühstück",
          shiftTypeId: "10",
          archivedAt: "2026-09-01T00:00:00Z",
        },
      ],
    });
    render(<ShiftTypesPage />);

    expect(
      screen.getByText("Frühstück (nicht mehr angeboten)"),
    ).toBeInTheDocument();

    save();

    await waitFor(() =>
      expect(shiftTypeService.updateShiftType).toHaveBeenCalledWith(
        "10",
        expect.objectContaining({ categoryIds: ["1", "3"] }),
      ),
    );
  });

  it("keeps an in-progress draft when categories finish loading", async () => {
    setCategories({ data: undefined, isLoading: true });
    const { rerender } = render(<ShiftTypesPage />);

    fireEvent.change(screen.getByDisplayValue("Betreuung"), {
      target: { value: "Neue Betreuung" },
    });

    setCategories({ data: CATEGORIES });
    rerender(<ShiftTypesPage />);

    expect(screen.getByDisplayValue("Neue Betreuung")).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText("Essen")).toBeInTheDocument());
  });

  it("keeps a create draft when categories finish loading", () => {
    setCategories({ data: undefined, isLoading: true });
    const { rerender } = render(<ShiftTypesPage />);

    fireEvent.click(
      screen.getAllByRole("button", { name: "Schichtart anlegen" })[0]!,
    );
    const modal = within(screen.getByRole("dialog"));
    fireEvent.change(modal.getByLabelText("Name*"), {
      target: { value: "Neue Schichtart" },
    });
    fireEvent.change(modal.getByLabelText("Beschreibung"), {
      target: { value: "Bleibt erhalten" },
    });

    setCategories({ data: CATEGORIES });
    rerender(<ShiftTypesPage />);

    expect(screen.getByDisplayValue("Neue Schichtart")).toBeInTheDocument();
    expect(screen.getByDisplayValue("Bleibt erhalten")).toBeInTheDocument();
  });

  it("omits the mapping while the categories are still loading", async () => {
    setCategories({ data: undefined, isLoading: true });
    render(<ShiftTypesPage />);

    save();

    await waitFor(() =>
      expect(shiftTypeService.updateShiftType).toHaveBeenCalled(),
    );
    const payload = shiftTypeService.updateShiftType.mock.calls[0]?.[1] as
      Record<string, unknown> | undefined;
    expect(payload).not.toHaveProperty("categoryIds");
  });

  it("omits the mapping when the category list failed to load", async () => {
    setCategories({ data: undefined, error: new Error("down") });
    render(<ShiftTypesPage />);

    expect(
      screen.queryByText("Kategorien des Betreuungsplans"),
    ).not.toBeInTheDocument();

    save();

    await waitFor(() =>
      expect(shiftTypeService.updateShiftType).toHaveBeenCalled(),
    );
    const payload = shiftTypeService.updateShiftType.mock.calls[0]?.[1] as
      Record<string, unknown> | undefined;
    expect(payload).not.toHaveProperty("categoryIds");
  });

  it("leaves the mapping untouched when only the offering is switched off", async () => {
    render(<ShiftTypesPage />);

    fireEvent.click(
      screen.getByRole("button", { name: /Aktionen für Betreuung/ }),
    );
    fireEvent.click(
      screen.getByRole("menuitem", { name: "Nicht mehr anbieten" }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Nicht mehr anbieten" }),
    );

    await waitFor(() =>
      expect(shiftTypeService.updateShiftType).toHaveBeenCalled(),
    );
    const payload = shiftTypeService.updateShiftType.mock.calls[0]?.[1] as
      Record<string, unknown> | undefined;
    expect(payload).toMatchObject({ isActive: false });
    expect(payload).not.toHaveProperty("categoryIds");
  });
});

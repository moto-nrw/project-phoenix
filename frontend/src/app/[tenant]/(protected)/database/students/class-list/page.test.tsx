import "@testing-library/jest-dom/vitest";
import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import ClassListPage from "./page";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), back: vi.fn() }),
  usePathname: () => "/test-tenant/database/students/class-list",
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: null, status: "authenticated" }),
}));

vi.mock("~/lib/tenant-path", () => ({
  useTenantAwarePath: () => (path: string) => path,
}));

vi.mock("~/components/ui/hooks/useIsMobile", () => ({
  useIsMobile: () => false,
}));

interface SwrSlot {
  data: unknown;
  error: unknown;
  isLoading: boolean;
  mutate: ReturnType<typeof vi.fn>;
}

const swr = vi.hoisted(() => new Map<string, SwrSlot>());
vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string) => swr.get(key),
  useTenantMutate: () => vi.fn(() => Promise.resolve()),
}));

function slot(data: unknown, error?: unknown): SwrSlot {
  return {
    data,
    error,
    isLoading: false,
    mutate: vi.fn(() => Promise.resolve(undefined)),
  };
}

const unavailable = () =>
  new ApiError("load exploded", 503, { code: "general.unavailable" });

const entry = {
  id: "e1",
  firstName: "Lea",
  lastName: "Ohne",
  schoolClass: "1a",
  matchingStudentIds: [],
};

function renderPage() {
  return render(
    <ToastProvider>
      <ClassListPage />
    </ToastProvider>,
  );
}

beforeEach(() => {
  swr.clear();
  swr.set("class-list-class-suggestions", slot([]));
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

describe("Klassenliste: Zähler bei Ladefehler (#2517)", () => {
  it("shows no zero counts next to a failed entries load", async () => {
    swr.set("class-list-entries", slot(undefined, unavailable()));
    swr.set("class-list-roster-students", slot(undefined, unavailable()));

    renderPage();

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Klassenliste"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/0 Kinder/)).not.toBeInTheDocument();
    expect(screen.queryByText(/0 ohne Betreuung/)).not.toBeInTheDocument();
  });

  it("drops the child total when only the roster students failed", async () => {
    swr.set("class-list-entries", slot([entry]));
    swr.set("class-list-roster-students", slot(undefined, unavailable()));

    renderPage();

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Liste der angelegten Kinder"),
      ),
    ).toBeInTheDocument();
    // Die Einträge zählen weiter, die Gesamtzahl ohne die angelegten Kinder
    // stünde falsch da.
    expect(screen.getByText(/1 ohne Betreuung/)).toBeInTheDocument();
    expect(screen.queryByText(/\d+ Kind(er)? ·/)).not.toBeInTheDocument();
    expect(
      screen.queryByText(/davon sind in moto angelegt/),
    ).not.toBeInTheDocument();
  });

  it("counts entries and roster students once both loaded", () => {
    swr.set("class-list-entries", slot([entry]));
    swr.set(
      "class-list-roster-students",
      slot([
        {
          id: "s1",
          firstName: "Max",
          lastName: "Muster",
          schoolClass: "1a",
        },
      ]),
    );

    renderPage();

    expect(screen.getByText(/2 Kinder · 1 ohne Betreuung/)).toBeInTheDocument();
    expect(
      screen.getByText(/1 davon sind in moto angelegt/),
    ).toBeInTheDocument();
  });
});

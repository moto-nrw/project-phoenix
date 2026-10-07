import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

import TagesinformationenPage from "./page";

interface SwrSlot {
  data: unknown;
  error: unknown;
  isLoading: boolean;
  mutate: () => Promise<unknown>;
}

const state = vi.hoisted(() => ({
  isAdmin: true,
  today: undefined as unknown as SwrSlot,
  all: undefined as unknown as SwrSlot,
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), back: vi.fn() }),
  usePathname: () => "/schule/tagesinformationen",
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: { user: { id: "7" } }, status: "authenticated" }),
}));

vi.mock("~/lib/auth-utils", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/auth-utils")>()),
  hasEffectiveAdminScope: () => state.isAdmin,
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null) =>
    key === null
      ? { data: undefined, error: undefined, isLoading: false, mutate: vi.fn() }
      : key.startsWith("staff-notices-today")
        ? state.today
        : state.all,
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
  new ApiError("notices exploded", 503, { code: "general.unavailable" });

function renderPage() {
  return render(
    <ToastProvider>
      <TagesinformationenPage />
    </ToastProvider>,
  );
}

describe("Tagesinformationen: Statuszeile bei Ladefehler (#2517)", () => {
  beforeEach(() => {
    state.isAdmin = true;
    state.today = slot([]);
    state.all = slot([]);
  });

  it("shows no zero counts when both lists failed to load", async () => {
    state.today = slot(undefined, unavailable());
    state.all = slot(undefined, unavailable());

    renderPage();

    expect(
      await screen.findByText(
        catalogText(
          "general.unavailable",
          "die Liste der Tagesinformationen von heute",
        ),
      ),
    ).toBeInTheDocument();
    expect(
      await screen.findByText(
        catalogText(
          "general.unavailable",
          "die Liste aller Tagesinformationen",
        ),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/0 heute/)).not.toBeInTheDocument();
    expect(screen.queryByText(/0 insgesamt/)).not.toBeInTheDocument();
  });

  it("keeps the count of the list that did load", async () => {
    state.all = slot(undefined, unavailable());

    renderPage();

    expect(
      await screen.findByText(
        catalogText(
          "general.unavailable",
          "die Liste aller Tagesinformationen",
        ),
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("0 heute")).toBeInTheDocument();
    expect(screen.queryByText(/insgesamt/)).not.toBeInTheDocument();
  });
});

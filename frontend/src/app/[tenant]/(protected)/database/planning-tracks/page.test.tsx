import "@testing-library/jest-dom/vitest";
import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import PlanningTracksPage from "./page";
import { useTimetableEnabled } from "~/lib/tenant-context";

vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), back: vi.fn() }),
  usePathname: () => "/test-tenant/database/planning-tracks",
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("~/hooks/useUpdateUrlParams", () => ({
  useUpdateUrlParams: () => vi.fn(),
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

const planningTrackService = vi.hoisted(() => ({
  list: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  reorder: vi.fn(),
  archive: vi.fn(),
  restore: vi.fn(),
}));
vi.mock("~/lib/planning-track-api", () => ({ planningTrackService }));

vi.mock("~/lib/tenant-context", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/tenant-context")>()),
  useTimetableEnabled: vi.fn(),
}));

const swrState = vi.hoisted(() => ({
  data: undefined as unknown,
  error: undefined as unknown,
  isLoading: true,
}));
vi.mock("~/lib/swr", () => ({
  useSWRAuth: () => swrState,
  useTenantMutate: () => vi.fn(() => Promise.resolve()),
}));

beforeEach(() => {
  vi.mocked(useTimetableEnabled).mockReturnValue(true);
  swrState.data = undefined;
  swrState.error = undefined;
  swrState.isLoading = true;
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

describe("Planungsspuren", () => {
  it("shows an unavailable state when the care plan is switched off", () => {
    vi.mocked(useTimetableEnabled).mockReturnValue(false);

    render(<PlanningTracksPage />);

    expect(
      screen.getByTestId("planning-tracks-disabled-state"),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Planungsspur anlegen" }),
    ).not.toBeInTheDocument();
  });

  it("keeps creating unavailable until the current order has loaded", () => {
    const { rerender } = render(<PlanningTracksPage />);

    for (const action of screen.getAllByRole("button", {
      name: "Planungsspur anlegen",
    })) {
      expect(action).toBeDisabled();
    }

    swrState.data = [
      { id: "1", name: "Jahrgang 1", color: "#5080D8", sortOrder: 0 },
    ];
    swrState.isLoading = false;
    rerender(<PlanningTracksPage />);

    for (const action of screen.getAllByRole("button", {
      name: "Planungsspur anlegen",
    })) {
      expect(action).toBeEnabled();
    }
  });
});

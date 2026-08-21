// Die Ansicht "Beendete Betreuungen" (#2487).
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";

import type { EndedCarePage } from "~/lib/care-exit-api";
import Page from "./page";

const { mockFetchEndedCare, mockResumeCare, mockHasPermission } = vi.hoisted(
  () => ({
    mockFetchEndedCare: vi.fn(),
    mockResumeCare: vi.fn(),
    mockHasPermission: vi.fn(),
  }),
);

vi.mock("~/lib/care-exit-api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/care-exit-api")>();
  return {
    ...actual,
    fetchEndedCare: mockFetchEndedCare,
    resumeCare: mockResumeCare,
  };
});

vi.mock("~/lib/auth-utils", () => ({
  hasPermission: mockHasPermission,
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: { user: {} }, status: "authenticated" }),
}));

vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
  useRouter: () => ({ back: vi.fn(), push: vi.fn(), replace: vi.fn() }),
  usePathname: () => "/database/students/ended-care",
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null, fetcher: () => Promise<unknown>) => {
    if (key === null) return { data: undefined, isLoading: false, error: null };
    // Suche und Seite stecken im Schlüssel, deshalb wird hier bei jedem
    // Schlüsselwechsel wirklich geladen — genau wie in SWR.
    lastSwrKey = key;
    void fetcher();
    return {
      data: swrState.data,
      isLoading: swrState.isLoading,
      error: swrState.error,
      mutate: vi.fn(() => fetcher()),
    };
  },
  useTenantMutate: () => vi.fn(),
  useTenantMutateMatching: () => vi.fn(),
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ success: vi.fn(), error: vi.fn() }),
}));

let lastSwrKey: string | null = null;

const swrState: {
  data: EndedCarePage | undefined;
  isLoading: boolean;
  error: unknown;
} = { data: undefined, isLoading: false, error: null };

function page(
  items: EndedCarePage["items"],
  overrides: Partial<EndedCarePage> = {},
): EndedCarePage {
  return { items, total: items.length, page: 1, pageSize: 50, ...overrides };
}

describe("Beendete Betreuungen", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    lastSwrKey = null;
    mockHasPermission.mockReturnValue(true);
    swrState.isLoading = false;
    swrState.error = null;
    swrState.data = page([
      {
        studentId: "1",
        firstName: "Mia",
        lastName: "Muster",
        schoolClass: "3a",
        lastCareDay: "2026-08-15",
        reason: "moved_away",
        reasonNote: null,
        recordedAt: "2026-08-10",
      },
      {
        studentId: "2",
        firstName: "Ben",
        lastName: "Wirth",
        schoolClass: "2b",
        lastCareDay: "2026-07-31",
        reason: null,
        reasonNote: null,
        recordedAt: null,
      },
    ]);
    mockFetchEndedCare.mockResolvedValue(swrState.data);
  });

  it("lists each child with its last care day and reason", async () => {
    render(<Page />);

    expect(await screen.findByText("Muster, Mia")).toBeVisible();
    expect(screen.getByText("15.08.2026")).toBeVisible();
    expect(screen.getByText("Umzug")).toBeVisible();
  });

  it("says plainly when no reason was recorded", async () => {
    render(<Page />);
    expect(await screen.findByText("Kein Grund hinterlegt")).toBeVisible();
  });

  it("explains what the view is and that nothing was deleted", async () => {
    render(<Page />);
    expect(
      await screen.findByText(/Die Daten dieser Kinder bleiben erhalten/),
    ).toBeVisible();
    expect(
      screen.getByText(
        /Abgänge aus dem Jahrgangswechsel stehen weiterhin dort/,
      ),
    ).toBeVisible();
  });

  // Gesucht wird auf dem Server. Eine Suche, die nur die geladene Seite
  // durchsieht, findet Kinder nicht, die weiter hinten stehen (#2487).
  it("hands the search term to the server instead of filtering the page", async () => {
    render(<Page />);
    await screen.findByText("Muster, Mia");

    fireEvent.change(screen.getByPlaceholderText("Name oder Klasse suchen…"), {
      target: { value: "Wirth" },
    });

    await waitFor(() =>
      expect(mockFetchEndedCare).toHaveBeenCalledWith(
        expect.objectContaining({ search: "Wirth", page: 1 }),
      ),
    );
  });

  // Ohne Blättern zeigte die Ansicht nur die erste Seite und behauptete, das
  // seien alle (#2487).
  it("pages through more children than fit on one page", async () => {
    swrState.data = page(swrState.data!.items, { total: 137 });
    mockFetchEndedCare.mockResolvedValue(swrState.data);

    render(<Page />);
    await screen.findByText("Muster, Mia");

    expect(screen.getByText("137")).toBeVisible();
    expect(screen.getByText("Seite 1 von 3")).toBeVisible();
    expect(
      screen.getByRole("button", { name: "Vorherige Seite" }),
    ).toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: "Nächste Seite" }));

    await waitFor(() =>
      expect(mockFetchEndedCare).toHaveBeenCalledWith(
        expect.objectContaining({ page: 2 }),
      ),
    );
    expect(lastSwrKey).toContain(":2:");
  });

  it("opens the resume dialog and demands the explicit review", async () => {
    render(<Page />);
    await screen.findByText("Muster, Mia");

    fireEvent.click(
      screen.getAllByRole("button", { name: "Wieder aufnehmen" })[0]!,
    );

    const confirm = await screen.findByRole("button", {
      name: "Betreuung wieder aufnehmen",
    });
    expect(confirm).toBeDisabled();
    expect(
      screen.getByText(
        /Gruppe, Angebote, Wochenplan und Zeiten schaltet moto nicht von selbst wieder ein/,
      ),
    ).toBeVisible();

    fireEvent.click(screen.getByRole("checkbox"));
    expect(confirm).toBeEnabled();

    fireEvent.click(confirm);
    await waitFor(() =>
      expect(mockResumeCare).toHaveBeenCalledWith(
        "1",
        expect.any(String),
        true,
      ),
    );
  });

  it("is closed to anybody without the delete permission", () => {
    mockHasPermission.mockReturnValue(false);
    render(<Page />);
    expect(
      screen.getByText(/nur für Personen mit der Berechtigung/),
    ).toBeVisible();
    expect(screen.queryByText("Muster, Mia")).toBeNull();
  });
});

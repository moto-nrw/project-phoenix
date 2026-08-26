/**
 * Tests for OperatorSchoolSettingsPage.
 * Verifies schema loading, error state, save/reset callbacks, and that
 * null/empty tabs are rendered gracefully.
 */
import { Suspense, act } from "react";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const {
  mockUseSession,
  mockFetchSchema,
  mockFetchImpact,
  mockSetValue,
  mockResetValue,
  mockListSchools,
  mockSearchParams,
} = vi.hoisted(() => ({
  mockUseSession: vi.fn(),
  mockFetchSchema: vi.fn(),
  mockFetchImpact: vi.fn(),
  mockSetValue: vi.fn(),
  mockResetValue: vi.fn(),
  mockListSchools: vi.fn(),
  mockSearchParams: { value: new URLSearchParams() },
}));

vi.mock("next-auth/react", () => ({
  useSession: mockUseSession,
}));

vi.mock("next/navigation", () => ({
  useSearchParams: () => mockSearchParams.value,
}));

vi.mock("next/link", () => ({
  default: ({
    href,
    children,
    className,
  }: {
    href: string;
    children: React.ReactNode;
    className?: string;
  }) => (
    <a href={href} className={className}>
      {children}
    </a>
  ),
}));

vi.mock("~/lib/operator/operator-settings-api", () => ({
  fetchOperatorSettingsSchema: mockFetchSchema,
  fetchBookingAuthorityImpact: mockFetchImpact,
  setOperatorSettingValue: mockSetValue,
  resetOperatorSettingValue: mockResetValue,
}));

vi.mock("~/lib/operator/provisioning-api", () => ({
  operatorProvisioningService: {
    listSchools: mockListSchools,
  },
}));

function plannedBookingAuthorityImpact() {
  return {
    referenceDate: "2026-08-25",
    blockingChildren: [],
    plannedCompletions: [
      {
        studentId: "8",
        firstName: "Noah",
        lastName: "Beispiel",
        schoolClass: "3b",
        firstBookinglessDay: "2026-09-01",
      },
    ],
  };
}

// The settings-category component is exercised in its own tests; stub it
// here so we can assert onSave/onReset routing without the full field chrome.
vi.mock("~/components/settings/settings-category", () => ({
  SettingsCategory: ({
    category,
    onSave,
    onReset,
    onBookingAuthorityEnable,
  }: {
    category: { key: string; label: string };
    onSave: (key: string, value: unknown) => Promise<string | null>;
    onReset: (key: string) => Promise<string | null>;
    onBookingAuthorityEnable?: () => Promise<void>;
  }) => (
    <div data-testid={`category-${category.key}`}>
      <span>{category.label}</span>
      <button
        type="button"
        onClick={() => void onSave("operations.session_end_time", "18:30")}
      >
        save-{category.key}
      </button>
      <button type="button" onClick={() => void onBookingAuthorityEnable?.()}>
        enable-booking-mode-{category.key}
      </button>
      <button
        type="button"
        onClick={() => void onReset("operations.session_end_time")}
      >
        reset-{category.key}
      </button>
    </div>
  ),
}));

// The UI primitives are trivial — keep stubs minimal.
vi.mock("~/components/ui/alert", () => ({
  Alert: ({ type, message }: { type: string; message: string }) => (
    <div role="alert" data-type={type}>
      {message}
    </div>
  ),
}));

vi.mock("~/components/ui/skeleton", () => ({
  Skeleton: ({ className }: { className?: string }) => (
    <div data-testid="skeleton" className={className} />
  ),
}));

import OperatorSchoolSettingsPage from "./page";

async function renderPage(id = "42") {
  let result!: ReturnType<typeof render>;
  await act(async () => {
    result = render(
      <Suspense fallback={<div data-testid="suspense-fallback" />}>
        <OperatorSchoolSettingsPage params={Promise.resolve({ id })} />
      </Suspense>,
    );
  });
  return result;
}

describe("OperatorSchoolSettingsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseSession.mockReturnValue({ status: "authenticated" });
    mockListSchools.mockResolvedValue([]);
    mockSearchParams.value = new URLSearchParams();
  });

  it("shows skeletons while loading", async () => {
    mockFetchSchema.mockImplementation(
      () => new Promise(() => undefined), // never resolves
    );

    await renderPage();

    // Skeletons appear immediately in the loading state
    const skeletons = await screen.findAllByTestId("skeleton");
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it("renders tabs and categories when schema loads", async () => {
    mockFetchSchema.mockResolvedValue({
      tabs: [
        {
          key: "operations",
          label: "Ops",
          categories: [{ key: "sessions", label: "Sessions", items: [] }],
        },
        {
          key: "gdpr",
          label: "GDPR",
          categories: [{ key: "retention", label: "Retention", items: [] }],
        },
      ],
    });

    await renderPage();

    await waitFor(() => {
      expect(screen.getByTestId("category-sessions")).toBeDefined();
      expect(screen.getByTestId("category-retention")).toBeDefined();
    });
    // German tab label mapping applied
    expect(screen.getByText("Betrieb")).toBeDefined();
    expect(screen.getByText("Datenschutz")).toBeDefined();
  });

  it("renders school name in heading when provisioning list includes the school", async () => {
    mockFetchSchema.mockResolvedValue({ tabs: [] });
    mockListSchools.mockResolvedValue([
      { id: "42", name: "Testschule" },
      { id: "99", name: "Other" },
    ]);

    await renderPage("42");

    await waitFor(() => {
      const heading = screen.getByRole("heading", { level: 1 });
      expect(heading.textContent).toContain("Testschule");
    });
  });

  it("shows empty state when schema has no tabs", async () => {
    mockFetchSchema.mockResolvedValue({ tabs: [] });

    await renderPage();

    await waitFor(() => {
      expect(
        screen.getByText(/Keine Einstellungen für diese Schule verfügbar/),
      ).toBeDefined();
    });
  });

  it("shows empty state when tabs is null (defensive)", async () => {
    // Backend historically could return tabs: null; the page must handle it.
    mockFetchSchema.mockResolvedValue({
      tabs: null,
    } as unknown as { tabs: [] });

    await renderPage();

    await waitFor(() => {
      expect(
        screen.getByText(/Keine Einstellungen für diese Schule verfügbar/),
      ).toBeDefined();
    });
  });

  it("shows error alert when schema fetch throws", async () => {
    mockFetchSchema.mockRejectedValue(new Error("server down"));

    await renderPage();

    await waitFor(() => {
      const alert = screen.getByRole("alert");
      expect(alert.textContent).toContain(
        "Einstellungen konnten nicht geladen werden",
      );
    });
  });

  it("invokes setOperatorSettingValue on save", async () => {
    mockFetchSchema.mockResolvedValue({
      tabs: [
        {
          key: "operations",
          label: "Ops",
          categories: [{ key: "sessions", label: "Sessions", items: [] }],
        },
      ],
    });
    mockSetValue.mockResolvedValue(null);

    await renderPage();

    const saveBtn = await screen.findByRole("button", {
      name: "save-sessions",
    });
    fireEvent.click(saveBtn);

    await waitFor(() => {
      expect(mockSetValue).toHaveBeenCalledWith(
        "42",
        "operations.session_end_time",
        "18:30",
      );
    });
  });

  it("invokes resetOperatorSettingValue on reset", async () => {
    mockFetchSchema.mockResolvedValue({
      tabs: [
        {
          key: "operations",
          label: "Ops",
          categories: [{ key: "sessions", label: "Sessions", items: [] }],
        },
      ],
    });
    mockResetValue.mockResolvedValue(null);

    await renderPage();

    const resetBtn = await screen.findByRole("button", {
      name: "reset-sessions",
    });
    fireEvent.click(resetBtn);

    await waitFor(() => {
      expect(mockResetValue).toHaveBeenCalledWith(
        "42",
        "operations.session_end_time",
      );
    });
  });

  it("blocks activation when the impact lists a child without care days", async () => {
    mockFetchSchema.mockResolvedValue({
      tabs: [
        {
          key: "operations",
          label: "Ops",
          categories: [{ key: "sessions", label: "Sessions", items: [] }],
        },
      ],
    });
    mockFetchImpact.mockResolvedValue({
      referenceDate: "2026-08-25",
      blockingChildren: [
        {
          studentId: "7",
          firstName: "Mia",
          lastName: "Muster",
          schoolClass: "2a",
        },
      ],
      plannedCompletions: [],
    });

    await renderPage();
    fireEvent.click(
      await screen.findByRole("button", {
        name: "enable-booking-mode-sessions",
      }),
    );

    expect(await screen.findByText(/Mia Muster/)).toBeDefined();
    expect(
      screen.getByRole("button", { name: "Buchungsmodus aktivieren" }),
    ).toBeDisabled();
    expect(screen.getByText("Geplante Abschlüsse: 0")).toBeDefined();
    expect(
      screen.getByText("Es werden keine Abschlüsse geplant."),
    ).toBeDefined();
    expect(mockSetValue).not.toHaveBeenCalled();
  });

  it("shows planned completions and enables the reviewed activation", async () => {
    mockFetchSchema.mockResolvedValue({
      tabs: [
        {
          key: "operations",
          label: "Ops",
          categories: [{ key: "sessions", label: "Sessions", items: [] }],
        },
      ],
    });
    mockFetchImpact.mockResolvedValue(plannedBookingAuthorityImpact());
    mockSetValue.mockResolvedValue(null);

    await renderPage();
    fireEvent.click(
      await screen.findByRole("button", {
        name: "enable-booking-mode-sessions",
      }),
    );

    expect(await screen.findByText(/Noah Beispiel/)).toBeDefined();
    expect(screen.getByText(/ab 01.09.2026/)).toBeDefined();
    fireEvent.click(
      screen.getByRole("button", { name: "Buchungsmodus aktivieren" }),
    );

    await waitFor(() => {
      expect(mockSetValue).toHaveBeenCalledWith(
        "42",
        "enrollment.bookings_authoritative",
        true,
      );
    });
  });

  it("does not fetch schema when session is not authenticated", async () => {
    mockUseSession.mockReturnValue({ status: "unauthenticated" });

    await renderPage();

    // Give effects a moment to run; fetch must not be called.
    await new Promise((r) => setTimeout(r, 10));
    expect(mockFetchSchema).not.toHaveBeenCalled();
  });

  it("swallows school-name lookup failures without crashing", async () => {
    mockFetchSchema.mockResolvedValue({ tabs: [] });
    mockListSchools.mockRejectedValue(new Error("listSchools failed"));

    await renderPage();

    await waitFor(() => {
      expect(
        screen.getByText(/Keine Einstellungen für diese Schule verfügbar/),
      ).toBeDefined();
    });
  });

  // --- Back link ---

  it("defaults the back link to the global schools list when ?back is absent", async () => {
    mockFetchSchema.mockResolvedValue({ tabs: [] });

    await renderPage();

    const link = await screen.findByText("Zu den Schulen");
    expect(link.closest("a")?.getAttribute("href")).toBe("/operator/schools");
  });

  it("uses ?back URL pointing back to the school drill-in", async () => {
    mockFetchSchema.mockResolvedValue({ tabs: [] });
    mockSearchParams.value = new URLSearchParams({
      back: "/operator/organizations/test-org/schools/test-school",
    });

    await renderPage();

    const link = await screen.findByText("Zurück zur Schule");
    expect(link.closest("a")?.getAttribute("href")).toBe(
      "/operator/organizations/test-org/schools/test-school",
    );
  });

  it("falls back to /operator/schools when ?back points outside /operator (open-redirect guard)", async () => {
    mockFetchSchema.mockResolvedValue({ tabs: [] });
    mockSearchParams.value = new URLSearchParams({
      back: "https://evil.example.com/path",
    });

    await renderPage();

    const link = await screen.findByText("Zu den Schulen");
    expect(link.closest("a")?.getAttribute("href")).toBe("/operator/schools");
  });
});

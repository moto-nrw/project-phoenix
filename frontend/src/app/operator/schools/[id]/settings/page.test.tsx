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
  mockSetValue,
  mockResetValue,
  mockListSchools,
} = vi.hoisted(() => ({
  mockUseSession: vi.fn(),
  mockFetchSchema: vi.fn(),
  mockSetValue: vi.fn(),
  mockResetValue: vi.fn(),
  mockListSchools: vi.fn(),
}));

vi.mock("next-auth/react", () => ({
  useSession: mockUseSession,
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
  setOperatorSettingValue: mockSetValue,
  resetOperatorSettingValue: mockResetValue,
}));

vi.mock("~/lib/operator/provisioning-api", () => ({
  operatorProvisioningService: {
    listSchools: mockListSchools,
  },
}));

// The settings-category component is exercised in its own tests; stub it
// here so we can assert onSave/onReset routing without the full field chrome.
vi.mock("~/components/settings/settings-category", () => ({
  SettingsCategory: ({
    category,
    onSave,
    onReset,
  }: {
    category: { key: string; label: string };
    onSave: (key: string, value: unknown) => Promise<string | null>;
    onReset: (key: string) => Promise<string | null>;
  }) => (
    <div data-testid={`category-${category.key}`}>
      <span>{category.label}</span>
      <button
        type="button"
        onClick={() => void onSave("operations.session_end_time", "18:30")}
      >
        save-{category.key}
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
});

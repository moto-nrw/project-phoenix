import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { OGSettingsPanel } from "./og-settings-panel";
import type { ResolvedSetting } from "~/lib/settings-helpers";

// Mock next-auth/react
vi.mock("next-auth/react", () => ({
  getSession: vi.fn(),
}));

// Mock settings-api
const mockFetchOGSettings = vi.fn();
const mockUpdateOGSetting = vi.fn();
const mockResetOGSetting = vi.fn();

vi.mock("~/lib/settings-api", () => ({
  fetchOGSettings: (...args: unknown[]) =>
    mockFetchOGSettings(...args) as Promise<unknown>,
  updateOGSetting: (...args: unknown[]) =>
    mockUpdateOGSetting(...args) as Promise<unknown>,
  resetOGSetting: (...args: unknown[]) =>
    mockResetOGSetting(...args) as Promise<unknown>,
}));

// Mock ToastContext
const mockToastSuccess = vi.fn();
const mockToastError = vi.fn();

vi.mock("~/contexts/ToastContext", () => ({
  useToast: vi.fn(() => ({
    success: mockToastSuccess,
    error: mockToastError,
  })),
}));

// Mock sub-components
vi.mock("./setting-input", () => ({
  SettingInput: ({
    setting,
  }: {
    setting: { key: string; description?: string };
  }) => (
    <div data-testid={`setting-input-${setting.key}`}>
      input-{setting.key}
    </div>
  ),
  SettingLoadingSpinner: ({ variant }: { variant?: string }) => (
    <div data-testid="loading-spinner" data-variant={variant}>
      Loading...
    </div>
  ),
}));

vi.mock("./setting-history", () => ({
  SettingHistory: ({
    title,
    ogId,
  }: {
    title?: string;
    ogId?: string;
  }) => (
    <div data-testid="setting-history" data-ogid={ogId}>
      {title}
    </div>
  ),
}));

function createMockSetting(
  overrides: Partial<ResolvedSetting> = {},
): ResolvedSetting {
  return {
    key: "test.setting",
    value: "test-value",
    type: "string",
    category: "session",
    description: "Test Setting",
    isDefault: true,
    isActive: true,
    canModify: true,
    ...overrides,
  };
}

describe("OGSettingsPanel", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows loading spinner with green variant", () => {
    mockFetchOGSettings.mockReturnValue(new Promise(vi.fn()));
    render(<OGSettingsPanel ogId="5" />);

    expect(screen.getByTestId("loading-spinner")).toBeInTheDocument();
    expect(screen.getByTestId("loading-spinner")).toHaveAttribute(
      "data-variant",
      "green",
    );
  });

  it("shows empty state when no settings", async () => {
    mockFetchOGSettings.mockResolvedValue([]);
    render(<OGSettingsPanel ogId="5" />);

    await waitFor(() => {
      expect(
        screen.getByText(
          "Keine Einstellungen für diese Gruppe verfügbar.",
        ),
      ).toBeInTheDocument();
    });
  });

  it("fetches settings for the given ogId", async () => {
    mockFetchOGSettings.mockResolvedValue([createMockSetting()]);
    render(<OGSettingsPanel ogId="42" />);

    await waitFor(() => {
      expect(mockFetchOGSettings).toHaveBeenCalledWith("42");
    });
  });

  it("shows ogName header when provided", async () => {
    mockFetchOGSettings.mockResolvedValue([createMockSetting()]);
    render(<OGSettingsPanel ogId="5" ogName="OG West" />);

    await waitFor(() => {
      expect(
        screen.getByText("Einstellungen für OG West"),
      ).toBeInTheDocument();
    });
  });

  it("does not show ogName header when not provided", async () => {
    mockFetchOGSettings.mockResolvedValue([createMockSetting()]);
    render(<OGSettingsPanel ogId="5" />);

    await waitFor(() => {
      expect(screen.getByText("Test Setting")).toBeInTheDocument();
    });

    expect(
      screen.queryByText(/Einstellungen für/),
    ).not.toBeInTheDocument();
  });

  it("renders settings after loading", async () => {
    mockFetchOGSettings.mockResolvedValue([
      createMockSetting({
        key: "session.timeout",
        description: "Session Timeout",
      }),
    ]);
    render(<OGSettingsPanel ogId="5" />);

    await waitFor(() => {
      expect(
        screen.getByTestId("setting-input-session.timeout"),
      ).toBeInTheDocument();
    });
  });

  it("shows category label", async () => {
    mockFetchOGSettings.mockResolvedValue([
      createMockSetting({ category: "session" }),
    ]);
    render(<OGSettingsPanel ogId="5" />);

    await waitFor(() => {
      expect(screen.getByText("Sitzung")).toBeInTheDocument();
    });
  });

  it("shows 'Angepasst' badge for non-default settings", async () => {
    mockFetchOGSettings.mockResolvedValue([
      createMockSetting({ isDefault: false }),
    ]);
    render(<OGSettingsPanel ogId="5" />);

    await waitFor(() => {
      expect(screen.getByText("Angepasst")).toBeInTheDocument();
    });
  });

  it("shows reset button for non-default modifiable settings", async () => {
    mockFetchOGSettings.mockResolvedValue([
      createMockSetting({ isDefault: false, canModify: true }),
    ]);
    render(<OGSettingsPanel ogId="5" />);

    await waitFor(() => {
      expect(screen.getByText("Zurücksetzen")).toBeInTheDocument();
    });
  });

  it("hides reset button for default settings", async () => {
    mockFetchOGSettings.mockResolvedValue([
      createMockSetting({ isDefault: true }),
    ]);
    render(<OGSettingsPanel ogId="5" />);

    await waitFor(() => {
      expect(screen.getByText("Test Setting")).toBeInTheDocument();
    });

    expect(screen.queryByText("Zurücksetzen")).not.toBeInTheDocument();
  });

  it("shows history when showHistory=true", async () => {
    mockFetchOGSettings.mockResolvedValue([createMockSetting()]);
    render(<OGSettingsPanel ogId="5" showHistory />);

    await waitFor(() => {
      expect(screen.getByTestId("setting-history")).toBeInTheDocument();
      expect(screen.getByText("Letzte Änderungen")).toBeInTheDocument();
    });
  });

  it("hides history when showHistory=false", async () => {
    mockFetchOGSettings.mockResolvedValue([createMockSetting()]);
    render(<OGSettingsPanel ogId="5" showHistory={false} />);

    await waitFor(() => {
      expect(screen.getByText("Test Setting")).toBeInTheDocument();
    });

    expect(screen.queryByTestId("setting-history")).not.toBeInTheDocument();
  });

  it("shows toast error when fetch fails", async () => {
    vi.spyOn(console, "error").mockImplementation(vi.fn());
    mockFetchOGSettings.mockRejectedValue(new Error("Failed"));
    render(<OGSettingsPanel ogId="5" />);

    await waitFor(() => {
      expect(mockToastError).toHaveBeenCalledWith(
        "Fehler beim Laden der Einstellungen",
      );
    });
  });
});

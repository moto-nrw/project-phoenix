import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { SettingHistory } from "./setting-history";
import type { SettingChange } from "~/lib/settings-helpers";

// Mock settings-api
const mockFetchSettingHistory = vi.fn();
const mockFetchOGSettingHistory = vi.fn();
const mockFetchOGKeyHistory = vi.fn();

vi.mock("~/lib/settings-api", () => ({
  fetchSettingHistory: (...args: unknown[]) =>
    mockFetchSettingHistory(...args) as Promise<unknown>,
  fetchOGSettingHistory: (...args: unknown[]) =>
    mockFetchOGSettingHistory(...args) as Promise<unknown>,
  fetchOGKeyHistory: (...args: unknown[]) =>
    mockFetchOGKeyHistory(...args) as Promise<unknown>,
}));


function createMockChange(
  overrides: Partial<SettingChange> = {},
): SettingChange {
  return {
    id: "1",
    settingKey: "session.timeout",
    scopeType: "system",
    changeType: "update",
    oldValue: 20,
    newValue: 30,
    reason: "Testing reason",
    accountEmail: "admin@example.com",
    createdAt: new Date("2024-06-15T10:30:00Z"),
    ...overrides,
  };
}

describe("SettingHistory", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows loading spinner initially", () => {
    mockFetchSettingHistory.mockReturnValue(new Promise(vi.fn())); // never resolves
    render(<SettingHistory />);

    expect(screen.getByText("Änderungsverlauf")).toBeInTheDocument();
    // Loading spinner is an animated div
    const spinner = document.querySelector(".animate-spin");
    expect(spinner).toBeInTheDocument();
  });

  it("shows default title", () => {
    mockFetchSettingHistory.mockReturnValue(new Promise(vi.fn()));
    render(<SettingHistory />);

    expect(screen.getByText("Änderungsverlauf")).toBeInTheDocument();
  });

  it("shows custom title", () => {
    mockFetchSettingHistory.mockReturnValue(new Promise(vi.fn()));
    render(<SettingHistory title="Letzte Änderungen" />);

    expect(screen.getByText("Letzte Änderungen")).toBeInTheDocument();
  });

  it("shows empty state when no history", async () => {
    mockFetchSettingHistory.mockResolvedValue([]);
    render(<SettingHistory />);

    await waitFor(() => {
      expect(
        screen.getByText("Keine Änderungen vorhanden."),
      ).toBeInTheDocument();
    });
  });

  it("calls fetchOGKeyHistory when ogId AND settingKey provided", async () => {
    mockFetchOGKeyHistory.mockResolvedValue([]);
    render(<SettingHistory ogId="5" settingKey="session.timeout" limit={10} />);

    await waitFor(() => {
      expect(mockFetchOGKeyHistory).toHaveBeenCalledWith(
        "5",
        "session.timeout",
        10,
      );
    });
  });

  it("calls fetchOGSettingHistory when only ogId provided", async () => {
    mockFetchOGSettingHistory.mockResolvedValue([]);
    render(<SettingHistory ogId="5" limit={10} />);

    await waitFor(() => {
      expect(mockFetchOGSettingHistory).toHaveBeenCalledWith("5", 10);
    });
  });

  it("calls fetchSettingHistory when no ogId provided", async () => {
    mockFetchSettingHistory.mockResolvedValue([]);
    render(<SettingHistory filters={{ scopeType: "system" }} limit={15} />);

    await waitFor(() => {
      expect(mockFetchSettingHistory).toHaveBeenCalledWith(
        expect.objectContaining({ scopeType: "system", limit: 15 }),
      );
    });
  });

  it("renders history entries with change type badges", async () => {
    const changes = [
      createMockChange({ id: "1", changeType: "create" }),
      createMockChange({ id: "2", changeType: "update" }),
      createMockChange({ id: "3", changeType: "delete" }),
      createMockChange({ id: "4", changeType: "reset" }),
    ];
    mockFetchSettingHistory.mockResolvedValue(changes);
    render(<SettingHistory />);

    await waitFor(() => {
      expect(screen.getByText("Erstellt")).toBeInTheDocument();
      expect(screen.getByText("Geändert")).toBeInTheDocument();
      expect(screen.getByText("Gelöscht")).toBeInTheDocument();
      expect(screen.getByText("Zurückgesetzt")).toBeInTheDocument();
    });
  });

  it("shows error message when API fails", async () => {
    vi.spyOn(console, "error").mockImplementation(vi.fn());
    mockFetchSettingHistory.mockRejectedValue(new Error("API error"));
    render(<SettingHistory />);

    await waitFor(() => {
      expect(
        screen.getByText("Fehler beim Laden des Verlaufs"),
      ).toBeInTheDocument();
    });
  });

  it("shows old value and new value for update entries", async () => {
    mockFetchSettingHistory.mockResolvedValue([
      createMockChange({ oldValue: "alt", newValue: "neu" }),
    ]);
    render(<SettingHistory />);

    await waitFor(() => {
      expect(screen.getByText("alt")).toBeInTheDocument();
      expect(screen.getByText("neu")).toBeInTheDocument();
    });
  });

  it("shows boolean values as Ja/Nein", async () => {
    mockFetchSettingHistory.mockResolvedValue([
      createMockChange({ oldValue: false, newValue: true }),
    ]);
    render(<SettingHistory />);

    await waitFor(() => {
      expect(screen.getByText("Nein")).toBeInTheDocument();
      expect(screen.getByText("Ja")).toBeInTheDocument();
    });
  });

  it("shows reason text", async () => {
    mockFetchSettingHistory.mockResolvedValue([
      createMockChange({ reason: "Performance tuning" }),
    ]);
    render(<SettingHistory />);

    await waitFor(() => {
      expect(screen.getByText(/"Performance tuning"/)).toBeInTheDocument();
    });
  });

  it("shows accountEmail", async () => {
    mockFetchSettingHistory.mockResolvedValue([
      createMockChange({ accountEmail: "user@test.com" }),
    ]);
    render(<SettingHistory />);

    await waitFor(() => {
      expect(screen.getByText("von user@test.com")).toBeInTheDocument();
    });
  });

  it("shows scope type info", async () => {
    mockFetchSettingHistory.mockResolvedValue([
      createMockChange({ scopeType: "system", scopeId: "1" }),
    ]);
    render(<SettingHistory />);

    await waitFor(() => {
      expect(screen.getByText(/Bereich: System/)).toBeInTheDocument();
    });
  });

  it("shows footer when history.length >= limit", async () => {
    const changes = Array.from({ length: 5 }, (_, i) =>
      createMockChange({ id: String(i + 1) }),
    );
    mockFetchSettingHistory.mockResolvedValue(changes);
    render(<SettingHistory limit={5} />);

    await waitFor(() => {
      expect(
        screen.getByText("Zeige die letzten 5 Änderungen"),
      ).toBeInTheDocument();
    });
  });

  it("does not show footer when history.length < limit", async () => {
    mockFetchSettingHistory.mockResolvedValue([createMockChange()]);
    render(<SettingHistory limit={20} />);

    await waitFor(() => {
      expect(screen.getByText("session.timeout")).toBeInTheDocument();
    });

    expect(screen.queryByText(/Zeige die letzten/)).not.toBeInTheDocument();
  });

  it("does not show values for delete entries", async () => {
    mockFetchSettingHistory.mockResolvedValue([
      createMockChange({
        changeType: "delete",
        oldValue: "old",
        newValue: undefined,
      }),
    ]);
    render(<SettingHistory />);

    await waitFor(() => {
      expect(screen.getByText("Gelöscht")).toBeInTheDocument();
    });

    // For delete/reset, old→new values are not shown
    expect(screen.queryByText("old")).not.toBeInTheDocument();
  });

  it("formats null/undefined values as dash", async () => {
    mockFetchSettingHistory.mockResolvedValue([
      createMockChange({ oldValue: undefined, newValue: null }),
    ]);
    render(<SettingHistory />);

    await waitFor(() => {
      expect(screen.getByText("session.timeout")).toBeInTheDocument();
    });
  });

  it("formats object values as JSON", async () => {
    mockFetchSettingHistory.mockResolvedValue([
      createMockChange({
        oldValue: undefined,
        newValue: { key: "val" },
      }),
    ]);
    render(<SettingHistory />);

    await waitFor(() => {
      expect(
        screen.getByText(JSON.stringify({ key: "val" })),
      ).toBeInTheDocument();
    });
  });
});

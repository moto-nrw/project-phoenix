import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, waitFor, fireEvent, screen } from "@testing-library/react";

const mockFetchSchema = vi.fn<() => Promise<unknown>>();
const mockSetSettingValue = vi.fn<() => Promise<string | null>>();
const mockResetSettingValue = vi.fn<() => Promise<string | null>>();

vi.mock("~/lib/settings-api", () => ({
  fetchSettingsSchema: () => mockFetchSchema(),
  setSettingValue: (_k: string, _v: unknown) => mockSetSettingValue(),
  resetSettingValue: (_k: string) => mockResetSettingValue(),
}));

const { useSettingsTabs } = await import("./settings-page");

const mockSchema = {
  tabs: [
    {
      key: "operations",
      label: "Betrieb",
      categories: [
        {
          key: "sessions",
          label: "Sitzungen",
          items: [
            {
              key: "ops.enabled",
              label: "Aktiviert",
              description: "Toggle feature",
              type: "boolean" as const,
              default: true,
              value: true,
              is_default: true,
              writable: true,
              visible: true,
              sort_order: 1,
              validation: null,
              depends_on: null,
              options: null,
            },
            {
              key: "ops.time",
              label: "Uhrzeit",
              description: "Time setting",
              type: "time" as const,
              default: "18:00",
              value: "18:00",
              is_default: true,
              writable: true,
              visible: true,
              sort_order: 2,
              validation: null,
              depends_on: null,
              options: null,
            },
          ],
        },
      ],
    },
    {
      key: "gdpr",
      label: "Datenschutz",
      categories: [],
    },
  ],
};

interface TabsResult {
  tabs: { id: string; label: string; icon: string }[];
  renderTab: (tabId: string) => React.ReactNode;
}

function HookWrapper({
  onResult,
}: {
  readonly onResult: (r: TabsResult | null) => void;
}) {
  const result = useSettingsTabs();
  onResult(result);
  return null;
}

// Renders the actual SettingsContent via the hook's renderTab
function RenderedTab({ tabId }: { readonly tabId: string }) {
  const result = useSettingsTabs();
  if (!result) return <div data-testid="no-tabs">No tabs</div>;
  return <div>{result.renderTab(tabId)}</div>;
}

describe("useSettingsTabs", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockSetSettingValue.mockResolvedValue(null);
    mockResetSettingValue.mockResolvedValue(null);
  });

  it("returns null when schema is null (no access)", async () => {
    mockFetchSchema.mockResolvedValue(null);
    let captured: TabsResult | null = null;

    render(<HookWrapper onResult={(r) => (captured = r)} />);
    await waitFor(() => {
      expect(captured).toBeNull();
    });
  });

  it("returns tabs from schema with German labels", async () => {
    mockFetchSchema.mockResolvedValue(mockSchema);
    let captured: TabsResult | null = null;

    render(<HookWrapper onResult={(r) => (captured = r)} />);
    await waitFor(() => {
      expect(captured).not.toBeNull();
    });
    expect(captured!.tabs).toHaveLength(2);
    expect(captured!.tabs[0]!.label).toBe("Betrieb");
    expect(captured!.tabs[1]!.label).toBe("Datenschutz");
  });

  it("returns tabs with settings- prefix IDs", async () => {
    mockFetchSchema.mockResolvedValue(mockSchema);
    let captured: TabsResult | null = null;

    render(<HookWrapper onResult={(r) => (captured = r)} />);
    await waitFor(() => {
      expect(captured).not.toBeNull();
    });
    expect(captured!.tabs[0]!.id).toBe("settings-operations");
  });

  it("returns null when schema has empty tabs", async () => {
    mockFetchSchema.mockResolvedValue({ tabs: [] });
    let captured: TabsResult | null = null;

    render(<HookWrapper onResult={(r) => (captured = r)} />);
    await waitFor(() => {
      expect(captured).toBeNull();
    });
  });

  it("tabs have icon paths", async () => {
    mockFetchSchema.mockResolvedValue(mockSchema);
    let captured: TabsResult | null = null;

    render(<HookWrapper onResult={(r) => (captured = r)} />);
    await waitFor(() => {
      expect(captured).not.toBeNull();
    });
    expect(typeof captured!.tabs[0]!.icon).toBe("string");
    expect(captured!.tabs[0]!.icon.length).toBeGreaterThan(0);
  });
});

describe("SettingsContent (via renderTab)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockSetSettingValue.mockResolvedValue(null);
    mockResetSettingValue.mockResolvedValue(null);
  });

  it("renders no-tabs when schema is null", async () => {
    mockFetchSchema.mockResolvedValue(null);
    render(<RenderedTab tabId="settings-operations" />);
    await waitFor(() => {
      expect(screen.getByTestId("no-tabs")).toBeDefined();
    });
  });

  it("renders settings items after loading", async () => {
    mockFetchSchema.mockResolvedValue(mockSchema);

    render(<RenderedTab tabId="settings-operations" />);
    expect(await screen.findByText("Aktiviert")).toBeDefined();
    expect(await screen.findByText("Uhrzeit")).toBeDefined();
  });

  it("shows nothing when schema is null (no access)", async () => {
    mockFetchSchema.mockResolvedValue(null);

    const { container } = render(<RenderedTab tabId="settings-operations" />);
    await waitFor(() => {
      expect(container.querySelector(".animate-spin")).toBeNull();
    });
  });

  it("shows Keine Einstellungen for unknown tab", async () => {
    mockFetchSchema.mockResolvedValue(mockSchema);

    render(<RenderedTab tabId="settings-nonexistent" />);
    expect(
      await screen.findByText("Keine Einstellungen verfügbar."),
    ).toBeDefined();
  });

  it("saves boolean value on toggle click", async () => {
    mockFetchSchema.mockResolvedValue(mockSchema);

    render(<RenderedTab tabId="settings-operations" />);
    const toggle = await screen.findByRole("switch");
    fireEvent.click(toggle);

    await waitFor(() => {
      expect(mockSetSettingValue).toHaveBeenCalled();
    });
  });

  it("renders category heading from schema", async () => {
    mockFetchSchema.mockResolvedValue(mockSchema);

    render(<RenderedTab tabId="settings-operations" />);
    expect(await screen.findByText("Sitzungen")).toBeDefined();
  });

  it("updates value optimistically after save (no immediate re-fetch)", async () => {
    mockFetchSchema.mockResolvedValue(mockSchema);

    render(<RenderedTab tabId="settings-operations" />);
    await screen.findByText("Aktiviert");

    const fetchCountBefore = mockFetchSchema.mock.calls.length;

    const toggle = screen.getByRole("switch");
    fireEvent.click(toggle);

    await waitFor(() => {
      expect(mockSetSettingValue).toHaveBeenCalled();
    });

    // No immediate re-fetch — uses optimistic update
    expect(mockFetchSchema.mock.calls.length).toBe(fetchCountBefore);
  });
});

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, waitFor } from "@testing-library/react";

const mockFetchSchema = vi.fn<() => Promise<unknown>>();

vi.mock("~/lib/settings-api", () => ({
  fetchSettingsSchema: () => mockFetchSchema(),
  setSettingValue: vi.fn().mockResolvedValue(null),
  resetSettingValue: vi.fn().mockResolvedValue(null),
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
              description: "Toggle",
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

describe("useSettingsTabs", () => {
  beforeEach(() => {
    vi.clearAllMocks();
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
    expect(captured!.tabs[1]!.id).toBe("settings-gdpr");
  });

  it("renderTab returns a React element", async () => {
    mockFetchSchema.mockResolvedValue(mockSchema);
    let captured: TabsResult | null = null;

    render(<HookWrapper onResult={(r) => (captured = r)} />);
    await waitFor(() => {
      expect(captured).not.toBeNull();
    });

    const element = captured!.renderTab("settings-operations");
    expect(element).toBeDefined();
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
    expect(captured!.tabs[0]!.icon).toBeTruthy();
    expect(typeof captured!.tabs[0]!.icon).toBe("string");
  });
});

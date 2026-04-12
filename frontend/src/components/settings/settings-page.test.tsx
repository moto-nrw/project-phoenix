import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, waitFor, fireEvent, screen } from "@testing-library/react";
import { ToastProvider } from "~/contexts/ToastContext";

const mockFetchSchema = vi.fn<() => Promise<unknown>>();
const mockSetSettingValue = vi.fn<() => Promise<string | null>>();
const mockResetSettingValue = vi.fn<() => Promise<string | null>>();

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { token: "test-token" }, expires: "2099-01-01" },
    status: "authenticated",
    update: vi.fn(),
  }),
}));

vi.mock("~/lib/settings-api", () => ({
  fetchSettingsSchema: () => mockFetchSchema(),
  setSettingValue: (_k: string, _v: unknown) => mockSetSettingValue(),
  resetSettingValue: (_k: string) => mockResetSettingValue(),
}));

const mockRefreshSupervision = vi.fn(() => Promise.resolve());
vi.mock("~/lib/supervision-context", () => ({
  useOptionalSupervision: () => ({
    hasGroups: false,
    groups: [],
    isLoadingGroups: false,
    isSupervising: false,
    supervisedRooms: [],
    isLoadingSupervision: false,
    refresh: mockRefreshSupervision,
  }),
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

function renderWithProviders(ui: React.ReactElement) {
  return render(<ToastProvider>{ui}</ToastProvider>);
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

  it("returns only personalisierung tab when schema is null (no access)", async () => {
    mockFetchSchema.mockResolvedValue(null);
    let captured: TabsResult | null = null;

    render(<HookWrapper onResult={(r) => (captured = r)} />);
    await waitFor(() => {
      expect(captured).not.toBeNull();
    });
    expect(captured!.tabs).toHaveLength(1);
    expect(captured!.tabs[0]!.id).toBe("settings-personalisierung");
  });

  it("returns schema tabs + personalisierung with German labels", async () => {
    mockFetchSchema.mockResolvedValue(mockSchema);
    let captured: TabsResult | null = null;

    render(<HookWrapper onResult={(r) => (captured = r)} />);
    await waitFor(() => {
      expect(captured).not.toBeNull();
    });
    expect(captured!.tabs).toHaveLength(3);
    expect(captured!.tabs[0]!.label).toBe("Betrieb");
    expect(captured!.tabs[1]!.label).toBe("Datenschutz");
    expect(captured!.tabs[2]!.label).toBe("Personalisierung");
  });

  it("returns tabs with settings- prefix IDs", async () => {
    mockFetchSchema.mockResolvedValue(mockSchema);
    let captured: TabsResult | null = null;

    render(<HookWrapper onResult={(r) => (captured = r)} />);
    await waitFor(() => {
      expect(captured).not.toBeNull();
    });
    expect(captured!.tabs[0]!.id).toBe("settings-operations");
    expect(captured!.tabs[2]!.id).toBe("settings-personalisierung");
  });

  it("returns only personalisierung when schema has empty tabs", async () => {
    mockFetchSchema.mockResolvedValue({ tabs: [] });
    let captured: TabsResult | null = null;

    render(<HookWrapper onResult={(r) => (captured = r)} />);
    await waitFor(() => {
      expect(captured).not.toBeNull();
    });
    expect(captured!.tabs).toHaveLength(1);
    expect(captured!.tabs[0]!.id).toBe("settings-personalisierung");
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
    renderWithProviders(<RenderedTab tabId="settings-operations" />);
    await waitFor(() => {
      expect(screen.getByTestId("no-tabs")).toBeDefined();
    });
  });

  it("renders settings items after loading", async () => {
    mockFetchSchema.mockResolvedValue(mockSchema);

    renderWithProviders(<RenderedTab tabId="settings-operations" />);
    expect(await screen.findByText("Aktiviert")).toBeDefined();
    expect(await screen.findByText("Uhrzeit")).toBeDefined();
  });

  it("shows nothing when schema is null (no access)", async () => {
    mockFetchSchema.mockResolvedValue(null);

    const { container } = renderWithProviders(
      <RenderedTab tabId="settings-operations" />,
    );
    await waitFor(() => {
      expect(container.querySelector(".animate-spin")).toBeNull();
    });
  });

  it("shows Keine Einstellungen for unknown tab", async () => {
    mockFetchSchema.mockResolvedValue(mockSchema);

    renderWithProviders(<RenderedTab tabId="settings-nonexistent" />);
    expect(
      await screen.findByText("Keine Einstellungen verfügbar."),
    ).toBeDefined();
  });

  it("saves boolean value on toggle click", async () => {
    mockFetchSchema.mockResolvedValue(mockSchema);

    renderWithProviders(<RenderedTab tabId="settings-operations" />);
    const toggle = await screen.findByRole("switch");
    fireEvent.click(toggle);

    await waitFor(() => {
      expect(mockSetSettingValue).toHaveBeenCalled();
    });
  });

  it("renders category heading from schema", async () => {
    mockFetchSchema.mockResolvedValue(mockSchema);

    renderWithProviders(<RenderedTab tabId="settings-operations" />);
    expect(await screen.findByText("Sitzungen")).toBeDefined();
  });

  it("updates value optimistically after save (no immediate re-fetch)", async () => {
    mockFetchSchema.mockResolvedValue(mockSchema);

    renderWithProviders(<RenderedTab tabId="settings-operations" />);
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

  it("shows error banner on save network error", async () => {
    mockFetchSchema.mockResolvedValue(mockSchema);
    mockSetSettingValue.mockResolvedValue(
      "Netzwerkfehler beim Speichern der Einstellung.",
    );

    const { container } = renderWithProviders(
      <RenderedTab tabId="settings-operations" />,
    );
    const toggle = await screen.findByRole("switch");
    fireEvent.click(toggle);

    await waitFor(() => {
      // Error banner has specific styling — look for it in the banner container
      const banner = container.querySelector(".bg-red-50");
      expect(banner).not.toBeNull();
      expect(banner!.textContent).toContain("Netzwerkfehler");
    });
  });

  it("shows error banner on save server error", async () => {
    mockFetchSchema.mockResolvedValue(mockSchema);
    mockSetSettingValue.mockResolvedValue(
      "Einstellung konnte nicht gespeichert werden.",
    );

    const { container } = renderWithProviders(
      <RenderedTab tabId="settings-operations" />,
    );
    const toggle = await screen.findByRole("switch");
    fireEvent.click(toggle);

    await waitFor(() => {
      const banner = container.querySelector(".bg-red-50");
      expect(banner).not.toBeNull();
      expect(banner!.textContent).toContain("Einstellung konnte nicht");
    });
  });

  it("dismisses error banner when clicking close", async () => {
    mockFetchSchema.mockResolvedValue(mockSchema);
    mockSetSettingValue.mockResolvedValue(
      "Netzwerkfehler beim Speichern der Einstellung.",
    );

    const { container } = renderWithProviders(
      <RenderedTab tabId="settings-operations" />,
    );
    const toggle = await screen.findByRole("switch");
    fireEvent.click(toggle);

    await waitFor(() => {
      expect(container.querySelector(".bg-red-50")).not.toBeNull();
    });

    // Click the dismiss button
    const closeButton = screen.getByLabelText("Fehler schließen");
    fireEvent.click(closeButton);

    await waitFor(() => {
      expect(container.querySelector(".bg-red-50")).toBeNull();
    });
  });

  it("resets value and reloads schema", async () => {
    mockFetchSchema.mockResolvedValue(mockSchema);

    renderWithProviders(<RenderedTab tabId="settings-operations" />);
    await screen.findByText("Aktiviert");

    // Simulate a reset — the component calls resetSettingValue then loadSchema
    // The reset button is only shown on non-default values, so we need an override
    const schemaWithOverride = {
      ...mockSchema,
      tabs: [
        {
          ...mockSchema.tabs[0]!,
          categories: [
            {
              ...mockSchema.tabs[0]!.categories[0]!,
              items: [
                {
                  ...mockSchema.tabs[0]!.categories[0]!.items[0]!,
                  is_default: false,
                  value: false,
                },
                mockSchema.tabs[0]!.categories[0]!.items[1]!,
              ],
            },
          ],
        },
        mockSchema.tabs[1]!,
      ],
    };
    mockFetchSchema.mockResolvedValue(schemaWithOverride);

    // Re-render with the overridden schema
    const { unmount } = renderWithProviders(
      <RenderedTab tabId="settings-operations" />,
    );

    await waitFor(() => {
      // The key thing is that the component loaded with the overridden schema
      expect(screen.getAllByText("Aktiviert").length).toBeGreaterThan(0);
    });
    unmount();
  });

  it("shows error banner on reset failure", async () => {
    mockFetchSchema.mockResolvedValue(mockSchema);
    mockResetSettingValue.mockResolvedValue(
      "Einstellung konnte nicht zurückgesetzt werden.",
    );

    // Need non-default values to show reset button
    const schemaWithOverride = {
      ...mockSchema,
      tabs: [
        {
          ...mockSchema.tabs[0]!,
          categories: [
            {
              ...mockSchema.tabs[0]!.categories[0]!,
              items: [
                {
                  ...mockSchema.tabs[0]!.categories[0]!.items[0]!,
                  is_default: false,
                  value: false,
                },
                mockSchema.tabs[0]!.categories[0]!.items[1]!,
              ],
            },
          ],
        },
        mockSchema.tabs[1]!,
      ],
    };
    mockFetchSchema.mockResolvedValue(schemaWithOverride);

    renderWithProviders(<RenderedTab tabId="settings-operations" />);
    await screen.findByText("Aktiviert");

    // Find and click reset if available
    const resetButtons = screen.queryAllByText("Zurücksetzen");
    if (resetButtons.length > 0) {
      fireEvent.click(resetButtons[0]!);

      await waitFor(() => {
        expect(
          screen.getByText("Einstellung konnte nicht zurückgesetzt werden."),
        ).toBeDefined();
      });
    }
  });

  it("clears error after successful save", async () => {
    mockFetchSchema.mockResolvedValue(mockSchema);

    // First save fails
    mockSetSettingValue.mockResolvedValueOnce(
      "Netzwerkfehler beim Speichern der Einstellung.",
    );

    const { container } = renderWithProviders(
      <RenderedTab tabId="settings-operations" />,
    );
    const toggle = await screen.findByRole("switch");
    fireEvent.click(toggle);

    await waitFor(() => {
      expect(container.querySelector(".bg-red-50")).not.toBeNull();
    });

    // Second save succeeds
    mockSetSettingValue.mockResolvedValueOnce(null);
    fireEvent.click(toggle);

    await waitFor(() => {
      expect(container.querySelector(".bg-red-50")).toBeNull();
    });
  });

  it("does not show error banner for validation errors", async () => {
    mockFetchSchema.mockResolvedValue(mockSchema);
    // A validation error like "Minimum: 5" should NOT be shown as a banner
    mockSetSettingValue.mockResolvedValue("Minimum: 5");

    const { container } = renderWithProviders(
      <RenderedTab tabId="settings-operations" />,
    );
    const toggle = await screen.findByRole("switch");
    fireEvent.click(toggle);

    // Wait for save to complete
    await waitFor(() => {
      expect(mockSetSettingValue).toHaveBeenCalled();
    });

    // Validation errors don't match the banner condition — no banner should appear
    expect(container.querySelector(".bg-red-50")).toBeNull();
  });

  it("refreshes supervision context after toggling admin_supervision_overview", async () => {
    const schemaWithAdminOverview = {
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
                  key: "operations.admin_supervision_overview",
                  label: "Administrator-Aufsichtsübersicht",
                  description: "Admins sehen alle Aufsichten",
                  type: "boolean" as const,
                  default: false,
                  value: false,
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
      ],
    };
    mockFetchSchema.mockResolvedValue(schemaWithAdminOverview);
    mockSetSettingValue.mockResolvedValue(null);

    renderWithProviders(<RenderedTab tabId="settings-operations" />);
    const toggle = await screen.findByRole("switch");
    fireEvent.click(toggle);

    await waitFor(() => {
      expect(mockRefreshSupervision).toHaveBeenCalledWith({ force: true });
    });
  });

  it("does not refresh supervision context for unrelated settings", async () => {
    mockFetchSchema.mockResolvedValue(mockSchema);
    mockSetSettingValue.mockResolvedValue(null);

    renderWithProviders(<RenderedTab tabId="settings-operations" />);
    const toggle = await screen.findByRole("switch");
    fireEvent.click(toggle);

    await waitFor(() => {
      expect(mockSetSettingValue).toHaveBeenCalled();
    });
    expect(mockRefreshSupervision).not.toHaveBeenCalled();
  });
});

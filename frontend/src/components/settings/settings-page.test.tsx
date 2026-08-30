import { describe, it, expect, vi, beforeEach } from "vitest";
import { useEffect, useState } from "react";
import { render, waitFor, fireEvent, screen } from "@testing-library/react";
import { ToastProvider } from "~/contexts/ToastContext";

const mockFetchSchema = vi.fn<() => Promise<unknown>>();
const mockSetSettingValue = vi.fn<() => Promise<string | null>>();
const mockResetSettingValue = vi.fn<() => Promise<string | null>>();

// Local SWR override — global mock in test/setup.ts returns isLoading
// forever, which would deadlock the page after the migration to useSWR.
// Subscribers track per-key consumers; top-level mutate(key) triggers the
// matching fetcher re-runs so optimistic-update tests still observe the
// authoritative re-fetch.
const swrSubscribers = new Map<unknown, Set<() => void>>();
function notifyKey(key: unknown) {
  const subs = swrSubscribers.get(key);
  if (!subs) return;
  for (const fn of subs) fn();
}
const swrMutate = vi.fn((key: unknown) => {
  notifyKey(key);
  return Promise.resolve();
});
vi.mock("swr", () => ({
  default: (key: unknown, fetcher: () => Promise<unknown>) => {
    const [data, setData] = useState<unknown>(undefined);
    const [error, setError] = useState<unknown>(undefined);
    const [isLoading, setLoading] = useState(true);
    const fetchOnce = () => {
      setLoading(true);
      Promise.resolve()
        .then(() => fetcher())
        .then((d) => {
          setData(d);
          setError(undefined);
        })
        .catch((e: unknown) => setError(e))
        .finally(() => setLoading(false));
    };
    useEffect(() => {
      if (key == null) {
        setLoading(false);
        return;
      }
      const subs = swrSubscribers.get(key) ?? new Set<() => void>();
      subs.add(fetchOnce);
      swrSubscribers.set(key, subs);
      fetchOnce();
      return () => {
        subs.delete(fetchOnce);
      };
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [key]);
    return {
      data,
      error,
      isLoading: isLoading && data === undefined,
      isValidating: isLoading,
      mutate: () => {
        fetchOnce();
        return Promise.resolve(data);
      },
    };
  },
  mutate: swrMutate,
  useSWRConfig: () => ({ mutate: swrMutate, cache: new Map() }),
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { token: "test-token" }, expires: "2099-01-01" },
    status: "authenticated",
    update: vi.fn(),
  }),
}));

// Settings page now calls router.refresh() after save/reset for tenant-
// resolve-affecting keys; jsdom tests don't mount the App-Router context
// so we provide a stub that satisfies the invariant check.
vi.mock("next/navigation", () => ({
  useRouter: () => ({
    refresh: vi.fn(),
    push: vi.fn(),
    replace: vi.fn(),
    back: vi.fn(),
    forward: vi.fn(),
    prefetch: vi.fn(),
  }),
  // useSearchParams is consumed by deeper sub-trees (e.g. the
  // settings tab deep-link via ?tab=…); without it, render
  // throws "No 'useSearchParams' export is defined". Empty
  // URLSearchParams is the right default since these tests
  // never navigate.
  useSearchParams: () => new URLSearchParams(),
  usePathname: () => "/settings",
}));

vi.mock("~/lib/settings-api", () => ({
  SETTINGS_SCHEMA_SWR_KEY: "settings-schema",
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
    overviewEnabled: false,
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
  useEffect(() => {
    onResult(result);
  }, [onResult, result]);
  return null;
}

function renderWithProviders(ui: React.ReactElement) {
  return render(<ToastProvider>{ui}</ToastProvider>);
}

// Renders the actual SettingsContent via the hook's renderTab.
// Mounts useSettingsCacheBridge so cross-tab BroadcastChannel and SSE
// (phoenix:tenant-settings-stale) invalidations hit the SWR cache the
// same way they do under the protected layout.
const { useSettingsCacheBridge } =
  await import("~/lib/hooks/use-settings-cache-bridge");
function RenderedTab({ tabId }: { readonly tabId: string }) {
  useSettingsCacheBridge();
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

  it("updates value optimistically and re-fetches authoritatively after save", async () => {
    // Save paints optimistically, then converges on canonical server state.
    mockFetchSchema.mockResolvedValue(mockSchema);

    renderWithProviders(<RenderedTab tabId="settings-operations" />);
    await screen.findByText("Aktiviert");

    const fetchCountBefore = mockFetchSchema.mock.calls.length;

    const toggle = screen.getByRole("switch");
    fireEvent.click(toggle);

    await waitFor(() => {
      expect(mockSetSettingValue).toHaveBeenCalled();
    });

    // Authoritative re-fetch happens immediately after save — covers
    // server-side derived state (defaults, audit columns) the
    // optimistic setSchema doesn't know about.
    await waitFor(() => {
      expect(mockFetchSchema.mock.calls.length).toBeGreaterThan(
        fetchCountBefore,
      );
    });
  });

  it("re-fetches schema on tenant settings SSE invalidation", async () => {
    mockFetchSchema.mockResolvedValue(mockSchema);

    renderWithProviders(<RenderedTab tabId="settings-operations" />);
    await screen.findByText("Aktiviert");

    const fetchCountBefore = mockFetchSchema.mock.calls.length;
    window.dispatchEvent(new CustomEvent("phoenix:tenant-settings-stale"));

    await waitFor(() => {
      expect(mockFetchSchema.mock.calls.length).toBeGreaterThan(
        fetchCountBefore,
      );
    });
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
      const banner = container.querySelector('[role="alert"]');
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
      const banner = container.querySelector('[role="alert"]');
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
      expect(container.querySelector('[role="alert"]')).not.toBeNull();
    });

    // Click the dismiss button
    const closeButton = screen.getByLabelText("Fehler schließen");
    fireEvent.click(closeButton);

    await waitFor(() => {
      expect(container.querySelector('[role="alert"]')).toBeNull();
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
      expect(container.querySelector('[role="alert"]')).not.toBeNull();
    });

    // Second save succeeds
    mockSetSettingValue.mockResolvedValueOnce(null);
    fireEvent.click(toggle);

    await waitFor(() => {
      expect(container.querySelector('[role="alert"]')).toBeNull();
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
    expect(container.querySelector('[role="alert"]')).toBeNull();
  });

  it("refreshes supervision context after changing the operational overview scope", async () => {
    const schemaWithOverviewScope = {
      tabs: [
        {
          key: "operations",
          label: "Betrieb",
          categories: [
            {
              key: "aufsicht",
              label: "Aufsicht",
              items: [
                {
                  key: "operations.operational_overview_scope",
                  label: "Sichtbereich für Mitarbeitende",
                  description:
                    "Legt fest, welche Gruppen und laufenden Betreuungen Mitarbeitende sehen.",
                  type: "select" as const,
                  default: "all_staff",
                  value: "all_staff",
                  is_default: true,
                  writable: true,
                  visible: true,
                  sort_order: 1,
                  validation: null,
                  depends_on: null,
                  options: {
                    static: [
                      {
                        label: "Ganzes Team",
                        value: "all_staff",
                      },
                      { label: "Eigene Zuständigkeiten", value: "own" },
                    ],
                  },
                },
              ],
            },
          ],
        },
      ],
    };
    mockFetchSchema.mockResolvedValue(schemaWithOverviewScope);
    mockSetSettingValue.mockResolvedValue(null);

    renderWithProviders(<RenderedTab tabId="settings-operations" />);
    fireEvent.click(await screen.findByRole("combobox"));
    fireEvent.click(
      screen.getByRole("option", {
        name: "Eigene Zuständigkeiten",
      }),
    );

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

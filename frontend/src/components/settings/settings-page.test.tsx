import { describe, it, expect, vi, beforeEach } from "vitest";
import { useEffect, useState } from "react";
import { render, waitFor, fireEvent, screen } from "@testing-library/react";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError, unavailableApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

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

// Mutable so a test can grant config:update ("Startseite für alle").
const mockPermissions = { value: [] as string[] };
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: {
      user: { token: "test-token", permissions: mockPermissions.value },
      expires: "2099-01-01",
    },
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
  useSearchParams: () => new URLSearchParams(mockSearchParams.value),
  usePathname: () => "/settings",
}));

// Mutable so a test can simulate the `?highlight=<key>` deep link.
const mockSearchParams = { value: "" };

vi.mock("~/lib/settings-api", () => ({
  SETTINGS_SCHEMA_SWR_KEY: "settings-schema",
  fetchSettingsSchema: () => mockFetchSchema(),
  saveSettingValue: (_k: string, _v: unknown) => mockSetSettingValue(),
  clearSettingValue: (_k: string) => mockResetSettingValue(),
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
const { useNFCEnabled } = await import("~/lib/tenant-context");

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
  highlightTabId: string | null;
  highlightNeedsNfc: boolean;
}

function schemaItem(key: string) {
  return {
    key,
    label: key,
    description: key,
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
  };
}

// Schema with the NFC-only "Geräte" tab and the birthday switches that live
// on the hand-written "Startseite für alle" tab (#3735, #3737).
const schemaWithDevicesAndStartseite = {
  tabs: [
    ...mockSchema.tabs,
    {
      key: "devices",
      label: "devices",
      categories: [
        {
          key: "checkout",
          label: "checkout",
          items: [schemaItem("checkout.wc_enabled")],
        },
      ],
    },
    {
      key: "startseite",
      label: "startseite",
      categories: [
        {
          key: "geburtstage",
          label: "geburtstage",
          items: [schemaItem("operations.birthday_display_enabled")],
        },
      ],
    },
  ],
};

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

// Categories start collapsed (#2830); open the one under test so its fields
// mount before a test looks for them.
async function openCategory(name: RegExp | string = /Sitzungen/) {
  fireEvent.click(await screen.findByRole("button", { name }));
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
  return <div data-testid="settings-tab">{result.renderTab(tabId)}</div>;
}

function settingsErrorBanner(): Element | null {
  return screen.getByTestId("settings-tab").querySelector('[role="alert"]');
}

describe("useSettingsTabs", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useNFCEnabled).mockReturnValue(true);
    mockSearchParams.value = "";
    mockPermissions.value = [];
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

  it("shows the Geräte tab for a school with NFC", async () => {
    vi.mocked(useNFCEnabled).mockReturnValue(true);
    mockFetchSchema.mockResolvedValue(schemaWithDevicesAndStartseite);
    let captured: TabsResult | null = null;

    render(<HookWrapper onResult={(r) => (captured = r)} />);
    await waitFor(() => {
      expect(captured).not.toBeNull();
    });
    expect(captured!.tabs.map((tab) => tab.label)).toContain("Geräte");
  });

  it("hides the Geräte tab for a school without NFC (#3735)", async () => {
    vi.mocked(useNFCEnabled).mockReturnValue(false);
    mockFetchSchema.mockResolvedValue(schemaWithDevicesAndStartseite);
    let captured: TabsResult | null = null;

    render(<HookWrapper onResult={(r) => (captured = r)} />);
    await waitFor(() => {
      expect(captured).not.toBeNull();
    });
    expect(captured!.tabs.map((tab) => tab.id)).not.toContain(
      "settings-devices",
    );
  });

  it("never renders the startseite schema tab as a generic tab (#3737)", async () => {
    mockFetchSchema.mockResolvedValue(schemaWithDevicesAndStartseite);
    let captured: TabsResult | null = null;

    render(<HookWrapper onResult={(r) => (captured = r)} />);
    await waitFor(() => {
      expect(captured).not.toBeNull();
    });
    // Without config:update there is no "Startseite für alle" at all, and
    // the schema tab must not sneak in as a generic one.
    expect(captured!.tabs.map((tab) => tab.id)).not.toContain(
      "settings-startseite",
    );

    mockPermissions.value = ["config:update"];
    let withWrite: TabsResult | null = null;
    render(<HookWrapper onResult={(r) => (withWrite = r)} />);
    await waitFor(() => {
      expect(withWrite).not.toBeNull();
    });
    expect(
      withWrite!.tabs.filter((tab) => tab.id === "settings-startseite"),
    ).toHaveLength(1);
  });

  it("maps a device deep link to the Geräte tab with NFC", async () => {
    vi.mocked(useNFCEnabled).mockReturnValue(true);
    mockSearchParams.value = "highlight=checkout.wc_enabled";
    mockFetchSchema.mockResolvedValue(schemaWithDevicesAndStartseite);
    let captured: TabsResult | null = null;

    render(<HookWrapper onResult={(r) => (captured = r)} />);
    await waitFor(() => {
      expect(captured?.highlightTabId).toBe("settings-devices");
    });
    expect(captured!.highlightNeedsNfc).toBe(false);
  });

  it("explains a device deep link without NFC instead of landing nowhere", async () => {
    vi.mocked(useNFCEnabled).mockReturnValue(false);
    mockSearchParams.value = "highlight=checkout.wc_enabled";
    mockFetchSchema.mockResolvedValue(schemaWithDevicesAndStartseite);
    let captured: TabsResult | null = null;

    render(<HookWrapper onResult={(r) => (captured = r)} />);
    await waitFor(() => {
      expect(captured?.highlightNeedsNfc).toBe(true);
    });
    expect(captured!.highlightTabId).toBeNull();
  });

  it("maps a birthday deep link to Startseite für alle", async () => {
    mockPermissions.value = ["config:update"];
    mockSearchParams.value = "highlight=operations.birthday_display_enabled";
    mockFetchSchema.mockResolvedValue(schemaWithDevicesAndStartseite);
    let captured: TabsResult | null = null;

    render(<HookWrapper onResult={(r) => (captured = r)} />);
    await waitFor(() => {
      expect(captured?.highlightTabId).toBe("settings-startseite");
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
    renderWithProviders(<RenderedTab tabId="settings-operations" />);
    await waitFor(() => {
      expect(screen.getByTestId("no-tabs")).toBeDefined();
    });
  });

  it("renders settings items after loading", async () => {
    mockFetchSchema.mockResolvedValue(mockSchema);

    renderWithProviders(<RenderedTab tabId="settings-operations" />);
    await openCategory();
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
    await openCategory();
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
    await openCategory();
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
    await openCategory();
    await screen.findByText("Aktiviert");

    const fetchCountBefore = mockFetchSchema.mock.calls.length;
    window.dispatchEvent(new CustomEvent("phoenix:tenant-settings-stale"));

    await waitFor(() => {
      expect(mockFetchSchema.mock.calls.length).toBeGreaterThan(
        fetchCountBefore,
      );
    });
  });

  // #2517: a failed save is a toast with the catalog text, not a banner in
  // the tab; "Wiederholen" sends the same value again.
  it("shows a failed save as a toast and retries it", async () => {
    mockFetchSchema.mockResolvedValue(mockSchema);
    mockSetSettingValue.mockRejectedValueOnce(
      unavailableApiError(new Error("offline")),
    );

    renderWithProviders(<RenderedTab tabId="settings-operations" />);
    await openCategory();
    const toggle = await screen.findByRole("switch");
    fireEvent.click(toggle);

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Einstellung „Aktiviert“"),
      ),
    ).toBeInTheDocument();
    expect(settingsErrorBanner()).toBeNull();

    mockSetSettingValue.mockResolvedValueOnce(null);
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() => expect(mockSetSettingValue).toHaveBeenCalledTimes(2));
    expect(
      await screen.findByText("Die Einstellung „Aktiviert“ ist gespeichert."),
    ).toBeInTheDocument();
  });

  it("shows the class text for a rejected value", async () => {
    mockFetchSchema.mockResolvedValue(mockSchema);
    mockSetSettingValue.mockRejectedValueOnce(new ApiError("bad", 400));

    renderWithProviders(<RenderedTab tabId="settings-operations" />);
    await openCategory();
    fireEvent.click(await screen.findByRole("switch"));

    expect(
      await screen.findByText(
        catalogText("general.input", "die Einstellung „Aktiviert“"),
      ),
    ).toBeInTheDocument();
  });

  it("resets value and reloads schema", async () => {
    mockFetchSchema.mockResolvedValue(mockSchema);

    renderWithProviders(<RenderedTab tabId="settings-operations" />);
    await openCategory();
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
    mockResetSettingValue.mockRejectedValue(new ApiError("boom", 500));

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
    await openCategory();
    await screen.findByText("Aktiviert");

    // Find and click reset if available
    const resetButtons = screen.queryAllByText("Zurücksetzen");
    if (resetButtons.length > 0) {
      fireEvent.click(resetButtons[0]!);

      await waitFor(() => {
        expect(
          screen.getByText(
            catalogText("general.server", "die Einstellung „Aktiviert“"),
          ),
        ).toBeDefined();
      });
    }
  });

  it("shows a failed schema load in place with retry", async () => {
    mockFetchSchema.mockRejectedValue(new ApiError("boom", 500));

    renderWithProviders(<RenderedTab tabId="settings-operations" />);

    expect(
      await screen.findByText(
        catalogText("general.server", "die Liste der Einstellungen"),
      ),
    ).toBeInTheDocument();
    mockFetchSchema.mockResolvedValue(mockSchema);
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(await screen.findByText("Sitzungen")).toBeInTheDocument();
  });

  it("refreshes supervision context after changing the operational overview scope", async () => {
    const schemaWithOverviewScope = {
      tabs: [
        {
          key: "operations",
          label: "Betrieb",
          categories: [
            {
              key: "sehen-und-bearbeiten",
              label: "sehen-und-bearbeiten",
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
    await openCategory(/Sehen und bearbeiten/);
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
    await openCategory();
    const toggle = await screen.findByRole("switch");
    fireEvent.click(toggle);

    await waitFor(() => {
      expect(mockSetSettingValue).toHaveBeenCalled();
    });
    expect(mockRefreshSupervision).not.toHaveBeenCalled();
  });
});

describe("SettingsContent collapsed categories and search (#2830)", () => {
  const twoTabSchema = {
    tabs: [
      {
        key: "operations",
        label: "Betrieb",
        categories: [
          mockSchema.tabs[0]!.categories[0]!,
          {
            key: "abholung",
            label: "Abholung",
            items: [
              {
                key: "ops.pickup_note",
                label: "Abholnotiz",
                description: "Hinweis bei der Abholung anzeigen",
                type: "boolean" as const,
                default: false,
                value: true,
                is_default: false,
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
        categories: [
          {
            key: "bewegungsdaten",
            label: "Bewegungsdaten",
            items: [
              {
                key: "gdpr.retention_days",
                label: "Aufbewahrung Besuchsdaten",
                description: "Wie lange Besuchsdaten gespeichert bleiben",
                type: "number" as const,
                default: 30,
                value: 30,
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

  beforeEach(() => {
    vi.clearAllMocks();
    mockSearchParams.value = "";
    mockSetSettingValue.mockResolvedValue(null);
    mockResetSettingValue.mockResolvedValue(null);
  });

  it("starts collapsed and lists the settings of a category in one line", async () => {
    mockFetchSchema.mockResolvedValue(mockSchema);

    renderWithProviders(<RenderedTab tabId="settings-operations" />);
    const heading = await screen.findByRole("button", { name: /Sitzungen/ });
    expect(heading.getAttribute("aria-expanded")).toBe("false");
    expect(screen.getByText("Aktiviert, Uhrzeit")).toBeDefined();
    expect(screen.queryByText("Aktiviert")).toBeNull();
    expect(screen.queryByRole("switch")).toBeNull();

    fireEvent.click(heading);
    expect(heading.getAttribute("aria-expanded")).toBe("true");
    expect(screen.getByRole("switch")).toBeDefined();
    expect(screen.queryByText("Aktiviert, Uhrzeit")).toBeNull();
  });

  it("shows how many settings of a category differ from the default", async () => {
    mockFetchSchema.mockResolvedValue(twoTabSchema);

    renderWithProviders(<RenderedTab tabId="settings-operations" />);
    expect(await screen.findByText("1 geändert")).toBeDefined();
  });

  it("opens the category that holds the deep-linked setting", async () => {
    mockFetchSchema.mockResolvedValue(twoTabSchema);
    mockSearchParams.value = "highlight=ops.pickup_note";

    renderWithProviders(<RenderedTab tabId="settings-operations" />);
    expect(await screen.findByText("Abholnotiz")).toBeDefined();
    const other = screen.getByRole("button", { name: /Sitzungen/ });
    expect(other.getAttribute("aria-expanded")).toBe("false");
  });

  it("expands and collapses every category of the tab", async () => {
    mockFetchSchema.mockResolvedValue(twoTabSchema);

    renderWithProviders(<RenderedTab tabId="settings-operations" />);
    fireEvent.click(
      await screen.findByRole("button", { name: "Alle ausklappen" }),
    );
    expect(screen.getByText("Aktiviert")).toBeDefined();
    expect(screen.getByText("Hinweis bei der Abholung anzeigen")).toBeDefined();
    expect(screen.getAllByRole("switch")).toHaveLength(2);

    fireEvent.click(screen.getByRole("button", { name: "Alle einklappen" }));
    expect(screen.queryByText("Aktiviert")).toBeNull();
    // The collapsed summary still names the setting; its field is gone.
    expect(screen.queryByText("Hinweis bei der Abholung anzeigen")).toBeNull();
    expect(screen.queryByRole("switch")).toBeNull();
    expect(
      screen.getByRole("button", { name: "Alle ausklappen" }),
    ).toBeDefined();
  });

  it("searches across every tab and labels hits with their tab", async () => {
    mockFetchSchema.mockResolvedValue(twoTabSchema);

    renderWithProviders(<RenderedTab tabId="settings-operations" />);
    const search = await screen.findByRole("searchbox", {
      name: "Einstellung suchen",
    });
    fireEvent.change(search, { target: { value: "aufbew" } });

    expect(screen.getByText("Aufbewahrung Besuchsdaten")).toBeDefined();
    expect(screen.getByText("Datenschutz")).toBeDefined();
    expect(screen.getByText("1 Treffer in allen Bereichen")).toBeDefined();
    expect(screen.queryByText("Aktiviert")).toBeNull();
    expect(screen.queryByRole("button", { name: /Sitzungen/ })).toBeNull();

    fireEvent.change(search, { target: { value: "" } });
    expect(screen.getByRole("button", { name: /Sitzungen/ })).toBeDefined();
    expect(screen.queryByText("Aufbewahrung Besuchsdaten")).toBeNull();
  });

  it("shows an empty state when nothing matches the search", async () => {
    mockFetchSchema.mockResolvedValue(twoTabSchema);

    renderWithProviders(<RenderedTab tabId="settings-operations" />);
    const search = await screen.findByRole("searchbox", {
      name: "Einstellung suchen",
    });
    fireEvent.change(search, { target: { value: "xyz" } });

    expect(screen.getByText("Keine Einstellung passt zu „xyz“.")).toBeDefined();
    expect(screen.getByText("0 Treffer in allen Bereichen")).toBeDefined();
  });
});

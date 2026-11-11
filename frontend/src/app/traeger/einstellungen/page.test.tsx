import { useTenantMutateMatching } from "~/lib/swr";
import { traegerKeys } from "~/lib/swr/portal-keys";
import { SWRConfig } from "~/lib/swr/testing";
vi.unmock("swr");
import {
  fireEvent,
  render as rtlRender,
  screen,
  waitFor,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type { ResolvedSetting, SettingsSchema } from "~/lib/settings-api";
import { catalogText } from "~/test/error-catalog-text";

const { mockFetchSchema, mockFetchMasterData, mockSave, mockClear, route } =
  vi.hoisted(() => ({
    mockFetchSchema: vi.fn(),
    mockFetchMasterData: vi.fn(),
    mockSave: vi.fn(),
    mockClear: vi.fn(),
    route: { search: "" },
  }));

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(route.search),
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: {
      user: {
        id: "A",
        scope: "org",
        orgId: 7,
        permissions: ["traeger_overview:read", "traeger_settings:manage"],
      },
    },
    status: "authenticated",
  }),
}));
// The link to the letterhead page needs no portal host here.
vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path,
}));
vi.mock("~/lib/traeger-api", () => ({
  saveTraegerContact: vi.fn(),
  fetchTraegerSettingsSchema: () => mockFetchSchema(),
  fetchTraegerMasterData: () => mockFetchMasterData(),
  saveTraegerSettingValue: (key: string, value: unknown) =>
    mockSave(key, value),
  clearTraegerSettingValue: (key: string) => mockClear(key),
}));

import TraegerSettingsPage from "./page";

function item(overrides: Partial<ResolvedSetting>): ResolvedSetting {
  return {
    key: "parent.news_enabled",
    label: "Neuigkeiten im Elternportal",
    description: "Eltern sehen die Neuigkeiten der OGS.",
    type: "boolean",
    default: false,
    value: false,
    is_default: true,
    writable: true,
    visible: true,
    sort_order: 1,
    access_policy: "shared",
    levels: "beide",
    source: "default",
    validation: null,
    depends_on: null,
    options: null,
    ...overrides,
  };
}

function schema(items: ResolvedSetting[]): SettingsSchema {
  return {
    tabs: [
      {
        key: "operations",
        label: "operations",
        categories: [{ key: "elternportal", label: "elternportal", items }],
      },
    ],
  };
}

const NEWS_FROM_TRAEGER = item({
  value: true,
  is_default: false,
  source: "traeger",
  traeger_changed_at: "2026-09-01T08:00:00Z",
});

const STAFF_NAME_DEFAULT = item({
  key: "parent.message_staff_name_visible",
  label: "Namen des Teams zeigen",
  sort_order: 2,
});

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((next) => {
    resolve = next;
  });
  return { promise, resolve };
}

function renderPage() {
  return render(
    <ToastProvider>
      <TraegerSettingsPage />
    </ToastProvider>,
  );
}

describe("TraegerSettingsPage", () => {
  beforeEach(() => {
    mockFetchSchema.mockReset();
    mockFetchMasterData.mockReset();
    mockSave.mockReset().mockResolvedValue(undefined);
    mockClear.mockReset().mockResolvedValue(undefined);
    route.search = "";
  });

  it("opens the area the address names, e.g. from the dashboard hints (#4675)", async () => {
    route.search = "bereich=dashboard";
    mockFetchSchema.mockResolvedValue({
      tabs: [
        ...schema([NEWS_FROM_TRAEGER]).tabs,
        {
          key: "dashboard",
          label: "dashboard",
          categories: [
            {
              key: "auffaelligkeiten",
              label: "auffaelligkeiten",
              items: [
                item({
                  key: "traeger_dashboard.alert_open_enrollments",
                  label: "Offene Anmeldungen ab",
                  type: "number",
                  default: 1,
                  value: 1,
                  levels: "traeger",
                }),
              ],
            },
          ],
        },
      ],
    } satisfies SettingsSchema);
    renderPage();

    expect(
      await screen.findByRole("spinbutton", { name: "Offene Anmeldungen ab" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Dashboard" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(
      screen.queryByRole("switch", { name: "Neuigkeiten im Elternportal" }),
    ).not.toBeInTheDocument();
  });

  it.each([503, 403])(
    "keeps loaded content only for recoverable schema refresh errors (%s)",
    async (status) => {
      mockFetchSchema.mockResolvedValue(schema([NEWS_FROM_TRAEGER]));
      render(
        <ToastProvider>
          <RefreshSchema />
          <TraegerSettingsPage />
        </ToastProvider>,
      );
      expect(
        await screen.findByRole("switch", {
          name: "Neuigkeiten im Elternportal",
        }),
      ).toBeInTheDocument();
      const code =
        status === 503 ? "general.unavailable" : "general.permission";
      mockFetchSchema.mockRejectedValue(
        new ApiError("refused", status, { code }),
      );
      fireEvent.click(screen.getByRole("button", { name: "Refresh schema" }));
      // Anker: the current schema failure is shown after a complete initial load.
      expect(
        await screen.findByText(
          catalogText(code, "die Liste der Einstellungen"),
        ),
      ).toBeInTheDocument();
      if (status === 503)
        expect(
          screen.queryByRole("switch", { name: "Neuigkeiten im Elternportal" }),
        ).toBeInTheDocument();
      else
        expect(
          screen.queryByRole("switch", { name: "Neuigkeiten im Elternportal" }),
        ).not.toBeInTheDocument();
    },
  );

  it("shares one schema request between mounted settings consumers", async () => {
    mockFetchSchema.mockResolvedValue(schema([NEWS_FROM_TRAEGER]));
    render(
      <ToastProvider>
        <TraegerSettingsPage />
        <TraegerSettingsPage />
      </ToastProvider>,
    );
    expect(
      await screen.findAllByText(
        "Vom Träger festgelegt · geändert am 01.09.2026",
      ),
    ).toHaveLength(2);
    expect(mockFetchSchema).toHaveBeenCalledTimes(1);
  });

  it("shows each setting with where its value comes from", async () => {
    mockFetchSchema.mockResolvedValue(
      schema([NEWS_FROM_TRAEGER, STAFF_NAME_DEFAULT]),
    );
    renderPage();

    expect(
      await screen.findByText("Vom Träger festgelegt · geändert am 01.09.2026"),
    ).toBeInTheDocument();
    // Die Erklärung steht hinter dem Info-Knopf am Titel (#5146).
    fireEvent.click(
      screen.getByRole("button", { name: "Erklärung zu Allgemein" }),
    );
    expect(
      screen.getByRole("dialog", { name: "Erklärung zu Allgemein" }),
    ).toHaveTextContent(
      "Hier legen Sie Werte für alle Schulen Ihres Trägers fest. Wo es erlaubt ist, kann eine Schule einen eigenen Wert eintragen.",
    );
    // Tab and category carry the names of the school settings page; the
    // tab is a page tab of the scaffold (#5146).
    expect(screen.getByRole("tab", { name: "Betrieb" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(
      screen.getByRole("heading", { name: "Elternportal" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Standard")).toBeInTheDocument();
  });

  it("saves a Träger value and reads the list again", async () => {
    mockFetchSchema.mockResolvedValue(schema([STAFF_NAME_DEFAULT]));
    renderPage();

    fireEvent.click(
      await screen.findByRole("switch", { name: "Namen des Teams zeigen" }),
    );

    await waitFor(() =>
      expect(mockSave).toHaveBeenCalledWith(
        "parent.message_staff_name_visible",
        true,
      ),
    );
    await waitFor(() => expect(mockFetchSchema).toHaveBeenCalledTimes(2));
  });

  it("removes a Träger value with Zurücksetzen", async () => {
    mockFetchSchema.mockResolvedValue(schema([NEWS_FROM_TRAEGER]));
    renderPage();

    fireEvent.click(
      await screen.findByRole("button", {
        name: "Zurücksetzen",
        description: "Stellt den Wert wieder auf die Grundeinstellung.",
      }),
    );

    await waitFor(() =>
      expect(mockClear).toHaveBeenCalledWith("parent.news_enabled"),
    );
    await waitFor(() => expect(mockFetchSchema).toHaveBeenCalledTimes(2));
  });

  it("shows the values read-only without the right to change them", async () => {
    mockFetchSchema.mockResolvedValue(
      schema([{ ...NEWS_FROM_TRAEGER, writable: false }]),
    );
    renderPage();

    expect(
      await screen.findByText(
        "Sie können die Werte nur ansehen. Ändern kann, wer im Träger-Büro das Recht dazu hat.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Nur Lesen")).toBeInTheDocument();
    expect(
      screen.getByRole("switch", { name: "Neuigkeiten im Elternportal" }),
    ).toBeDisabled();
    expect(
      screen.queryByRole("button", {
        name: "Zurücksetzen",
        description: "Stellt den Wert wieder auf die Grundeinstellung.",
      }),
    ).toBeNull();
  });

  it("shows a failed load where the list is missing and retries", async () => {
    mockFetchSchema
      .mockRejectedValueOnce(
        new ApiError("unavailable", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValueOnce(schema([NEWS_FROM_TRAEGER]));
    renderPage();

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Liste der Einstellungen"),
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    expect(
      await screen.findByText("Neuigkeiten im Elternportal"),
    ).toBeInTheDocument();
    expect(mockFetchSchema).toHaveBeenCalledTimes(2);
  });

  describe("payment text of the letters (#5622)", () => {
    function holidaySchema(text: string, asksDebtorNumber: boolean) {
      return {
        tabs: [
          {
            key: "ferien",
            label: "ferien",
            categories: [
              {
                key: "ferien-bestaetigung",
                label: "ferien-bestaetigung",
                items: [
                  item({
                    key: "parent_billing.debtor_number_on_confirm",
                    label: "Debitorennummer beim Bestätigen abfragen",
                    value: asksDebtorNumber,
                    levels: "traeger",
                  }),
                ],
              },
              {
                key: "ferien-schreiben",
                label: "ferien-schreiben",
                items: [
                  item({
                    key: "parent_billing.payment_text",
                    label: "Zahlungstext im Schreiben",
                    type: "textarea",
                    default: "",
                    value: text,
                    levels: "traeger",
                    validation: { allow_empty: true, max_length: 600 },
                  }),
                ],
              },
            ],
          },
        ],
      } satisfies SettingsSchema;
    }

    it("shows the built-in sentence as example and in the preview", async () => {
      mockFetchSchema.mockResolvedValue(holidaySchema("", true));
      renderPage();

      const field = await screen.findByRole("textbox", {
        name: "Zahlungstext im Schreiben",
      });
      expect(field).toHaveAttribute(
        "placeholder",
        "Bitte überweisen Sie {Betrag} bis zum {Zahlungsfrist} auf das Konto unten. Geben Sie als Verwendungszweck die Rechnungsnummer {Rechnungsnummer} an.",
      );
      // The limit counts characters like the backend, not UTF-16 units.
      expect(field).not.toHaveAttribute("maxlength");
      expect(
        screen.getByRole("heading", { name: "Schreiben an die Familien" }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("figure", { name: "Vorschau des Zahlungstexts" }),
      ).toHaveTextContent(
        "Bitte überweisen Sie 124,00 € bis zum 15.12.2026 auf das Konto unten. Geben Sie als Verwendungszweck Ihre Debitorennummer 10 4711 an.",
      );
    });

    it("previews the Träger's own text with example values", async () => {
      mockFetchSchema.mockResolvedValue(
        holidaySchema(
          "Bitte überweisen Sie {Betrag} bis zum {Zahlungsfrist}. Verwendungszweck: {Rechnungsnummer}, {Debitorennummer}. Haben Sie uns bereits ein SEPA-Lastschriftmandat erteilt, buchen wir den Betrag ab.",
          false,
        ),
      );
      renderPage();

      expect(
        await screen.findByRole("figure", {
          name: "Vorschau des Zahlungstexts",
        }),
      ).toHaveTextContent(
        "Bitte überweisen Sie 124,00 € bis zum 15.12.2026. Verwendungszweck: FE-2026-0007, 10 4711. Haben Sie uns bereits ein SEPA-Lastschriftmandat erteilt, buchen wir den Betrag ab.",
      );
    });

    it("saves 600 characters with an emoji at the end", async () => {
      mockFetchSchema.mockResolvedValue(holidaySchema("", false));
      renderPage();

      const field = await screen.findByRole("textbox", {
        name: "Zahlungstext im Schreiben",
      });
      const text = `{Betrag}${"a".repeat(591)}😀`;
      expect([...text]).toHaveLength(600);
      fireEvent.change(field, { target: { value: text } });
      fireEvent.blur(field);

      await waitFor(() =>
        expect(mockSave).toHaveBeenCalledWith(
          "parent_billing.payment_text",
          text,
        ),
      );
    });

    it("refuses an unknown placeholder at the field before saving", async () => {
      mockFetchSchema.mockResolvedValue(holidaySchema("", false));
      renderPage();

      const field = await screen.findByRole("textbox", {
        name: "Zahlungstext im Schreiben",
      });
      fireEvent.change(field, {
        target: { value: "Bitte überweisen Sie {Betrag} für {Kind}." },
      });
      fireEvent.blur(field);

      const hint = await screen.findByText(
        "„{Kind}“ ist kein Platzhalter. Erlaubt sind {Betrag}, {Zahlungsfrist}, {Rechnungsnummer} und {Debitorennummer}.",
      );
      expect(field).toHaveAttribute("aria-invalid", "true");
      expect(field).toHaveAttribute("aria-describedby", hint.id);
      expect(mockSave).not.toHaveBeenCalled();

      fireEvent.change(field, {
        target: { value: "Bitte überweisen Sie {Betrag}." },
      });
      fireEvent.blur(field);
      await waitFor(() =>
        expect(mockSave).toHaveBeenCalledWith(
          "parent_billing.payment_text",
          "Bitte überweisen Sie {Betrag}.",
        ),
      );
      expect(field).not.toHaveAttribute("aria-invalid");
    });
  });

  it("reads the contact only when its area opens and shows it there (#5685)", async () => {
    mockFetchSchema.mockResolvedValue(schema([item({})]));
    mockFetchMasterData.mockResolvedValue({
      contact: { name: "Petra Lindner", phone: "", email: "" },
      withdrawal: { phone: "", email: "" },
      logo: null,
      bankAccounts: [],
    });
    render(
      <ToastProvider>
        <TraegerSettingsPage />
      </ToastProvider>,
    );

    fireEvent.click(await screen.findByRole("tab", { name: "Kontakt" }));
    expect(await screen.findByLabelText("Name")).toHaveValue("Petra Lindner");
    expect(mockFetchMasterData).toHaveBeenCalledTimes(1);
  });

  it("uses the page loading state while the contact opens (#5685)", async () => {
    const contact = deferred<{
      contact: { name: string; phone: string; email: string };
      withdrawal: { phone: string; email: string };
      logo: null;
      bankAccounts: [];
    }>();
    mockFetchSchema.mockResolvedValue(schema([item({})]));
    mockFetchMasterData.mockReturnValue(contact.promise);
    renderPage();

    fireEvent.click(await screen.findByRole("tab", { name: "Kontakt" }));
    expect(
      await screen.findByRole("status", {
        name: "Der Bereich „Allgemein“ wird geladen…",
      }),
    ).toBeInTheDocument();
    expect(mockFetchMasterData).toHaveBeenCalledTimes(1);

    contact.resolve({
      contact: { name: "Petra Lindner", phone: "", email: "" },
      withdrawal: { phone: "", email: "" },
      logo: null,
      bankAccounts: [],
    });
    expect(await screen.findByLabelText("Name")).toHaveValue("Petra Lindner");
    // Anker: Die Kontaktkarte ist geladen.
    expect(
      screen.queryByRole("status", {
        name: "Der Bereich „Allgemein“ wird geladen…",
      }),
    ).not.toBeInTheDocument();
  });

  it("shows a failed contact load in its area and retries", async () => {
    mockFetchSchema.mockResolvedValue(schema([item({})]));
    mockFetchMasterData
      .mockRejectedValueOnce(
        new ApiError("unavailable", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValue({
        contact: { name: "Petra Lindner", phone: "", email: "" },
        withdrawal: { phone: "", email: "" },
        logo: null,
        bankAccounts: [],
      });
    render(
      <ToastProvider>
        <TraegerSettingsPage />
      </ToastProvider>,
    );

    fireEvent.click(await screen.findByRole("tab", { name: "Kontakt" }));
    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Kontaktdaten"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByLabelText("Name")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(await screen.findByLabelText("Name")).toHaveValue("Petra Lindner");
  });

  it("leaves the branding to the letterhead page (#4653)", async () => {
    mockFetchSchema.mockResolvedValue({
      tabs: [
        ...schema([STAFF_NAME_DEFAULT]).tabs,
        {
          key: "branding",
          label: "branding",
          categories: [
            {
              key: "briefkopf",
              label: "briefkopf",
              items: [
                item({
                  key: "branding.footer_text",
                  label: "Fußzeile",
                  type: "textarea",
                  value: "",
                  levels: "traeger",
                }),
              ],
            },
          ],
        },
      ],
    });
    renderPage();

    expect(
      await screen.findByRole("switch", { name: "Namen des Teams zeigen" }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Fußzeile")).not.toBeInTheDocument();
  });
});

function render(ui: React.ReactElement) {
  return rtlRender(
    <SWRConfig value={{ provider: () => new Map() }}>{ui}</SWRConfig>,
  );
}

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: "A", scope: "org", orgId: 7 } },
    status: "authenticated",
  }),
}));

function RefreshSchema() {
  const refresh = useTenantMutateMatching([traegerKeys.settings()]);
  return (
    <button
      onClick={() => {
        void refresh().catch(() => undefined);
      }}
    >
      Refresh schema
    </button>
  );
}

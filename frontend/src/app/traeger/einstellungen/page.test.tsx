import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type { ResolvedSetting, SettingsSchema } from "~/lib/settings-api";
import { catalogText } from "~/test/error-catalog-text";

const { mockFetchSchema, mockSave, mockClear } = vi.hoisted(() => ({
  mockFetchSchema: vi.fn(),
  mockSave: vi.fn(),
  mockClear: vi.fn(),
}));

vi.mock("~/lib/traeger-api", () => ({
  fetchTraegerSettingsSchema: () => mockFetchSchema(),
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
    mockSave.mockReset().mockResolvedValue(undefined);
    mockClear.mockReset().mockResolvedValue(undefined);
  });

  it("shows each setting with where its value comes from", async () => {
    mockFetchSchema.mockResolvedValue(
      schema([NEWS_FROM_TRAEGER, STAFF_NAME_DEFAULT]),
    );
    renderPage();

    expect(
      await screen.findByText("Vom Träger festgelegt · geändert am 01.09.2026"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Hier legen Sie Werte für alle Schulen Ihres Trägers fest. Wo es erlaubt ist, kann eine Schule einen eigenen Wert eintragen.",
      ),
    ).toBeInTheDocument();
    // Tab and category carry the names of the school settings page.
    expect(
      screen.getByRole("heading", { name: "Betrieb" }),
    ).toBeInTheDocument();
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

    fireEvent.click(await screen.findByTitle("Auf Standard zurücksetzen"));

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
    expect(screen.queryByTitle("Auf Standard zurücksetzen")).toBeNull();
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

  it("leaves the branding to the page Briefkopf (#4653)", async () => {
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

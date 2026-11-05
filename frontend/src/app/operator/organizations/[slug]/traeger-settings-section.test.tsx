vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: {
      user: { id: "operator-1", scope: "operator" },
      expires: "2099-01-01",
    },
    status: "authenticated",
  }),
}));
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { SWRConfig } from "~/lib/swr/testing";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { ResolvedSetting, SettingsSchema } from "~/lib/settings-api";

const { mockFetchSchema, mockSet, mockReset } = vi.hoisted(() => ({
  mockFetchSchema: vi.fn(),
  mockSet: vi.fn(),
  mockReset: vi.fn(),
}));

vi.unmock("swr");

vi.mock("~/lib/operator/operator-settings-api", () => ({
  fetchOperatorTraegerSettingsSchema: (id: string) => mockFetchSchema(id),
  setOperatorTraegerSettingValue: (id: string, key: string, value: unknown) =>
    mockSet(id, key, value),
  resetOperatorTraegerSettingValue: (id: string, key: string) =>
    mockReset(id, key),
}));

import { TraegerSettingsSection } from "./traeger-settings-section";

function item(overrides: Partial<ResolvedSetting>): ResolvedSetting {
  return {
    key: "parent_billing.debtor_number_on_confirm",
    label: "Debitorennummer beim Bestätigen abfragen",
    description: "Beim Bestätigen fragt moto die Debitorennummer ab.",
    type: "boolean",
    default: false,
    value: false,
    is_default: true,
    writable: true,
    visible: true,
    sort_order: 1,
    access_policy: "shared",
    levels: "traeger",
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
        key: "ferien",
        label: "ferien",
        categories: [{ key: "abrechnung", label: "abrechnung", items }],
      },
    ],
  };
}

function renderSection() {
  return render(
    <ToastProvider>
      <SWRConfig value={{ provider: () => new Map(), dedupingInterval: 0 }}>
        <TraegerSettingsSection organizationId="5" />
      </SWRConfig>
    </ToastProvider>,
  );
}

describe("TraegerSettingsSection", () => {
  beforeEach(() => {
    mockFetchSchema.mockReset();
    mockSet.mockReset().mockResolvedValue(undefined);
    mockReset.mockReset().mockResolvedValue(undefined);
  });

  it("changes a Träger value of the chosen Träger and reads the settings again", async () => {
    mockFetchSchema.mockResolvedValueOnce(schema([item({})])).mockResolvedValue(
      schema([
        item({
          value: true,
          is_default: false,
          source: "traeger",
          traeger_changed_at: "2026-11-05T08:00:00Z",
        }),
      ]),
    );
    renderSection();

    const toggle = await screen.findByRole("switch", {
      name: "Debitorennummer beim Bestätigen abfragen",
    });
    expect(mockFetchSchema).toHaveBeenCalledWith("5");
    expect(
      screen.getByText(
        "Gilt für alle Schulen. Die Schulen können den Wert nicht ändern.",
      ),
    ).toBeInTheDocument();

    fireEvent.click(toggle);

    await waitFor(() =>
      expect(mockSet).toHaveBeenCalledWith(
        "5",
        "parent_billing.debtor_number_on_confirm",
        true,
      ),
    );
    expect(
      await screen.findByText("Vom Träger festgelegt · geändert am 05.11.2026"),
    ).toBeInTheDocument();
  });

  it("shows the payment text with example values in the holiday tab", async () => {
    mockFetchSchema.mockResolvedValue(
      schema([
        item({}),
        item({
          key: "parent_billing.payment_text",
          label: "Zahlungstext",
          type: "textarea",
          default: "",
          value: "Bitte überweisen Sie {Betrag}.",
          sort_order: 2,
        }),
      ]),
    );
    renderSection();

    expect(
      await screen.findByRole("figure", { name: "Vorschau des Zahlungstexts" }),
    ).toHaveTextContent(/Bitte überweisen Sie \d/);
  });
});

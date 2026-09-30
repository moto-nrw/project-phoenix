import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { HomeBlockPolicies } from "~/lib/home-blocks";

const BIRTHDAYS_ENABLED_KEY = "birthday.display_enabled";
const BIRTHDAYS_STAFF_KEY = "birthday.display_include_staff";

const mockSavePolicies = vi.fn();
const mockState = {
  value: {
    overrides: {},
    policies: {} as HomeBlockPolicies,
    canManagePolicies: true,
  },
};

vi.mock("~/lib/hooks/use-home-layout", () => ({
  useHomeLayout: () => ({
    state: mockState.value,
    isLoading: false,
    save: vi.fn(),
    reset: vi.fn(),
    savePolicies: (policies: HomeBlockPolicies) =>
      mockSavePolicies(policies) as Promise<void>,
  }),
}));

vi.mock("~/lib/tenant-context", () => ({
  usePresenceMode: () => "detailed",
  useOpenCareGroupMode: () => false,
  useNFCEnabled: () => true,
}));

// Die beiden Geburtstags-Schalter der Schule (#3737) kommen aus dem
// Einstellungs-Schema, Reiter "startseite".
function birthdaySetting(
  key: string,
  label: string,
  value: boolean,
  dependsOn?: { key: string; condition: string; value: unknown },
) {
  return {
    key,
    label,
    description: `${label} (Beschreibung)`,
    type: "boolean" as const,
    default: value,
    value,
    is_default: true,
    writable: true,
    visible: true,
    sort_order: 1,
    access_policy: "shared" as const,
    depends_on: dependsOn,
  };
}
const mockSchema = {
  value: { tabs: [] as unknown[] },
};
function setBirthdaySettings(enabled: boolean, staff: boolean) {
  mockSchema.value = {
    tabs: [
      {
        key: "startseite",
        label: "startseite",
        categories: [
          {
            key: "geburtstage",
            label: "geburtstage",
            items: [
              birthdaySetting(
                BIRTHDAYS_ENABLED_KEY,
                "Geburtstage auf der Startseite",
                enabled,
              ),
              birthdaySetting(
                BIRTHDAYS_STAFF_KEY,
                "Geburtstage von Mitarbeitenden mitanzeigen",
                staff,
                {
                  key: BIRTHDAYS_ENABLED_KEY,
                  condition: "eq",
                  value: true,
                },
              ),
            ],
          },
        ],
      },
    ],
  };
}
const mockRevalidateSchema = vi.fn();
vi.mock("~/lib/hooks/use-settings-schema", () => ({
  useSettingsSchema: () => ({
    data: mockSchema.value,
    isLoading: false,
    mutate: mockRevalidateSchema,
  }),
}));

const mockSetSettingValue = vi.fn();
vi.mock("~/lib/settings-api", () => ({
  setSettingValue: (key: string, value: unknown) =>
    mockSetSettingValue(key, value) as Promise<string | null>,
}));
vi.mock("~/lib/settings-broadcast", () => ({
  notifySettingsChanged: vi.fn(),
}));

const { HomeBlocksTab } = await import("./home-blocks-tab");

describe("HomeBlocksTab", () => {
  beforeEach(() => {
    mockState.value = {
      overrides: {},
      policies: {},
      canManagePolicies: true,
    };
    mockSavePolicies.mockReset().mockImplementation(async (policies) => {
      mockState.value = { ...mockState.value, policies };
    });
    setBirthdaySettings(true, false);
    mockSetSettingValue.mockReset().mockResolvedValue(null);
    mockRevalidateSchema.mockReset().mockResolvedValue(undefined);
  });

  it("sperrt 'Speichern', solange nichts geändert wurde", () => {
    render(<HomeBlocksTab />);

    expect(screen.getByRole("button", { name: "Speichern" })).toBeDisabled();
  });

  it("speichert eine Vorgabe für einen Baustein", async () => {
    render(<HomeBlocksTab />);

    const group = screen.getByRole("group", {
      name: "Vorgabe für Geburtstage",
    });
    fireEvent.click(
      within(group).getByRole("button", { name: "Immer anzeigen" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(mockSavePolicies).toHaveBeenCalledWith({
        "section.birthdays": "required",
      }),
    );
    expect(screen.getByRole("button", { name: "Speichern" })).toBeDisabled();
    expect(screen.getByText("Gespeichert.")).toBeInTheDocument();
  });

  it("speichert 'Frei wählbar' nicht mit", async () => {
    // "Die Schule hat keine Meinung" ist der Normalfall. Würde er gespeichert,
    // wäre eine spätere Änderung des Standards nicht mehr von einer bewussten
    // Entscheidung zu unterscheiden.
    mockState.value = {
      overrides: {},
      policies: { "section.birthdays": "disabled" },
      canManagePolicies: true,
    };
    render(<HomeBlocksTab />);

    const group = screen.getByRole("group", {
      name: "Vorgabe für Geburtstage",
    });
    fireEvent.click(
      within(group).getByRole("button", { name: "Frei wählbar" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => expect(mockSavePolicies).toHaveBeenCalledWith({}));
  });

  it("meldet einen fehlgeschlagenen Speicherversuch", async () => {
    mockSavePolicies.mockRejectedValue(new Error("boom"));
    render(<HomeBlocksTab />);

    const group = screen.getByRole("group", {
      name: "Vorgabe für Geburtstage",
    });
    fireEvent.click(within(group).getByRole("button", { name: "Aus" }));
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(/Das Speichern hat nicht geklappt/),
    ).toBeInTheDocument();
  });

  it("bietet Bausteine, die es im Betriebsmodus nicht gibt, nicht an", () => {
    render(<HomeBlocksTab />);

    // NFC und Räume sind hier an, die offene Betreuung aus — also gibt es
    // Auslastung, aber keinen Grund, etwas Unsichtbares vorzugeben.
    expect(screen.getByText("Auslastung")).toBeInTheDocument();
    expect(screen.getByText("Laufende Betreuung")).toBeInTheDocument();
  });

  it("zeigt die Geburtstags-Schalter direkt an der Geburtstagskarte", () => {
    render(<HomeBlocksTab />);

    expect(
      screen.getByRole("switch", { name: "Geburtstage auf der Startseite" }),
    ).toHaveAttribute("aria-checked", "true");
    expect(
      screen.getByRole("switch", {
        name: "Geburtstage von Mitarbeitenden mitanzeigen",
      }),
    ).toHaveAttribute("aria-checked", "false");
  });

  it("speichert einen geänderten Geburtstags-Schalter mit 'Speichern'", async () => {
    render(<HomeBlocksTab />);

    fireEvent.click(
      screen.getByRole("switch", {
        name: "Geburtstage von Mitarbeitenden mitanzeigen",
      }),
    );
    expect(mockSetSettingValue).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(mockSetSettingValue).toHaveBeenCalledWith(
        BIRTHDAYS_STAFF_KEY,
        true,
      ),
    );
    expect(mockSetSettingValue).toHaveBeenCalledTimes(1);
    expect(mockSavePolicies).not.toHaveBeenCalled();
    expect(mockRevalidateSchema).toHaveBeenCalled();
  });

  it("blendet Vorgabe und Personal-Schalter aus, solange Geburtstage aus sind", () => {
    setBirthdaySettings(false, false);
    render(<HomeBlocksTab />);

    expect(
      screen.getByRole("switch", { name: "Geburtstage auf der Startseite" }),
    ).toHaveAttribute("aria-checked", "false");
    expect(
      screen.queryByRole("switch", {
        name: "Geburtstage von Mitarbeitenden mitanzeigen",
      }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("group", { name: "Vorgabe für Geburtstage" }),
    ).not.toBeInTheDocument();
  });

  it("meldet einen fehlgeschlagenen Schalter und bleibt ungespeichert", async () => {
    mockSetSettingValue.mockResolvedValue(
      "Einstellung konnte nicht gespeichert werden",
    );
    render(<HomeBlocksTab />);

    fireEvent.click(
      screen.getByRole("switch", { name: "Geburtstage auf der Startseite" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(/Das Speichern hat nicht geklappt/),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Speichern" })).toBeEnabled();
  });
});

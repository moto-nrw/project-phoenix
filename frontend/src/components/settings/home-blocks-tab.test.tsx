import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { HomeBlockPolicies } from "~/lib/home-blocks";

const mockSavePolicies = vi.fn();
const mockState = {
  value: {
    overrides: {},
    policies: {} as HomeBlockPolicies,
    canManagePolicies: true,
  },
};

vi.mock("~/lib/home-layout-api", () => ({
  HOME_LAYOUT_SWR_KEY: "home-layout",
  saveHomeBlockPolicies: (policies: HomeBlockPolicies) =>
    mockSavePolicies(policies) as Promise<void>,
}));

vi.mock("~/lib/hooks/use-home-layout", () => ({
  useHomeLayout: () => ({
    state: mockState.value,
    isLoading: false,
    save: vi.fn(),
    reset: vi.fn(),
  }),
}));

vi.mock("~/lib/tenant-context", () => ({
  usePresenceMode: () => "detailed",
  useOpenCareGroupMode: () => false,
  useNFCEnabled: () => true,
}));

const { HomeBlocksTab } = await import("./home-blocks-tab");

describe("HomeBlocksTab", () => {
  beforeEach(() => {
    mockSavePolicies.mockReset().mockResolvedValue(undefined);
    mockState.value = {
      overrides: {},
      policies: {},
      canManagePolicies: true,
    };
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
    expect(screen.getByText("Aktive Gruppen")).toBeInTheDocument();
  });
});

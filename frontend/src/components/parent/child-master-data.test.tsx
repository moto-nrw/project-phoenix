import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import type React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ChildMasterDataView } from "./child-master-data";
import {
  getChildFeatures,
  getChildMasterData,
  submitMasterDataRequest,
  updateMasterDataField,
  type ChildFeatures,
  type ChildMasterData,
} from "~/lib/parent-api";

vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
}));

vi.mock("next/link", () => ({
  default: ({
    href,
    children,
    ...props
  }: {
    href: string;
    children: React.ReactNode;
  }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

vi.mock("~/lib/parent-api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/parent-api")>();
  return {
    ...actual,
    getChildFeatures: vi.fn(),
    getChildMasterData: vi.fn(),
    submitMasterDataRequest: vi.fn(),
    updateMasterDataField: vi.fn(),
  };
});

const mockGetFeatures = vi.mocked(getChildFeatures);
const mockGetMasterData = vi.mocked(getChildMasterData);
const mockSubmit = vi.mocked(submitMasterDataRequest);
const mockUpdateField = vi.mocked(updateMasterDataField);

function features(overrides: Partial<ChildFeatures> = {}): ChildFeatures {
  return {
    sick_note_enabled: true,
    notes_enabled: true,
    pickup_change_enabled: true,
    related_accounts_invite_enabled: true,
    related_accounts_remove_enabled: true,
    master_data_edit_enabled: true,
    master_data_contact_edit_enabled: true,
    master_data_request_enabled: true,
    ...overrides,
  };
}

function masterData(overrides: Partial<ChildMasterData> = {}): ChildMasterData {
  return {
    student_id: "42",
    first_name: "Lara",
    last_name: "Beispiel",
    birthday: "2018-03-04",
    school_class: "2a",
    status: "active",
    health_info: "Allergie",
    guardian_profile_id: "77",
    email: "parent@example.test",
    address_street: "Musterweg 1",
    address_city: "Köln",
    address_postal_code: "50667",
    preferred_contact_method: "email",
    language_preference: "de",
    primary_phone: "+491234",
    allowed_departure_modes: {
      mon: ["pickup"],
      tue: ["bus", "alone"],
      wed: [],
    },
    pending_changes: [],
    ...overrides,
  };
}

describe("ChildMasterDataView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetFeatures.mockResolvedValue(features());
    mockGetMasterData.mockResolvedValue(masterData());
    mockUpdateField.mockResolvedValue(masterData({ health_info: "Neue Info" }));
    mockSubmit.mockResolvedValue([]);
  });

  it("loads and renders editable master data with departure summary", async () => {
    render(<ChildMasterDataView studentId="42" />);

    expect(
      await screen.findByRole("heading", { name: "Stammdaten" }),
    ).toBeInTheDocument();
    expect(screen.getByDisplayValue("Lara")).toBeInTheDocument();
    expect(screen.getByDisplayValue("Allergie")).toBeInTheDocument();
    expect(screen.getAllByText("Wird abgeholt").length).toBeGreaterThan(0);
    expect(screen.getByText("Bus, Geht allein")).toBeInTheDocument();
    expect(screen.getAllByText("Keine Angabe").length).toBeGreaterThan(0);
    expect(screen.getByRole("link", { name: /Zurück/ })).toHaveAttribute(
      "href",
      "/parents/children/42",
    );
  });

  it("auto-saves direct-edit fields on blur", async () => {
    render(<ChildMasterDataView studentId="42" />);

    const health = await screen.findByDisplayValue("Allergie");
    fireEvent.change(health, { target: { value: "Neue Info" } });
    fireEvent.blur(health);

    await waitFor(() =>
      expect(mockUpdateField).toHaveBeenCalledWith(
        "42",
        "student",
        "health_info",
        "Neue Info",
      ),
    );
    expect(await screen.findByText("Gespeichert")).toBeInTheDocument();
  });

  it("submits approval-required identity changes and refreshes pending state", async () => {
    mockGetMasterData.mockResolvedValueOnce(masterData()).mockResolvedValueOnce(
      masterData({
        pending_changes: [
          {
            id: "900",
            target: "person",
            field_key: "first_name",
            old_value: "Lara",
            new_value: "Lea",
            status: "pending",
            created_at: "2026-06-24T12:00:00Z",
          },
        ],
      }),
    );

    render(<ChildMasterDataView studentId="42" />);

    const firstName = await screen.findByDisplayValue("Lara");
    const identityHeading = screen.getByRole("heading", {
      name: "Angaben zum Kind",
    });
    const identitySection = identityHeading.closest("section");
    if (!identitySection) {
      throw new Error("identity section not found");
    }

    fireEvent.change(firstName, { target: { value: "Lea" } });
    fireEvent.click(
      within(identitySection).getByRole("button", {
        name: "Änderung anfragen",
      }),
    );

    await waitFor(() =>
      expect(mockSubmit).toHaveBeenCalledWith("42", [
        { target: "person", field_key: "first_name", value: "Lea" },
      ]),
    );
    expect(
      await screen.findByText(
        "Anfrage gesendet. Das Team prüft Ihre Änderung.",
      ),
    ).toBeInTheDocument();
    expect(await screen.findByText("In Prüfung")).toBeInTheDocument();
    expect(await screen.findByDisplayValue("Lara")).toBeDisabled();

    fireEvent.click(
      within(identitySection).getByRole("button", {
        name: "Änderung anfragen",
      }),
    );
    expect(mockSubmit).toHaveBeenCalledTimes(1);
  });

  it("submits departure mode changes for review", async () => {
    mockGetMasterData.mockResolvedValueOnce(masterData()).mockResolvedValueOnce(
      masterData({
        pending_changes: [
          {
            id: "901",
            target: "departure",
            field_key: "allowed_departure_modes",
            old_value: { mon: ["pickup"], tue: ["bus", "alone"] },
            new_value: {
              mon: ["pickup"],
              tue: ["bus", "alone"],
              wed: ["pickup"],
            },
            status: "pending",
            created_at: "2026-06-24T12:00:00Z",
          },
        ],
      }),
    );

    render(<ChildMasterDataView studentId="42" />);

    const departureSection = await screen.findByRole("heading", {
      name: "Dauerhafte Gehzeiten",
    });
    const section = departureSection.closest("section");
    if (!section) {
      throw new Error("departure section not found");
    }

    fireEvent.click(screen.getByLabelText("Mi Wird abgeholt"));
    fireEvent.click(
      within(section).getByRole("button", { name: "Änderung anfragen" }),
    );

    await waitFor(() =>
      expect(mockSubmit).toHaveBeenCalledWith("42", [
        {
          target: "departure",
          field_key: "allowed_departure_modes",
          value: {
            mon: ["pickup"],
            tue: ["alone", "bus"],
            wed: ["pickup"],
          },
        },
      ]),
    );
  });

  it("does not offer accompanied departure requests without companion-note support", async () => {
    mockGetMasterData.mockResolvedValue(
      masterData({
        allowed_departure_modes: {
          mon: ["accompanied"],
        },
      }),
    );

    render(<ChildMasterDataView studentId="42" />);

    const departureSection = await screen.findByRole("heading", {
      name: "Dauerhafte Gehzeiten",
    });
    const section = departureSection.closest("section");
    if (!section) {
      throw new Error("departure section not found");
    }

    expect(
      screen.queryByLabelText("Mo Mit anderem Kind"),
    ).not.toBeInTheDocument();
    expect(screen.getByLabelText("Mo Geht allein")).toBeDisabled();
    expect(
      within(section).getByRole("button", { name: "Änderung anfragen" }),
    ).toBeDisabled();
  });

  it("shows disabled messaging when feature flags are off", async () => {
    mockGetFeatures.mockResolvedValue(
      features({
        master_data_edit_enabled: false,
        master_data_contact_edit_enabled: false,
        master_data_request_enabled: false,
      }),
    );

    render(<ChildMasterDataView studentId="42" />);

    await screen.findAllByText(
      "Änderungsanfragen sind bei dieser OGS deaktiviert.",
    );
    expect(
      screen.getByText(
        "Das Bearbeiten der Stammdaten ist bei dieser OGS deaktiviert.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByDisplayValue("Allergie")).toBeDisabled();
  });

  it("keeps health editable but disables contact fields when contact edits are off", async () => {
    mockGetFeatures.mockResolvedValue(
      features({ master_data_contact_edit_enabled: false }),
    );

    render(<ChildMasterDataView studentId="42" />);

    expect(await screen.findByDisplayValue("Allergie")).not.toBeDisabled();
    expect(screen.getByDisplayValue("parent@example.test")).toBeDisabled();
    expect(screen.getByDisplayValue("+491234")).toBeDisabled();
    expect(screen.getByDisplayValue("Musterweg 1")).toBeDisabled();
    expect(
      screen.getByText(
        "Das Bearbeiten der Stammdaten ist bei dieser OGS deaktiviert.",
      ),
    ).toBeInTheDocument();
  });

  it("renders a load error if either request fails", async () => {
    mockGetMasterData.mockRejectedValue(new Error("boom"));

    render(<ChildMasterDataView studentId="42" />);

    expect(
      await screen.findByText(
        "Die Stammdaten konnten nicht geladen werden. Bitte aktualisieren Sie die Seite.",
      ),
    ).toBeInTheDocument();
  });

  it("shows direct-save and request-submit errors", async () => {
    mockUpdateField.mockRejectedValueOnce(new Error("write failed"));
    mockSubmit.mockRejectedValueOnce(new Error("request failed"));

    render(<ChildMasterDataView studentId="42" />);

    const health = await screen.findByDisplayValue("Allergie");
    fireEvent.change(health, { target: { value: "Neue Info" } });
    fireEvent.blur(health);
    expect(
      await screen.findByText("Die Änderung konnte nicht gespeichert werden."),
    ).toBeInTheDocument();

    fireEvent.change(screen.getByDisplayValue("Lara"), {
      target: { value: "Lea" },
    });
    const identityHeading = screen.getByRole("heading", {
      name: "Angaben zum Kind",
    });
    const identitySection = identityHeading.closest("section");
    if (!identitySection) {
      throw new Error("identity section not found");
    }
    fireEvent.click(
      within(identitySection).getByRole("button", {
        name: "Änderung anfragen",
      }),
    );

    expect(
      await screen.findByText("Die Anfrage konnte nicht gesendet werden."),
    ).toBeInTheDocument();
  });
});

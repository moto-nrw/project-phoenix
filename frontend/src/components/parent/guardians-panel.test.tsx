import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import GuardiansPanel from "./guardians-panel";
import type { ChildGuardian } from "~/lib/parent-api";

const mocks = vi.hoisted(() => ({
  listChildGuardians: vi.fn(),
  updateGuardianContact: vi.fn(),
  updateGuardianRelationship: vi.fn(),
}));

vi.mock("~/lib/parent-api", () => {
  class ParentApiError extends Error {
    readonly status: number;
    readonly code?: string;

    constructor(message: string, status: number, code?: string) {
      super(message);
      this.name = "ParentApiError";
      this.status = status;
      this.code = code;
    }
  }

  return {
    ParentApiError,
    listChildGuardians: mocks.listChildGuardians,
    updateGuardianContact: mocks.updateGuardianContact,
    updateGuardianRelationship: mocks.updateGuardianRelationship,
  };
});

const editableGuardian: ChildGuardian = {
  guardian_profile_id: "7",
  student_guardian_id: "70",
  first_name: "Helga",
  last_name: "Schneider",
  email: "helga@example.test",
  phones: [
    {
      phone_number: "0211 111111",
      phone_type: "home",
      label: "Oma Zuhause",
      is_primary: false,
    },
    {
      phone_number: "0151 222222",
      phone_type: "mobile",
      label: "Oma Handy",
      is_primary: true,
    },
  ],
  address_street: "Hauptstr. 1",
  address_city: "Düsseldorf",
  address_postal_code: "40210",
  relationship_type: "relative",
  is_primary: false,
  is_emergency_contact: true,
  can_pickup: true,
  pickup_notes: "Nur freitags",
  emergency_priority: 2,
  has_account: false,
  is_self: false,
  can_edit_contact: true,
  can_manage_pickup: true,
  contact_locked_own_account: false,
  contact_locked_shared: false,
};

describe("GuardiansPanel", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.listChildGuardians.mockResolvedValue([editableGuardian]);
    mocks.updateGuardianContact.mockResolvedValue(editableGuardian);
    mocks.updateGuardianRelationship.mockResolvedValue(editableGuardian);
  });

  it("preserves phone labels and the existing primary phone on contact save", async () => {
    render(<GuardiansPanel studentId="42" />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Kontakt bearbeiten" }),
    );
    const emailInput = document.querySelector<HTMLInputElement>(
      'input[type="email"]',
    );
    expect(emailInput).toBeTruthy();
    fireEvent.change(emailInput!, { target: { value: "neu@example.test" } });

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(mocks.updateGuardianContact).toHaveBeenCalledTimes(1);
    });
    expect(mocks.updateGuardianContact).toHaveBeenCalledWith(
      "42",
      "7",
      expect.objectContaining({
        email: "neu@example.test",
        phones: [
          {
            phone_number: "0211 111111",
            phone_type: "home",
            label: "Oma Zuhause",
            is_primary: false,
          },
          {
            phone_number: "0151 222222",
            phone_type: "mobile",
            label: "Oma Handy",
            is_primary: true,
          },
        ],
      }),
    );
  });

  it("maps parent API error codes to localized modal errors", async () => {
    const { ParentApiError } = await import("~/lib/parent-api");
    mocks.updateGuardianContact.mockRejectedValue(
      new ParentApiError(
        "parent: guardian with own portal account cannot be edited by another parent",
        403,
        "guardian_has_own_account",
      ),
    );

    render(<GuardiansPanel studentId="42" />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Kontakt bearbeiten" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(
        "Diese Person verwaltet ihre Kontaktdaten über ein eigenes Elternkonto.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/parent: guardian/)).not.toBeInTheDocument();
  });

  it("shows the own-account hint only when contact_locked_own_account is set", async () => {
    const lockedGuardian: ChildGuardian = {
      ...editableGuardian,
      guardian_profile_id: "8",
      student_guardian_id: "80",
      first_name: "Mehmet",
      last_name: "Yilmaz",
      has_account: true,
      can_edit_contact: false,
      can_manage_pickup: false,
      contact_locked_own_account: true,
    };
    mocks.listChildGuardians.mockResolvedValue([
      editableGuardian,
      lockedGuardian,
    ]);

    render(<GuardiansPanel studentId="42" />);

    // The locked guardian shows the explanation and no edit affordance.
    expect(
      await screen.findByText(
        "Diese Person pflegt ihre Kontaktdaten über ihr eigenes Konto selbst.",
      ),
    ).toBeInTheDocument();
    // The editable guardian still exposes the edit button and no hint duplicate.
    expect(
      screen.getByRole("button", { name: "Kontakt bearbeiten" }),
    ).toBeInTheDocument();
    expect(
      screen.getAllByText(
        "Diese Person pflegt ihre Kontaktdaten über ihr eigenes Konto selbst.",
      ),
    ).toHaveLength(1);
  });
});

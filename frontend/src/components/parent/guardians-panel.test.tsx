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
  has_account: false,
  is_self: false,
  can_edit_contact: true,
  can_manage_pickup: true,
  contact_locked_own_account: false,
  contact_locked_shared: false,
  contact_locked_social_worker: false,
  contact_locked_full_guardian: false,
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

    fireEvent.click(await screen.findByRole("button", { name: "Bearbeiten" }));
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

    fireEvent.click(await screen.findByRole("button", { name: "Bearbeiten" }));
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(
        "Diese Person verwaltet ihre Kontaktdaten über ein eigenes Elternkonto.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/parent: guardian/)).not.toBeInTheDocument();
  });

  it("reaches the pickup-note action without exposing pickup flags for an edit-only guardian", async () => {
    // can_edit_contact without can_manage_pickup (e.g. the caller's own row):
    // the backend permits a pickup_notes edit but not the safety-critical flags.
    // The relationship action must be reachable for the note, while the
    // can_pickup / emergency controls stay hidden.
    const noteOnlyGuardian: ChildGuardian = {
      ...editableGuardian,
      // Flags off so the row renders no pickup/emergency badges — keeps the
      // "controls hidden" assertions scoped to the modal (the badge labels are
      // identical strings to the checkbox labels).
      can_pickup: false,
      is_emergency_contact: false,
      can_edit_contact: true,
      can_manage_pickup: false,
    };
    mocks.listChildGuardians.mockResolvedValue([noteOnlyGuardian]);

    render(<GuardiansPanel studentId="42" />);

    // The note action is reachable (labelled as the pickup note, not "manage").
    fireEvent.click(
      await screen.findByRole("button", { name: "Hinweis zur Abholung" }),
    );

    // The note field is present; the flag controls are not.
    expect(document.querySelector("#pickup-notes")).toBeTruthy();
    expect(screen.queryByText("Darf abholen")).not.toBeInTheDocument();
    expect(screen.queryByText("Notfallkontakt")).not.toBeInTheDocument();

    const notes = document.querySelector<HTMLTextAreaElement>("#pickup-notes")!;
    fireEvent.change(notes, { target: { value: "Kommt mit dem Bus" } });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(mocks.updateGuardianRelationship).toHaveBeenCalledTimes(1);
    });
    // Only the note travels — no flag fields, so the pickup.manage gate is not tripped.
    expect(mocks.updateGuardianRelationship).toHaveBeenCalledWith("42", "7", {
      pickup_notes: "Kommt mit dem Bus",
    });
  });

  it("offers no pickup-note action for a contact-locked guardian", async () => {
    // Option B: the pickup note follows contact editability. A guardian whose
    // contact is locked (account holder) exposes neither the contact pencil nor
    // the pickup-note action; the backend would reject a note edit there too.
    const lockedGuardian: ChildGuardian = {
      ...editableGuardian,
      guardian_profile_id: "9",
      student_guardian_id: "90",
      first_name: "Onkel",
      last_name: "Ali",
      can_pickup: false,
      is_emergency_contact: false,
      can_edit_contact: false,
      can_manage_pickup: false,
      has_account: true,
      contact_locked_own_account: true,
    };
    mocks.listChildGuardians.mockResolvedValue([lockedGuardian]);

    render(<GuardiansPanel studentId="42" />);

    expect(await screen.findByText("Onkel Ali")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Bearbeiten" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Hinweis zur Abholung" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Abholrecht verwalten" }),
    ).not.toBeInTheDocument();
  });

  it("sends an empty string (not null) when an existing pickup note is cleared", async () => {
    // Finding #1: clearing a note must send "" so the backend stores NULL. A
    // null would unmarshal to a nil *string the backend treats as "unchanged".
    const noteEditableGuardian: ChildGuardian = {
      ...editableGuardian,
      can_pickup: false,
      is_emergency_contact: false,
      can_manage_pickup: false,
      pickup_notes: "Nur freitags",
    };
    mocks.listChildGuardians.mockResolvedValue([noteEditableGuardian]);

    render(<GuardiansPanel studentId="42" />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Hinweis zur Abholung" }),
    );

    const notes = document.querySelector<HTMLTextAreaElement>("#pickup-notes")!;
    expect(notes.value).toBe("Nur freitags");
    fireEvent.change(notes, { target: { value: "   " } });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(mocks.updateGuardianRelationship).toHaveBeenCalledTimes(1);
    });
    expect(mocks.updateGuardianRelationship).toHaveBeenCalledWith("42", "7", {
      pickup_notes: "",
    });
  });

  it("toggles a pickup flag without sending a phantom note for a manage-only guardian", async () => {
    // Finding #1 (review pass 3): a pickup-manage-only caller (no contact edit)
    // sees the flags but not the note textarea. If legacy/staff pickup_notes has
    // surrounding whitespace, the note must NOT travel — gating on
    // can_edit_contact (and trimming the original) keeps a flag-only edit from
    // tripping the guardian.edit gate and failing an otherwise valid toggle.
    const manageOnlyGuardian: ChildGuardian = {
      ...editableGuardian,
      guardian_profile_id: "11",
      student_guardian_id: "110",
      first_name: "Opa",
      last_name: "Klein",
      can_pickup: false,
      is_emergency_contact: false,
      can_edit_contact: false,
      can_manage_pickup: true,
      // Surrounding whitespace: the hidden, untouched note would "differ" from
      // its trimmed self and leak into the payload without the fix.
      pickup_notes: "  Kommt mit dem Bus  ",
    };
    mocks.listChildGuardians.mockResolvedValue([manageOnlyGuardian]);

    render(<GuardiansPanel studentId="42" />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Abholrecht verwalten" }),
    );

    // The flag controls are present; the note textarea is not.
    expect(screen.getByText("Darf abholen")).toBeInTheDocument();
    expect(document.querySelector("#pickup-notes")).toBeFalsy();

    const canPickup = document.querySelector<HTMLInputElement>(
      "#guardian-can-pickup",
    )!;
    fireEvent.click(canPickup);
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(mocks.updateGuardianRelationship).toHaveBeenCalledTimes(1);
    });
    // Only the flag travels — no pickup_notes, so the guardian.edit gate the
    // backend applies to notes is never tripped.
    expect(mocks.updateGuardianRelationship).toHaveBeenCalledWith("42", "11", {
      can_pickup: true,
    });
  });

  it("offers no edit affordance for a redacted account-holder guardian", async () => {
    // A guardian with their own account is redacted (no contact data) and
    // read-only to other parents — the row lists the name but no edit button.
    const lockedGuardian: ChildGuardian = {
      ...editableGuardian,
      guardian_profile_id: "8",
      student_guardian_id: "80",
      first_name: "Mehmet",
      last_name: "Yilmaz",
      email: undefined,
      phones: [],
      address_street: undefined,
      address_city: undefined,
      address_postal_code: undefined,
      pickup_notes: undefined,
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

    // The redacted guardian is still listed by name.
    expect(await screen.findByText("Mehmet Yilmaz")).toBeInTheDocument();
    // Only the editable guardian exposes an edit button.
    expect(screen.getAllByRole("button", { name: "Bearbeiten" })).toHaveLength(
      1,
    );
  });
});

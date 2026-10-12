import "@testing-library/jest-dom/vitest";
import {
  fireEvent,
  render as renderPlain,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import type React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import GuardiansPanel from "./guardians-panel";
import { ToastProvider } from "~/contexts/ToastContext";
import { catalogText } from "~/test/error-catalog-text";
import {
  ParentApiError,
  type ChildGuardian,
  type RelatedAccount,
} from "~/lib/parent-api";

// The shared error path reports row actions through the toast provider
// (#2518).
function render(ui: React.ReactElement) {
  return renderPlain(ui, { wrapper: ToastProvider });
}

const mocks = vi.hoisted(() => ({
  listChildGuardians: vi.fn(),
  listRelatedAccounts: vi.fn(),
  createGuardianContact: vi.fn(),
  inviteRelatedAccount: vi.fn(),
  removeRelatedAccount: vi.fn(),
  updateGuardianContact: vi.fn(),
  updateGuardianRelationship: vi.fn(),
}));

vi.mock("~/lib/parent-api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/parent-api")>();
  return {
    ParentApiError: actual.ParentApiError,
    listChildGuardians: mocks.listChildGuardians,
    listRelatedAccounts: mocks.listRelatedAccounts,
    createGuardianContact: mocks.createGuardianContact,
    inviteRelatedAccount: mocks.inviteRelatedAccount,
    removeRelatedAccount: mocks.removeRelatedAccount,
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
    mocks.listRelatedAccounts.mockResolvedValue([]);
    mocks.updateGuardianContact.mockResolvedValue(editableGuardian);
    mocks.updateGuardianRelationship.mockResolvedValue(editableGuardian);
    mocks.createGuardianContact.mockResolvedValue(editableGuardian);
  });

  it("preserves phone labels and the existing primary phone on contact save", async () => {
    render(
      <GuardiansPanel studentId="42" canInvite={false} canRemove={false} />,
    );

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

  it("explains access in a dialog and sends the invitation", async () => {
    mocks.inviteRelatedAccount.mockResolvedValue({
      outcome: "invited",
      guardian_profile_id: "8",
    });

    render(<GuardiansPanel studentId="42" canInvite canRemove={false} />);

    expect(await screen.findByText("Für die OGS sichtbar")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Einladen" }));

    const dialog = screen.getByRole("dialog", { name: "App-Zugang geben" });
    expect(
      within(dialog).getByText(
        "Hat die Person bereits ein Konto, wird es verbunden. Andernfalls erhält sie eine Einladung per E-Mail. Je nach Einstellung prüft die OGS Ihre Anfrage zuerst.",
      ),
    ).toBeInTheDocument();

    fireEvent.change(
      within(dialog).getByLabelText(
        "E-Mail-Adresse der Person, die Sie einladen möchten",
      ),
      { target: { value: "person@example.test" } },
    );
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Einladung senden" }),
    );

    await waitFor(() => {
      expect(mocks.inviteRelatedAccount).toHaveBeenCalledWith(
        "42",
        "person@example.test",
        undefined,
      );
    });
    expect(
      await screen.findByText("Einladung an person@example.test gesendet."),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("dialog", { name: "App-Zugang geben" }),
    ).not.toBeInTheDocument();
  });

  it("keeps invitation errors inside the dialog", async () => {
    mocks.inviteRelatedAccount.mockRejectedValue(
      new ParentApiError("diag", 503, "general.unavailable"),
    );

    render(<GuardiansPanel studentId="42" canInvite canRemove={false} />);

    fireEvent.click(await screen.findByRole("button", { name: "Einladen" }));
    const dialog = screen.getByRole("dialog", { name: "App-Zugang geben" });
    fireEvent.change(
      within(dialog).getByLabelText(
        "E-Mail-Adresse der Person, die Sie einladen möchten",
      ),
      { target: { value: "person@example.test" } },
    );
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Einladung senden" }),
    );

    expect(
      await within(dialog).findByText(
        catalogText("general.unavailable", "die Einladung"),
      ),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByRole("button", { name: "Wiederholen" }),
    ).toBeInTheDocument();
  });

  it("adds a pickup contact without inviting an account", async () => {
    render(
      <GuardiansPanel
        studentId="42"
        canInvite
        canRemove={false}
        canAddContact
        canManagePickup
      />,
    );

    fireEvent.click(
      await screen.findByRole("button", {
        name: /Kontakt oder Abholperson hinzufügen/,
      }),
    );

    const dialog = screen.getByRole("dialog", { name: "Kontakt hinzufügen" });
    fireEvent.change(within(dialog).getByLabelText("Vorname"), {
      target: { value: "Erika" },
    });
    fireEvent.change(within(dialog).getByLabelText("Nachname"), {
      target: { value: "Klein" },
    });
    fireEvent.click(
      within(dialog).getByRole("checkbox", { name: /Darf abholen/ }),
    );
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Kontakt hinzufügen" }),
    );

    await waitFor(() => {
      expect(mocks.createGuardianContact).toHaveBeenCalledWith(
        "42",
        expect.objectContaining({
          first_name: "Erika",
          last_name: "Klein",
          relationship_type: "relative",
          can_pickup: true,
          is_emergency_contact: false,
        }),
      );
    });
    expect(mocks.inviteRelatedAccount).not.toHaveBeenCalled();
  });

  it("lists only active or invited accounts in connected accounts", async () => {
    const relatedAccounts: RelatedAccount[] = [
      {
        guardian_profile_id: "21",
        first_name: "Jürgen",
        last_name: "Schulze",
        email: "juergen@example.test",
        relationship_type: "parent",
        is_primary: true,
        status: "active",
        is_self: true,
      },
      {
        guardian_profile_id: "22",
        first_name: "Magdalena",
        last_name: "Schulze",
        email: "magdalena@example.test",
        relationship_type: "relative",
        is_primary: false,
        status: "no_account",
        is_self: false,
      },
      {
        guardian_profile_id: "23",
        first_name: "Klaus",
        last_name: "Schulze",
        email: "klaus@example.test",
        relationship_type: "parent",
        is_primary: false,
        status: "active_no_access",
        is_self: false,
      },
      {
        guardian_profile_id: "24",
        first_name: "Petra",
        last_name: "Schulze",
        email: "petra@example.test",
        relationship_type: "parent",
        is_primary: false,
        status: "pending",
        is_self: false,
      },
    ];
    mocks.listRelatedAccounts.mockResolvedValue(relatedAccounts);

    render(
      <GuardiansPanel studentId="42" canInvite={false} canRemove={false} />,
    );

    expect(
      await screen.findByRole("heading", { name: "Verbundene Konten" }),
    ).toBeInTheDocument();
    expect(screen.getByText("juergen@example.test")).toBeInTheDocument();
    expect(screen.getByText("petra@example.test")).toBeInTheDocument();
    expect(
      screen.queryByText("magdalena@example.test"),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("klaus@example.test")).not.toBeInTheDocument();
  });

  it("separates contacts and accounts with the standard section spacing", async () => {
    mocks.listRelatedAccounts.mockResolvedValue([
      {
        guardian_profile_id: "7",
        first_name: "Helga",
        last_name: "Schneider",
        email: "helga@example.test",
        relationship_type: "relative",
        is_primary: false,
        status: "active",
        is_self: false,
      },
    ] satisfies RelatedAccount[]);

    render(
      <GuardiansPanel studentId="42" canInvite={false} canRemove={false} />,
    );

    const contacts = await screen.findByRole("heading", {
      name: "Abholberechtigte und Kontakte",
    });
    const accounts = screen.getByRole("heading", {
      name: "Verbundene Konten",
    });
    expect(
      contacts
        .closest("section")
        ?.querySelector('[data-parent-tour="child-guardians"]'),
    ).toBeInTheDocument();
    expect(contacts.closest("section")?.parentElement).toBe(
      accounts.closest("section")?.parentElement,
    );
    expect(contacts.closest("section")?.parentElement).toHaveClass("space-y-5");
  });

  it("shows a parent API error code in the dialog via the shared catalog", async () => {
    mocks.updateGuardianContact.mockRejectedValue(
      new ParentApiError(
        "parent: guardian with own portal account cannot be edited by another parent",
        403,
        "care.guardian_has_own_account",
      ),
    );

    render(
      <GuardiansPanel studentId="42" canInvite={false} canRemove={false} />,
    );

    fireEvent.click(await screen.findByRole("button", { name: "Bearbeiten" }));
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    const dialog = screen.getByRole("dialog");
    expect(
      await within(dialog).findByText(
        catalogText("care.guardian_has_own_account", "die Änderung am Kontakt"),
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

    render(
      <GuardiansPanel studentId="42" canInvite={false} canRemove={false} />,
    );

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

    render(
      <GuardiansPanel studentId="42" canInvite={false} canRemove={false} />,
    );

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

    render(
      <GuardiansPanel studentId="42" canInvite={false} canRemove={false} />,
    );

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

    render(
      <GuardiansPanel studentId="42" canInvite={false} canRemove={false} />,
    );

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

    render(
      <GuardiansPanel studentId="42" canInvite={false} canRemove={false} />,
    );

    // The redacted guardian is still listed by name.
    expect(await screen.findByText("Mehmet Yilmaz")).toBeInTheDocument();
    // Only the editable guardian exposes an edit button.
    expect(screen.getAllByRole("button", { name: "Bearbeiten" })).toHaveLength(
      1,
    );
  });

  // #2518: each section reports its own failed load in place, with a retry,
  // and never as an empty list.
  it("shows load errors where the lists are missing and retries them", async () => {
    mocks.listChildGuardians
      .mockRejectedValueOnce(
        new ParentApiError("diag", 503, "general.unavailable"),
      )
      .mockResolvedValue([editableGuardian]);
    mocks.listRelatedAccounts
      .mockRejectedValueOnce(
        new ParentApiError("diag", 503, "general.unavailable"),
      )
      .mockResolvedValue([]);

    render(
      <GuardiansPanel studentId="42" canInvite={false} canRemove={false} />,
    );

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Liste der Kontakte"),
      ),
    ).toBeInTheDocument();
    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Liste der verbundenen Konten"),
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Noch keine Konten verbunden."),
    ).not.toBeInTheDocument();
    expect(screen.queryByText(/diag/)).not.toBeInTheDocument();

    fireEvent.click(screen.getAllByRole("button", { name: "Wiederholen" })[0]!);

    expect(
      await screen.findByRole("button", { name: "Bearbeiten" }),
    ).toBeInTheDocument();
    expect(mocks.listChildGuardians).toHaveBeenCalledTimes(2);
  });

  it("reports a failed access removal through the shared toast", async () => {
    mocks.listRelatedAccounts.mockResolvedValue([
      {
        guardian_profile_id: "24",
        first_name: "Petra",
        last_name: "Schulze",
        email: "petra@example.test",
        relationship_type: "parent",
        is_primary: false,
        status: "active",
        is_self: false,
      },
    ]);
    mocks.removeRelatedAccount.mockRejectedValue(
      new ParentApiError("diag", 403, "care.remove_disabled"),
    );

    render(<GuardiansPanel studentId="42" canInvite={false} canRemove />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Zugang entziehen" }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Ja, Zugang entziehen" }),
    );

    expect(
      await screen.findByText(
        catalogText("care.remove_disabled", "die Freigabe für dieses Konto"),
      ),
    ).toBeInTheDocument();
  });

  it("checks the name before saving and keeps a pickup save error in its dialog", async () => {
    mocks.updateGuardianRelationship.mockRejectedValue(
      new ParentApiError("diag", 403, "care.pickup_change_disabled"),
    );

    render(
      <GuardiansPanel studentId="42" canInvite={false} canRemove={false} />,
    );

    fireEvent.click(await screen.findByRole("button", { name: "Bearbeiten" }));
    const contactDialog = screen.getByRole("dialog");
    fireEvent.change(within(contactDialog).getByLabelText("Vorname"), {
      target: { value: "  " },
    });
    fireEvent.click(
      within(contactDialog).getByRole("button", { name: "Speichern" }),
    );
    expect(
      await within(contactDialog).findByText(
        "Vor- und Nachname sind erforderlich.",
      ),
    ).toBeInTheDocument();
    expect(mocks.updateGuardianContact).not.toHaveBeenCalled();
    fireEvent.click(
      within(contactDialog).getByRole("button", { name: "Abbrechen" }),
    );

    fireEvent.click(
      await screen.findByRole("button", { name: "Abholrecht verwalten" }),
    );
    const pickupDialog = screen.getByRole("dialog");
    fireEvent.click(
      pickupDialog.querySelector<HTMLInputElement>("#guardian-can-pickup")!,
    );
    fireEvent.click(
      within(pickupDialog).getByRole("button", { name: "Speichern" }),
    );
    expect(
      await within(pickupDialog).findByText(
        catalogText("care.pickup_change_disabled", "die Abholregelung"),
      ),
    ).toBeInTheDocument();
  });
});

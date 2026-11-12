import { render, screen, fireEvent } from "@testing-library/react";
import { afterEach, describe, it, expect, vi } from "vitest";
import GuardianList from "./guardian-list";

// Bearbeiten liegt im Kebab der Karte (Bauart 1 Regel 4).
function rowMenuTriggers() {
  return screen.getAllByRole("button", { name: /^Aktionen für/ });
}
function closeRowMenu() {
  fireEvent.keyDown(document, { key: "Escape" });
}

vi.mock("@/lib/guardian-helpers", () => ({
  getGuardianFullName: (g: { firstName?: string; lastName?: string }) =>
    `${g.firstName ?? ""} ${g.lastName ?? ""}`.trim(),
  getRelationshipTypeLabel: (type: string) => {
    const labels: Record<string, string> = {
      mother: "Mutter",
      father: "Vater",
      guardian: "Vormund",
    };
    return labels[type] ?? type;
  },
  getLanguageLabel: (code: string) => {
    const labels: Record<string, string> = {
      de: "Deutsch",
      en: "English",
      tr: "Türkisch",
      ar: "Arabisch",
    };
    return labels[code] ?? code.toUpperCase();
  },
  PHONE_TYPE_LABELS: {
    mobile: "Mobil",
    home: "Telefon",
    work: "Dienstlich",
    other: "Sonstige",
  },
}));

vi.mock("./guardian-contact-actions", () => ({
  GuardianContactActions: ({
    email,
    phone,
    phoneNumbers,
    contactName,
  }: {
    email?: string;
    phone?: string;
    phoneNumbers?: Array<{ number: string }>;
    contactName: string;
  }) => (
    <div data-testid="contact-actions">
      <span data-testid="contact-email">{email}</span>
      <span data-testid="contact-phone">{phone}</span>
      <span data-testid="contact-phone-count">{phoneNumbers?.length ?? 0}</span>
      <span data-testid="contact-name">{contactName}</span>
    </div>
  ),
  buildGuardianMailtoHref: (email: string, contactName?: string) =>
    `mailto:${email}?subject=${encodeURIComponent(
      contactName ? `Betreff: ${contactName}` : "Kontaktanfrage",
    )}`,
  buildGuardianTelHref: (phoneNumber: string) =>
    `tel:${phoneNumber.replaceAll(/\s+/g, "")}`,
}));

const mockGuardians = [
  {
    id: "1",
    firstName: "Anna",
    lastName: "Müller",
    email: "anna@example.com",
    preferredContactMethod: "email",
    languagePreference: "de",
    hasAccount: false,
    phoneNumbers: [
      {
        id: "phone-1",
        phoneNumber: "01234567890",
        phoneType: "home" as const,
        isPrimary: true,
        priority: 1,
      },
      {
        id: "phone-2",
        phoneNumber: "01234567891",
        phoneType: "mobile" as const,
        isPrimary: false,
        priority: 2,
      },
    ],
    relationshipId: "rel-1",
    relationshipType: "mother",
    isPrimary: true,
    isEmergencyContact: true,
    canPickup: true,
    emergencyPriority: 1,
  },
  {
    id: "2",
    firstName: "Hans",
    lastName: "Müller",
    email: "hans@example.com",
    preferredContactMethod: "phone",
    languagePreference: "de",
    hasAccount: false,
    phoneNumbers: [],
    relationshipId: "rel-2",
    relationshipType: "father",
    isPrimary: false,
    isEmergencyContact: false,
    canPickup: true,
    emergencyPriority: 2,
  },
];

describe("GuardianList", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  // The visitor of the public demo takes the name of a seed guardian, who
  // keeps the seed address; the demo shows no seed address (#5033).
  it("hides a seed address in the demo build", () => {
    vi.stubEnv("NEXT_PUBLIC_APP_ENV", "demo");
    const seeded = {
      ...mockGuardians[0]!,
      firstName: "Christian",
      lastName: "Wolf",
      email: "sabine.schneider@demo-ogs-nord.moto-ogs.de",
    };
    render(<GuardianList guardians={[seeded]} onInvite={vi.fn()} />);

    expect(screen.getAllByText("Christian Wolf").length).toBeGreaterThan(0);
    expect(screen.getByTestId("contact-email")).toBeEmptyDOMElement();
    expect(
      screen.queryByText("sabine.schneider@demo-ogs-nord.moto-ogs.de"),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("Nicht angegeben")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /^Aktionen für/ }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText("Für die Einladung fehlt die E-Mail-Adresse."),
    ).not.toBeInTheDocument();
  });

  it("renders empty state when no guardians", () => {
    render(<GuardianList guardians={[]} />);

    expect(
      screen.getByText("Keine Erziehungsberechtigten zugewiesen"),
    ).toBeInTheDocument();
  });

  it("renders guardian names", () => {
    render(<GuardianList guardians={mockGuardians} />);

    expect(screen.getAllByText("Anna Müller").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Hans Müller").length).toBeGreaterThan(0);
  });

  it("shows primary badge for primary guardian", () => {
    render(<GuardianList guardians={mockGuardians} />);

    // Multiple "Primär" badges may exist (guardian primary badge + phone primary indicator)
    expect(screen.getAllByText("Primär").length).toBeGreaterThan(0);
  });

  it("shows relationship type when showRelationship is true", () => {
    render(<GuardianList guardians={mockGuardians} showRelationship={true} />);

    expect(screen.getByText("Mutter")).toBeInTheDocument();
    expect(screen.getByText("Vater")).toBeInTheDocument();
  });

  it("hides relationship type when showRelationship is false", () => {
    render(<GuardianList guardians={mockGuardians} showRelationship={false} />);

    // Anker: die Karten der beiden Personen sind gerendert.
    expect(screen.getAllByText("Anna Müller").length).toBeGreaterThan(0);
    expect(screen.queryByText("Mutter")).not.toBeInTheDocument();
    expect(screen.queryByText("Vater")).not.toBeInTheDocument();
  });

  it("displays contact information", () => {
    render(<GuardianList guardians={mockGuardians} />);

    expect(screen.getAllByText("anna@example.com").length).toBeGreaterThan(0);
    expect(screen.getAllByText("01234567890").length).toBeGreaterThan(0);
    expect(screen.getAllByText("01234567891").length).toBeGreaterThan(0);
  });

  it("shows 'Nicht angegeben' for missing phone", () => {
    render(<GuardianList guardians={mockGuardians} />);

    // Hans has no phone, should show Nicht angegeben
    expect(screen.getAllByText("Nicht angegeben").length).toBeGreaterThan(0);
  });

  it("shows emergency contact indicator", () => {
    render(<GuardianList guardians={mockGuardians} showRelationship={true} />);

    expect(screen.getByText("Notfallkontakt")).toBeInTheDocument();
  });

  it("renders edit buttons when not readOnly", () => {
    const onEdit = vi.fn();
    render(<GuardianList guardians={mockGuardians} onEdit={onEdit} />);

    // Zeilenaktionen liegen im Kebab der Karte (Bauart 1 Regel 4): jede
    // Karte hat ein Menü, jedes Menü bietet Bearbeiten an.
    const triggers = rowMenuTriggers();
    expect(triggers.length).toBe(2);
    for (const trigger of triggers) {
      fireEvent.click(trigger);
      expect(
        screen.getByRole("menuitem", { name: "Bearbeiten" }),
      ).toBeInTheDocument();
      closeRowMenu();
    }
  });

  it("hides action buttons when readOnly", () => {
    render(
      <GuardianList
        guardians={mockGuardians}
        onEdit={vi.fn()}
        readOnly={true}
      />,
    );

    // Anker: die Karten der beiden Personen sind gerendert.
    expect(screen.getAllByText("Anna Müller").length).toBeGreaterThan(0);
    expect(
      screen.queryByRole("button", { name: /^Aktionen für/ }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("Bearbeiten")).not.toBeInTheDocument();
  });

  it("calls onEdit when edit button clicked", () => {
    const onEdit = vi.fn();
    render(<GuardianList guardians={mockGuardians} onEdit={onEdit} />);

    fireEvent.click(rowMenuTriggers()[0]!);
    fireEvent.click(screen.getByRole("menuitem", { name: "Bearbeiten" }));

    expect(onEdit).toHaveBeenCalledWith(mockGuardians[0]);
  });

  it("renders GuardianContactActions for each guardian", () => {
    render(<GuardianList guardians={mockGuardians} />);

    const contactActions = screen.getAllByTestId("contact-actions");
    expect(contactActions.length).toBe(2);
  });

  it("passes every guardian phone option to contact actions", () => {
    render(<GuardianList guardians={mockGuardians} />);

    const phoneCounts = screen.getAllByTestId("contact-phone-count");
    expect(phoneCounts[0]).toHaveTextContent("2");
    expect(phoneCounts[1]).toHaveTextContent("0");
  });

  it("shows the honest status and grant-access action for active_no_access", () => {
    const onInvite = vi.fn();
    const guardian = {
      ...mockGuardians[1]!,
      hasAccount: true,
      accountStatus: "active_no_access" as const,
      guardianRole: "emergency_contact" as const,
    };
    render(<GuardianList guardians={[guardian]} onInvite={onInvite} />);

    expect(
      screen.getByText("Konto aktiv, kein Portalzugriff"),
    ).toBeInTheDocument();
    fireEvent.click(rowMenuTriggers()[0]!);
    fireEvent.click(screen.getByRole("menuitem", { name: "Zugriff gewähren" }));
    expect(onInvite).toHaveBeenCalledWith(guardian);
  });

  it("offers Einladen in the row menu for a guardian without an account", () => {
    const guardian = {
      ...mockGuardians[1]!,
      accountStatus: "none" as const,
    };
    render(<GuardianList guardians={[guardian]} onInvite={vi.fn()} />);

    fireEvent.click(rowMenuTriggers()[0]!);
    expect(screen.getByText("Einladen")).toBeInTheDocument();
  });

  it("hides the invite action entirely for active accounts", () => {
    const guardian = {
      ...mockGuardians[1]!,
      hasAccount: true,
      accountStatus: "active" as const,
    };
    render(<GuardianList guardians={[guardian]} onInvite={vi.fn()} />);

    // Anker: die Karte zeigt den Kontostatus, an dem die Aktion hängt.
    expect(screen.getByText("Konto aktiv")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /^Aktionen für/ }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("Einladen")).not.toBeInTheDocument();
    expect(screen.queryByText("Zugriff gewähren")).not.toBeInTheDocument();
  });

  it("offers re-invite in the row menu for a pending guardian without an account", () => {
    const guardian = {
      ...mockGuardians[1]!,
      accountStatus: "pending" as const,
    };
    render(
      <GuardianList
        guardians={[guardian]}
        onInvite={vi.fn()}
        onEdit={vi.fn()}
      />,
    );

    expect(screen.getByText("Einladung offen")).toBeInTheDocument();
    fireEvent.click(rowMenuTriggers()[0]!);
    expect(
      screen.getByRole("menuitem", { name: "Erneut einladen" }),
    ).toBeInTheDocument();
  });

  // #6294: a link that no longer works must not read as an open invitation.
  it("marks a dead invitation link and offers a new invitation", () => {
    const onInvite = vi.fn();
    const guardian = {
      ...mockGuardians[1]!,
      accountStatus: "link_invalid" as const,
    };
    render(
      <GuardianList
        guardians={[guardian]}
        onInvite={onInvite}
        onEdit={vi.fn()}
      />,
    );

    expect(screen.getByText("Link gilt nicht mehr")).toBeInTheDocument();
    expect(screen.queryByText("Einladung offen")).not.toBeInTheDocument();
    expect(
      screen.getByText(
        "Der Einladungslink gilt nicht mehr. Mit „Einladung neu senden“ bekommt die Person einen neuen Link.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(rowMenuTriggers()[0]!);
    fireEvent.click(
      screen.getByRole("menuitem", { name: "Einladung neu senden" }),
    );
    expect(onInvite).toHaveBeenCalledWith(guardian);
  });

  it("explains a dead invitation link without an action when read-only", () => {
    const guardian = {
      ...mockGuardians[1]!,
      accountStatus: "link_invalid" as const,
    };
    render(<GuardianList guardians={[guardian]} readOnly />);

    expect(screen.getByText("Link gilt nicht mehr")).toBeInTheDocument();
    expect(
      screen.getByText("Der Einladungslink gilt nicht mehr."),
    ).toBeInTheDocument();
  });

  it("shows a disabled sending state in the row menu while inviting", () => {
    const onInvite = vi.fn();
    const guardian = {
      ...mockGuardians[1]!,
      accountStatus: "pending" as const,
    };
    render(
      <GuardianList
        guardians={[guardian]}
        onInvite={onInvite}
        invitingGuardianId={guardian.id}
      />,
    );

    fireEvent.click(rowMenuTriggers()[0]!);
    const inviteItem = screen.getByRole("menuitem", {
      name: "Wird eingeladen",
    });
    expect(inviteItem).toBeDisabled();
    fireEvent.click(inviteItem);
    expect(onInvite).not.toHaveBeenCalled();
  });

  it("explains why a person without an email address cannot be invited", () => {
    const guardian = {
      ...mockGuardians[1]!,
      email: undefined,
      accountStatus: "none" as const,
    };
    render(
      <GuardianList
        guardians={[guardian]}
        onInvite={vi.fn()}
        onEdit={vi.fn()}
      />,
    );

    expect(
      screen.getByText("Für die Einladung fehlt die E-Mail-Adresse."),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Einladen" }),
    ).not.toBeInTheDocument();
    expect(rowMenuTriggers()).toHaveLength(1);
  });

  it("hides the invite action while an account holder's upgrade approval is pending", () => {
    const guardian = {
      ...mockGuardians[1]!,
      hasAccount: true,
      accountStatus: "pending" as const,
      guardianRole: "emergency_contact" as const,
    };
    render(<GuardianList guardians={[guardian]} onInvite={vi.fn()} />);

    expect(screen.getByText("Einladung offen")).toBeInTheDocument();
    // Ohne Einladen und ohne Bearbeiten gibt es keinen Eintrag, also auch
    // kein Menü (Bauart 2 Regel 7).
    expect(
      screen.queryByRole("button", { name: /^Aktionen für/ }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("Erneut einladen")).not.toBeInTheDocument();
    expect(screen.queryByText("Zugriff gewähren")).not.toBeInTheDocument();
  });

  it("hides the invite action for school-managed social-worker contacts", () => {
    const guardian = {
      ...mockGuardians[1]!,
      accountStatus: "active_no_access" as const,
      guardianRole: "social_worker" as const,
    };
    render(<GuardianList guardians={[guardian]} onInvite={vi.fn()} />);

    // Anker: dieselbe Statuszeile wie beim Notfallkontakt mit Zugriff gewähren.
    expect(
      screen.getByText("Konto aktiv, kein Portalzugriff"),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /^Aktionen für/ }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("Zugriff gewähren")).not.toBeInTheDocument();
    expect(screen.queryByText("Einladen")).not.toBeInTheDocument();
  });
});

describe("GuardianList mail state (#5934)", () => {
  const guardian = {
    id: "7",
    firstName: "Ada",
    lastName: "Muster",
    email: "ada@example.com",
    relationshipId: "1",
    relationshipType: "mother",
    isPrimary: false,
    isEmergencyContact: false,
    canPickup: false,
    emergencyPriority: 1,
    accountStatus: "pending",
  } as unknown as Parameters<typeof GuardianList>[0]["guardians"][number];

  function openMenu() {
    fireEvent.click(rowMenuTriggers()[0]!);
  }

  it("never reads an accepted mail as delivered", () => {
    render(
      <GuardianList
        guardians={[guardian]}
        emailStates={{ "7": { mailOutcome: "accepted" } }}
        onInvite={() => undefined}
      />,
    );
    expect(screen.getByText("Versendet")).toBeInTheDocument();
    expect(screen.queryByText("Zugestellt")).not.toBeInTheDocument();
  });

  it("shows a delivery only after the report", () => {
    render(
      <GuardianList
        guardians={[guardian]}
        emailStates={{ "7": { mailOutcome: "delivered" } }}
      />,
    );
    expect(screen.getByText("Zugestellt")).toBeInTheDocument();
  });

  it("explains a blocked address, its next step and offers no invitation", () => {
    render(
      <GuardianList
        guardians={[guardian]}
        emailStates={{
          "7": {
            mailOutcome: "blocked",
            addressMark: "failed",
            sendBlock: "address",
          },
        }}
        onInvite={() => undefined}
        onEdit={() => undefined}
        onLiftEmailBlock={() => undefined}
      />,
    );
    expect(screen.getByText("Adresse prüfen")).toBeInTheDocument();
    expect(screen.getByText(/Sie ist deshalb gesperrt/)).toBeInTheDocument();
    expect(screen.getByText(/„Sperre aufheben“/)).toBeInTheDocument();
    openMenu();
    expect(
      screen.getByRole("menuitem", { name: "Sperre aufheben" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("menuitem", { name: "Bearbeiten" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("menuitem", { name: "Erneut einladen" }),
    ).not.toBeInTheDocument();
  });

  it("offers no lift for a complaint", () => {
    render(
      <GuardianList
        guardians={[guardian]}
        emailStates={{
          "7": { addressMark: "complained", sendBlock: "address" },
        }}
        onEdit={() => undefined}
        onLiftEmailBlock={() => undefined}
      />,
    );
    expect(screen.getByText(/als unerwünscht gemeldet/)).toBeInTheDocument();
    openMenu();
    expect(
      screen.getByRole("menuitem", { name: "Bearbeiten" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("menuitem", { name: "Sperre aufheben" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("menuitem", { name: "Mails wieder senden" }),
    ).not.toBeInTheDocument();
  });

  it("blocks a new invitation while the mail is delayed and says why", () => {
    render(
      <GuardianList
        guardians={[guardian]}
        emailStates={{ "7": { mailOutcome: "delayed", sendBlock: "delayed" } }}
        onInvite={() => undefined}
        onEdit={() => undefined}
      />,
    );
    expect(screen.getByText("Verzögert")).toBeInTheDocument();
    expect(screen.getByText(/nicht noch einmal/)).toBeInTheDocument();
    openMenu();
    expect(
      screen.getByRole("menuitem", { name: "Bearbeiten" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("menuitem", { name: "Erneut einladen" }),
    ).not.toBeInTheDocument();
  });

  it("lets the school stop mail and take the stop back", () => {
    const onStop = vi.fn();
    const { rerender } = render(
      <GuardianList
        guardians={[guardian]}
        emailStates={{}}
        onStopEmails={onStop}
      />,
    );
    openMenu();
    fireEvent.click(screen.getByRole("menuitem", { name: "Mails stoppen" }));
    expect(onStop).toHaveBeenCalledWith(guardian);

    rerender(
      <GuardianList
        guardians={[guardian]}
        emailStates={{
          "7": { emailsStopped: true, sendBlock: "stopped" },
        }}
        onResumeEmails={() => undefined}
      />,
    );
    expect(screen.getByText("Versand gestoppt")).toBeInTheDocument();
    expect(screen.getByText(/Anmeldecodes/)).toBeInTheDocument();
    expect(screen.queryByText("Adresse prüfen")).not.toBeInTheDocument();
  });

  it("offers no invitation while the provider holds the address and names the support", () => {
    render(
      <GuardianList
        guardians={[guardian]}
        emailStates={{
          "7": {
            mailOutcome: "provider_blocked",
            providerBlock: "active",
            sendBlock: "provider",
          },
        }}
        onInvite={() => undefined}
      />,
    );
    expect(screen.getByText("Gesperrt")).toBeInTheDocument();
    expect(screen.getByText(/moto-Support/)).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Aktionen für/ }),
    ).not.toBeInTheDocument();
  });

  it("says when a stopped person's address still gets mail", () => {
    render(
      <GuardianList
        guardians={[guardian]}
        emailStates={{
          "7": { emailsStopped: true, stopShared: true, sendBlock: "stopped" },
        }}
      />,
    );
    expect(screen.getByText("Versand gestoppt")).toBeInTheDocument();
    expect(screen.getByText(/gehen deshalb weiter raus/)).toBeInTheDocument();
  });

  it("shows a bounce and a stop side by side, each with its own step", () => {
    const onLift = vi.fn();
    const onResume = vi.fn();
    render(
      <GuardianList
        guardians={[guardian]}
        emailStates={{
          "7": {
            addressMark: "failed",
            emailsStopped: true,
            sendBlock: "stopped",
          },
        }}
        onLiftEmailBlock={onLift}
        onResumeEmails={onResume}
        onStopEmails={() => undefined}
      />,
    );
    expect(screen.getByText("Adresse prüfen")).toBeInTheDocument();
    expect(screen.getByText("Versand gestoppt")).toBeInTheDocument();
    openMenu();
    fireEvent.click(screen.getByRole("menuitem", { name: "Sperre aufheben" }));
    expect(onLift).toHaveBeenCalledWith(guardian);
    openMenu();
    fireEvent.click(
      screen.getByRole("menuitem", { name: "Mails wieder senden" }),
    );
    expect(onResume).toHaveBeenCalledWith(guardian);
  });
});

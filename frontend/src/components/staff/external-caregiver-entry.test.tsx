import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";

const { createExternal, inviteExternal } = vi.hoisted(() => ({
  createExternal: vi.fn(),
  inviteExternal: vi.fn(),
}));

vi.mock("~/lib/staff-api", () => ({
  staffService: { createExternal, inviteExternal },
}));

import { ExternalCaregiverEntry } from "./external-caregiver-entry";

function openEntry(): void {
  fireEvent.click(
    screen.getByRole("button", { name: "Externe Person eintragen" }),
  );
}

function fillEntry(
  first: string,
  last: string,
  email = "lea@example.test",
  organization = "",
): void {
  fireEvent.change(screen.getByLabelText("Vorname"), {
    target: { value: first },
  });
  fireEvent.change(screen.getByLabelText("Nachname"), {
    target: { value: last },
  });
  fireEvent.change(screen.getByLabelText("E-Mail-Adresse"), {
    target: { value: email },
  });
  fireEvent.change(screen.getByLabelText("Organisation (freiwillig)"), {
    target: { value: organization },
  });
}

function submit(): void {
  fireEvent.click(screen.getByRole("button", { name: "Eintragen" }));
}

describe("ExternalCaregiverEntry", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    createExternal.mockResolvedValue({
      id: "88",
      name: "Lea Gast",
      firstName: "Lea",
      lastName: "Gast",
      isExternal: true,
      externalOrganization: "Musikschule",
      externalAccountStatus: "invitation_pending",
    });
    inviteExternal.mockResolvedValue({
      id: "5",
      name: "Lea Gast",
      firstName: "Lea",
      lastName: "Gast",
      isExternal: true,
      externalAccountStatus: "invitation_pending",
    });
  });

  it("records and invites an external person and hands the new entry back", async () => {
    const onAdded = vi.fn();
    render(<ExternalCaregiverEntry existing={[]} onAdded={onAdded} />);

    openEntry();
    fillEntry(" Lea ", "Gast", " lea@example.test ", "Musikschule");
    submit();

    await waitFor(() =>
      expect(onAdded).toHaveBeenCalledWith({
        kind: "created",
        staff: expect.objectContaining({ id: "88" }),
      }),
    );
    expect(createExternal).toHaveBeenCalledWith({
      firstName: "Lea",
      lastName: "Gast",
      email: "lea@example.test",
      organization: "Musikschule",
    });
    expect(
      screen.getByRole("button", { name: "Externe Person eintragen" }),
    ).toBeInTheDocument();
  });

  it("tells the user that the person gets an email invitation", () => {
    render(<ExternalCaregiverEntry existing={[]} onAdded={vi.fn()} />);

    openEntry();

    expect(
      screen.getByText(/Die Person bekommt eine Einladung per E-Mail\./),
    ).toBeInTheDocument();
  });

  it("asks for both names before sending anything", () => {
    const onAdded = vi.fn();
    render(<ExternalCaregiverEntry existing={[]} onAdded={onAdded} />);

    openEntry();
    fillEntry("Lea", "  ");
    submit();

    expect(
      screen.getByText("Bitte geben Sie Vor- und Nachnamen ein."),
    ).toBeInTheDocument();
    expect(screen.getByText("Bitte Nachnamen eingeben.")).toBeInTheDocument();
    expect(createExternal).not.toHaveBeenCalled();
    expect(onAdded).not.toHaveBeenCalled();
  });

  it("requires the email address and marks the field", () => {
    const onAdded = vi.fn();
    render(<ExternalCaregiverEntry existing={[]} onAdded={onAdded} />);

    openEntry();
    fillEntry("Lea", "Gast", "  ");
    submit();

    const email = screen.getByLabelText("E-Mail-Adresse");
    expect(
      screen.getByText("Bitte prüfen Sie die E-Mail-Adresse."),
    ).toBeInTheDocument();
    expect(email).toHaveAttribute("aria-invalid", "true");
    expect(email).toHaveAccessibleDescription("Bitte E-Mail-Adresse eingeben.");
    expect(email).toHaveFocus();
    expect(createExternal).not.toHaveBeenCalled();
    expect(onAdded).not.toHaveBeenCalled();
  });

  it("rejects an address that is not one", () => {
    render(<ExternalCaregiverEntry existing={[]} onAdded={vi.fn()} />);

    openEntry();
    fillEntry("Lea", "Gast", "lea-at-example");
    submit();

    expect(screen.getByLabelText("E-Mail-Adresse")).toHaveAccessibleDescription(
      "Bitte eine gültige E-Mail-Adresse eingeben.",
    );
    expect(createExternal).not.toHaveBeenCalled();
  });

  it("puts the server's email field error on the email field", async () => {
    createExternal.mockRejectedValueOnce(
      new ApiError("bad email", 400, {
        code: "general.input",
        errors: [{ field: "email", reason: "invalid" }],
      }),
    );
    render(<ExternalCaregiverEntry existing={[]} onAdded={vi.fn()} />);

    openEntry();
    fillEntry("Lea", "Gast");
    submit();

    const email = screen.getByLabelText("E-Mail-Adresse");
    await waitFor(() => expect(email).toHaveAttribute("aria-invalid", "true"));
    expect(screen.getByLabelText("Vorname")).not.toHaveAttribute(
      "aria-invalid",
    );
  });

  it("marks the email field when the address already has access", async () => {
    createExternal.mockRejectedValueOnce(
      new ApiError("conflict", 409, {
        code: "identity.account_already_has_tenant_access",
      }),
    );
    render(<ExternalCaregiverEntry existing={[]} onAdded={vi.fn()} />);

    openEntry();
    fillEntry("Lea", "Gast");
    submit();

    const email = screen.getByLabelText("E-Mail-Adresse");
    await waitFor(() =>
      expect(email).toHaveAccessibleDescription(
        "Bitte eine andere E-Mail-Adresse eingeben.",
      ),
    );
    expect(email).toHaveAttribute("aria-invalid", "true");
  });

  it("selects an invited person who is already in the list instead of adding a twin", () => {
    const onAdded = vi.fn();
    render(
      <ExternalCaregiverEntry
        existing={[
          {
            id: "5",
            fullName: "Lea Gast",
            isExternal: true,
            accountStatus: "invitation_pending",
          },
        ]}
        onAdded={onAdded}
      />,
    );

    openEntry();
    fillEntry("  lea ", "GAST");
    submit();

    expect(onAdded).toHaveBeenCalledWith({ kind: "existing", id: "5" });
    expect(createExternal).not.toHaveBeenCalled();
    expect(inviteExternal).not.toHaveBeenCalled();
  });

  it("invites a recorded person without an account instead of adding a twin", async () => {
    const onAdded = vi.fn();
    render(
      <ExternalCaregiverEntry
        existing={[
          {
            id: "5",
            fullName: "Lea Gast",
            isExternal: true,
            accountStatus: "none",
          },
        ]}
        onAdded={onAdded}
      />,
    );

    openEntry();
    fillEntry("Lea", "Gast", "lea@example.test");
    submit();

    await waitFor(() =>
      expect(onAdded).toHaveBeenCalledWith({
        kind: "invited",
        staff: expect.objectContaining({
          id: "5",
          externalAccountStatus: "invitation_pending",
        }),
      }),
    );
    expect(inviteExternal).toHaveBeenCalledWith("5", "lea@example.test");
    expect(createExternal).not.toHaveBeenCalled();
  });

  it("does not select an internal person with the same name", async () => {
    const onAdded = vi.fn();
    render(
      <ExternalCaregiverEntry
        existing={[{ id: "5", fullName: "Lea Gast", isExternal: false }]}
        onAdded={onAdded}
      />,
    );

    openEntry();
    fillEntry("Lea", "Gast");
    submit();

    await waitFor(() => expect(createExternal).toHaveBeenCalledOnce());
    expect(onAdded).toHaveBeenCalledWith({
      kind: "created",
      staff: expect.objectContaining({ id: "88" }),
    });
  });

  it("keeps Enter and Escape inside the panel when the dialog is a form", async () => {
    const onSubmit = vi.fn((event: { preventDefault: () => void }) =>
      event.preventDefault(),
    );
    const onAdded = vi.fn();
    render(
      <form onSubmit={onSubmit}>
        <ExternalCaregiverEntry existing={[]} onAdded={onAdded} />
      </form>,
    );

    openEntry();
    fillEntry("Lea", "Gast");
    fireEvent.keyDown(screen.getByLabelText("Nachname"), { key: "Enter" });
    await waitFor(() => expect(onAdded).toHaveBeenCalledOnce());
    expect(onSubmit).not.toHaveBeenCalled();

    openEntry();
    fireEvent.keyDown(screen.getByLabelText("Vorname"), { key: "Escape" });
    // Anker: das Feld war offen, Escape schließt nur das Feld.
    expect(
      screen.getByRole("button", { name: "Externe Person eintragen" }),
    ).toBeInTheDocument();
    expect(screen.queryByLabelText("Vorname")).not.toBeInTheDocument();
  });
});

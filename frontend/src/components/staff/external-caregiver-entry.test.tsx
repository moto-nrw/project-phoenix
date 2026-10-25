import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

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

function chooseAccess(label: RegExp): void {
  fireEvent.click(screen.getByRole("radio", { name: label }));
}

/**
 * Fills the names and, unless the access stays „Kein Zugang“, picks
 * „Eigene Termine betreuen“ and types the address.
 */
function fillEntry(
  first: string,
  last: string,
  email: string | null = "lea@example.test",
  organization = "",
): void {
  fireEvent.change(screen.getByLabelText("Vorname"), {
    target: { value: first },
  });
  fireEvent.change(screen.getByLabelText("Nachname"), {
    target: { value: last },
  });
  fireEvent.change(screen.getByLabelText("Organisation (freiwillig)"), {
    target: { value: organization },
  });
  if (email === null) return;
  chooseAccess(/Eigene Termine betreuen/);
  fireEvent.change(screen.getByLabelText("E-Mail-Adresse"), {
    target: { value: email },
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
    render(<ExternalCaregiverEntry onAdded={onAdded} />);

    openEntry();
    fillEntry(" Lea ", "Gast", " lea@example.test ", "Musikschule");
    submit();

    await waitFor(() =>
      expect(onAdded).toHaveBeenCalledWith(
        expect.objectContaining({ id: "88" }),
      ),
    );
    expect(createExternal).toHaveBeenCalledWith({
      firstName: "Lea",
      lastName: "Gast",
      email: "lea@example.test",
      organization: "Musikschule",
      access: "supervise",
    });
    expect(
      screen.getByRole("button", { name: "Externe Person eintragen" }),
    ).toBeInTheDocument();
  });

  // ADR 0091: three access levels, „Kein Zugang“ by default.
  it("offers the three access levels, each with what the person can do", () => {
    render(<ExternalCaregiverEntry onAdded={vi.fn()} canGrantView />);

    openEntry();

    const none = screen.getByRole("radio", { name: /Kein Zugang/ });
    expect(none).toBeChecked();
    expect(none).toHaveAccessibleName(
      "Kein Zugang Nur für die Planung. Eine OGS-Kraft startet die Termine.",
    );
    expect(
      screen.getByRole("radio", { name: /Nur ansehen/ }),
    ).toHaveAccessibleName(
      "Nur ansehen Sieht Kinder, Räume und AGs. Keine Elterndaten, keine Änderungen.",
    );
    expect(
      screen.getByRole("radio", { name: /Eigene Termine betreuen/ }),
    ).toHaveAccessibleName(
      "Eigene Termine betreuen Startet und beendet eigene Termine. Checkt Kinder ein und aus.",
    );
  });

  // „Nur ansehen“ grants the role „Gast“, which needs users:manage.
  it("hides „Nur ansehen“ without the right to grant it", () => {
    render(<ExternalCaregiverEntry onAdded={vi.fn()} />);

    openEntry();

    // Anker: die Zugangswahl steht da.
    expect(
      screen.getByRole("radio", { name: /Kein Zugang/ }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("radio", { name: /Eigene Termine betreuen/ }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("radio", { name: /Nur ansehen/ }),
    ).not.toBeInTheDocument();
  });

  it("records „Kein Zugang“ without email field, address or invitation hint", async () => {
    const onAdded = vi.fn();
    createExternal.mockResolvedValueOnce({
      id: "89",
      name: "Tom Platz",
      isExternal: true,
      externalAccountStatus: "none",
    });
    render(<ExternalCaregiverEntry onAdded={onAdded} canGrantView />);

    openEntry();
    // Anker: das Feld ist offen.
    expect(screen.getByLabelText("Vorname")).toBeInTheDocument();
    expect(screen.queryByLabelText("E-Mail-Adresse")).not.toBeInTheDocument();
    expect(screen.queryByText(/Einladung per E-Mail/)).not.toBeInTheDocument();

    fillEntry("Tom", "Platz", null);
    submit();

    await waitFor(() =>
      expect(onAdded).toHaveBeenCalledWith(
        expect.objectContaining({ id: "89" }),
      ),
    );
    expect(createExternal).toHaveBeenCalledWith({
      firstName: "Tom",
      lastName: "Platz",
      organization: "",
      access: "none",
    });
  });

  it("asks for the address and names the invitation for „Nur ansehen“", async () => {
    render(<ExternalCaregiverEntry onAdded={vi.fn()} canGrantView />);

    openEntry();
    fillEntry("Lea", "Gast", null);
    chooseAccess(/Nur ansehen/);

    expect(
      screen.getByText(
        "Die Person bekommt eine Einladung per E-Mail. Damit meldet sie sich selbst bei moto an.",
      ),
    ).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("E-Mail-Adresse"), {
      target: { value: "lea@example.test" },
    });
    submit();

    await waitFor(() =>
      expect(createExternal).toHaveBeenCalledWith(
        expect.objectContaining({
          access: "view",
          email: "lea@example.test",
        }),
      ),
    );
  });

  it("puts the server's access field error on the access choice", async () => {
    createExternal.mockRejectedValueOnce(
      new ApiError("bad access", 400, {
        code: "staff.external_access_invalid",
        errors: [{ field: "access", reason: "invalid" }],
      }),
    );
    render(<ExternalCaregiverEntry onAdded={vi.fn()} canGrantView />);

    openEntry();
    fillEntry("Lea", "Gast");
    submit();

    const radio = screen.getByRole("radio", {
      name: /Eigene Termine betreuen/,
    });
    await waitFor(() => expect(radio).toHaveAttribute("aria-invalid", "true"));
    expect(
      screen.getByText(
        catalogText(
          "staff.external_access_invalid",
          "das Eintragen der externen Person",
        ),
      ),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("E-Mail-Adresse")).not.toHaveAttribute(
      "aria-invalid",
    );
  });

  it("asks for both names before sending anything", () => {
    const onAdded = vi.fn();
    render(<ExternalCaregiverEntry onAdded={onAdded} />);

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
    render(<ExternalCaregiverEntry onAdded={onAdded} />);

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
    render(<ExternalCaregiverEntry onAdded={vi.fn()} />);

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
    render(<ExternalCaregiverEntry onAdded={vi.fn()} />);

    openEntry();
    fillEntry("Lea", "Gast");
    submit();

    const email = screen.getByLabelText("E-Mail-Adresse");
    await waitFor(() => expect(email).toHaveAttribute("aria-invalid", "true"));
    expect(screen.getByLabelText("Vorname")).not.toHaveAttribute(
      "aria-invalid",
    );
  });

  // The backend names the email field for an address that is taken (#4573).
  it.each([
    "identity.account_already_has_tenant_access",
    "identity.email_already_exists",
  ] as const)(
    "puts %s on the email field with its catalog text",
    async (code) => {
      createExternal.mockRejectedValueOnce(
        new ApiError("conflict", 409, {
          code,
          errors: [{ field: "email", reason: "taken" }],
        }),
      );
      render(<ExternalCaregiverEntry onAdded={vi.fn()} />);

      openEntry();
      fillEntry("Lea", "Gast");
      submit();

      const email = screen.getByLabelText("E-Mail-Adresse");
      await waitFor(() =>
        expect(email).toHaveAttribute("aria-invalid", "true"),
      );
      expect(
        screen.getByText(
          catalogText(code, "das Eintragen der externen Person"),
        ),
      ).toBeInTheDocument();
      await waitFor(() => expect(email).toHaveFocus());
    },
  );

  it.each([[new ApiError("forbidden", 403, { code: "general.permission" })]])(
    "shows a refusal without field through the catalog",
    async (error) => {
      createExternal.mockRejectedValueOnce(error);
      const onAdded = vi.fn();
      render(<ExternalCaregiverEntry onAdded={onAdded} />);

      openEntry();
      fillEntry("Lea", "Gast");
      submit();

      expect(
        await screen.findByText(
          catalogText(
            error.code as "general.permission",
            "das Eintragen der externen Person",
          ),
        ),
      ).toBeInTheDocument();
      expect(screen.getByLabelText("E-Mail-Adresse")).not.toHaveAttribute(
        "aria-invalid",
      );
      expect(onAdded).not.toHaveBeenCalled();
    },
  );

  it("keeps Enter and Escape inside the panel when the dialog is a form", async () => {
    const onSubmit = vi.fn((event: { preventDefault: () => void }) =>
      event.preventDefault(),
    );
    const onAdded = vi.fn();
    render(
      <form onSubmit={onSubmit}>
        <ExternalCaregiverEntry onAdded={onAdded} />
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

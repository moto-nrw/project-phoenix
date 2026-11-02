import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

const api = vi.hoisted(() => ({
  fetchTraegerBuero: vi.fn(),
  inviteTraegerAccount: vi.fn(),
  changeTraegerRoles: vi.fn(),
  removeTraegerMember: vi.fn(),
  revokeTraegerInvitation: vi.fn(),
  fetchTraegerAccountHistory: vi.fn(),
}));

vi.mock("~/lib/traeger-api", () => api);
vi.mock("next-auth/react", () => ({
  signOut: vi.fn(),
  useSession: () => ({
    data: { user: { email: "leitung@traeger.example" } },
    status: "authenticated",
  }),
}));

import TraegerAccountsPage from "./page";

const BUERO = {
  members: [
    {
      accountId: "1",
      email: "leitung@traeger.example",
      roles: ["leadership", "administration"],
    },
    { accountId: "2", email: "kasse@traeger.example", roles: ["billing"] },
  ],
  invitations: [
    {
      id: "9",
      email: "neu@traeger.example",
      roles: ["read_only"],
      expiresAt: "2026-09-11T10:00:00Z",
    },
  ],
};

const HISTORY = {
  entries: [
    {
      id: "12",
      occurredAt: "2026-09-08T14:05:00Z",
      action: "roles_changed",
      actorEmail: "leitung@traeger.example",
      targetEmail: "kasse@traeger.example",
      rolesBefore: ["read_only"],
      rolesAfter: ["billing"],
    },
    {
      id: "11",
      occurredAt: "2026-09-07T08:00:00Z",
      action: "member_removed",
      actorEmail: "",
      targetEmail: "weg@traeger.example",
      rolesBefore: ["personnel"],
      rolesAfter: [],
    },
  ],
  hasMore: true,
};

function renderPage() {
  return render(
    <ToastProvider>
      <TraegerAccountsPage />
    </ToastProvider>,
  );
}

async function openMenu(email: string) {
  const [menu] = await screen.findAllByRole("button", {
    name: `Aktionen für ${email}`,
  });
  fireEvent.click(menu!);
}

describe("TraegerAccountsPage", () => {
  beforeEach(() => {
    for (const fn of Object.values(api)) fn.mockReset();
    api.fetchTraegerBuero.mockResolvedValue(BUERO);
    api.fetchTraegerAccountHistory.mockResolvedValue(HISTORY);
  });

  it("lists the people with their roles and the open invitations", async () => {
    renderPage();

    const [kasse] = await screen.findAllByText("kasse@traeger.example");
    expect(kasse).toBeInTheDocument();
    expect(screen.getAllByText("Abrechnung")[0]).toBeInTheDocument();
    expect(screen.getAllByText("(Sie)")[0]).toBeInTheDocument();
    expect(screen.getByText("Offene Einladungen")).toBeInTheDocument();
    expect(screen.getAllByText("neu@traeger.example")[0]).toBeInTheDocument();
    expect(screen.getAllByText("Nur lesen")[0]).toBeInTheDocument();
  });

  it("invites a person with the chosen roles", async () => {
    api.inviteTraegerAccount.mockResolvedValue(undefined);
    renderPage();
    await screen.findAllByText("kasse@traeger.example");

    fireEvent.click(screen.getByRole("button", { name: "Person einladen" }));
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Einladung senden" }),
    );
    // Without an address and a role nothing is sent.
    expect(
      await within(dialog).findByText(
        "Bitte wählen Sie mindestens eine Rolle.",
      ),
    ).toBeInTheDocument();
    expect(api.inviteTraegerAccount).not.toHaveBeenCalled();

    fireEvent.change(within(dialog).getByLabelText(/E-Mail-Adresse/), {
      target: { value: "personal@traeger.example" },
    });
    fireEvent.click(within(dialog).getByRole("checkbox", { name: /Personal/ }));
    fireEvent.click(
      within(dialog).getByRole("checkbox", { name: /Nur lesen/ }),
    );
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Einladung senden" }),
    );

    await waitFor(() =>
      expect(api.inviteTraegerAccount).toHaveBeenCalledWith({
        email: "personal@traeger.example",
        roles: ["personnel", "read_only"],
      }),
    );
    expect(
      await screen.findByText("Die Einladung ist verschickt."),
    ).toBeInTheDocument();
    expect(api.fetchTraegerBuero).toHaveBeenCalledTimes(2);
  });

  it("changes the roles of a person", async () => {
    api.changeTraegerRoles.mockResolvedValue(undefined);
    renderPage();

    await openMenu("kasse@traeger.example");
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Rollen ändern" }),
    );
    const dialog = await screen.findByRole("dialog");
    const billing = within(dialog).getByRole("checkbox", {
      name: /Abrechnung/,
    });
    expect(billing).toBeChecked();
    fireEvent.click(within(dialog).getByRole("checkbox", { name: /Personal/ }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(api.changeTraegerRoles).toHaveBeenCalledWith("2", [
        "billing",
        "personnel",
      ]),
    );
  });

  it("keeps Leitung for its only holder and says why", async () => {
    api.changeTraegerRoles.mockResolvedValue(undefined);
    renderPage();

    await openMenu("leitung@traeger.example");
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Rollen ändern" }),
    );
    const dialog = await screen.findByRole("dialog");
    const leadership = within(dialog).getByRole("checkbox", {
      name: /Leitung/,
    });
    expect(leadership).toBeChecked();
    expect(leadership).toBeDisabled();
    expect(
      within(dialog).getByText(/Nur diese Person hat die Rolle Leitung/),
    ).toBeInTheDocument();
    fireEvent.click(
      within(dialog).getByRole("checkbox", { name: /Schulzugang/ }),
    );
    fireEvent.click(within(dialog).getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(api.changeTraegerRoles).toHaveBeenCalledWith("1", [
        "leadership",
        "administration",
        "school_access",
      ]),
    );
  });

  it("lets Leitung pass on once another person holds it", async () => {
    api.fetchTraegerBuero.mockResolvedValue({
      ...BUERO,
      members: [
        ...BUERO.members,
        {
          accountId: "3",
          email: "zweite@traeger.example",
          roles: ["leadership"],
        },
      ],
    });
    renderPage();

    await openMenu("leitung@traeger.example");
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Rollen ändern" }),
    );
    const dialog = await screen.findByRole("dialog");
    expect(
      within(dialog).getByRole("checkbox", { name: /Leitung/ }),
    ).toBeEnabled();
    expect(
      within(dialog).getByText(
        "Darf Personen einladen, ihre Rollen ändern und sie entfernen.",
      ),
    ).toBeInTheDocument();
  });

  it("shows why the last person with Leitung cannot leave", async () => {
    api.removeTraegerMember.mockRejectedValue(
      new ApiError("conflict", 409, { code: "traeger.last_administrator" }),
    );
    renderPage();

    await openMenu("leitung@traeger.example");
    fireEvent.click(
      await screen.findByRole("menuitem", {
        name: "Aus dem Träger-Büro entfernen",
      }),
    );
    const dialog = await screen.findByRole("dialog");
    // The two-step gate: the first click arms, the second removes.
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Ja, entfernen" }),
    );
    fireEvent.click(
      await within(dialog).findByRole("button", { name: "Entfernen" }),
    );

    await waitFor(() =>
      expect(api.removeTraegerMember).toHaveBeenCalledWith("1"),
    );
    expect(
      await within(dialog).findByText(
        catalogText("traeger.last_administrator", "das Entfernen"),
      ),
    ).toBeInTheDocument();
  });

  it("shows the history of changes and loads older entries", async () => {
    api.fetchTraegerAccountHistory
      .mockResolvedValueOnce(HISTORY)
      .mockResolvedValueOnce({
        entries: [
          {
            id: "10",
            occurredAt: "2026-09-01T09:00:00Z",
            action: "invitation_created",
            actorEmail: "leitung@traeger.example",
            targetEmail: "alt@traeger.example",
            rolesBefore: [],
            rolesAfter: ["read_only"],
          },
        ],
        hasMore: false,
      });
    renderPage();

    expect(await screen.findByText("Verlauf")).toBeInTheDocument();
    expect(
      (await screen.findAllByText("Rollen geändert"))[0],
    ).toBeInTheDocument();
    expect(
      screen.getAllByText("Vorher: Nur lesen. Jetzt: Abrechnung.")[0],
    ).toBeInTheDocument();
    expect(screen.getAllByText("Entfernt")[0]).toBeInTheDocument();
    expect(screen.getAllByText("weg@traeger.example")[0]).toBeInTheDocument();
    expect(screen.getAllByText("Bisher: Personal.")[0]).toBeInTheDocument();
    expect(screen.getAllByText("Gelöschtes Konto")[0]).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Ältere Einträge laden" }),
    );
    await waitFor(() =>
      expect(api.fetchTraegerAccountHistory).toHaveBeenLastCalledWith("11"),
    );
    expect(
      (await screen.findAllByText("alt@traeger.example"))[0],
    ).toBeInTheDocument();
    expect(screen.getAllByText("Eingeladen")[0]).toBeInTheDocument();
    // Anker: die ältere Seite ist geladen.
    expect(screen.getAllByText("alt@traeger.example")[0]).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Ältere Einträge laden" }),
    ).not.toBeInTheDocument();
  });

  it("shows no history without the right to manage the accounts", async () => {
    api.fetchTraegerBuero.mockRejectedValue(
      new ApiError("forbidden", 403, { code: "traeger.no_membership" }),
    );
    renderPage();

    // Anker: die Seite zeigt den Ladefehler statt der Personen.
    expect(
      await screen.findByText(
        catalogText("traeger.no_membership", "die Personen des Träger-Büros"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("Verlauf")).not.toBeInTheDocument();
  });

  it("shows the history with the right to manage the accounts", async () => {
    renderPage();

    // Anker: die Personen sind geladen.
    expect(
      (await screen.findAllByText("kasse@traeger.example"))[0],
    ).toBeInTheDocument();
    expect(await screen.findByText("Verlauf")).toBeInTheDocument();
  });

  it("reads the history again after a change", async () => {
    api.changeTraegerRoles.mockResolvedValue(undefined);
    renderPage();
    await waitFor(() =>
      expect(api.fetchTraegerAccountHistory).toHaveBeenCalledTimes(1),
    );

    await openMenu("kasse@traeger.example");
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Rollen ändern" }),
    );
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("checkbox", { name: /Personal/ }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(api.fetchTraegerAccountHistory).toHaveBeenCalledTimes(2),
    );
    expect(api.fetchTraegerAccountHistory).toHaveBeenLastCalledWith();
  });
});

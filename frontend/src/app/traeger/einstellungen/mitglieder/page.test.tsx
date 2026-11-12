import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { deferred } from "~/test/helpers/form-modal-pending";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { ErrorContactProvider } from "~/lib/error-contact";
import { catalogText } from "~/test/error-catalog-text";
import { SAVED_BUT_STALE_SUFFIX } from "~/lib/hooks/use-refresh-after-write";

const api = vi.hoisted(() => ({
  fetchTraegerBuero: vi.fn(),
  inviteTraegerAccount: vi.fn(),
  changeTraegerRoles: vi.fn(),
  changeTraegerMemberName: vi.fn(),
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
import {
  expectPageReloadedAfterRefusal,
  resetPageReloads,
} from "~/test/helpers/refused-refresh";

const BUERO = {
  members: [
    {
      accountId: "1",
      email: "leitung@traeger.example",
      name: "Lea Leitung",
      roles: ["leadership", "administration"],
    },
    {
      accountId: "2",
      email: "kasse@traeger.example",
      name: "",
      roles: ["billing"],
    },
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
      targetName: "",
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

  // #5992: the role Dokumente has its own row with a plain description. A
  // person who holds it keeps it when someone changes their other roles.
  it("shows the role Dokumente and keeps it when roles change", async () => {
    api.changeTraegerRoles.mockResolvedValue(undefined);
    api.fetchTraegerBuero.mockResolvedValue({
      ...BUERO,
      members: [
        ...BUERO.members,
        {
          accountId: "3",
          name: "",
          email: "verwaltung@traeger.example",
          roles: ["administration", "documents"],
        },
      ],
    });
    renderPage();

    const [labels] = await screen.findAllByText("Verwaltung, Dokumente");
    expect(labels).toBeInTheDocument();
    await openMenu("verwaltung@traeger.example");
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Rollen ändern" }),
    );
    const dialog = await screen.findByRole("dialog");
    expect(
      within(dialog).getByRole("checkbox", { name: /Dokumente/ }),
    ).toBeChecked();
    expect(
      within(dialog).getByText(
        "Darf Dokumente für Eltern und Schulen hochladen, freigeben und löschen.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("checkbox", { name: /Personal/ }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(api.changeTraegerRoles).toHaveBeenCalledWith("3", [
        "administration",
        "documents",
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
          name: "",
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

  // #4693: the name of each person, and a clear hint for one without.
  it("shows the name of each person and marks a missing one", async () => {
    renderPage();

    expect((await screen.findAllByText("Lea Leitung"))[0]).toBeInTheDocument();
    expect(screen.getAllByText("Kein Name")[0]).toBeInTheDocument();
    expect(screen.getAllByText("kasse@traeger.example")[0]).toBeInTheDocument();
  });

  it("enters the name of a person and shows it in the row", async () => {
    const named = { ...BUERO.members[1]!, name: "Karl Kasse" };
    api.changeTraegerMemberName.mockResolvedValue(named);
    api.fetchTraegerBuero
      .mockResolvedValueOnce(BUERO)
      .mockResolvedValue({ ...BUERO, members: [BUERO.members[0], named] });
    renderPage();

    await openMenu("kasse@traeger.example");
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Namen eintragen" }),
    );
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Speichern" }));
    expect(
      await within(dialog).findAllByText("Bitte geben Sie einen Namen ein."),
    ).not.toHaveLength(0);
    expect(api.changeTraegerMemberName).not.toHaveBeenCalled();

    fireEvent.change(within(dialog).getByLabelText(/Name/), {
      target: { value: " Karl Kasse " },
    });
    fireEvent.click(within(dialog).getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(api.changeTraegerMemberName).toHaveBeenCalledWith(
        "2",
        "Karl Kasse",
      ),
    );
    expect((await screen.findAllByText("Karl Kasse"))[0]).toBeInTheDocument();
    expect(
      await screen.findByText("Der Name ist gespeichert."),
    ).toBeInTheDocument();
    // The list and the history are read again.
    await waitFor(() =>
      expect(api.fetchTraegerAccountHistory).toHaveBeenCalledTimes(2),
    );
  });

  it("offers to change an existing name", async () => {
    renderPage();

    await openMenu("leitung@traeger.example");
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Namen ändern" }),
    );
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByLabelText(/Name/)).toHaveValue("Lea Leitung");
  });

  it("names a changed name in the history", async () => {
    api.fetchTraegerAccountHistory.mockResolvedValue({
      entries: [
        {
          id: "13",
          occurredAt: "2026-09-08T15:00:00Z",
          action: "name_changed",
          actorEmail: "leitung@traeger.example",
          targetEmail: "kasse@traeger.example",
          targetName: "Karl Kasse",
          rolesBefore: [],
          rolesAfter: [],
        },
      ],
      hasMore: false,
    });
    renderPage();

    expect(
      (await screen.findAllByText("Name geändert"))[0],
    ).toBeInTheDocument();
    expect(
      screen.getAllByText("Neuer Name: Karl Kasse.")[0],
    ).toBeInTheDocument();
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

  // #6871: the billing role without „Konten verwalten“ gets the 403 of the
  // permission check, and the page sends it to the Leitung of its Büro.
  it("sends the billing role to the Leitung of the Träger-Büro", async () => {
    api.fetchTraegerBuero.mockRejectedValue(new ApiError("Forbidden", 403));
    render(
      <ErrorContactProvider contact="traeger">
        <ToastProvider>
          <TraegerAccountsPage />
        </ToastProvider>
      </ErrorContactProvider>,
    );

    expect(
      await screen.findByText(
        "Für die Personen des Träger-Büros fehlt Ihnen die Berechtigung. Bitte fragen Sie die Leitung Ihres Träger-Büros.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/fragen Sie die Schule/)).not.toBeInTheDocument();
    expect(screen.queryByText("kasse@traeger.example")).not.toBeInTheDocument();
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

  it.each([403, 503])(
    "handles a %s reload while another member dialog is open",
    async (status) => {
      const read = deferred<typeof BUERO>();
      api.fetchTraegerBuero
        .mockResolvedValueOnce(BUERO)
        .mockReturnValueOnce(read.promise);
      api.revokeTraegerInvitation.mockResolvedValue(undefined);
      renderPage();
      await openMenu("die Einladung an neu@traeger.example");
      fireEvent.click(
        await screen.findByRole("menuitem", { name: /zurücknehmen/ }),
      );
      const dialog = await screen.findByRole("dialog");
      fireEvent.click(
        within(dialog).getByRole("button", { name: "Ja, zurücknehmen" }),
      );
      fireEvent.click(
        await within(dialog).findByRole("button", { name: "Zurücknehmen" }),
      );
      await waitFor(() =>
        expect(api.fetchTraegerBuero).toHaveBeenCalledTimes(2),
      );
      await openMenu("kasse@traeger.example");
      fireEvent.click(
        await screen.findByRole("menuitem", { name: "Rollen ändern" }),
      );
      expect(await screen.findByRole("dialog")).toBeVisible();
      resetPageReloads();
      await act(async () => read.reject(new ApiError("failed", status)));
      if (status === 403) {
        // #6801: the page loads anew; its first load shows the refusal.
        await expectPageReloadedAfterRefusal();
        return;
      }
      // Anker: der Nachladefehler bestätigt den abgeschlossenen Widerruf.
      expect(
        await screen.findByText(
          `${catalogText("general.unavailable", "die Anzeige des Träger-Büros")} ${SAVED_BUT_STALE_SUFFIX}`,
        ),
      ).toBeVisible();
      expect(
        screen.getAllByText("kasse@traeger.example").length,
      ).toBeGreaterThan(0);
      expect(screen.getByRole("dialog")).toBeVisible();
    },
  );

  // #6652: a confirmed write whose reload fails is no failed action and does
  // not replace the list: the error stands above it, says the change is
  // saved, and its retry only reads the list again.
  describe("reload after a confirmed write (#6652)", () => {
    const reloadText = `${catalogText("general.unavailable", "die Anzeige des Träger-Büros")} ${SAVED_BUT_STALE_SUFFIX}`;

    beforeEach(() => {
      api.fetchTraegerBuero
        .mockReset()
        .mockResolvedValueOnce(BUERO)
        .mockRejectedValueOnce(
          new ApiError("down", 503, { code: "general.unavailable" }),
        )
        .mockResolvedValue(BUERO);
    });

    async function expectListKept(write: ReturnType<typeof vi.fn>) {
      expect(await screen.findByText(reloadText)).toBeInTheDocument();
      // Anker: die bestätigte Liste bleibt stehen.
      expect(
        screen.getAllByText("kasse@traeger.example").length,
      ).toBeGreaterThan(0);
      fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
      await waitFor(() =>
        expect(api.fetchTraegerBuero).toHaveBeenCalledTimes(3),
      );
      expect(write).toHaveBeenCalledTimes(1);
    }

    it("keeps an invitation sent and the list shown when the reload fails", async () => {
      api.inviteTraegerAccount.mockResolvedValue(undefined);
      renderPage();
      await screen.findAllByText("kasse@traeger.example");

      fireEvent.click(screen.getByRole("button", { name: "Person einladen" }));
      const dialog = await screen.findByRole("dialog");
      fireEvent.change(within(dialog).getByLabelText(/E-Mail-Adresse/), {
        target: { value: "personal@traeger.example" },
      });
      fireEvent.click(
        within(dialog).getByRole("checkbox", { name: /Personal/ }),
      );
      fireEvent.click(
        within(dialog).getByRole("button", { name: "Einladung senden" }),
      );

      await expectListKept(api.inviteTraegerAccount);
    });

    it("keeps changed roles and the list shown when the reload fails", async () => {
      api.changeTraegerRoles.mockResolvedValue(undefined);
      renderPage();

      await openMenu("kasse@traeger.example");
      fireEvent.click(
        await screen.findByRole("menuitem", { name: "Rollen ändern" }),
      );
      const dialog = await screen.findByRole("dialog");
      fireEvent.click(
        within(dialog).getByRole("checkbox", { name: /Personal/ }),
      );
      fireEvent.click(
        within(dialog).getByRole("button", { name: "Speichern" }),
      );

      await expectListKept(api.changeTraegerRoles);
    });

    it("keeps a revoked invitation revoked and the list shown when the reload fails", async () => {
      api.revokeTraegerInvitation.mockResolvedValue(undefined);
      renderPage();

      await openMenu("die Einladung an neu@traeger.example");
      fireEvent.click(
        await screen.findByRole("menuitem", { name: /zurücknehmen/ }),
      );
      const dialog = await screen.findByRole("dialog");
      fireEvent.click(
        within(dialog).getByRole("button", { name: "Ja, zurücknehmen" }),
      );
      fireEvent.click(
        await within(dialog).findByRole("button", { name: "Zurücknehmen" }),
      );

      await expectListKept(api.revokeTraegerInvitation);
    });
  });
});

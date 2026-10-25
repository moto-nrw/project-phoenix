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
      roles: ["administration"],
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

  it("shows why the last person with Verwaltung cannot leave", async () => {
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
});

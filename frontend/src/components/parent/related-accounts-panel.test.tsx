import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import RelatedAccountsPanel from "./related-accounts-panel";
import type { RelatedAccount } from "~/lib/parent-api";

const mockList = vi.fn();
const mockInvite = vi.fn();
const mockRemove = vi.fn();

vi.mock("~/lib/parent-api", () => ({
  listRelatedAccounts: (studentId: string): unknown => mockList(studentId),
  inviteRelatedAccount: (studentId: string, email: string): unknown =>
    mockInvite(studentId, email),
  removeRelatedAccount: (
    studentId: string,
    guardianProfileId: string,
  ): unknown => mockRemove(studentId, guardianProfileId),
}));

const primaryActive: RelatedAccount = {
  guardian_profile_id: "1",
  first_name: "Sabine",
  last_name: "Schneider",
  email: "sabine.schneider@email.de",
  relationship_type: "parent",
  is_primary: true,
  status: "active",
  is_self: false,
};

const secondaryPending: RelatedAccount = {
  guardian_profile_id: "2",
  first_name: "Markus",
  last_name: "Wolf",
  email: "markus.wolf@email.de",
  relationship_type: "guardian",
  is_primary: false,
  status: "pending",
  is_self: false,
};

const staffContactNoAccount: RelatedAccount = {
  guardian_profile_id: "3",
  first_name: "Clara",
  last_name: "Kontakt",
  email: "clara.kontakt@email.de",
  relationship_type: "guardian",
  is_primary: false,
  status: "no_account",
  is_self: false,
};

describe("RelatedAccountsPanel", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockList.mockResolvedValue([primaryActive, secondaryPending]);
    mockInvite.mockResolvedValue({
      outcome: "invited",
      guardian_profile_id: "3",
    });
    mockRemove.mockResolvedValue(undefined);
  });

  it("lists linked accounts with their status", async () => {
    render(
      <RelatedAccountsPanel
        studentId="1"
        canInvite={false}
        canRemove={false}
      />,
    );
    await waitFor(() =>
      expect(screen.getByText("Sabine Schneider")).toBeInTheDocument(),
    );
    expect(screen.getByText("Markus Wolf")).toBeInTheDocument();
    expect(screen.getByText(/Konto aktiv/)).toBeInTheDocument();
    expect(screen.getByText(/Einladung offen/)).toBeInTheDocument();
  });

  it("renders no-account contacts without a remove affordance", async () => {
    mockList.mockResolvedValue([primaryActive, staffContactNoAccount]);
    render(
      <RelatedAccountsPanel studentId="1" canInvite={false} canRemove={true} />,
    );

    await waitFor(() =>
      expect(screen.getByText("Clara Kontakt")).toBeInTheDocument(),
    );
    expect(screen.getByText(/Kontakt ohne Konto/)).toBeInTheDocument();
    expect(screen.queryByTitle("Zugang entfernen")).not.toBeInTheDocument();
  });

  it("does not offer a remove affordance for the parent's own row", async () => {
    // A non-primary guardian who is the logged-in parent: self-removal is
    // rejected by the backend, so the panel must not offer the action.
    const self: RelatedAccount = {
      ...secondaryPending,
      guardian_profile_id: "9",
      first_name: "Ich",
      last_name: "Selbst",
      is_self: true,
    };
    mockList.mockResolvedValue([primaryActive, self]);
    render(
      <RelatedAccountsPanel studentId="1" canInvite={false} canRemove={true} />,
    );

    await waitFor(() =>
      expect(screen.getByText("Ich Selbst")).toBeInTheDocument(),
    );
    expect(screen.queryByTitle("Zugang entfernen")).not.toBeInTheDocument();
  });

  it("hides the invite control when inviting is disabled", async () => {
    render(
      <RelatedAccountsPanel
        studentId="1"
        canInvite={false}
        canRemove={false}
      />,
    );
    await waitFor(() => screen.getByText("Sabine Schneider"));
    expect(
      screen.queryByRole("button", { name: /Einladen/ }),
    ).not.toBeInTheDocument();
  });

  it("invites a person by email when inviting is enabled", async () => {
    render(
      <RelatedAccountsPanel studentId="1" canInvite={true} canRemove={false} />,
    );
    await waitFor(() => screen.getByText("Sabine Schneider"));

    fireEvent.click(screen.getByRole("button", { name: /Einladen/ }));
    const input = screen.getByPlaceholderText("E-Mail-Adresse");
    fireEvent.change(input, { target: { value: "oma@example.test" } });
    fireEvent.click(screen.getByRole("button", { name: /Senden/ }));

    await waitFor(() =>
      expect(mockInvite).toHaveBeenCalledWith("1", "oma@example.test"),
    );
  });

  it("removes a non-primary account but never the primary", async () => {
    render(
      <RelatedAccountsPanel studentId="1" canInvite={false} canRemove={true} />,
    );
    await waitFor(() => screen.getByText("Markus Wolf"));

    // Exactly one remove affordance — only the non-primary account has it.
    expect(screen.getAllByTitle("Zugang entfernen")).toHaveLength(1);

    fireEvent.click(screen.getByTitle("Zugang entfernen"));
    // two-step confirm
    fireEvent.click(screen.getByRole("button", { name: "Entfernen" }));

    await waitFor(() => expect(mockRemove).toHaveBeenCalledWith("1", "2"));
  });
});

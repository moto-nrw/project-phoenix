import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import { InvitationOwnerAcceptForm } from "./invitation-owner-accept-form";
import { listAllTenants } from "~/lib/tenant-api";
import { acceptInvitation } from "~/lib/invitation-api";

vi.mock("~/lib/invitation-api", () => ({ acceptInvitation: vi.fn() }));
vi.mock("~/lib/tenant-api", () => ({ listAllTenants: vi.fn() }));

const invitation = {
  email: "owner@example.com",
  roleName: "user",
  expiresAt: "2027-01-01T12:00:00Z",
  firstName: "Alex",
  lastName: "Owner",
  requiresAccountLogin: true,
};

describe("existing-account invitation acceptance", () => {
  beforeEach(() => vi.clearAllMocks());

  it("joins with the current session without requesting or changing a password", async () => {
    vi.mocked(acceptInvitation).mockResolvedValue({});
    render(
      <InvitationOwnerAcceptForm
        token="membership-offer"
        invitation={invitation}
      />,
    );
    expect(screen.queryByLabelText("Passwort")).not.toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /Anmeldung öffnen/ }),
    ).toHaveAttribute("target", "_blank");
    fireEvent.click(screen.getByRole("button", { name: "Einladung annehmen" }));
    await screen.findByText("Einladung angenommen");
    expect(acceptInvitation).toHaveBeenCalledWith("membership-offer", {
      existingAccount: true,
      firstName: "Alex",
      lastName: "Owner",
      password: "",
      confirmPassword: "",
    });
    expect(
      screen.getByText(/Ihre bisherigen Zugänge bleiben bestehen/),
    ).toBeInTheDocument();
  });

  it("explains an unavailable school list and allows retrying", async () => {
    vi.mocked(listAllTenants).mockResolvedValueOnce({
      tenants: [],
      status: "error",
    });
    render(
      <InvitationOwnerAcceptForm
        token="membership-offer"
        invitation={invitation}
      />,
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Andere moto-Adresse wählen" }),
    );
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Die Schulliste ist gerade nicht verfügbar",
    );
    vi.mocked(listAllTenants).mockResolvedValueOnce({
      tenants: [
        {
          tenantId: 1,
          organizationId: 1,
          slug: "school",
          name: "School",
          subdomain: "school",
          organizationName: "Org",
        },
      ],
      status: "ok",
    });
    fireEvent.click(screen.getByRole("button", { name: "Erneut laden" }));
    await waitFor(() =>
      expect(screen.queryByRole("alert")).not.toBeInTheDocument(),
    );
    expect(listAllTenants).toHaveBeenCalledTimes(2);
  });

  it.each([
    ["INVITATION_ACCOUNT_LOGIN_REQUIRED", /Bitte melden Sie sich zuerst/],
    ["INVITATION_ACCOUNT_MISMATCH", /Sie sind mit einem anderen Konto/],
    ["ACCOUNT_INACTIVE", /Ihr Konto ist gesperrt/],
  ])("explains rejected acceptance: %s", async (code, message) => {
    vi.mocked(acceptInvitation).mockRejectedValue(
      Object.assign(new Error("backend detail"), { code }),
    );
    render(
      <InvitationOwnerAcceptForm
        token="membership-offer"
        invitation={invitation}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Einladung annehmen" }));
    await waitFor(() =>
      expect(screen.getByRole("alert")).toHaveTextContent(message),
    );
    expect(screen.queryByText("Einladung angenommen")).not.toBeInTheDocument();
  });
});

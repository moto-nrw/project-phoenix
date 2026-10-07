import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import { InvitationOwnerAcceptForm } from "./invitation-owner-accept-form";
import { listAllTenants } from "~/lib/tenant-api";
import { acceptInvitation } from "~/lib/invitation-api";
import { ApiError } from "~/lib/api-error";
import type { ErrorCode } from "~/lib/error-codes.generated";
import { catalogText } from "~/test/error-catalog-text";

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
      "Die Liste der Schulen ist gerade nicht erreichbar",
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

  // Catalog text per code (#2517). "Log in first" answers 401; it stays
  // in the form instead of jumping to the login screen.
  it.each<[ErrorCode, number]>([
    ["identity.invitation_account_login_required", 401],
    ["identity.invitation_account_mismatch", 403],
    ["identity.account_inactive", 403],
  ])("explains rejected acceptance: %s", async (code, status) => {
    const assign = vi
      .spyOn(window.location, "assign")
      .mockImplementation(() => undefined);
    vi.mocked(acceptInvitation).mockRejectedValue(
      new ApiError("backend detail", status, { code }),
    );
    render(
      <InvitationOwnerAcceptForm
        token="membership-offer"
        invitation={invitation}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Einladung annehmen" }));
    await waitFor(() =>
      expect(screen.getByRole("alert")).toHaveTextContent(
        catalogText(code, "die Einladung"),
      ),
    );
    expect(screen.queryByText("Einladung angenommen")).not.toBeInTheDocument();
    expect(assign).not.toHaveBeenCalled();
    assign.mockRestore();
  });
});

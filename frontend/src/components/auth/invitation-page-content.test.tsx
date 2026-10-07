import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";
import { InvitationPageContent } from "./invitation-page-content";

const { validateInvitation } = vi.hoisted(() => ({
  validateInvitation: vi.fn(),
}));
vi.mock("~/lib/invitation-api", () => ({ validateInvitation }));
vi.mock("~/lib/tenant-context", () => ({ useTenantSafe: () => null }));
vi.mock("./invitation-accept-form", () => ({
  InvitationAcceptForm: () => <div data-testid="accept-form" />,
}));
vi.mock("./invitation-owner-accept-form", () => ({
  InvitationOwnerAcceptForm: () => <div data-testid="owner-form" />,
}));

describe("InvitationPageContent (#2517)", () => {
  afterEach(() => {
    validateInvitation.mockReset();
    vi.restoreAllMocks();
  });

  it("names a missing token without asking the API", () => {
    render(<InvitationPageContent token={null} />);

    expect(screen.getByText(/Der Link ist unvollständig/)).toBeInTheDocument();
    expect(validateInvitation).not.toHaveBeenCalled();
  });

  it("shows the catalog text of an expired invitation in place", async () => {
    vi.spyOn(console, "warn").mockImplementation(() => undefined);
    validateInvitation.mockRejectedValue(
      new ApiError("invitation expired", 410, {
        code: "identity.invitation_expired",
      }),
    );
    render(<InvitationPageContent token="t-1" />);

    expect(
      await screen.findByText(
        catalogText("identity.invitation_expired", "die Einladung"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByTestId("accept-form")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Wiederholen" }),
    ).not.toBeInTheDocument();
  });

  it("retries a failed load and then shows the form", async () => {
    vi.spyOn(console, "error").mockImplementation(() => undefined);
    validateInvitation.mockRejectedValueOnce(new ApiError("boom", 500));
    validateInvitation.mockResolvedValueOnce({
      email: "new@example.com",
      roleName: "user",
      expiresAt: "2027-01-01T12:00:00Z",
    });
    render(<InvitationPageContent token="t-1" />);

    await screen.findByText(catalogText("general.server", "die Einladung"));
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    expect(await screen.findByTestId("accept-form")).toBeInTheDocument();
    await waitFor(() => expect(validateInvitation).toHaveBeenCalledTimes(2));
  });
});

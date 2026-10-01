import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  markAllAdminRequestsRead: vi.fn(),
  toastError: vi.fn(),
  toastSuccess: vi.fn(),
}));

vi.mock("~/components/enrollment/admin-enrollments-list", () => ({
  AdminEnrollmentsList: () => <div>Liste</div>,
}));
vi.mock("~/components/enrollment/phase-expiry-warnings", () => ({
  PhaseExpiryWarnings: () => null,
}));
vi.mock("~/components/ui/desktop-only-notice", () => ({
  DesktopOnlyNotice: () => null,
}));
vi.mock("~/lib/hooks/use-require-permission", () => ({
  useRequirePermission: vi.fn(() => ({ isReady: true, isLoading: false })),
}));
vi.mock("~/lib/enrollment-admin-api", () => ({
  markAllAdminRequestsRead: mocks.markAllAdminRequestsRead,
}));
vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ success: mocks.toastSuccess, error: mocks.toastError }),
}));

import Page from "./page";

describe("AdminEnrollmentsPage (#3778)", () => {
  beforeEach(() => {
    for (const mock of Object.values(mocks)) mock.mockReset();
  });

  it("markiert über das Menü im Kopf alle Anmeldungen als gelesen", async () => {
    mocks.markAllAdminRequestsRead.mockResolvedValue(undefined);
    render(<Page />);

    fireEvent.click(
      screen.getByRole("button", { name: "Weitere Aktionen für Anmeldungen" }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", {
        name: "Alle als gelesen markieren",
      }),
    );

    await waitFor(() => {
      expect(mocks.markAllAdminRequestsRead).toHaveBeenCalledTimes(1);
    });
    await waitFor(() => {
      expect(mocks.toastSuccess).toHaveBeenCalledWith(
        "Alle Anmeldungen sind als gelesen markiert.",
      );
    });
  });

  it("meldet einen Fehler beim Markieren", async () => {
    mocks.markAllAdminRequestsRead.mockRejectedValue(
      new Error(
        "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.",
      ),
    );
    render(<Page />);

    fireEvent.click(
      screen.getByRole("button", { name: "Weitere Aktionen für Anmeldungen" }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", {
        name: "Alle als gelesen markieren",
      }),
    );

    await waitFor(() => {
      expect(mocks.toastError).toHaveBeenCalledWith(
        "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.",
      );
    });
  });
});

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  markAllAdminRequestsRead: vi.fn(),
  fetchEmailSubscription: vi.fn(),
  setEmailSubscription: vi.fn(),
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
vi.mock("~/lib/notification-preferences-api", () => ({
  fetchEmailSubscription: mocks.fetchEmailSubscription,
  setEmailSubscription: mocks.setEmailSubscription,
}));
vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ success: mocks.toastSuccess, error: mocks.toastError }),
}));

import Page from "./page";

describe("AdminEnrollmentsPage (#3778)", () => {
  beforeEach(() => {
    for (const mock of Object.values(mocks)) mock.mockReset();
    mocks.fetchEmailSubscription.mockResolvedValue(false);
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

  describe("E-Mail bei neuer Anmeldung (#3780)", () => {
    const emailEntry = "E-Mail an mich bei neuer Anmeldung";

    async function openEmailEntry() {
      fireEvent.click(
        screen.getByRole("button", {
          name: "Weitere Aktionen für Anmeldungen",
        }),
      );
      return screen.findByRole("menuitemcheckbox", { name: emailEntry });
    }

    it("zeigt den eigenen Stand und schaltet ihn um", async () => {
      mocks.fetchEmailSubscription.mockResolvedValue(true);
      mocks.setEmailSubscription.mockResolvedValue(undefined);
      render(<Page />);
      await waitFor(() => {
        expect(mocks.fetchEmailSubscription).toHaveBeenCalledWith(
          "enrollment_submitted",
        );
      });

      const entry = await openEmailEntry();
      await waitFor(() => {
        expect(entry).toHaveAttribute("aria-checked", "true");
      });
      fireEvent.click(entry);

      await waitFor(() => {
        expect(mocks.setEmailSubscription).toHaveBeenCalledWith(
          "enrollment_submitted",
          false,
        );
      });
      expect(mocks.toastSuccess).toHaveBeenCalledWith(
        "Sie bekommen keine E-Mail mehr bei neuen Anmeldungen.",
      );
      expect(await openEmailEntry()).toHaveAttribute("aria-checked", "false");
    });

    it("schaltet ein und bestätigt es", async () => {
      mocks.setEmailSubscription.mockResolvedValue(undefined);
      render(<Page />);
      await waitFor(() => {
        expect(mocks.fetchEmailSubscription).toHaveBeenCalled();
      });

      fireEvent.click(await openEmailEntry());

      await waitFor(() => {
        expect(mocks.toastSuccess).toHaveBeenCalledWith(
          "Sie bekommen jetzt bei jeder neuen Anmeldung eine E-Mail.",
        );
      });
      expect(mocks.setEmailSubscription).toHaveBeenCalledWith(
        "enrollment_submitted",
        true,
      );
    });

    it("nimmt den Haken zurück, wenn das Speichern scheitert", async () => {
      mocks.setEmailSubscription.mockRejectedValue(new Error("boom"));
      render(<Page />);
      await waitFor(() => {
        expect(mocks.fetchEmailSubscription).toHaveBeenCalled();
      });

      fireEvent.click(await openEmailEntry());

      await waitFor(() => {
        expect(mocks.toastError).toHaveBeenCalledWith(
          "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.",
        );
      });
      expect(await openEmailEntry()).toHaveAttribute("aria-checked", "false");
    });

    it("sperrt weitere Änderungen, bis das Speichern abgeschlossen ist", async () => {
      let finishSave!: () => void;
      mocks.setEmailSubscription.mockReturnValue(
        new Promise<void>((resolve) => {
          finishSave = resolve;
        }),
      );
      render(<Page />);

      fireEvent.click(await openEmailEntry());
      const pendingEntry = await openEmailEntry();
      expect(pendingEntry).toHaveAttribute("aria-checked", "true");
      expect(pendingEntry).toBeDisabled();
      fireEvent.click(pendingEntry);
      expect(mocks.setEmailSubscription).toHaveBeenCalledTimes(1);

      finishSave();
      await waitFor(() => expect(mocks.toastSuccess).toHaveBeenCalled());
      await waitFor(() => expect(pendingEntry).not.toBeDisabled());
    });

    it("zeigt keinen falschen Haken, wenn der Stand nicht lädt", async () => {
      mocks.fetchEmailSubscription.mockRejectedValue(new Error("boom"));
      render(<Page />);
      await waitFor(() => {
        expect(mocks.fetchEmailSubscription).toHaveBeenCalled();
      });

      fireEvent.click(
        screen.getByRole("button", {
          name: "Weitere Aktionen für Anmeldungen",
        }),
      );

      await screen.findByRole("menuitem", {
        name: "Alle als gelesen markieren",
      });
      expect(
        screen.queryByRole("menuitemcheckbox", { name: emailEntry }),
      ).not.toBeInTheDocument();
    });
  });
});

/**
 * Tests for GuardianDeleteModal
 * Covers the three steps: choose (two option cards), the per-child unlink
 * confirmation, and the admin-only full-delete confirmation.
 */
import {
  render,
  screen,
  waitFor,
  fireEvent,
  act,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { GuardianDeleteModal } from "./guardian-delete-modal";

// Mock Modal component — renders title + footer so the kit-driven chrome is
// observable in tests.
vi.mock("~/components/ui/modal", () => ({
  Modal: ({
    isOpen,
    onClose,
    title,
    children,
    footer,
  }: {
    isOpen: boolean;
    onClose: () => void;
    title: string;
    children: React.ReactNode;
    footer?: React.ReactNode;
  }) =>
    isOpen ? (
      <div data-testid="modal">
        {title && <h2>{title}</h2>}
        <button type="button" onClick={onClose}>
          Close
        </button>
        {children}
        {footer}
      </div>
    ) : null,
}));

const handlers = {
  onClose: vi.fn(),
  onSelectUnlink: vi.fn(),
  onSelectFullDelete: vi.fn(),
  onConfirmUnlink: vi.fn(),
  onConfirmFullDelete: vi.fn(),
  onBack: vi.fn(),
};

function renderModal(
  props: Partial<React.ComponentProps<typeof GuardianDeleteModal>> = {},
) {
  return render(
    <GuardianDeleteModal
      isOpen={true}
      guardianName="John Doe"
      {...handlers}
      {...props}
    />,
  );
}

describe("GuardianDeleteModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("choose step", () => {
    it("renders the modal when open", async () => {
      renderModal();
      await waitFor(() => {
        expect(screen.getByTestId("modal")).toBeInTheDocument();
      });
    });

    it("does not render when closed", () => {
      renderModal({ isOpen: false });
      expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
    });

    it("shows the guardian name in the title and the choice prompt", async () => {
      renderModal();
      await waitFor(() => {
        expect(screen.getByText("John Doe entfernen")).toBeInTheDocument();
        expect(screen.getByText("Was möchten Sie tun?")).toBeInTheDocument();
      });
    });

    it("calls onSelectUnlink when the unlink card is clicked", async () => {
      renderModal();
      await act(async () => {
        fireEvent.click(screen.getByText("Nur von diesem Kind entfernen"));
      });
      expect(handlers.onSelectUnlink).toHaveBeenCalledTimes(1);
    });

    it("hides the full-delete card for non-admins", () => {
      renderModal({ canFullDelete: false });
      expect(screen.queryByText("Vollständig löschen")).not.toBeInTheDocument();
    });

    it("offers the full-delete card to admins and calls onSelectFullDelete", async () => {
      renderModal({ canFullDelete: true });
      await act(async () => {
        fireEvent.click(screen.getByText("Vollständig löschen"));
      });
      expect(handlers.onSelectFullDelete).toHaveBeenCalledTimes(1);
    });

    it("calls onClose when cancel is clicked", async () => {
      renderModal();
      await act(async () => {
        fireEvent.click(screen.getByText("Abbrechen"));
      });
      expect(handlers.onClose).toHaveBeenCalledTimes(1);
    });

    it("disables the option cards when loading", async () => {
      renderModal({ canFullDelete: true, isLoading: true });
      await waitFor(() => {
        expect(
          screen.getByText("Nur von diesem Kind entfernen").closest("button"),
        ).toBeDisabled();
        expect(
          screen.getByText("Vollständig löschen").closest("button"),
        ).toBeDisabled();
      });
    });
  });

  describe("confirm-unlink step", () => {
    it("shows the unlink confirmation copy", async () => {
      renderModal({ step: "confirm-unlink" });
      await waitFor(() => {
        expect(
          screen.getByText("Von diesem Kind entfernen?"),
        ).toBeInTheDocument();
        expect(
          screen.getByText(/Für eventuelle Geschwister bleibt/i),
        ).toBeInTheDocument();
      });
    });

    it("calls onConfirmUnlink when the remove button is clicked", async () => {
      renderModal({ step: "confirm-unlink" });
      await act(async () => {
        fireEvent.click(screen.getByText("Entfernen"));
      });
      expect(handlers.onConfirmUnlink).toHaveBeenCalledTimes(1);
    });

    it("labels back as Abbrechen for non-admins and calls onBack", async () => {
      renderModal({ step: "confirm-unlink", canFullDelete: false });
      await act(async () => {
        fireEvent.click(screen.getByText("Abbrechen"));
      });
      expect(handlers.onBack).toHaveBeenCalledTimes(1);
    });

    it("labels back as Zurück for admins", () => {
      renderModal({ step: "confirm-unlink", canFullDelete: true });
      expect(screen.getByText("Zurück")).toBeInTheDocument();
    });

    it("shows loading text and disables buttons when loading", async () => {
      renderModal({ step: "confirm-unlink", isLoading: true });
      await waitFor(() => {
        const loading = screen.getByText("Wird entfernt...");
        expect(loading.closest("button")).toBeDisabled();
      });
    });
  });

  describe("confirm-full step", () => {
    it("shows the full-delete heading and the backend warning", async () => {
      renderModal({
        step: "confirm-full",
        fullDeleteWarning: "Noch mit 2 Kind(ern) verknüpft: Anna, Ben.",
      });
      await waitFor(() => {
        expect(screen.getByText(/Komplett löschen\?/i)).toBeInTheDocument();
        expect(
          screen.getByText(/Noch mit 2 Kind\(ern\) verknüpft/),
        ).toBeInTheDocument();
        expect(
          screen.getByText(
            /Diese Aktion kann nicht rückgängig gemacht werden/i,
          ),
        ).toBeInTheDocument();
      });
    });

    it("shows a loading hint and disables the delete button while the warning loads", async () => {
      renderModal({ step: "confirm-full", isWarningLoading: true });
      await waitFor(() => {
        expect(
          screen.getByText(/Betroffene Kinder werden geprüft/i),
        ).toBeInTheDocument();
        expect(screen.getByText("Endgültig löschen")).toBeDisabled();
      });
    });

    it("calls onConfirmFullDelete when the final delete button is clicked", async () => {
      renderModal({ step: "confirm-full" });
      await act(async () => {
        fireEvent.click(screen.getByText("Endgültig löschen"));
      });
      expect(handlers.onConfirmFullDelete).toHaveBeenCalledTimes(1);
    });

    it("calls onBack when back is clicked", async () => {
      renderModal({ step: "confirm-full" });
      await act(async () => {
        fireEvent.click(screen.getByText("Zurück"));
      });
      expect(handlers.onBack).toHaveBeenCalledTimes(1);
    });

    it("shows full-delete loading text and disables buttons when loading", async () => {
      renderModal({ step: "confirm-full", isLoading: true });
      await waitFor(() => {
        expect(screen.getByText("Zurück")).toBeDisabled();
        const loading = screen.getByText("Wird gelöscht...");
        expect(loading.closest("button")).toBeDisabled();
      });
    });
  });
});

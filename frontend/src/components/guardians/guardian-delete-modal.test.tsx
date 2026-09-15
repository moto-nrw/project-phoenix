/**
 * Tests for GuardianDeleteModal (#3110: one ConfirmDeleteModal).
 * Covers the non-admin per-child unlink, the admin scope choice, and the
 * admin-only full delete with its affected-children warning and typed gate.
 */
import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { ModalProvider } from "~/components/dashboard/modal-context";
import { GuardianDeleteModal } from "./guardian-delete-modal";

const handlers = {
  onClose: vi.fn(),
  onScopeChange: vi.fn(),
  onConfirmUnlink: vi.fn(),
  onConfirmFullDelete: vi.fn(),
};

function renderModal(
  props: Partial<React.ComponentProps<typeof GuardianDeleteModal>> = {},
) {
  return render(
    <ModalProvider>
      <GuardianDeleteModal
        isOpen={true}
        guardianName="John Doe"
        {...handlers}
        {...props}
      />
    </ModalProvider>,
  );
}

describe("GuardianDeleteModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("does not render when closed", () => {
    renderModal({ isOpen: false });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  describe("without full-delete permission", () => {
    it("offers only the two-step per-child unlink", () => {
      renderModal({ canFullDelete: false });

      expect(
        screen.getByRole("dialog", { name: "John Doe entfernen" }),
      ).toBeInTheDocument();
      expect(
        screen.getByText(/Für eventuelle Geschwister bleibt/),
      ).toBeInTheDocument();
      expect(screen.queryByRole("radio")).not.toBeInTheDocument();

      fireEvent.click(screen.getByRole("button", { name: "Entfernen" }));
      fireEvent.click(
        screen.getByRole("button", { name: "Von diesem Kind entfernen" }),
      );
      expect(handlers.onConfirmUnlink).toHaveBeenCalledTimes(1);
      expect(handlers.onConfirmFullDelete).not.toHaveBeenCalled();
    });

    it("calls onClose when cancel is clicked", () => {
      renderModal();
      fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));
      expect(handlers.onClose).toHaveBeenCalledTimes(1);
    });

    it("shows loading text and disables the buttons when loading", () => {
      renderModal({ isLoading: true });
      expect(screen.getByRole("button", { name: "Entfernen" })).toBeDisabled();
      expect(screen.getByRole("button", { name: "Abbrechen" })).toBeDisabled();
    });
  });

  describe("with full-delete permission", () => {
    it("keeps the deletion blocked until a scope is chosen", () => {
      renderModal({ canFullDelete: true, scope: null });

      expect(screen.getByText("Was möchten Sie tun?")).toBeInTheDocument();
      expect(
        screen.getByRole("radio", { name: /Nur von diesem Kind entfernen/ }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("radio", { name: /Vollständig löschen/ }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: "Von diesem Kind entfernen" }),
      ).toBeDisabled();

      fireEvent.click(
        screen.getByRole("radio", { name: /Vollständig löschen/ }),
      );
      expect(handlers.onScopeChange).toHaveBeenCalledWith("full");
    });

    it("confirms the per-child unlink once that scope is chosen", () => {
      renderModal({ canFullDelete: true, scope: "unlink" });

      const confirm = screen.getByRole("button", {
        name: "Von diesem Kind entfernen",
      });
      expect(confirm).toBeEnabled();
      fireEvent.click(confirm);
      expect(handlers.onConfirmUnlink).toHaveBeenCalledTimes(1);
      expect(handlers.onConfirmFullDelete).not.toHaveBeenCalled();
    });

    it("holds the full delete while the affected-children warning loads", () => {
      renderModal({
        canFullDelete: true,
        scope: "full",
        isWarningLoading: true,
      });

      expect(
        screen.getByText(/Betroffene Kinder werden geprüft/),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: "Endgültig löschen" }),
      ).toBeDisabled();
    });

    it("requires the typed name for the full delete and shows the warning", () => {
      renderModal({
        canFullDelete: true,
        scope: "full",
        fullDeleteWarning: "Noch mit 2 Kind(ern) verknüpft: Anna, Ben.",
      });

      expect(
        screen.getByText(/Noch mit 2 Kind\(ern\) verknüpft/),
      ).toBeInTheDocument();
      expect(
        screen.getByText(/Diese Aktion kann nicht rückgängig gemacht werden/),
      ).toBeInTheDocument();

      const confirm = screen.getByRole("button", { name: "Endgültig löschen" });
      expect(confirm).toBeDisabled();

      const input = screen.getByLabelText(
        "Geben Sie zur Bestätigung den Namen ein:",
      );
      fireEvent.change(input, { target: { value: "John Do" } });
      expect(confirm).toBeDisabled();
      fireEvent.change(input, { target: { value: "John Doe" } });
      expect(confirm).toBeEnabled();

      fireEvent.click(confirm);
      expect(handlers.onConfirmFullDelete).toHaveBeenCalledTimes(1);
      expect(handlers.onConfirmUnlink).not.toHaveBeenCalled();
    });

    it("disables the full delete while loading", () => {
      renderModal({
        canFullDelete: true,
        scope: "full",
        fullDeleteWarning: "Warnung",
        isLoading: true,
      });
      expect(
        screen.getByRole("button", { name: "Wird gelöscht…" }),
      ).toBeDisabled();
    });
  });
});

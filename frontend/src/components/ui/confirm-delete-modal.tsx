"use client";

import { type ReactNode, useCallback, useEffect, useState } from "react";
import { Button } from "./button";
import { Modal } from "./modal";

// Shared destructive-confirmation shell for the operator drill-in modals.
// Two gate modes capture both flows we need today:
//
//   - twoStep: first click flips to a second confirm button. Used for delete
//     actions where the consequence is described in copy (e.g. devices).
//   - textConfirm: the user must type a known string before the confirm
//     button enables. Used where we want stronger friction (persons, since
//     soft-delete anonymizes data and is irreversible).
//
// The shell owns gate state, the destructive button, the cancel button, the
// error region and the visual frame. Consumers wire the API call through
// `onConfirm` and pass any domain-specific copy/warning via `description`
// and `warningSlot`. The shell intentionally does NOT log — the consumer
// keeps the createLogger scope for the action it owns.

type GateConfig =
  | {
      readonly mode: "twoStep";
      readonly firstStepLabel?: string;
    }
  | {
      readonly mode: "textConfirm";
      readonly expected: string;
      readonly inputId: string;
      readonly label: ReactNode;
      readonly placeholder?: string;
      readonly preview?: ReactNode;
    };

interface ConfirmDeleteModalProps {
  readonly isOpen: boolean;
  readonly title: string;
  readonly description: ReactNode;
  readonly warningSlot?: ReactNode;
  readonly gate: GateConfig;
  // Externally-driven gate. When true, the destructive flow is blocked: the
  // two-step "advance" button and the final confirm button are both disabled.
  // Used to hold deletion until a prerequisite (e.g. a blast-radius preview)
  // has loaded successfully.
  readonly confirmDisabled?: boolean;
  readonly onConfirm: () => Promise<void> | void;
  readonly onClose: () => void;
  readonly loading: boolean;
  readonly error: string;
  readonly confirmLabel?: string;
  readonly loadingLabel?: string;
}

export function ConfirmDeleteModal({
  isOpen,
  title,
  description,
  warningSlot,
  gate,
  confirmDisabled: externalConfirmDisabled = false,
  onConfirm,
  onClose,
  loading,
  error,
  confirmLabel = "Endgültig löschen",
  loadingLabel = "Wird gelöscht...",
}: ConfirmDeleteModalProps) {
  const [confirmed, setConfirmed] = useState(false);
  const [textInput, setTextInput] = useState("");

  // Reset internal gate state whenever the modal is closed externally so the
  // next open is always a fresh confirmation flow.
  useEffect(() => {
    if (!isOpen) {
      setConfirmed(false);
      setTextInput("");
    }
  }, [isOpen]);

  const close = useCallback(() => {
    setConfirmed(false);
    setTextInput("");
    onClose();
  }, [onClose]);

  const inFirstStep = gate.mode === "twoStep" && !confirmed;
  const textGatePassed =
    gate.mode === "textConfirm" && textInput === gate.expected;
  // The external gate blocks both advancing the two-step flow and the final
  // confirm, so the flow cannot be completed while the prerequisite is unmet.
  const gateBlocked = loading || externalConfirmDisabled;
  const confirmDisabled =
    gateBlocked || (gate.mode === "textConfirm" && !textGatePassed);

  const footer = (
    <>
      <Button
        type="button"
        variant="outline"
        size="md"
        onClick={close}
        disabled={loading}
        className="disabled:cursor-not-allowed"
      >
        Abbrechen
      </Button>
      {inFirstStep ? (
        <Button
          type="button"
          variant="danger"
          size="md"
          onClick={() => setConfirmed(true)}
          disabled={gateBlocked}
          className="disabled:cursor-not-allowed"
        >
          {gate.mode === "twoStep" && gate.firstStepLabel
            ? gate.firstStepLabel
            : "Ja, löschen"}
        </Button>
      ) : (
        <Button
          type="button"
          variant="danger"
          size="md"
          onClick={() => void onConfirm()}
          disabled={confirmDisabled}
          className="disabled:cursor-not-allowed"
        >
          {loading ? loadingLabel : confirmLabel}
        </Button>
      )}
    </>
  );

  return (
    <Modal
      isOpen={isOpen}
      onClose={close}
      title={title}
      footer={footer}
      isDismissDisabled={loading}
      isBackdropDismissDisabled
    >
      <div className="text-sm text-gray-600">{description}</div>

      {warningSlot && <div className="mt-3">{warningSlot}</div>}

      {gate.mode === "textConfirm" && (
        <div className="mt-4">
          <label
            htmlFor={gate.inputId}
            className="block text-sm font-medium text-gray-700"
          >
            {gate.label}
          </label>
          {gate.preview && (
            <div className="mb-1 text-sm font-medium text-gray-900">
              {gate.preview}
            </div>
          )}
          <input
            id={gate.inputId}
            type="text"
            value={textInput}
            onChange={(event) => setTextInput(event.target.value)}
            placeholder={gate.placeholder}
            className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-[#FF3130] focus:ring-1 focus:ring-[#FF3130] focus:outline-none"
            autoComplete="off"
          />
        </div>
      )}

      {error && (
        <div className="mt-3 rounded-lg bg-[#FF3130]/10 px-3 py-2 text-sm text-[#CC2626]">
          {error}
        </div>
      )}
    </Modal>
  );
}

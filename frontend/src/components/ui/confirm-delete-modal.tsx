"use client";

import { type ReactNode, useCallback, useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { FocusScope } from "@radix-ui/react-focus-scope";

// Shared destructive-confirmation shell for every destructive action in the
// product. BAUARTEN-SPEC Bauart 2 Regel 6: Löschen ist portalweit dieses
// Bauteil — kein window.confirm, kein eigenes Löschmodal je Domäne.
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
  loadingLabel = "Wird gelöscht…",
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

  if (!isOpen) return null;

  const inFirstStep = gate.mode === "twoStep" && !confirmed;
  const textGatePassed =
    gate.mode === "textConfirm" && textInput === gate.expected;
  // The external gate blocks both advancing the two-step flow and the final
  // confirm, so the flow cannot be completed while the prerequisite is unmet.
  const gateBlocked = loading || externalConfirmDisabled;
  const confirmDisabled =
    gateBlocked || (gate.mode === "textConfirm" && !textGatePassed);

  const modalContent = (
    // Wrapping in FocusScope pushes a new entry onto Radix's focusScopesStack,
    // which auto-pauses any parent scope — the Modal/FormModal this dialog is
    // usually opened from, or a Vaul drawer. Without it the parent trap treats
    // the body-portaled confirmation as "outside" and pulls focus straight
    // back, so the confirm/cancel buttons are unreachable by keyboard and a
    // textConfirm input cannot hold focus. Matches Modal/FormModal.
    <FocusScope asChild loop trapped>
      {/* z-[9999] matches the kit Modal/FormModal overlays: expanded
          moto-content-surface cards carry z-index 60 (globals.css :has rule),
          which painted over the old z-50 overlay when the modal sat next to a
          SectionCard (#1424). */}
      <div
        data-modal-focus-scope="true"
        className="fixed inset-0 z-[9999] flex items-center justify-center bg-black/50"
        // pointerEvents: 'auto' is required when this dialog is rendered while
        // a Radix/Vaul dialog (e.g. the mobile master/detail drawer) has set
        // `document.body { pointer-events: none }`. Portaled to body, the
        // overlay otherwise inherits `none` and the confirm/cancel buttons are
        // dead. Matches the kit Modal/FormModal overlays.
        style={{ pointerEvents: "auto" }}
      >
        <div className="mx-4 w-full max-w-md rounded-2xl bg-white p-6 shadow-xl">
          <h3 className="text-lg font-semibold text-gray-900">{title}</h3>
          <div className="mt-2 text-sm text-gray-600">{description}</div>

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
                className="focus:border-moto-red focus:ring-moto-red w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:ring-1 focus:outline-none"
                autoComplete="off"
              />
            </div>
          )}

          {error && (
            <div className="bg-moto-red/10 text-moto-red-strong mt-3 rounded-lg px-3 py-2 text-sm">
              {error}
            </div>
          )}

          <div className="mt-5 flex justify-end gap-3">
            <button
              type="button"
              onClick={close}
              disabled={loading}
              className="rounded-lg border border-gray-200 px-4 py-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-50 disabled:opacity-50"
            >
              Abbrechen
            </button>
            {inFirstStep ? (
              <button
                type="button"
                onClick={() => setConfirmed(true)}
                disabled={gateBlocked}
                className="bg-moto-red hover:bg-moto-red-hover rounded-lg px-4 py-2 text-sm font-medium text-white transition-colors disabled:cursor-not-allowed disabled:opacity-50"
              >
                {gate.mode === "twoStep" && gate.firstStepLabel
                  ? gate.firstStepLabel
                  : "Ja, löschen"}
              </button>
            ) : (
              <button
                type="button"
                onClick={() => void onConfirm()}
                disabled={confirmDisabled}
                className="bg-moto-red hover:bg-moto-red-hover rounded-lg px-4 py-2 text-sm font-medium text-white transition-colors disabled:cursor-not-allowed disabled:opacity-50"
              >
                {loading ? loadingLabel : confirmLabel}
              </button>
            )}
          </div>
        </div>
      </div>
    </FocusScope>
  );

  // Rendered into the body like the kit Modal/FormModal. A `fixed` overlay left
  // inside the tree is positioned against the nearest ancestor carrying a
  // filter, transform or backdrop-filter — and `moto-content-surface` (every
  // SectionCard, InfoCard and StatCard) carries `backdrop-filter`. Nested in a
  // card the dialog would be laid out inside that card and clipped by its
  // `overflow-hidden`, with the page behind it still interactive.
  if (typeof document !== "undefined") {
    return createPortal(modalContent, document.body);
  }

  return modalContent;
}

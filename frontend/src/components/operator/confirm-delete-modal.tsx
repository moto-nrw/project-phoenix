"use client";

import { type ReactNode, useCallback, useEffect, useState } from "react";

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

  if (!isOpen) return null;

  const inFirstStep = gate.mode === "twoStep" && !confirmed;
  const textGatePassed =
    gate.mode === "textConfirm" && textInput === gate.expected;
  const confirmDisabled =
    loading || (gate.mode === "textConfirm" && !textGatePassed);

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
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
              className="rounded-lg bg-[#FF3130] px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-[#CC2626]"
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
              className="rounded-lg bg-[#FF3130] px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-[#CC2626] disabled:cursor-not-allowed disabled:opacity-50"
            >
              {loading ? loadingLabel : confirmLabel}
            </button>
          )}
        </div>
      </div>
    </div>
  );
}

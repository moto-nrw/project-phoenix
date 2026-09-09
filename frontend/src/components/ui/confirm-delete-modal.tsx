"use client";

import { type ReactNode, useCallback, useEffect, useState } from "react";
import { Button } from "./button";
import { ChoiceTile } from "./choice-tile";
import { Modal } from "./modal";
import { Radio } from "./radio";

// Shared destructive-confirmation shell for every destructive action in the
// product. BAUARTEN-SPEC Bauart 2 Regel 6: Löschen ist portalweit dieses
// Bauteil — kein window.confirm, kein eigenes Löschmodal je Domäne, kein
// ConfirmationModal für Löschen (Ratsche `bauart/one-delete-confirm`).
// Two gate modes capture both flows we need today:
//
//   - twoStep: first click flips to a second confirm button. Used for delete
//     actions where the consequence is described in copy (e.g. devices).
//   - textConfirm: the user must type a known string before the confirm
//     button enables. Used where we want stronger friction (persons, since
//     soft-delete anonymizes data and is irreversible).
//
// Where a deletion needs a scope first (only this occurrence or the whole
// series, only this child or the whole profile), the `scope` slot renders
// the choice inside the same dialog (#3110) instead of stacking a
// ChoiceModal in front. Picking a scope IS the first deliberate step of the
// two-step gate: the confirm button stays disabled until a scope is chosen
// and then runs the deletion directly, so a scoped deletion costs the same
// two interactions as an unscoped one. textConfirm keeps its typed gate on
// top of the scope.
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

export interface ConfirmDeleteScopeOption {
  readonly value: string;
  readonly label: string;
  readonly description?: string;
  /** Offers the scope but blocks it, e.g. when its prerequisite failed to
   *  load. Keeping the option visible explains what exists. */
  readonly disabled?: boolean;
}

export interface ConfirmDeleteScope {
  /** Question above the options, e.g. „Was soll gelöscht werden?“ */
  readonly label: string;
  /** Radio group name; also prefixes the option ids. */
  readonly name: string;
  readonly options: ReadonlyArray<ConfirmDeleteScopeOption>;
  /** Controlled selection; `null` until the user has chosen. */
  readonly value: string | null;
  readonly onChange: (value: string) => void;
}

interface ConfirmDeleteModalProps {
  readonly isOpen: boolean;
  readonly title: string;
  readonly description: ReactNode;
  readonly warningSlot?: ReactNode;
  readonly gate: GateConfig;
  /** Scope choice rendered between description and warning (see above). */
  readonly scope?: ConfirmDeleteScope;
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
  scope,
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

  const scopeChosen =
    scope === undefined ||
    scope.options.some(
      (option) => option.value === scope.value && !option.disabled,
    );
  // A scoped two-step dialog has no separate "Ja, löschen" click: the scope
  // pick is that step.
  const inFirstStep =
    gate.mode === "twoStep" && scope === undefined && !confirmed;
  // An empty expected string never passes: a consumer whose expected value
  // has not loaded yet must not find the gate already open.
  const textGatePassed =
    gate.mode === "textConfirm" &&
    gate.expected !== "" &&
    textInput === gate.expected;
  // The external gate blocks both advancing the two-step flow and the final
  // confirm, so the flow cannot be completed while the prerequisite is unmet.
  const gateBlocked = loading || externalConfirmDisabled || !scopeChosen;
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

      {scope && (
        <fieldset className="mt-4" disabled={loading}>
          <legend className="mb-2 text-sm font-medium text-gray-700">
            {scope.label}
          </legend>
          <div className="flex flex-col gap-2">
            {scope.options.map((option) => {
              const id = `${scope.name}-${option.value}`;
              const disabled = loading || option.disabled === true;
              return (
                <ChoiceTile
                  key={option.value}
                  htmlFor={id}
                  selected={scope.value === option.value}
                  disabled={disabled}
                  className="items-start p-3"
                >
                  <Radio
                    id={id}
                    name={scope.name}
                    value={option.value}
                    checked={scope.value === option.value}
                    disabled={disabled}
                    onChange={() => scope.onChange(option.value)}
                    className="mt-0.5"
                  />
                  <span className="min-w-0 flex-1">
                    <span className="block text-sm font-medium text-gray-900">
                      {option.label}
                    </span>
                    {option.description && (
                      <span className="mt-0.5 block text-xs font-normal text-gray-600">
                        {option.description}
                      </span>
                    )}
                  </span>
                </ChoiceTile>
              );
            })}
          </div>
        </fieldset>
      )}

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
    </Modal>
  );
}

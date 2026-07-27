"use client";

/**
 * ChoiceModal — generic N-option scope dialog built on the shared Modal
 * (inherits portal, z-[9999], FocusScope and useModal registration).
 *
 * Use it when an action needs the user to pick exactly one of a small set
 * of scopes/strategies before proceeding, e.g. "Nur diesen Termin" vs.
 * "Diesen und alle folgenden". Options render as a vertical stack of
 * full-width buttons; selecting one calls onSelect with its value. The
 * caller owns closing the dialog (or setting isBusy while it works).
 */

import { Button } from "./button";
import { Modal } from "./modal";

interface ChoiceModalOption {
  value: string;
  label: string;
  description?: string;
  /** Offers the scope but blocks it — e.g. when the data it needs failed to
   *  load. Keeping the option visible explains what exists; hiding it would
   *  make the dialog silently different from one attempt to the next. */
  disabled?: boolean;
}

interface ChoiceModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly title: string;
  readonly description?: string;
  readonly options: Array<ChoiceModalOption>;
  readonly onSelect: (value: string) => void;
  /** Disables all options (and Abbrechen) while the caller is working. */
  readonly isBusy?: boolean;
}

export function ChoiceModal({
  isOpen,
  onClose,
  title,
  description,
  options,
  onSelect,
  isBusy = false,
}: ChoiceModalProps) {
  // While the caller is working, every dismiss path (backdrop, Escape,
  // close button, Abbrechen) must be inert — closing mid-save would let
  // the user re-trigger an already-running mutation.
  const handleClose = () => {
    if (isBusy) return;
    onClose();
  };

  const footer = (
    <Button
      type="button"
      variant="outline"
      size="md"
      onClick={handleClose}
      disabled={isBusy}
    >
      Abbrechen
    </Button>
  );

  return (
    <Modal isOpen={isOpen} onClose={handleClose} title={title} footer={footer}>
      {description && (
        <p className="mb-4 text-sm text-gray-600">{description}</p>
      )}
      <div role="group" className="flex flex-col gap-2">
        {options.map((option) => (
          <Button
            key={option.value}
            type="button"
            variant="outline"
            size="md"
            disabled={isBusy || option.disabled}
            onClick={() => onSelect(option.value)}
            className="w-full"
          >
            <span className="flex w-full flex-col items-start text-left">
              <span className="text-sm font-semibold text-gray-900">
                {option.label}
              </span>
              {option.description && (
                <span className="text-xs font-normal text-gray-500">
                  {option.description}
                </span>
              )}
            </span>
          </Button>
        ))}
      </div>
    </Modal>
  );
}

"use client";

import type { ReactNode } from "react";
import { Modal, type ModalSize } from "./modal";
import { DialogActions, DialogErrorSlot } from "./dialog-parts";
import type { FormErrorInput } from "./form-error";
import type { FormModalPending } from "./use-form-modal-pending";

interface FormModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly title: string;
  readonly children: ReactNode;
  /**
   * Footer actions. A dismissal action uses `ModalCancelButton`, which
   * `isDismissDisabled` locks; submit actions stay independent of it.
   */
  readonly footer?: ReactNode;
  readonly size?: ModalSize;
  // Where to position the modal on mobile viewports
  // 'bottom' mimics a bottom sheet; 'center' behaves like a classic modal
  readonly mobilePosition?: "bottom" | "center";
  /**
   * The pending contract from `useFormModalPending` (#6533). While its save
   * runs, every dismissal path (close X, backdrop, Escape) and every footer
   * action is locked and the dialog is `aria-busy`: a request that still
   * commits must not look cancelled. Every dialog that saves passes it;
   * `ui-kit/form-modal-pending` fails a `FormModal` without it.
   */
  readonly pending?: FormModalPending;
  /**
   * Locks every way to close the dialog without a running operation; the
   * one name for this lock in every kit dialog (#6575). A running `pending` save locks it too.
   */
  readonly isDismissDisabled?: boolean;
  /** Keep backdrop taps from discarding in-progress form input (#3370). */
  readonly isBackdropDismissDisabled?: boolean;
  /**
   * Temporarily removes this surface while a child confirmation is shown,
   * without unmounting the child and losing its pending state.
   */
  readonly suspended?: boolean;
  /**
   * The form's validation or save error. Rendered as an `Alert` at the top
   * of the content area and scrolled into view when it changes, so every
   * form reports failure in the same place (BAUARTEN-SPEC, Bauart 2 Regel 5).
   * Field-level problems additionally go to the field (`Input` `error`).
   */
  readonly error?: FormErrorInput;
}

/**
 * FormModal — a form inside the kit `Modal`. The dialog shell (portal, focus
 * trap, scroll lock, backdrop, panel, close X, exit animation, dismissal lock)
 * is `Modal`'s; this adds only the form defaults: a wider dialog, a bottom
 * sheet on phones, the footer row (`DialogActions`), the error slot
 * (`DialogErrorSlot`) and the pending lock of `useFormModalPending`.
 */
export function FormModal({
  isOpen,
  onClose,
  title,
  children,
  footer,
  size = "lg",
  mobilePosition = "bottom",
  pending,
  isDismissDisabled = false,
  isBackdropDismissDisabled = false,
  suspended = false,
  error,
}: FormModalProps) {
  const busy = pending?.pending ?? false;

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={title}
      size={size}
      mobileSheet={mobilePosition === "bottom"}
      sheetFooter="row"
      isDismissDisabled={isDismissDisabled || busy}
      busy={busy}
      isBackdropDismissDisabled={isBackdropDismissDisabled}
      suspended={suspended}
      footer={
        footer ? (
          // Form actions stay one right-aligned row on every width
          // (`sheetFooter="row"`); the row locks while a save runs.
          <DialogActions locked={busy} className="m-0 flex-1 border-0 p-0">
            {footer}
          </DialogActions>
        ) : undefined
      }
    >
      <DialogErrorSlot error={error} />
      {children}
    </Modal>
  );
}

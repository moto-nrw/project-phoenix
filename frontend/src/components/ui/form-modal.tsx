"use client";

import { useEffect } from "react";
import type { ReactNode } from "react";
import { Modal, type ModalSize } from "./modal";
import { FormErrorAlert } from "./form-error-alert";
import type { FormErrorInput } from "./form-error";

interface FormModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly title: string;
  readonly children: ReactNode;
  readonly footer?: ReactNode;
  readonly size?: ModalSize;
  // Where to position the modal on mobile viewports
  // 'bottom' mimics a bottom sheet; 'center' behaves like a classic modal
  readonly mobilePosition?: "bottom" | "center";
  // Blocks every dismissal path (close icon, backdrop, Escape): for modals
  // whose in-flight request must not look cancelled while it still commits.
  readonly closeDisabled?: boolean;
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
 * sheet on phones, the footer row and the `FormErrorAlert` slot.
 */
export function FormModal({
  isOpen,
  onClose,
  title,
  children,
  footer,
  size = "lg",
  mobilePosition = "bottom",
  closeDisabled = false,
  isBackdropDismissDisabled = false,
  suspended = false,
  error,
}: FormModalProps) {
  const visible = isOpen && !suspended;

  // Window events kept from the former standalone shell (pinned by
  // form-modal.test.tsx); nothing in the app listens to them today.
  useEffect(() => {
    globalThis.dispatchEvent(
      new CustomEvent(visible ? "mobile-modal-open" : "mobile-modal-close"),
    );
  }, [visible]);

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={title}
      size={size}
      mobileSheet={mobilePosition === "bottom"}
      isDismissDisabled={closeDisabled}
      isBackdropDismissDisabled={isBackdropDismissDisabled}
      suspended={suspended}
      footer={
        footer ? (
          // Form actions stay one right-aligned row on every width; the
          // Modal footer around it adds the phone safe-area inset.
          <div className="flex flex-1 flex-wrap justify-end gap-3">
            {footer}
          </div>
        ) : undefined
      }
    >
      <FormErrorAlert message={error} className="mb-4" />
      {children}
    </Modal>
  );
}

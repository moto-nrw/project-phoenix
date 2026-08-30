"use client";

import React, { useEffect, useCallback } from "react";
import { createPortal } from "react-dom";
import { FocusScope } from "@radix-ui/react-focus-scope";
import { useModal } from "../dashboard/modal-context";
import { useScrollLock } from "~/components/ui/hooks/useScrollLock";
import { useLatest } from "~/lib/hooks/use-latest";
import {
  Drawer,
  DrawerClose,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
} from "~/components/ui/drawer";
import { BELOW_SM, useMediaQuery } from "~/lib/hooks/use-media-query";

// Shared a11y contract for all modal dialogs (also consumed by form-modal).
export const dialogAriaProps = {
  role: "dialog" as const,
  "aria-modal": true,
};

function getModalAnimationClass(
  isAnimating: boolean,
  isExiting: boolean,
): string {
  if (isAnimating && !isExiting) return "animate-modalEnter";
  if (isExiting) return "animate-modalExit";
  return "translate-y-8 scale-75 -rotate-1 opacity-0";
}

interface ModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly title: string;
  readonly children: React.ReactNode;
  readonly footer?: React.ReactNode;
  /**
   * Tailwind width utilities for the dialog container. Defaults to
   * `mx-4 w-[calc(100%-2rem)] max-w-lg` to keep existing form/confirm
   * modals unchanged. Pass a wider value (e.g. `max-w-4xl`) for detail
   * views that need more horizontal space.
   */
  readonly widthClass?: string;
  /**
   * Accessible label for the close button. Defaults to German; pass a
   * translated string on localized surfaces (e.g. the parents portal).
   */
  readonly closeLabel?: string;
  /**
   * Accessible label for the dismiss-on-tap backdrop. Defaults to German;
   * pass a translated string on localized surfaces.
   */
  readonly backdropLabel?: string;
  /** Prevent every dismissal path while an operation must finish in place. */
  readonly isDismissDisabled?: boolean;
  /**
   * Auf schmalen Schirmen als Sheet von unten statt als mittiges Fenster, mit
   * angehefteter Fussleiste und freiem Sicherheitsbereich. Ab `sm` bleibt es
   * das gewohnte mittige Fenster. Verlangt von der Eltern-App; alle anderen
   * Aufrufer bleiben unveraendert.
   */
  readonly mobileSheet?: boolean;
}

export function Modal(props: ModalProps) {
  const isPhone = useMediaQuery(BELOW_SM);

  if (props.mobileSheet && isPhone) {
    return <MobileSheetModal {...props} />;
  }

  return <DialogModal {...props} />;
}

function MobileSheetModal({
  isOpen,
  onClose,
  title,
  children,
  footer,
  closeLabel = "Modal schließen",
  isDismissDisabled = false,
}: ModalProps) {
  const { openModal, closeModal } = useModal();
  const onCloseRef = useLatest(onClose);

  useEffect(() => {
    if (!isOpen) return;
    openModal();
    return closeModal;
  }, [closeModal, isOpen, openModal]);

  const handleOpenChange = useCallback(
    (open: boolean) => {
      if (!open && !isDismissDisabled) onCloseRef.current();
    },
    [isDismissDisabled, onCloseRef],
  );

  if (!isOpen) return null;

  return (
    <Drawer
      open
      onOpenChange={handleOpenChange}
      dismissible={!isDismissDisabled}
    >
      <DrawerContent
        data-mobile-sheet="true"
        className="z-[9999] max-h-[calc(100dvh-env(safe-area-inset-top)-1rem)] overflow-hidden bg-white"
      >
        <DrawerHeader className="flex shrink-0 flex-row items-center justify-between border-b border-gray-100 px-4 pt-3 pb-4 text-left">
          <div className="min-w-0">
            <DrawerTitle className="text-lg leading-tight font-semibold text-gray-900">
              {title}
            </DrawerTitle>
            <DrawerDescription className="sr-only">{title}</DrawerDescription>
          </div>
          <DrawerClose asChild>
            <button
              type="button"
              disabled={isDismissDisabled}
              className="flex size-11 shrink-0 items-center justify-center rounded-xl text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 focus-visible:outline-2 focus-visible:outline-offset-2 active:bg-gray-200 disabled:cursor-not-allowed disabled:opacity-50"
              aria-label={closeLabel}
            >
              <svg
                className="size-6"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                strokeWidth={2}
                aria-hidden="true"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          </DrawerClose>
        </DrawerHeader>

        <div
          className="scrollbar-hidden min-h-0 flex-1 overflow-y-auto overscroll-contain p-4 leading-relaxed text-gray-700"
          data-modal-content="true"
        >
          {children}
        </div>

        {footer ? (
          <div className="flex shrink-0 flex-col gap-3 border-t border-gray-100 bg-white p-4 pb-[calc(1rem+env(safe-area-inset-bottom))]">
            {footer}
          </div>
        ) : null}
      </DrawerContent>
    </Drawer>
  );
}

function DialogModal({
  isOpen,
  onClose,
  title,
  children,
  footer,
  widthClass = "mx-4 w-[calc(100%-2rem)] max-w-lg",
  closeLabel = "Modal schließen",
  backdropLabel = "Hintergrund - Klicken zum Schließen",
  isDismissDisabled = false,
  mobileSheet = false,
}: ModalProps) {
  // Stable id so the dialog can reference its heading via aria-labelledby,
  // giving the dialog an accessible name (role="dialog" alone has none).
  const titleId = React.useId();
  const [isAnimating, setIsAnimating] = React.useState(false);
  const [isExiting, setIsExiting] = React.useState(false);
  const { openModal, closeModal } = useModal();

  // Store functions in refs to avoid effect re-runs
  const openModalRef = useLatest(openModal);
  const closeModalRef = useLatest(closeModal);

  // Use scroll lock hook
  useScrollLock(isOpen);

  // Store onClose in a ref so handleClose never changes identity
  const onCloseRef = useLatest(onClose);
  const isDismissDisabledRef = useLatest(isDismissDisabled);

  // Pending dismissal (exit-animation delay before onClose). Tracked so a
  // confirmation that starts during the 250ms window can cancel it —
  // otherwise the queued onClose would hide the modal while the confirmed
  // operation is still running.
  const dismissTimerRef = React.useRef<ReturnType<typeof setTimeout> | null>(
    null,
  );

  // Enhanced close handler with exit animation (stable — no deps on onClose)
  const handleClose = useCallback(() => {
    if (isDismissDisabledRef.current) return;
    if (dismissTimerRef.current !== null) return;
    setIsExiting(true);
    setIsAnimating(false);

    // Delay actual close to allow exit animation
    dismissTimerRef.current = setTimeout(() => {
      dismissTimerRef.current = null;
      // Backstop for a confirmation that started during the exit animation
      // but whose isDismissDisabled prop hasn't reached the effect below yet.
      if (isDismissDisabledRef.current) {
        setIsExiting(false);
        setIsAnimating(true);
        return;
      }
      onCloseRef.current();
    }, 250);
  }, [isDismissDisabledRef, onCloseRef]);

  // Cancel an in-flight dismissal as soon as dismissal gets locked, and bring
  // the dialog back from its exit animation.
  useEffect(() => {
    if (isDismissDisabled && dismissTimerRef.current !== null) {
      clearTimeout(dismissTimerRef.current);
      dismissTimerRef.current = null;
      setIsExiting(false);
      setIsAnimating(true);
    }
  }, [isDismissDisabled]);

  useEffect(() => {
    return () => {
      if (dismissTimerRef.current !== null) {
        clearTimeout(dismissTimerRef.current);
      }
    };
  }, []);

  // Handle modal context state for blur overlay
  useEffect(() => {
    if (isOpen) {
      const openModal = openModalRef.current;
      const closeModal = closeModalRef.current;
      openModal();
      return () => {
        closeModal();
      };
    }
  }, [closeModalRef, isOpen, openModalRef]);

  // Handle escape key and animations
  useEffect(() => {
    if (!isOpen) {
      setIsAnimating(false);
      setIsExiting(false);
      return;
    }

    // Ignore the Escape that is already dispatching when this listener is
    // attached: a Vaul drawer closes synchronously DURING the keydown, so a
    // modal that remounts in that same dispatch (e.g. the Betreuungsplan
    // detail modal resuming after the Termin-Editor, #1956) would receive
    // the very same event and immediately close itself. Deliberately NOT a
    // timestamp comparison — WebKit reports event.timeStamp on a wall-clock
    // basis (not time-origin relative, webkit.org/b/211101), so it cannot
    // be compared against performance.now(). window.event is deprecated but
    // exactly identifies the in-flight event; outside a dispatch it is
    // undefined and the guard never triggers.
    const inFlightEvent = window.event;
    const handleEscKey = (event: KeyboardEvent) => {
      if (event.key === "Escape" && event !== inFlightEvent) {
        handleClose();
      }
    };

    document.addEventListener("keydown", handleEscKey);

    // Trigger entrance animation with slight delay for smooth effect
    const animationTimer = setTimeout(() => {
      setIsAnimating(true);
    }, 10);

    return () => {
      document.removeEventListener("keydown", handleEscKey);
      clearTimeout(animationTimer);
    };
  }, [isOpen, handleClose]);

  if (!isOpen) return null;

  const modalContent = (
    // Wrapping in FocusScope pushes a new entry onto Radix's focusScopesStack,
    // which auto-pauses any parent FocusScope (notably Vaul's drawer focus
    // trap). Without this, taps on inputs inside this modal are stolen back
    // by the drawer because the modal is portaled to document.body and counts
    // as "outside" the drawer's scope.
    <FocusScope asChild loop trapped>
      <div
        data-modal-focus-scope="true"
        className={`fixed inset-0 z-[9999] flex justify-center ${
          mobileSheet ? "items-end sm:items-center" : "items-center"
        }`}
        // pointerEvents: 'auto' is required when this modal is rendered while
        // a Radix/Vaul dialog (e.g. the mobile master/detail drawer) has set
        // `document.body { pointer-events: none }`. Without this, the modal
        // is portaled to body and inherits `none`, leaving inputs unclickable.
        style={{
          position: "fixed",
          top: 0,
          left: 0,
          right: 0,
          bottom: 0,
          pointerEvents: "auto",
        }}
      >
        {/* Backdrop button - dismiss-on-tap target. Excluded from the tab
            order (tabIndex=-1) so the FocusScope auto-focus doesn't land on
            this invisible control; ESC is the keyboard equivalent. */}
        <button
          type="button"
          tabIndex={-1}
          onClick={handleClose}
          disabled={isDismissDisabled}
          aria-label={backdropLabel}
          className={`absolute inset-0 cursor-default border-none bg-transparent p-0 transition-all duration-200 ease-out ${
            isAnimating && !isExiting ? "bg-black/40" : "bg-black/0"
          }`}
          style={{
            animation:
              isAnimating && !isExiting
                ? "backdropEnter 200ms ease-out"
                : undefined,
          }}
        />
        {/* Dialog container */}
        <div
          className={`relative flex transform flex-col overflow-hidden overscroll-contain border border-gray-200/50 shadow-2xl ${
            mobileSheet
              ? "max-h-[calc(100dvh-3rem)] w-full max-w-none rounded-t-2xl sm:mx-4 sm:max-h-[calc(100dvh-2rem)] sm:w-[calc(100%-2rem)] sm:max-w-lg sm:rounded-2xl"
              : `${widthClass} max-h-[calc(100dvh-2rem)] rounded-2xl`
          } ${getModalAnimationClass(isAnimating, isExiting)}`}
          {...dialogAriaProps}
          aria-labelledby={title ? titleId : undefined}
          style={{
            background:
              "linear-gradient(135deg, rgba(255,255,255,0.95) 0%, rgba(248,250,252,0.98) 100%)",
            backdropFilter: "blur(20px)",
            boxShadow:
              "0 25px 50px -12px rgba(0, 0, 0, 0.25), 0 8px 16px -8px rgba(80, 128, 216, 0.15)",
            animationFillMode: "both",
          }}
        >
          {/* Header with close button - only show border if title exists */}
          {title ? (
            <div className="flex shrink-0 items-start gap-4 border-b border-gray-100 p-4 sm:p-6">
              <h3
                id={titleId}
                className="min-w-0 flex-1 text-lg font-semibold wrap-anywhere text-gray-900 sm:text-xl"
              >
                {title}
              </h3>
              <button
                type="button"
                onClick={handleClose}
                disabled={isDismissDisabled}
                className="group relative flex size-11 shrink-0 items-center justify-center rounded-xl text-gray-400 transition-all duration-200 hover:scale-105 hover:bg-gray-100 hover:text-gray-600 active:scale-95"
                aria-label={closeLabel}
              >
                {/* Animated X icon */}
                <svg
                  className="h-5 w-5 transition-transform duration-200 group-hover:rotate-90"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  strokeWidth={2}
                  aria-hidden="true"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M6 18L18 6M6 6l12 12"
                  />
                </svg>

                {/* Subtle hover glow */}
                <div
                  className="absolute inset-0 rounded-xl opacity-0 transition-opacity duration-200 group-hover:opacity-100"
                  style={{
                    boxShadow: "0 0 12px rgba(80,128,216,0.3)",
                  }}
                />
              </button>
            </div>
          ) : (
            /* X button positioned absolutely in top-right when no title */
            <button
              type="button"
              onClick={handleClose}
              disabled={isDismissDisabled}
              className="group absolute top-4 right-4 z-10 rounded-xl p-2 text-gray-400 transition-all duration-200 hover:scale-105 hover:bg-gray-100 hover:text-gray-600 active:scale-95"
              aria-label={closeLabel}
            >
              {/* Animated X icon */}
              <svg
                className="h-5 w-5 transition-transform duration-200 group-hover:rotate-90"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                strokeWidth={2}
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>

              {/* Subtle hover glow */}
              <div
                className="absolute inset-0 rounded-xl opacity-0 transition-opacity duration-200 group-hover:opacity-100"
                style={{
                  boxShadow: "0 0 12px rgba(80,128,216,0.3)",
                }}
              />
            </button>
          )}

          {/* Content area with hidden scrollbar and reveal animation */}
          <div
            className="scrollbar-hidden min-h-0 flex-1 overflow-y-auto overscroll-contain"
            data-modal-content="true"
          >
            <div
              className={`p-4 leading-relaxed text-gray-700 md:p-6 ${
                isAnimating && !isExiting
                  ? "animate-contentReveal"
                  : "opacity-0"
              }`}
            >
              {children}
            </div>
          </div>

          {/* Footer if provided */}
          {footer && (
            <div
              className={`flex shrink-0 gap-3 border-t border-gray-100 bg-gray-50/50 p-4 sm:p-6 ${
                mobileSheet
                  ? "flex-col pb-[calc(1rem+env(safe-area-inset-bottom))] sm:flex-row sm:justify-end sm:pb-6"
                  : "justify-end"
              }`}
            >
              {footer}
            </div>
          )}
        </div>
      </div>
    </FocusScope>
  );

  // Render to body to avoid any positioning issues
  if (typeof document !== "undefined") {
    return createPortal(modalContent, document.body);
  }

  return modalContent;
}

// A specialized confirmation modal with yes/no buttons
interface ConfirmationModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onConfirm: () => void;
  readonly title: string;
  readonly children: React.ReactNode;
  readonly confirmText?: string;
  readonly cancelText?: string;
  readonly isConfirmLoading?: boolean;
  readonly isConfirmDisabled?: boolean;
  /**
   * Lock every dismissal path (Escape, backdrop, X, Abbrechen) while the
   * confirm operation runs — opt-in, off by default so a stalled request can
   * still be cancelled. Use only when abandoning mid-flight would leave
   * inconsistent state (e.g. a multi-request operation).
   */
  readonly isDismissDisabled?: boolean;
  readonly confirmButtonClass?: string;
  /**
   * Text shown on the confirm button while isConfirmLoading. Defaults to
   * German; pass a translated string on localized surfaces.
   */
  readonly loadingText?: string;
  /** Forwarded to Modal — translated close-button aria-label. */
  readonly closeLabel?: string;
  /** Forwarded to Modal — translated backdrop aria-label. */
  readonly backdropLabel?: string;
  /** Render as a swipeable bottom sheet below the `sm` breakpoint. */
  readonly mobileSheet?: boolean;
}

export function ConfirmationModal({
  isOpen,
  onClose,
  onConfirm,
  title,
  children,
  confirmText = "Bestätigen",
  cancelText = "Abbrechen",
  isConfirmLoading = false,
  isConfirmDisabled = false,
  isDismissDisabled = false,
  confirmButtonClass = "bg-gray-900 hover:bg-gray-700",
  loadingText = "Wird geladen...",
  closeLabel,
  backdropLabel,
  mobileSheet = false,
}: ConfirmationModalProps) {
  const modalFooter = (
    <>
      <button
        type="button"
        onClick={onClose}
        disabled={isDismissDisabled}
        className={`${mobileSheet ? "hidden sm:block" : ""} flex-1 rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium whitespace-nowrap text-gray-700 transition-all duration-200 hover:scale-105 hover:border-gray-400 hover:bg-gray-50 hover:shadow-md active:scale-100 disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:scale-100 disabled:hover:border-gray-300 disabled:hover:bg-transparent disabled:hover:shadow-none`}
      >
        {cancelText}
      </button>

      <button
        type="button"
        onClick={onConfirm}
        disabled={isConfirmLoading || isConfirmDisabled}
        className={`flex-1 rounded-lg px-4 py-2 whitespace-nowrap ${confirmButtonClass} text-sm font-medium text-white transition-all duration-200 hover:scale-105 hover:shadow-lg active:scale-100 disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:scale-100`}
      >
        {isConfirmLoading ? (
          <span className="flex items-center justify-center gap-2">
            <svg
              className="h-4 w-4 animate-spin text-white"
              fill="none"
              viewBox="0 0 24 24"
            >
              <circle
                className="opacity-25"
                cx="12"
                cy="12"
                r="10"
                stroke="currentColor"
                strokeWidth="4"
              ></circle>
              <path
                className="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
              ></path>
            </svg>
            {loadingText}
          </span>
        ) : (
          confirmText
        )}
      </button>
    </>
  );

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={title}
      footer={modalFooter}
      isDismissDisabled={isDismissDisabled}
      closeLabel={closeLabel}
      backdropLabel={backdropLabel}
      mobileSheet={mobileSheet}
    >
      {children}
    </Modal>
  );
}

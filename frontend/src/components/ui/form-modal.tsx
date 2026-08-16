"use client";

import { useEffect, useCallback, useState, useId, useRef } from "react";
import type { ReactNode } from "react";
import { createPortal } from "react-dom";
import { FocusScope } from "@radix-ui/react-focus-scope";
import { useModal } from "../dashboard/modal-context";
import { useScrollLock } from "~/components/ui/hooks/useScrollLock";
import { dialogAriaProps } from "./modal";
import { useLatest } from "~/lib/hooks/use-latest";

interface FormModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly title: string;
  readonly children: ReactNode;
  readonly footer?: ReactNode;
  readonly size?: "sm" | "md" | "lg" | "xl";
  // Where to position the modal on mobile viewports
  // 'bottom' mimics a bottom sheet; 'center' behaves like a classic modal
  readonly mobilePosition?: "bottom" | "center";
  // Blocks every dismissal path (close icon, backdrop, Escape): for modals
  // whose in-flight request must not look cancelled while it still commits.
  readonly closeDisabled?: boolean;
}

export function FormModal({
  isOpen,
  onClose,
  title,
  children,
  footer,
  size = "lg",
  mobilePosition = "bottom",
  closeDisabled = false,
}: FormModalProps) {
  const [isAnimating, setIsAnimating] = useState(false);
  const [isExiting, setIsExiting] = useState(false);
  const titleId = useId();
  const { openModal, closeModal } = useModal();

  // Store functions in refs to avoid effect re-runs
  const openModalRef = useLatest(openModal);
  const closeModalRef = useLatest(closeModal);

  // Use scroll lock hook (handles overflow:hidden and event blocking)
  useScrollLock(isOpen);

  // Map size to max-width classes
  const sizeClasses = {
    sm: "max-w-md",
    md: "max-w-lg",
    lg: "max-w-2xl",
    xl: "max-w-4xl",
  };

  // Ref keeps handleClose referentially stable when the flag flips mid-save,
  // so the open/close effect below does not re-run and re-dispatch events.
  const closeDisabledRef = useLatest(closeDisabled);
  const pendingCloseTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Enhanced close handler with exit animation
  const handleClose = useCallback(() => {
    if (closeDisabledRef.current) return;
    setIsExiting(true);
    setIsAnimating(false);

    // Delay actual close to allow exit animation
    pendingCloseTimer.current = setTimeout(() => {
      pendingCloseTimer.current = null;
      // A save started during the exit delay wins over the queued close:
      // keep the modal mounted and bring it back until the request resolves.
      if (closeDisabledRef.current) {
        setIsExiting(false);
        setIsAnimating(true);
        return;
      }
      onClose();
    }, 250);
  }, [onClose, closeDisabledRef]);

  // A save started during the exit delay cancels the queued close for good.
  // The re-check inside the timeout is not enough on its own: a fast-failing
  // request can reset closeDisabled before the 250ms elapse, and the timeout
  // would then close the modal and discard the form state the user needs to
  // correct the error.
  useEffect(() => {
    if (closeDisabled && pendingCloseTimer.current) {
      clearTimeout(pendingCloseTimer.current);
      pendingCloseTimer.current = null;
      setIsExiting(false);
      setIsAnimating(true);
    }
  }, [closeDisabled]);

  useEffect(() => {
    return () => {
      if (pendingCloseTimer.current) clearTimeout(pendingCloseTimer.current);
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

  // Close on escape key press and handle animations
  useEffect(() => {
    let animationTimer: ReturnType<typeof setTimeout> | undefined;
    const handleEscKey = (event: KeyboardEvent) => {
      if (event.key === "Escape" && isOpen) {
        handleClose();
      }
    };

    if (isOpen) {
      document.addEventListener("keydown", handleEscKey);
      globalThis.dispatchEvent(new CustomEvent("mobile-modal-open"));

      animationTimer = setTimeout(() => {
        setIsAnimating(true);
      }, 10);
    } else {
      globalThis.dispatchEvent(new CustomEvent("mobile-modal-close"));
    }

    return () => {
      document.removeEventListener("keydown", handleEscKey);
      if (animationTimer) clearTimeout(animationTimer);
      if (!isOpen) {
        setIsAnimating(false);
        setIsExiting(false);
      }
    };
  }, [isOpen, handleClose]);

  if (!isOpen) return null;

  const radiusClass =
    mobilePosition === "bottom"
      ? "rounded-t-2xl md:rounded-2xl"
      : "rounded-2xl";
  const modalContent = (
    // Wrapping in FocusScope pushes a new entry onto Radix's focusScopesStack,
    // which auto-pauses any parent FocusScope (notably Vaul's drawer focus
    // trap). Without this, taps on inputs inside this modal are stolen back
    // by the drawer because the modal is portaled to document.body and counts
    // as "outside" the drawer's scope.
    <FocusScope asChild loop trapped>
      <div
        data-modal-focus-scope="true"
        className={`fixed inset-0 z-[9999] flex ${mobilePosition === "bottom" ? "items-end" : "items-center"} justify-center md:items-center md:p-6`}
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
          aria-label="Hintergrund - Klicken zum Schließen"
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
          // flex column so a tall footer (e.g. an expanded delete
          // confirmation) steals height from the scrollable content area
          // instead of being pushed past max-h and clipped by overflow-hidden.
          className={`relative flex w-full flex-col ${sizeClasses[size]} max-h-[90vh] md:max-h-[85vh] ${radiusClass} ${mobilePosition === "center" ? "mx-4" : ""} transform overflow-hidden border border-gray-200/50 shadow-2xl ${(() => {
            if (isAnimating && !isExiting) return "animate-modalEnter";
            if (isExiting) return "animate-modalExit";
            return "translate-y-8 scale-75 -rotate-1 opacity-0";
          })()}`}
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
          {/* Header with close button */}
          <div className="flex shrink-0 items-center justify-between border-b border-gray-100 p-4 md:p-6">
            {title && (
              <h3
                id={titleId}
                className="pr-4 text-lg font-semibold text-gray-900 md:text-xl"
              >
                {title}
              </h3>
            )}
            <button
              type="button"
              onClick={handleClose}
              className="group relative flex-shrink-0 rounded-xl p-2 text-gray-400 transition-all duration-200 hover:scale-105 hover:bg-gray-100 hover:text-gray-600 active:scale-95"
              aria-label="Modal schließen"
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
          </div>

          {/* Content area with custom scrollbar and reveal animation */}
          <div
            data-modal-content="true"
            className={`${footer ? "max-h-[calc(90vh-240px)] md:max-h-[calc(85vh-240px)]" : "max-h-[60vh] md:max-h-[70vh]"} min-h-0 scrollbar-thin scrollbar-thumb-gray-300 scrollbar-track-gray-100 overflow-y-auto`}
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

          {/* Footer if provided - now sticky at bottom */}
          {footer && (
            <div className="sticky bottom-0 flex shrink-0 flex-wrap justify-end gap-3 border-t border-gray-100 bg-gray-50/95 p-4 backdrop-blur-sm md:p-6">
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

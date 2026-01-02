import type React from "react";
import type { JSX } from "react";

// =============================================================================
// Event Handlers
// =============================================================================

/**
 * Stops event propagation for both click and keyboard events.
 * Used on modal content to prevent backdrop close when interacting with modal.
 * Note: Escape key is allowed to bubble so document-level close handler works.
 */
export const stopPropagation = {
  onClick: (e: React.MouseEvent) => e.stopPropagation(),
  onKeyDown: (e: React.KeyboardEvent) => {
    // Allow Escape to bubble up to document-level handler for modal close
    if (e.key !== "Escape") {
      e.stopPropagation();
    }
  },
};

/**
 * Common ARIA props for modal dialog container.
 */
export const dialogAriaProps = {
  role: "dialog" as const,
  "aria-modal": true,
};

/**
 * Returns the appropriate animation class for modal enter/exit transitions.
 * Used for consistent animation behavior across all modals.
 */
export function getModalAnimationClass(
  isAnimating: boolean,
  isExiting: boolean,
): string {
  if (isAnimating && !isExiting) return "animate-modalEnter";
  if (isExiting) return "animate-modalExit";
  return "translate-y-8 scale-75 -rotate-1 opacity-0";
}

/**
 * Returns the className for modal backdrop based on animation state.
 * Used for consistent backdrop styling across all modals.
 */
export function getBackdropClassName(
  isAnimating: boolean,
  isExiting: boolean,
): string {
  const bgClass = isAnimating && !isExiting ? "bg-black/40" : "bg-black/0";
  return `fixed inset-0 z-[9999] flex items-center justify-center transition-all duration-400 ease-out ${bgClass}`;
}

/**
 * Returns the style object for modal backdrop.
 */
export function getBackdropStyle(
  isAnimating: boolean,
  isExiting: boolean,
): React.CSSProperties {
  return {
    position: "fixed",
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    animation:
      isAnimating && !isExiting ? "backdropEnter 400ms ease-out" : undefined,
  };
}

/**
 * Common glassmorphism style for modal containers.
 */
export const modalContainerStyle: React.CSSProperties = {
  background:
    "linear-gradient(135deg, rgba(255,255,255,0.95) 0%, rgba(248,250,252,0.98) 100%)",
  backdropFilter: "blur(20px)",
  boxShadow:
    "0 25px 50px -12px rgba(0, 0, 0, 0.25), 0 8px 16px -8px rgba(80, 128, 216, 0.15)",
  animationFillMode: "both",
};

/**
 * Returns the className for modal dialog container.
 * Combines base styles with animation class.
 */
export function getModalDialogClassName(
  isAnimating: boolean,
  isExiting: boolean,
): string {
  return `relative mx-4 w-[calc(100%-2rem)] max-w-md transform overflow-hidden rounded-2xl border border-gray-200/50 shadow-2xl ${getModalAnimationClass(isAnimating, isExiting)}`;
}

/**
 * Common className for scrollable modal content area.
 */
export const scrollableContentClassName =
  "scrollbar-thin scrollbar-thumb-gray-300 scrollbar-track-gray-100 max-h-[calc(100vh-12rem)] overflow-y-auto sm:max-h-[calc(90vh-8rem)]";

/**
 * Returns the className for modal content animation wrapper.
 */
export function getContentAnimationClassName(
  isAnimating: boolean,
  isExiting: boolean,
): string {
  const animationClass =
    isAnimating && !isExiting ? "sm:animate-contentReveal" : "sm:opacity-0";
  return `p-4 md:p-6 ${animationClass}`;
}

// =============================================================================
// API Error Message Utilities
// =============================================================================

/**
 * Extracts a user-friendly error message from an API error.
 * Handles common HTTP status codes with German translations.
 *
 * @param err - The caught error object
 * @param action - The action being performed (e.g., "erstellen", "bearbeiten")
 * @param entityType - The type of entity (e.g., "Aktivitäten")
 * @param defaultMessage - Fallback message if no specific handling applies
 */
export function getApiErrorMessage(
  err: unknown,
  action: string,
  entityType: string,
  defaultMessage: string,
): string {
  if (!(err instanceof Error)) {
    return defaultMessage;
  }

  const message = err.message;

  if (message.includes("user is not authenticated")) {
    return `Sie müssen angemeldet sein, um ${entityType} zu ${action}.`;
  }
  if (message.includes("401")) {
    return "Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an.";
  }
  if (message.includes("403")) {
    return "Zugriff verweigert. Bitte melden Sie sich erneut an.";
  }
  if (message.includes("400")) {
    return "Ungültige Eingabedaten. Bitte überprüfen Sie Ihre Eingaben.";
  }

  return message;
}

// =============================================================================
// Reusable Modal UI Components (as JSX-returning functions)
// =============================================================================

/**
 * Props for the modal close button.
 */
interface ModalCloseButtonProps {
  onClose: () => void;
  ariaLabel?: string;
}

/**
 * Renders a close button with animated X icon and hover glow effect.
 * Used in modal headers for consistent close button styling.
 */
export function renderModalCloseButton({
  onClose,
  ariaLabel = "Modal schließen",
}: ModalCloseButtonProps): JSX.Element {
  return (
    <button
      onClick={onClose}
      className="group relative flex-shrink-0 rounded-xl p-2 text-gray-400 transition-all duration-200 hover:scale-105 hover:bg-gray-100 hover:text-gray-600 active:scale-95"
      aria-label={ariaLabel}
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
  );
}

/**
 * Props for the modal loading spinner.
 */
interface ModalLoadingSpinnerProps {
  message?: string;
}

/**
 * Renders a centered loading spinner with optional message.
 * Used for loading states in modal content areas.
 */
export function renderModalLoadingSpinner({
  message = "Wird geladen...",
}: ModalLoadingSpinnerProps = {}): JSX.Element {
  return (
    <div className="flex items-center justify-center py-12">
      <div className="flex flex-col items-center gap-4">
        <div className="h-12 w-12 animate-spin rounded-full border-t-2 border-b-2 border-blue-500"></div>
        <p className="text-gray-600">{message}</p>
      </div>
    </div>
  );
}

/**
 * Props for the modal error alert.
 */
interface ModalErrorAlertProps {
  message: string;
}

/**
 * Renders a styled error alert for modal forms.
 * Used for displaying validation and API errors.
 */
export function renderModalErrorAlert({
  message,
}: ModalErrorAlertProps): JSX.Element {
  return (
    <div className="relative overflow-hidden rounded-2xl border border-red-100 bg-red-50/80 p-4 backdrop-blur-sm">
      <div className="absolute inset-0 bg-gradient-to-br from-red-50/50 to-pink-50/50 opacity-50"></div>
      <div className="relative flex items-start gap-3">
        <div className="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg bg-red-100">
          <svg
            className="h-4 w-4 text-red-600"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={2.5}
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M12 9v3.75m9-.75a9 9 0 11-18 0 9 9 0 0118 0zm-9 3.75h.008v.008H12v-.008z"
            />
          </svg>
        </div>
        <p className="text-sm text-red-800">{message}</p>
      </div>
    </div>
  );
}

/**
 * Renders a button spinner SVG for loading states.
 * Used in submit buttons during form submission.
 */
export function renderButtonSpinner(): JSX.Element {
  return (
    <svg className="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
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
  );
}

/**
 * Props for the modal backdrop button.
 */
interface BackdropButtonProps {
  onClose: () => void;
  isAnimating: boolean;
  isExiting: boolean;
  ariaLabel?: string;
}

/**
 * Renders a native button element for modal backdrop.
 * Provides keyboard accessibility (Enter/Space) and proper screen reader support.
 * Used as a sibling to the modal dialog for proper accessibility structure.
 */
export function renderBackdropButton({
  onClose,
  isAnimating,
  isExiting,
  ariaLabel = "Hintergrund - Klicken zum Schließen",
}: BackdropButtonProps): JSX.Element {
  const bgClass = isAnimating && !isExiting ? "bg-black/40" : "bg-black/0";

  return (
    <button
      type="button"
      onClick={onClose}
      aria-label={ariaLabel}
      className={`absolute inset-0 cursor-default border-none bg-transparent p-0 transition-all duration-400 ease-out ${bgClass}`}
      style={getBackdropStyle(isAnimating, isExiting)}
    />
  );
}

/**
 * Props for the modal wrapper component.
 */
interface ModalWrapperProps {
  readonly onClose: () => void;
  readonly isAnimating: boolean;
  readonly isExiting: boolean;
  readonly children: React.ReactNode;
}

/**
 * Wrapper component for modal overlays.
 * Provides fixed positioning, backdrop button, and contains the modal dialog.
 * Centralizes the modal structure to prevent code duplication across modals.
 */
export function ModalWrapper({
  onClose,
  isAnimating,
  isExiting,
  children,
}: ModalWrapperProps): JSX.Element {
  return (
    <div
      className="fixed inset-0 z-[9999] flex items-center justify-center"
      style={{ position: "fixed", top: 0, left: 0, right: 0, bottom: 0 }}
    >
      {renderBackdropButton({ onClose, isAnimating, isExiting })}
      {children}
    </div>
  );
}

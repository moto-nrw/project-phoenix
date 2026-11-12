"use client";

import React, { useEffect, useCallback } from "react";
import { createPortal } from "react-dom";
import { FocusScope } from "@radix-ui/react-focus-scope";
import { useModal } from "./modal-context";
import { useScrollLock } from "~/components/ui/hooks/useScrollLock";
import { useLatest } from "~/lib/hooks/use-latest";
import {
  OVERLAY_BACKDROP_CLASS,
  OVERLAY_BACKDROP_TINT_CLASS,
  OVERLAY_BACKDROP_TINT_HIDDEN_CLASS,
  OVERLAY_PANEL_STYLE,
  Z_LAYER,
} from "./overlay-styles";
import { Button } from "./button";
import { CloseIcon } from "./close-icon";
import { SpinnerIcon } from "./icons";
import { useBusyFocus } from "./hooks/useBusyFocus";

const visibleModals: string[] = [];

const DEFAULT_CLOSE_LABEL = "Modal schließen";
// Neutral: Ob Eingaben verloren gehen, entscheidet der Inhalt, nicht das X.
const DEFAULT_CLOSE_TOOLTIP = "Schließt das Fenster.";

/** Confirm button of a confirmation dialog: the dialog already asked. */
export const CONFIRM_TOOLTIP =
  "Führt die Aktion sofort aus, ohne weitere Nachfrage.";

// Shared a11y contract for all modal dialogs.
const dialogAriaProps = {
  role: "dialog" as const,
  "aria-modal": true,
};

/** Maximum dialog width: `md` is the default confirmation/form width. */
export type ModalSize = "sm" | "md" | "lg" | "xl";

// Literal class strings so Tailwind finds them.
const MODAL_MAX_WIDTH: Record<ModalSize, string> = {
  sm: "max-w-md",
  md: "max-w-lg",
  lg: "max-w-2xl",
  xl: "max-w-4xl",
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
  /** Runs when an animated dismissal starts, before the delayed close callback. */
  readonly onDismissStart?: () => void;
  /** Resets the dialog animation without remounting its children. */
  readonly animationKey?: string;
  readonly title: string;
  /** Leading icon in the header, left of the title (e.g. a `ConceptIconTile`). */
  readonly icon?: React.ReactNode;
  /** One line under the title; becomes the dialog's accessible description. */
  readonly description?: React.ReactNode;
  readonly children: React.ReactNode;
  readonly footer?: React.ReactNode;
  /** Maximum dialog width; `md` (`max-w-lg`) by default. */
  readonly size?: ModalSize;
  /**
   * Tailwind width utilities for the dialog container. Overrides `size` on
   * the centered dialog; ignored with `mobileSheet`, which takes `size`.
   */
  readonly widthClass?: string;
  /**
   * Accessible label for the close button. Defaults to German; pass a
   * translated string on localized surfaces (e.g. the parents portal).
   */
  readonly closeLabel?: string;
  /**
   * What a click on the close X does (#6744). Defaults to a German sentence
   * while `closeLabel` keeps its German default; a localized surface that
   * passes `closeLabel` passes its own translated sentence here.
   */
  readonly closeTooltip?: string;
  /**
   * Accessible label for the dismiss-on-tap backdrop. Defaults to German;
   * pass a translated string on localized surfaces.
   */
  readonly backdropLabel?: string;
  /** Prevent every dismissal path while an operation must finish in place. */
  readonly isDismissDisabled?: boolean;
  /** Keep backdrop taps from discarding in-progress form input. */
  readonly isBackdropDismissDisabled?: boolean;
  /** Marks the dialog `aria-busy` while an operation runs inside it. */
  readonly busy?: boolean;
  /**
   * Auf schmalen Schirmen unten angeheftet und in voller Breite statt als
   * mittiges Fenster, mit Fussleiste über dem Sicherheitsbereich. Ab `sm`
   * bleibt es das gewohnte mittige Fenster. Bewusst kein Vaul-Drawer und
   * keine Wisch-Geste: dessen Zieh-Geste fing auf Android im Hochformat jede
   * Berührung ab (#3661). Geschlossen wird über X, Hintergrund oder Escape.
   */
  readonly mobileSheet?: boolean;
  /**
   * Fokus beim Öffnen auf den Titel statt auf das erste Bedienelement (das X).
   * Für Dialoge, die sich ohne Klick selbst öffnen: dort zeigte der Browser um
   * das X einen Tastatur-Fokusrahmen. Screenreader lesen den Titel vor, Tab
   * führt weiter zu den Aktionen (#3368).
   */
  readonly focusTitleOnOpen?: boolean;
  /**
   * Hides the dialog while a child confirmation is shown, without unmounting
   * the children and losing their pending state. While suspended the dialog
   * neither traps focus, locks scroll nor reacts to Escape.
   */
  readonly suspended?: boolean;
}

export function Modal({
  isOpen,
  onClose,
  onDismissStart,
  animationKey,
  title,
  icon,
  description,
  children,
  footer,
  size = "md",
  widthClass,
  closeLabel,
  closeTooltip,
  backdropLabel = "Hintergrund - Klicken zum Schließen",
  isDismissDisabled = false,
  isBackdropDismissDisabled = false,
  busy = false,
  mobileSheet = false,
  focusTitleOnOpen = false,
  suspended = false,
}: ModalProps) {
  // Stable id so the dialog can reference its heading via aria-labelledby,
  // giving the dialog an accessible name (role="dialog" alone has none).
  const titleId = React.useId();
  const closeName = closeLabel ?? DEFAULT_CLOSE_LABEL;
  const closeHint =
    closeTooltip ??
    (closeLabel === undefined ? DEFAULT_CLOSE_TOOLTIP : undefined);
  const descriptionId = React.useId();
  const visible = isOpen && !suspended;
  const titleRef = React.useRef<HTMLHeadingElement>(null);
  const panelRef = React.useRef<HTMLDivElement>(null);
  const [isAnimating, setIsAnimating] = React.useState(false);
  const [isExiting, setIsExiting] = React.useState(false);
  const { openModal, closeModal } = useModal();

  // Store functions in refs to avoid effect re-runs
  const openModalRef = useLatest(openModal);
  const closeModalRef = useLatest(closeModal);

  // Use scroll lock hook
  useScrollLock(visible);

  // Store onClose in a ref so handleClose never changes identity
  const onCloseRef = useLatest(onClose);
  const onDismissStartRef = useLatest(onDismissStart);
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
    onDismissStartRef.current?.();
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
  }, [isDismissDisabledRef, onCloseRef, onDismissStartRef]);

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

  // Keeps focus in the dialog while a busy footer is disabled (#6533).
  useBusyFocus(panelRef, busy);

  // Handle modal context state for blur overlay
  useEffect(() => {
    if (visible) {
      const openModal = openModalRef.current;
      const closeModal = closeModalRef.current;
      openModal();
      return () => {
        closeModal();
      };
    }
  }, [closeModalRef, visible, openModalRef]);

  useEffect(() => {
    if (!visible) return;
    visibleModals.push(titleId);
    return () => {
      const index = visibleModals.indexOf(titleId);
      if (index !== -1) visibleModals.splice(index, 1);
    };
  }, [visible, titleId]);

  // Handle escape key and animations
  useEffect(() => {
    if (!visible) {
      setIsAnimating(false);
      setIsExiting(false);
      return;
    }
    setIsAnimating(false);
    setIsExiting(false);

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
      if (
        event.key === "Escape" &&
        event !== inFlightEvent &&
        visibleModals.at(-1) === titleId
      ) {
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
  }, [visible, handleClose, animationKey, titleId]);

  if (!isOpen) return null;

  // Kept a direct child of the header row unless a description needs a
  // column, so a long title still shares the row with the close X.
  const heading = (
    <h3
      id={titleId}
      ref={titleRef}
      tabIndex={focusTitleOnOpen ? -1 : undefined}
      className="min-w-0 flex-1 py-0.5 text-lg font-semibold wrap-anywhere text-gray-900 outline-none sm:text-xl pointer-coarse:py-2"
    >
      {title}
    </h3>
  );

  const modalContent = (
    // Wrapping in FocusScope pushes a new entry onto Radix's focusScopesStack,
    // which auto-pauses any parent FocusScope (notably Vaul's drawer focus
    // trap). Without this, taps on inputs inside this modal are stolen back
    // by the drawer because the modal is portaled to document.body and counts
    // as "outside" the drawer's scope.
    <FocusScope
      asChild
      loop={!suspended}
      trapped={!suspended}
      onMountAutoFocus={
        focusTitleOnOpen && title
          ? (event) => {
              event.preventDefault();
              titleRef.current?.focus();
            }
          : undefined
      }
    >
      <div
        data-modal-focus-scope="true"
        className={`fixed inset-0 ${Z_LAYER.modal} ${suspended ? "hidden" : "flex"} justify-center ${
          mobileSheet ? "items-end sm:items-center" : "items-center"
        }`}
        aria-hidden={suspended || undefined}
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
          disabled={isDismissDisabled || isBackdropDismissDisabled}
          aria-label={backdropLabel}
          // No bg-transparent reset here: Tailwind emits it after bg-black/*
          // and the tint never rendered. The hidden state below is the reset.
          className={`absolute inset-0 cursor-default border-none p-0 ${OVERLAY_BACKDROP_CLASS} transition-all duration-200 ease-out ${
            isAnimating && !isExiting
              ? OVERLAY_BACKDROP_TINT_CLASS
              : OVERLAY_BACKDROP_TINT_HIDDEN_CLASS
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
          className={`relative flex transform flex-col overflow-hidden overscroll-contain border border-gray-200/50 shadow-2xl outline-none ${
            mobileSheet
              ? `max-h-[calc(100dvh-3rem-env(safe-area-inset-top))] w-full rounded-t-2xl max-sm:max-w-none sm:mx-4 sm:max-h-[calc(100dvh-2rem)] sm:w-[calc(100%-2rem)] sm:rounded-2xl ${MODAL_MAX_WIDTH[size]}`
              : `${widthClass ?? `mx-4 w-[calc(100%-2rem)] ${MODAL_MAX_WIDTH[size]}`} max-h-[calc(100dvh-2rem)] rounded-2xl`
          } ${getModalAnimationClass(isAnimating, isExiting)}`}
          {...dialogAriaProps}
          ref={panelRef}
          // Focus target while a busy dialog has disabled its own controls.
          tabIndex={busy ? -1 : undefined}
          aria-labelledby={title ? titleId : undefined}
          aria-describedby={title && description ? descriptionId : undefined}
          aria-busy={busy || undefined}
          style={{ ...OVERLAY_PANEL_STYLE, pointerEvents: "auto" }}
        >
          {/* Header with close button - only show border if title exists */}
          {title ? (
            <div className="flex shrink-0 items-start gap-4 border-b border-gray-100 p-4 sm:p-6">
              {icon ? <div className="shrink-0">{icon}</div> : null}
              {/* Das Polster hebt die 28-px-Zeile auf die Höhe des X (32 px
                  mit Maus, 44 px unter Fingerbedienung): eine Zeile steht
                  mittig neben dem X, lange Titel brechen darunter um. */}
              {description ? (
                <div className="min-w-0 flex-1">
                  {heading}
                  <p id={descriptionId} className="text-sm text-gray-500">
                    {description}
                  </p>
                </div>
              ) : (
                heading
              )}
              <Button
                type="button"
                variant="ghost"
                size="icon"
                onClick={handleClose}
                disabled={isDismissDisabled}
                className="shrink-0"
                aria-label={closeName}
                tooltip={closeHint}
                data-overlay-close=""
              >
                <CloseIcon />
              </Button>
            </div>
          ) : (
            /* X button positioned absolutely in top-right when no title */
            <Button
              type="button"
              variant="ghost"
              size="icon"
              onClick={handleClose}
              disabled={isDismissDisabled}
              className="absolute top-4 right-4 z-10"
              aria-label={closeName}
              tooltip={closeHint}
              data-overlay-close=""
            >
              <CloseIcon />
            </Button>
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

type ConfirmVariant = "primary" | "danger" | "warning" | "success";

// A specialized confirmation modal with yes/no buttons
interface ConfirmationModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onConfirm: () => void;
  readonly title: string;
  readonly children: React.ReactNode;
  readonly confirmText?: string;
  readonly cancelText?: string;
  /**
   * What the confirm button does (#6744), e.g. „Archiviert das Kind und
   * benachrichtigt die Eltern.“ Defaults to „Führt die Aktion sofort aus, ohne weitere
   * Nachfrage.“ while the dialog keeps its German `cancelText` and
   * `closeLabel`; a localized surface passes its own translated sentence.
   */
  readonly confirmTooltip?: string;
  /**
   * Why the confirm button is blocked while `isConfirmDisabled` is true; see
   * `Button`'s `disabledReason`.
   */
  readonly confirmDisabledReason?: string;
  /**
   * True while the confirmed operation runs. It also locks every dismissal
   * path (X, Escape, backdrop, Abbrechen) and marks the dialog `aria-busy`:
   * closing would look like a cancel while the request still commits, and
   * its answer would have no surface left (#6594).
   */
  readonly isConfirmLoading?: boolean;
  readonly isConfirmDisabled?: boolean;
  /** Lock every dismissal path regardless of `isConfirmLoading`. */
  readonly isDismissDisabled?: boolean;
  /**
   * Opt-out of the loading lock: the reason why leaving while the operation
   * runs is harmless (it only reads, or abandoning it is intended). A late
   * answer must then be bound to the dialog session (`useFormModalPending`'s
   * `op.isCurrent()`). `ui-kit/confirmation-dismiss-lock` requires a written
   * reason.
   */
  readonly dismissibleWhileLoading?: string;
  /** Forwarded to Modal — backdrop taps do not close the confirmation. */
  readonly isBackdropDismissDisabled?: boolean;
  /**
   * Tone of the confirm button, mapped onto the kit Button variants:
   * `primary` (neutral, default), `danger` (delete / irreversible),
   * `warning` (proceed despite a side effect), `success` (publish / release).
   */
  readonly confirmVariant?: ConfirmVariant;
  /**
   * Text shown on the confirm button while isConfirmLoading. Defaults to
   * German; pass a translated string on localized surfaces.
   */
  readonly loadingText?: string;
  /** Forwarded to Modal — translated close-button aria-label. */
  readonly closeLabel?: string;
  /** Forwarded to Modal — translated close-button sentence. */
  readonly closeTooltip?: string;
  /** Forwarded to Modal — translated backdrop aria-label. */
  readonly backdropLabel?: string;
  /** Bottom-anchored full-width dialog below `sm`; see `ModalProps.mobileSheet`. */
  readonly mobileSheet?: boolean;
}

export function ConfirmationModal({
  isOpen,
  onClose,
  onConfirm,
  title,
  children,
  confirmText = "Bestätigen",
  cancelText,
  confirmTooltip,
  confirmDisabledReason,
  isConfirmLoading = false,
  isConfirmDisabled = false,
  isDismissDisabled = false,
  dismissibleWhileLoading,
  isBackdropDismissDisabled = false,
  confirmVariant = "primary",
  loadingText = "Wird geladen...",
  closeLabel,
  closeTooltip,
  backdropLabel,
  mobileSheet = false,
}: ConfirmationModalProps) {
  // Der deutsche Standardsatz gilt nur, solange der Dialog deutsch spricht.
  const germanFrame = cancelText === undefined && closeLabel === undefined;
  const confirmHint =
    confirmTooltip ?? (germanFrame ? CONFIRM_TOOLTIP : undefined);
  const dismissLocked =
    isDismissDisabled || (isConfirmLoading && !dismissibleWhileLoading);
  const modalFooter = (
    <>
      <Button
        type="button"
        variant="outline"
        size="md"
        onClick={onClose}
        disabled={dismissLocked}
        className={`flex-1 whitespace-nowrap disabled:cursor-not-allowed ${
          mobileSheet ? "hidden sm:inline-flex" : ""
        }`}
        noTooltip="Abbrechen schließt den Dialog und ändert nichts."
      >
        {cancelText ?? "Abbrechen"}
      </Button>

      <Button
        type="button"
        variant={confirmVariant}
        size="md"
        onClick={onConfirm}
        disabled={isConfirmLoading || isConfirmDisabled}
        disabledReason={isConfirmLoading ? undefined : confirmDisabledReason}
        tooltip={confirmHint}
        className="flex-1 whitespace-nowrap disabled:cursor-not-allowed"
      >
        {isConfirmLoading ? (
          <span className="flex items-center justify-center gap-2">
            <SpinnerIcon className="h-4 w-4" />
            {loadingText}
          </span>
        ) : (
          confirmText
        )}
      </Button>
    </>
  );

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={title}
      footer={modalFooter}
      isDismissDisabled={dismissLocked}
      busy={isConfirmLoading}
      isBackdropDismissDisabled={isBackdropDismissDisabled}
      closeLabel={closeLabel}
      closeTooltip={closeTooltip}
      backdropLabel={backdropLabel}
      mobileSheet={mobileSheet}
    >
      {children}
    </Modal>
  );
}

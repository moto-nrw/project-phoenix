"use client";

import { Z_INDEX } from "./overlay-styles";
import {
  type CSSProperties,
  type HTMLAttributes,
  type ReactNode,
  type RefObject,
  useCallback,
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { createPortal } from "react-dom";

import { cn } from "~/lib/utils";

const VIEWPORT_MARGIN = 8;
const PANEL_GAP = 4;
const DEFAULT_MAX_HEIGHT = 420;
const DEFAULT_PORTAL_SCOPE = '[data-date-picker-focus-trap="true"]';

interface AnchoredPopoverProps {
  readonly open: boolean;
  readonly onOpenChange: (open: boolean) => void;
  readonly ariaLabel: string;
  readonly initialFocusRef?: RefObject<HTMLElement | null>;
  /**
   * Ob der Fokus beim Schließen auf den Auslöser zurückgeht. Aus nur für einen
   * Rahmen, der den Fokus nie bekommen hat, etwa beim Öffnen per Darüberfahren.
   */
  readonly restoreFocusOnClose?: boolean;
  /** Handler für den Rahmen selbst, samt Rand und Scrollleiste. */
  readonly panelHandlers?: Pick<
    HTMLAttributes<HTMLDivElement>,
    "onPointerEnter" | "onPointerLeave" | "onFocus"
  >;
  readonly preferredWidth?: number;
  readonly align?: "start" | "end";
  /** Vorfahre mit Fokusfalle, in dessen Baum der Rahmen portaliert wird. */
  readonly portalScopeSelector?: string;
  readonly className?: string;
  /** Fester Kopf über dem scrollenden Inhalt, mit Trennlinie und Innenabstand. */
  readonly header?: ReactNode;
  /** Fester Fuß unter dem scrollenden Inhalt, mit Trennlinie; Abstand vom Inhalt. */
  readonly footer?: ReactNode;
  /**
   * Nimmt den Fokus beim Öffnen in den Rahmen. Der Rahmen liegt am Ende des
   * Dokuments; ohne das erreicht die Tab-Taste vom Auslöser aus seinen Inhalt nicht.
   */
  readonly focusPanelOnOpen?: boolean;
  /**
   * Schließt erst beim Klick und verschluckt ihn. Für einen Rahmen über einer
   * Fläche, deren Knöpfe nicht versehentlich auslösen dürfen (Stempeluhr).
   */
  readonly swallowOutsideClick?: boolean;
  readonly renderTrigger: (props: {
    ref: RefObject<HTMLButtonElement | null>;
    open: boolean;
    panelId: string;
    toggle: () => void;
  }) => ReactNode;
  readonly children: (props: {
    close: () => void;
    overflowMenuPortal: { ownerId: string; zIndex: number };
  }) => ReactNode;
}

export function AnchoredPopover({
  open,
  onOpenChange,
  ariaLabel,
  initialFocusRef,
  restoreFocusOnClose = true,
  panelHandlers,
  preferredWidth,
  align = "start",
  portalScopeSelector = DEFAULT_PORTAL_SCOPE,
  className,
  header,
  footer,
  focusPanelOnOpen = false,
  swallowOutsideClick = false,
  renderTrigger,
  children,
}: AnchoredPopoverProps) {
  const panelId = useId();
  const triggerRef = useRef<HTMLButtonElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const hasFocusedOnOpenRef = useRef(false);
  const [panelStyle, setPanelStyle] = useState<CSSProperties | null>(null);
  const portalContainer =
    typeof document === "undefined"
      ? null
      : (triggerRef.current?.closest(portalScopeSelector) ?? document.body);

  const close = useCallback(
    (restoreFocus = true) => {
      onOpenChange(false);
      if (restoreFocus && restoreFocusOnClose) triggerRef.current?.focus();
    },
    [onOpenChange, restoreFocusOnClose],
  );

  const updatePosition = useCallback(() => {
    const trigger = triggerRef.current;
    if (!trigger) return;

    const rect = trigger.getBoundingClientRect();
    const viewportWidth = window.innerWidth;
    const viewportHeight = window.innerHeight;
    const maxWidth = Math.max(viewportWidth - VIEWPORT_MARGIN * 2, 0);
    const width = Math.min(preferredWidth ?? rect.width, maxWidth);
    const desiredLeft = align === "end" ? rect.right - width : rect.left;
    const left = Math.min(
      Math.max(desiredLeft, VIEWPORT_MARGIN),
      Math.max(viewportWidth - width - VIEWPORT_MARGIN, VIEWPORT_MARGIN),
    );
    const spaceBelow =
      viewportHeight - rect.bottom - PANEL_GAP - VIEWPORT_MARGIN;
    const spaceAbove = rect.top - PANEL_GAP - VIEWPORT_MARGIN;
    const openAbove =
      spaceBelow < Math.min(DEFAULT_MAX_HEIGHT, 280) && spaceAbove > spaceBelow;
    const availableHeight = Math.max(openAbove ? spaceAbove : spaceBelow, 0);
    const scope = trigger.closest(portalScopeSelector);
    const scopeRect = scope?.getBoundingClientRect();
    const coordinateLeft = scopeRect ? left - scopeRect.left : left;

    const style: CSSProperties = {
      position: scope ? "absolute" : "fixed",
      zIndex: Z_INDEX.menu,
      left: coordinateLeft,
      width,
      maxHeight: Math.min(DEFAULT_MAX_HEIGHT, availableHeight),
    };
    if (openAbove) {
      style.bottom = scopeRect
        ? scopeRect.bottom - rect.top + PANEL_GAP
        : viewportHeight - rect.top + PANEL_GAP;
    } else {
      style.top = rect.bottom + PANEL_GAP - (scopeRect?.top ?? 0);
    }
    setPanelStyle(style);
  }, [align, portalScopeSelector, preferredWidth]);

  useLayoutEffect(() => {
    if (!open) {
      setPanelStyle(null);
      return;
    }
    updatePosition();
    window.addEventListener("resize", updatePosition);
    window.addEventListener("scroll", updatePosition, true);
    return () => {
      window.removeEventListener("resize", updatePosition);
      window.removeEventListener("scroll", updatePosition, true);
    };
  }, [open, updatePosition]);

  useEffect(() => {
    if (!open) return;
    const belongsToPopoverMenu = (target: EventTarget | null) => {
      const element =
        target instanceof Element
          ? target
          : target instanceof Node
            ? target.parentElement
            : null;
      return (
        element
          ?.closest("[data-overflow-menu-owner]")
          ?.getAttribute("data-overflow-menu-owner") === panelId
      );
    };
    const isOutside = (target: Node) =>
      !triggerRef.current?.contains(target) &&
      !panelRef.current?.contains(target) &&
      !belongsToPopoverMenu(target);
    const handlePointerDown = (event: MouseEvent) => {
      if (isOutside(event.target as Node)) close(false);
    };
    const handleOutsideClick = (event: MouseEvent) => {
      if (!isOutside(event.target as Node)) return;
      event.preventDefault();
      event.stopPropagation();
      close(false);
    };
    const handleKeyDown = (event: globalThis.KeyboardEvent) => {
      if (belongsToPopoverMenu(event.target)) return;
      if (event.key === "Escape") close();
    };
    if (swallowOutsideClick) {
      document.addEventListener("click", handleOutsideClick, true);
    } else {
      document.addEventListener("mousedown", handlePointerDown);
    }
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("click", handleOutsideClick, true);
      document.removeEventListener("mousedown", handlePointerDown);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [close, open, panelId, swallowOutsideClick]);

  useEffect(() => {
    if (!open) {
      hasFocusedOnOpenRef.current = false;
      return;
    }
    if (!panelStyle || hasFocusedOnOpenRef.current) return;

    if (initialFocusRef?.current) initialFocusRef.current.focus();
    else if (focusPanelOnOpen) panelRef.current?.focus();
    hasFocusedOnOpenRef.current = true;
  }, [focusPanelOnOpen, initialFocusRef, open, panelStyle]);

  const hasSlots = header != null || footer != null;
  const content =
    open && portalContainer
      ? children({
          close: () => close(),
          overflowMenuPortal: { ownerId: panelId, zIndex: Z_INDEX.popover },
        })
      : null;

  const panel =
    open && portalContainer ? (
      <div
        ref={panelRef}
        id={panelId}
        role="dialog"
        aria-label={ariaLabel}
        tabIndex={focusPanelOnOpen ? -1 : undefined}
        data-overflow-menu-scope="true"
        {...panelHandlers}
        onKeyDown={(event) => {
          if (event.key !== "Escape") return;
          event.preventDefault();
          event.stopPropagation();
          event.nativeEvent.stopImmediatePropagation();
          close();
        }}
        className={cn(
          "moto-popover-surface rounded-xl border focus:outline-none",
          hasSlots
            ? "flex flex-col overflow-hidden"
            : "scrollbar-thin overflow-y-auto overscroll-contain",
          className,
        )}
        style={{ ...panelStyle, visibility: panelStyle ? "visible" : "hidden" }}
      >
        {hasSlots ? (
          <>
            {header != null ? (
              <div className="shrink-0 border-b border-gray-100 px-4 py-3">
                {header}
              </div>
            ) : null}
            <div className="min-h-0 flex-1 scrollbar-thin overflow-y-auto overscroll-contain">
              {content}
            </div>
            {footer != null ? (
              <div className="shrink-0 border-t border-gray-100">{footer}</div>
            ) : null}
          </>
        ) : (
          content
        )}
      </div>
    ) : null;

  return (
    <>
      {renderTrigger({
        ref: triggerRef,
        open,
        panelId,
        toggle: () => onOpenChange(!open),
      })}
      {panel && portalContainer ? createPortal(panel, portalContainer) : null}
    </>
  );
}

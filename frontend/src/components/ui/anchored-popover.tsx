"use client";

import {
  type CSSProperties,
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

interface AnchoredPopoverProps {
  readonly open: boolean;
  readonly onOpenChange: (open: boolean) => void;
  readonly ariaLabel: string;
  readonly initialFocusRef?: RefObject<HTMLElement | null>;
  readonly preferredWidth?: number;
  readonly className?: string;
  readonly renderTrigger: (props: {
    ref: RefObject<HTMLButtonElement | null>;
    open: boolean;
    panelId: string;
    toggle: () => void;
  }) => ReactNode;
  readonly children: (props: { close: () => void }) => ReactNode;
}

export function AnchoredPopover({
  open,
  onOpenChange,
  ariaLabel,
  initialFocusRef,
  preferredWidth,
  className,
  renderTrigger,
  children,
}: AnchoredPopoverProps) {
  const panelId = useId();
  const triggerRef = useRef<HTMLButtonElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const [panelStyle, setPanelStyle] = useState<CSSProperties | null>(null);
  const portalContainer =
    typeof document === "undefined"
      ? null
      : (triggerRef.current?.closest('[data-date-picker-focus-trap="true"]') ??
        document.body);

  const close = useCallback(
    (restoreFocus = true) => {
      onOpenChange(false);
      if (restoreFocus) triggerRef.current?.focus();
    },
    [onOpenChange],
  );

  const updatePosition = useCallback(() => {
    const trigger = triggerRef.current;
    if (!trigger) return;

    const rect = trigger.getBoundingClientRect();
    const viewportWidth = window.innerWidth;
    const viewportHeight = window.innerHeight;
    const maxWidth = Math.max(viewportWidth - VIEWPORT_MARGIN * 2, 0);
    const width = Math.min(preferredWidth ?? rect.width, maxWidth);
    const left = Math.min(
      Math.max(rect.left, VIEWPORT_MARGIN),
      Math.max(viewportWidth - width - VIEWPORT_MARGIN, VIEWPORT_MARGIN),
    );
    const spaceBelow =
      viewportHeight - rect.bottom - PANEL_GAP - VIEWPORT_MARGIN;
    const spaceAbove = rect.top - PANEL_GAP - VIEWPORT_MARGIN;
    const openAbove =
      spaceBelow < Math.min(DEFAULT_MAX_HEIGHT, 280) && spaceAbove > spaceBelow;
    const availableHeight = Math.max(openAbove ? spaceAbove : spaceBelow, 0);
    const scope = trigger.closest('[data-date-picker-focus-trap="true"]');
    const scopeRect = scope?.getBoundingClientRect();
    const coordinateLeft = scopeRect ? left - scopeRect.left : left;

    const style: CSSProperties = {
      position: scope ? "absolute" : "fixed",
      zIndex: 10000,
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
  }, [preferredWidth]);

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
    const handlePointerDown = (event: MouseEvent) => {
      const target = event.target as Node;
      if (
        triggerRef.current?.contains(target) ||
        panelRef.current?.contains(target)
      ) {
        return;
      }
      close(false);
    };
    document.addEventListener("mousedown", handlePointerDown);
    return () => {
      document.removeEventListener("mousedown", handlePointerDown);
    };
  }, [close, open]);

  useEffect(() => {
    if (!open || !panelStyle) return;
    initialFocusRef?.current?.focus();
  }, [initialFocusRef, open, panelStyle]);

  const panel =
    open && portalContainer ? (
      <div
        ref={panelRef}
        id={panelId}
        role="dialog"
        aria-label={ariaLabel}
        onKeyDown={(event) => {
          if (event.key !== "Escape") return;
          event.preventDefault();
          event.stopPropagation();
          event.nativeEvent.stopImmediatePropagation();
          close();
        }}
        className={cn(
          "scrollbar-thin overflow-y-auto overscroll-contain rounded-xl border border-gray-200 bg-white shadow-lg",
          className,
        )}
        style={{ ...panelStyle, visibility: panelStyle ? "visible" : "hidden" }}
      >
        {children({ close: () => close() })}
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

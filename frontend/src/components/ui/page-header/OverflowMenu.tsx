"use client";

import {
  useEffect,
  useId,
  useRef,
  useState,
  type CSSProperties,
  type KeyboardEvent,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";
import Link from "next/link";
import { Check, MoreVertical } from "lucide-react";

export interface OverflowMenuItem {
  /** Visible label. */
  readonly label: string;
  /** Optional left icon. */
  readonly icon?: ReactNode;
  /** Click handler. The menu closes automatically before this fires. */
  readonly onClick: () => void;
  /** Optional right-side hint, typically a count (e.g. "3"). */
  readonly badge?: string | number;
  /** Renders the item with a destructive (red) text colour. */
  readonly destructive?: boolean;
  /** When true, the item renders disabled and its onClick is suppressed. */
  readonly disabled?: boolean;
  /**
   * When set, the item renders as a link (next/link, or a plain anchor with
   * `target="_blank"` when `external` is true) instead of a button, keeping
   * native link semantics (middle-click, cmd-click). `onClick` still fires
   * after the menu closes — pass a no-op when only navigation is needed.
   */
  readonly href?: string;
  /** With `href`: open in a new tab via a plain anchor (noopener noreferrer). */
  readonly external?: boolean;
}

/** Non-interactive thin divider between item groups. */
interface OverflowMenuSeparator {
  readonly kind: "separator";
}

/** Non-interactive uppercase label grouping the entries below it. */
interface OverflowMenuHeader {
  readonly kind: "header";
  readonly label: string;
}

/** Single-select row with a checkmark shown when active (role="menuitemradio"). */
interface OverflowMenuRadioItem {
  readonly kind: "radio";
  readonly label: string;
  readonly checked: boolean;
  readonly onClick: () => void;
  readonly disabled?: boolean;
}

export type OverflowMenuEntry =
  | OverflowMenuItem
  | OverflowMenuHeader
  | OverflowMenuRadioItem
  | OverflowMenuSeparator;

interface OverflowMenuProps {
  /** Menu items to render. Empty array → renders nothing. */
  readonly items: readonly OverflowMenuEntry[];
  /** Accessible label for the trigger button. */
  readonly ariaLabel?: string;
  /** Called when the menu is opened from its trigger. */
  readonly onOpen?: () => void;
  /**
   * Trigger footprint. `"default"` is the 36px page-header kebab; `"sm"` is a
   * compact 24px kebab with a smaller icon and a more muted default color, for
   * dense inline chrome (e.g. a shift block in the Dienstplan grid) where the
   * full-size trigger would tower over the row. Additive — existing callers
   * omit it and get the unchanged 36px trigger.
   */
  readonly triggerSize?: "default" | "sm";
  /** Optional class for the trigger button (size/spacing tweaks). */
  readonly triggerClassName?: string;
  /** Optional visible trigger content; the default is the kebab icon. */
  readonly triggerContent?: ReactNode;
  /**
   * Optional CSS selector for an ancestor of the trigger. When set, the menu
   * stretches to that ancestor's width and sits inside it (8px inset), instead
   * of the default 220px popover anchored to the trigger. Use this when the
   * trigger lives in a narrow container (e.g. a meal-plan day column) where a
   * fixed-width popover would poke out the side and read as misaligned.
   */
  readonly matchContainerSelector?: string;
}

/**
 * Generic kebab (⋮) overflow menu. Anchored bottom-right of the trigger by
 * default; flips to bottom-left if the page edge is too close on the right, and
 * flips ABOVE the trigger when it sits too close to the viewport bottom.
 *
 * Closes on outside click, Escape key, or item activation. Built without a
 * popover library to avoid adding a Radix dep just for one use site — the
 * menu is small, anchored, and modal enough for a single page header.
 */
export function OverflowMenu({
  items,
  ariaLabel = "Weitere Aktionen",
  onOpen,
  triggerSize = "default",
  triggerClassName = "",
  triggerContent,
  matchContainerSelector,
}: OverflowMenuProps) {
  // Size variant: the "default" values are byte-for-byte the previous hardcoded
  // ones, so unchanged callers keep the exact 36px trigger + 20px icon +
  // gray-600 color. "sm" shrinks to a 24px trigger, 16px icon, and a muted
  // gray-400 (splitting color from the string avoids a Tailwind text-* conflict
  // that class-string order would not resolve).
  const triggerSizeClass =
    triggerContent != null ? "" : triggerSize === "sm" ? "size-6" : "size-9";
  const triggerColorClass =
    triggerSize === "sm" ? "text-gray-400" : "text-gray-600";
  const iconSizeClass = triggerSize === "sm" ? "size-4" : "size-5";
  const [isOpen, setIsOpen] = useState(false);
  // The menu renders in a portal on <body> with fixed positioning so it can
  // never be clipped by an ancestor's `overflow-hidden` (e.g. a rounded table
  // or card the kebab lives in). Position is derived from the trigger rect on
  // open. `right`/`left` mirror the previous right-0/left-0 anchoring.
  const [menuStyle, setMenuStyle] = useState<CSSProperties>({});
  const triggerRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const menuId = useId();

  // Close on outside click + Escape. Because the menu is fixed-positioned, also
  // close on scroll/resize so it never lingers detached from its trigger. Only
  // attached while open so we don't pay the listener cost on every page.
  useEffect(() => {
    if (!isOpen) return;

    const onPointer = (event: MouseEvent) => {
      const target = event.target as Node | null;
      if (target == null) return;
      if (triggerRef.current?.contains(target)) return;
      if (menuRef.current?.contains(target)) return;
      setIsOpen(false);
    };
    const onKey = (event: globalThis.KeyboardEvent) => {
      if (event.key === "Escape") {
        setIsOpen(false);
        triggerRef.current?.focus();
      }
    };
    const onReflow = (event: Event) => {
      // A viewport-bounded menu scrolls internally. That scroll bubbles through
      // the capture listener on window and must not close the menu itself.
      const target = event.target;
      if (
        event.type === "scroll" &&
        target instanceof Node &&
        menuRef.current?.contains(target)
      ) {
        return;
      }
      setIsOpen(false);
    };

    document.addEventListener("mousedown", onPointer);
    document.addEventListener("keydown", onKey);
    window.addEventListener("scroll", onReflow, true);
    window.addEventListener("resize", onReflow);
    return () => {
      document.removeEventListener("mousedown", onPointer);
      document.removeEventListener("keydown", onKey);
      window.removeEventListener("scroll", onReflow, true);
      window.removeEventListener("resize", onReflow);
    };
  }, [isOpen]);

  // Decide alignment when opening. The menu is 220px wide when the viewport
  // permits; constrain it on smaller viewports so every action stays reachable.
  //
  // - Default to right-anchored (`right-0` → menu extends LEFT from the
  //   trigger's right edge). This is safe in the common case where the
  //   kebab sits near the right edge of the page (action area).
  // - Flip to left-anchored (`left-0` → menu extends RIGHT) when the trigger
  //   sits near the LEFT edge with not enough room on its left.
  // - If neither side fits, use the side with more room and clamp to the
  //   viewport inset.
  const handleOpen = () => {
    const rect = triggerRef.current?.getBoundingClientRect();
    if (rect != null) {
      const gap = 4; // matches the old mt-1
      const viewportInset = 8;
      // Vertical anchoring: below the trigger by default, flipped ABOVE it when
      // there is not enough room below for the (conservatively estimated) menu
      // height. Without the flip, a trigger near the viewport bottom (e.g. the
      // last table row) drops the menu below the fold, where the first scroll
      // attempt immediately closes it via the scroll listener. Anchoring by
      // `bottom` needs no exact height — only the flip decision estimates one.
      const estimatedMenuHeight = items.length * 40 + 16;
      const roomAbove = Math.max(0, rect.top - gap - viewportInset);
      const roomBelow = Math.max(
        0,
        window.innerHeight - rect.bottom - gap - viewportInset,
      );
      const flipUp = roomBelow < estimatedMenuHeight && roomAbove > roomBelow;
      const vertical: CSSProperties = flipUp
        ? {
            bottom: window.innerHeight - rect.top + gap,
            maxHeight: roomAbove,
          }
        : { top: rect.bottom + gap, maxHeight: roomBelow };
      // Container-stretch mode: when an ancestor selector is given, size the
      // menu to that ancestor (8px inset both sides) so it sits cleanly INSIDE
      // a narrow container instead of poking out the side. The trigger only
      // contributes the vertical position here.
      const container = matchContainerSelector
        ? triggerRef.current?.closest(matchContainerSelector)
        : null;
      if (container != null) {
        const cr = container.getBoundingClientRect();
        const inset = 8;
        setMenuStyle({
          ...vertical,
          left: cr.left + inset,
          width: cr.width - inset * 2,
        });
      } else {
        const menuWidth = 220;
        const availableWidth = Math.max(
          0,
          window.innerWidth - viewportInset * 2,
        );
        const renderedWidth = Math.min(menuWidth, availableWidth);
        const roomLeft = rect.right - viewportInset;
        const roomRight = window.innerWidth - rect.left - viewportInset;
        const size: CSSProperties = {
          minWidth: renderedWidth,
          maxWidth: availableWidth,
        };
        const maxOffset = Math.max(
          viewportInset,
          window.innerWidth - renderedWidth - viewportInset,
        );
        const clampOffset = (offset: number) =>
          Math.min(Math.max(viewportInset, offset), maxOffset);
        const alignRight =
          roomLeft >= renderedWidth ||
          (roomRight < renderedWidth && roomLeft >= roomRight);
        const horizontal: CSSProperties = alignRight
          ? { right: clampOffset(window.innerWidth - rect.right) }
          : { left: clampOffset(rect.left) };
        const style: CSSProperties = { ...vertical, ...size, ...horizontal };
        setMenuStyle(style);
      }
    }
    if (!isOpen) onOpen?.();
    setIsOpen((prev) => !prev);
  };

  const onItemKey =
    (item: OverflowMenuItem | OverflowMenuRadioItem) =>
    (event: KeyboardEvent<HTMLButtonElement>) => {
      if (event.key === "Enter" || event.key === " ") {
        event.preventDefault();
        if (item.disabled) return;
        setIsOpen(false);
        item.onClick();
      }
    };

  if (items.length === 0) return null;

  return (
    <div className="relative inline-block">
      <button
        ref={triggerRef}
        type="button"
        onClick={handleOpen}
        aria-label={ariaLabel}
        aria-haspopup="menu"
        aria-expanded={isOpen}
        aria-controls={isOpen ? menuId : undefined}
        className={`inline-flex ${triggerSizeClass} items-center justify-center ${triggerContent == null ? "rounded-full" : ""} ${triggerColorClass} focus-visible:ring-moto-blue/50 transition-colors duration-150 hover:bg-gray-100 focus-visible:ring-2 focus-visible:outline-none active:bg-gray-200 ${triggerClassName}`}
      >
        {triggerContent ?? (
          <MoreVertical className={iconSizeClass} aria-hidden />
        )}
      </button>

      {isOpen
        ? createPortal(
            <div
              ref={menuRef}
              id={menuId}
              role="menu"
              aria-label={ariaLabel}
              style={menuStyle}
              // Surface mirrors DesktopFilters dropdown so menu / filter
              // popovers read as one component family — same border, radius,
              // and shadow elevation across the page. Fixed + portaled so it
              // escapes any clipping `overflow-hidden` ancestor. In
              // container-stretch mode the width comes from `menuStyle`, so the
              // 220px floor is dropped to let the menu match a narrow column.
              className={`fixed z-50 scrollbar-thin overflow-x-hidden overflow-y-auto rounded-xl border border-gray-200 bg-white py-1 shadow-lg ${
                matchContainerSelector ? "" : "min-w-[220px]"
              }`}
            >
              {items.map((entry, index) => {
                if ("kind" in entry && entry.kind === "header") {
                  return (
                    <div
                      key={`header-${entry.label}`}
                      className={`px-4 py-2 ${index > 0 ? "border-t border-gray-100" : ""}`}
                    >
                      <p className="text-[10px] font-semibold tracking-wider text-gray-500 uppercase">
                        {entry.label}
                      </p>
                    </div>
                  );
                }

                if ("kind" in entry && entry.kind === "radio") {
                  return (
                    <button
                      key={`radio-${entry.label}`}
                      type="button"
                      role="menuitemradio"
                      aria-checked={entry.checked}
                      disabled={entry.disabled}
                      onClick={() => {
                        if (entry.disabled) return;
                        setIsOpen(false);
                        entry.onClick();
                      }}
                      onKeyDown={onItemKey(entry)}
                      className={`flex w-full items-center justify-between px-4 py-2 text-left text-sm font-medium transition-colors ${
                        entry.disabled
                          ? "cursor-not-allowed opacity-50"
                          : "hover:bg-gray-50 active:bg-gray-100"
                      } ${entry.checked ? "text-gray-900" : "text-gray-700"}`}
                    >
                      <span>{entry.label}</span>
                      {entry.checked ? (
                        <Check className="size-4 text-gray-900" aria-hidden />
                      ) : null}
                    </button>
                  );
                }

                if ("kind" in entry && entry.kind === "separator") {
                  return (
                    <div
                      key={`separator-${index}`}
                      role="separator"
                      className="my-1 h-px bg-gray-100"
                    />
                  );
                }

                const item = entry;
                const colorClass = item.destructive
                  ? "text-moto-red"
                  : "text-gray-700";
                const interactive = item.disabled
                  ? "cursor-not-allowed opacity-50"
                  : "hover:bg-gray-50 active:bg-gray-100";
                const itemClassName = `flex w-full items-center gap-2 px-4 py-2 text-left text-sm font-medium transition-colors ${colorClass} ${interactive}`;

                const inner = (
                  <>
                    {item.icon != null ? (
                      <span className="flex size-4 flex-shrink-0 items-center justify-center text-gray-500">
                        {item.icon}
                      </span>
                    ) : null}
                    <span className="flex-1 truncate">{item.label}</span>
                    {item.badge != null ? (
                      <span className="ml-auto inline-flex h-5 min-w-5 items-center justify-center rounded-full bg-gray-100 px-1.5 text-[11px] font-semibold text-gray-700 tabular-nums">
                        {item.badge}
                      </span>
                    ) : null}
                  </>
                );

                if (item.href != null && !item.disabled) {
                  const onLinkActivate = () => {
                    setIsOpen(false);
                    item.onClick();
                  };
                  if (item.external) {
                    return (
                      <a
                        key={`item-${item.label}`}
                        role="menuitem"
                        tabIndex={0}
                        href={item.href}
                        target="_blank"
                        rel="noopener noreferrer"
                        onClick={onLinkActivate}
                        className={itemClassName}
                      >
                        {inner}
                      </a>
                    );
                  }
                  return (
                    <Link
                      key={`item-${item.label}`}
                      role="menuitem"
                      tabIndex={0}
                      href={item.href}
                      onClick={onLinkActivate}
                      className={itemClassName}
                    >
                      {inner}
                    </Link>
                  );
                }

                return (
                  <button
                    key={`item-${item.label}`}
                    type="button"
                    role="menuitem"
                    disabled={item.disabled}
                    onClick={() => {
                      if (item.disabled) return;
                      setIsOpen(false);
                      item.onClick();
                    }}
                    onKeyDown={onItemKey(item)}
                    className={itemClassName}
                  >
                    {inner}
                  </button>
                );
              })}
            </div>,
            document.body,
          )
        : null}
    </div>
  );
}

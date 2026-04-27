"use client";

import {
  useEffect,
  useId,
  useRef,
  useState,
  type KeyboardEvent,
  type ReactNode,
} from "react";
import { MoreVertical } from "lucide-react";

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
}

interface OverflowMenuProps {
  /** Menu items to render. Empty array → renders nothing. */
  readonly items: readonly OverflowMenuItem[];
  /** Accessible label for the trigger button. */
  readonly ariaLabel?: string;
  /** Optional class for the trigger button (size/spacing tweaks). */
  readonly triggerClassName?: string;
}

/**
 * Generic kebab (⋮) overflow menu. Anchored bottom-right of the trigger by
 * default; flips to bottom-left if the page edge is too close on the right.
 *
 * Closes on outside click, Escape key, or item activation. Built without a
 * popover library to avoid adding a Radix dep just for one use site — the
 * menu is small, anchored, and modal enough for a single page header.
 */
export function OverflowMenu({
  items,
  ariaLabel = "Weitere Aktionen",
  triggerClassName = "",
}: OverflowMenuProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [alignRight, setAlignRight] = useState(true);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const menuId = useId();

  // Close on outside click + Escape. Only attached while open so we don't
  // pay the listener cost on every page that mounts a menu.
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

    document.addEventListener("mousedown", onPointer);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onPointer);
      document.removeEventListener("keydown", onKey);
    };
  }, [isOpen]);

  // Decide alignment when opening. The menu is min 220px wide; we need to
  // ensure it stays inside the viewport regardless of where the trigger sits.
  //
  // - Default to right-anchored (`right-0` → menu extends LEFT from the
  //   trigger's right edge). This is safe in the common case where the
  //   kebab sits near the right edge of the page (action area).
  // - Only flip to left-anchored (`left-0` → menu extends RIGHT) when the
  //   trigger sits near the LEFT edge with not enough room on its left.
  const handleOpen = () => {
    const rect = triggerRef.current?.getBoundingClientRect();
    if (rect != null) {
      const spaceLeft = rect.left;
      // Flip to left-anchored only when there isn't enough room on the
      // trigger's left for the menu to extend leftward.
      setAlignRight(spaceLeft >= 220);
    }
    setIsOpen((prev) => !prev);
  };

  const onItemKey =
    (item: OverflowMenuItem) => (event: KeyboardEvent<HTMLButtonElement>) => {
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
        className={`inline-flex size-9 items-center justify-center rounded-full text-gray-600 transition-colors duration-150 hover:bg-gray-100 focus-visible:ring-2 focus-visible:ring-blue-500/50 focus-visible:outline-none active:bg-gray-200 ${triggerClassName}`}
      >
        <MoreVertical className="size-5" aria-hidden />
      </button>

      {isOpen ? (
        <div
          ref={menuRef}
          id={menuId}
          role="menu"
          aria-label={ariaLabel}
          // Surface mirrors DesktopFilters dropdown so menu / filter
          // popovers read as one component family — same border, radius,
          // and shadow elevation across the page.
          className={`absolute top-full z-50 mt-1 min-w-[220px] overflow-hidden rounded-xl border border-gray-200 bg-white py-1 shadow-lg ${
            alignRight ? "right-0" : "left-0"
          }`}
        >
          {items.map((item, index) => {
            const colorClass = item.destructive
              ? "text-red-600"
              : "text-gray-700";
            const interactive = item.disabled
              ? "cursor-not-allowed opacity-50"
              : "hover:bg-gray-50 active:bg-gray-100";

            return (
              <button
                key={`${item.label}-${index}`}
                type="button"
                role="menuitem"
                disabled={item.disabled}
                onClick={() => {
                  if (item.disabled) return;
                  setIsOpen(false);
                  item.onClick();
                }}
                onKeyDown={onItemKey(item)}
                className={`flex w-full items-center gap-2 px-4 py-2 text-left text-sm font-medium transition-colors ${colorClass} ${interactive}`}
              >
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
              </button>
            );
          })}
        </div>
      ) : null}
    </div>
  );
}

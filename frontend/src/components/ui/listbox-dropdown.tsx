"use client";

import { Check, ChevronDown, Search } from "lucide-react";
import {
  useCallback,
  type CSSProperties,
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
  type KeyboardEvent,
  type ReactNode,
  type RefObject,
} from "react";
import { createPortal } from "react-dom";

import { cn } from "~/lib/utils";

export interface ListboxDropdownOption<K extends string> {
  readonly value: K;
  readonly label: string;
  readonly disabled?: boolean;
}

interface ListboxDropdownProps<K extends string> {
  readonly value: K;
  readonly options: readonly ListboxDropdownOption<K>[];
  /**
   * Called with the option the user activated. In multi mode (see
   * {@link ListboxDropdownProps.selectedValues}) this is a TOGGLE — the caller
   * adds or removes the value from its own selection.
   */
  readonly onChange: (next: K) => void;
  /**
   * Turns the listbox into a multi-select: options render as checkable rows,
   * activating one toggles it and keeps the menu open, and the list announces
   * itself as multi-selectable. `value` still anchors keyboard focus (pass the
   * first selected value, or "" for none). Undefined = single select.
   */
  readonly selectedValues?: readonly K[];
  readonly id?: string;
  readonly ariaLabel?: string;
  readonly ariaLabelledBy?: string;
  readonly ariaDescribedBy?: string;
  readonly containerClassName?: string;
  readonly containerStyle?: CSSProperties;
  /**
   * Optional ancestor selector for the menu portal. Use when a fixed overlay
   * already owns a FocusScope that must contain the listbox options.
   */
  readonly portalScopeSelector?: string;
  /** Receives the wrapper node, e.g. to locate the surrounding <form>. */
  readonly containerNodeRef?: RefObject<HTMLDivElement | null>;
  readonly className?: string;
  readonly menuClassName?: string;
  /** Horizontal anchor of the portaled menu relative to the trigger. */
  readonly menuAlign?: "start" | "end";
  /**
   * Stacking level of the portaled menu. The default clears modal overlays
   * (z-[9999]); a caller that itself portals higher — the kit calendar sits at
   * z-[10001] — has to lift the menu above its own surface, or the menu opens
   * behind it.
   */
  readonly menuZIndex?: number;
  readonly optionClassName?: string;
  readonly activeOptionClassName?: string;
  readonly disabledOptionClassName?: string;
  readonly disabled?: boolean;
  readonly required?: boolean;
  readonly invalid?: boolean;
  readonly placeholder?: string;
  readonly triggerRole?: "button" | "combobox";
  readonly testId?: string;
  readonly renderTrigger?: (args: {
    selectedLabel: string;
    open: boolean;
  }) => ReactNode;
  /**
   * Controlled open state. Omit to let the listbox own it; pass it when the
   * caller has to close the menu itself after an action of its own (adding an
   * entry, saving a name).
   */
  readonly open?: boolean;
  readonly onOpenChange?: (open: boolean) => void;
  /**
   * Renders a filter field at the top of the menu. The caller owns the text
   * and passes the options it has already filtered — the same text usually
   * means something to the caller too (the name of an entry to add), so the
   * listbox must not keep it to itself. Typeahead switches off while the field
   * is there: it would fight the field for the same keystrokes.
   */
  readonly searchValue?: string;
  readonly onSearchChange?: (value: string) => void;
  readonly searchPlaceholder?: string;
  /** Keys the search field does not use itself (Enter and the like). */
  readonly onSearchKeyDown?: (event: KeyboardEvent<HTMLInputElement>) => void;
  /** Slot between the search field and the option list. */
  readonly menuHeader?: ReactNode;
  /** Slot under the option list, e.g. a row that adds the typed name. */
  readonly menuFooter?: ReactNode;
  /** Classes for the scrollable option list in search/slot mode. */
  readonly listClassName?: string;
  /**
   * Controls rendered next to an option, OUTSIDE its button — a button inside
   * a button is invalid, and the row must stay one option to a screen reader.
   */
  readonly renderOptionActions?: (
    option: ListboxDropdownOption<K>,
  ) => ReactNode;
}

const TYPEAHEAD_RESET_MS = 500;
const MENU_OFFSET_PX = 4;
const MENU_MAX_HEIGHT_PX = 288; // matches the previous max-h-72 menu cap
const MENU_VIEWPORT_MARGIN_PX = 8;

function firstEnabledIndex<K extends string>(
  options: readonly ListboxDropdownOption<K>[],
): number {
  return Math.max(
    options.findIndex((option) => !option.disabled),
    0,
  );
}

function selectedIndexForValue<K extends string>(
  options: readonly ListboxDropdownOption<K>[],
  value: K,
): number {
  const selectedIndex = options.findIndex((option) => option.value === value);
  return selectedIndex >= 0 ? selectedIndex : firstEnabledIndex(options);
}

function nextEnabledIndex<K extends string>(
  options: readonly ListboxDropdownOption<K>[],
  startIndex: number,
  direction: 1 | -1,
): number {
  if (options.length === 0) return -1;

  for (let offset = 1; offset <= options.length; offset++) {
    const index =
      (startIndex + direction * offset + options.length) % options.length;
    if (!options[index]?.disabled) return index;
  }

  return -1;
}

function isRepeatedCharacter(buffer: string): boolean {
  return buffer.length > 1 && [...buffer].every((char) => char === buffer[0]);
}

function typeaheadMatchIndex<K extends string>(
  options: readonly ListboxDropdownOption<K>[],
  buffer: string,
  startIndex: number,
  hasAnchor: boolean,
): number {
  if (options.length === 0 || buffer.length === 0) return -1;
  // A repeated single character cycles through options sharing that initial
  // letter instead of demanding a label like "aaa".
  const cycling = buffer.length === 1 || isRepeatedCharacter(buffer);
  const needle = (cycling ? buffer[0]! : buffer).toLowerCase();
  // Skip the start index only when it is a genuine selection/focus (cycle to
  // the NEXT match). With no real anchor — an empty value on a placeholder-only
  // select whose startIndex is synthesized to the first option — search from
  // index 0 inclusive so the first matching option is reachable.
  const firstOffset = cycling && hasAnchor ? 1 : 0;

  for (
    let offset = firstOffset;
    offset < firstOffset + options.length;
    offset++
  ) {
    const index = (startIndex + offset + options.length) % options.length;
    const option = options[index];
    if (
      option &&
      !option.disabled &&
      option.label.toLowerCase().startsWith(needle)
    ) {
      return index;
    }
  }

  return -1;
}

function consumeEscape(event: KeyboardEvent<HTMLButtonElement>): void {
  event.preventDefault();
  event.stopPropagation();
  event.nativeEvent.stopImmediatePropagation();
}

export function ListboxDropdown<K extends string>({
  value,
  options,
  onChange,
  selectedValues,
  id,
  ariaLabel,
  ariaLabelledBy,
  ariaDescribedBy,
  containerClassName = "relative",
  containerStyle,
  portalScopeSelector,
  containerNodeRef,
  className = "",
  menuClassName = "",
  menuAlign = "start",
  menuZIndex = 10000,
  optionClassName = "",
  activeOptionClassName = "",
  disabledOptionClassName = "",
  disabled = false,
  required = false,
  invalid = false,
  placeholder = "Bitte wählen",
  triggerRole = "button",
  testId,
  renderTrigger,
  open: openProp,
  onOpenChange,
  searchValue,
  onSearchChange,
  searchPlaceholder = "Suchen",
  onSearchKeyDown,
  menuHeader,
  menuFooter,
  listClassName = "",
  renderOptionActions,
}: ListboxDropdownProps<K>) {
  const generatedListboxId = useId();
  const listboxId = id ? `${id}-listbox` : generatedListboxId;
  const containerRef = useRef<HTMLDivElement>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);
  // The outermost portaled node: the option list itself, or the panel wrapping
  // it once a search field or a slot is in play.
  const menuRef = useRef<HTMLElement | null>(null);
  const optionRefs = useRef<Array<HTMLButtonElement | null>>([]);
  const typeaheadBufferRef = useRef("");
  const typeaheadTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  // Pointer-driven focus (hover) must not scroll the menu — re-centering under
  // the cursor makes long lists jump and options unclickable (#2212). Applies
  // only to hover-driven focusIndex changes; layout (menuStyle) updates still
  // keep the focused option in view. Do not clear in the effect on its own
  // (React Strict Mode re-runs effects and would re-enable hover scroll).
  const suppressFocusScrollRef = useRef(false);
  // One-shot scroll when the menu opens so an early mouseEnter (pointer over
  // the portaled list as it appears) cannot suppress revealing the selection.
  const forceOpenScrollRef = useRef(false);
  // Index captured at open time — open scroll must target this, not whatever
  // hover may have moved focusIndex to before the effect runs.
  const openScrollIndexRef = useRef<number | null>(null);
  // Track last effect inputs so hover-suppression does not also block
  // layout-only re-runs (resize/scroll reposition), and Strict Mode re-runs
  // do not re-scroll after a suppressed hover focus move.
  const lastScrollFocusIndexRef = useRef<number | null>(null);
  const lastScrollMenuStyleRef = useRef<CSSProperties | null>(null);
  const [uncontrolledOpen, setUncontrolledOpen] = useState(false);
  const open = openProp ?? uncontrolledOpen;
  const setOpen = useCallback(
    (next: boolean) => {
      if (openProp === undefined) setUncontrolledOpen(next);
      onOpenChange?.(next);
    },
    [onOpenChange, openProp],
  );
  const searchable = onSearchChange !== undefined;
  const searchRef = useRef<HTMLInputElement>(null);
  // Which half of the menu owns focus while a search field is present: the
  // field until an arrow key steps into the list, so typing keeps landing in
  // the field instead of being swallowed by an option.
  const [optionsFocused, setOptionsFocused] = useState(false);
  const [menuStyle, setMenuStyle] = useState<CSSProperties | null>(null);
  const selectedIndex = selectedIndexForValue(options, value);
  const [focusIndex, setFocusIndex] = useState(selectedIndex);
  const selectedOption = options.find((option) => option.value === value);
  const selectedLabel = selectedOption?.label ?? placeholder;
  const isMulti = selectedValues !== undefined;
  const selectedValueSet = new Set<string>(selectedValues ?? []);
  const portalContainer =
    typeof document === "undefined"
      ? null
      : ((portalScopeSelector
          ? buttonRef.current?.closest(portalScopeSelector)
          : null) ?? document.body);

  useEffect(() => {
    if (!open) return;
    const handleClick = (event: MouseEvent) => {
      const target = event.target as Node;
      // The menu is portaled to document.body, so it is NOT inside
      // containerRef — a press on an option must not count as outside.
      if (containerRef.current?.contains(target)) return;
      if (menuRef.current?.contains(target)) return;
      setOpen(false);
    };
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, [open, setOpen]);

  const updateMenuPosition = useCallback(() => {
    const trigger = buttonRef.current;
    if (!trigger) return;
    const rect = trigger.getBoundingClientRect();
    const viewportHeight = window.innerHeight;
    const viewportWidth = window.innerWidth;
    const spaceBelow =
      viewportHeight - rect.bottom - MENU_OFFSET_PX - MENU_VIEWPORT_MARGIN_PX;
    const spaceAbove = rect.top - MENU_OFFSET_PX - MENU_VIEWPORT_MARGIN_PX;
    const menuHeight = Math.min(
      menuRef.current?.scrollHeight ?? MENU_MAX_HEIGHT_PX,
      MENU_MAX_HEIGHT_PX,
    );
    const openUpward = spaceBelow < menuHeight && spaceAbove > spaceBelow;
    // Cap the menu to the space actually available on the chosen side. A hard
    // minimum height could push the menu past the viewport edge in short
    // viewports or with the mobile keyboard open, leaving lower options
    // unreachable; the browser then never gives the ul a scroll region there.
    const available = Math.max(openUpward ? spaceAbove : spaceBelow, 0);
    const maxHeight = Math.min(MENU_MAX_HEIGHT_PX, available);
    const maxWidth = viewportWidth - 2 * MENU_VIEWPORT_MARGIN_PX;
    // The rendered width, needed to clamp the horizontal anchor: the menu is
    // only as wide as the trigger at minimum, but its content may be wider, so
    // anchoring at the trigger edge alone can push it past the viewport on
    // narrow layouts and leave options unreachable.
    const menuWidth = Math.min(
      Math.max(menuRef.current?.offsetWidth ?? rect.width, rect.width),
      maxWidth,
    );
    const edgeInset = Math.max(
      viewportWidth - MENU_VIEWPORT_MARGIN_PX - menuWidth,
      MENU_VIEWPORT_MARGIN_PX,
    );
    const style: CSSProperties = {
      position: "fixed",
      // Above the modal/dialog overlays (z-[9999]) so menus opened from
      // dialog forms stack on top of the dialog instead of behind it.
      zIndex: menuZIndex,
      // Vaul/Radix dialogs run in modal mode set `body { pointer-events: none }`
      // and only re-enable it on the dialog content. Because this menu is
      // portaled to document.body (a sibling of that content), it would inherit
      // `none` and its options would be unclickable inside slide-overs. Forcing
      // `auto` restores selection. Outside-dismissal is handled separately by
      // the pointerdown guard on the menu below.
      pointerEvents: "auto",
      minWidth: rect.width,
      maxWidth,
      maxHeight,
    };
    // Clamp both anchors so the opposite edge stays inside the viewport; the
    // margin floor still wins when the menu is as wide as the viewport allows.
    if (menuAlign === "end") {
      style.right = Math.min(
        Math.max(viewportWidth - rect.right, MENU_VIEWPORT_MARGIN_PX),
        edgeInset,
      );
    } else {
      style.left = Math.min(
        Math.max(rect.left, MENU_VIEWPORT_MARGIN_PX),
        edgeInset,
      );
    }
    if (openUpward) {
      style.bottom = viewportHeight - rect.top + MENU_OFFSET_PX;
    } else {
      style.top = rect.bottom + MENU_OFFSET_PX;
    }
    setMenuStyle(style);
  }, [menuAlign, menuZIndex]);

  // Layout effect so the first visible frame is already positioned (the menu
  // renders hidden until menuStyle is set). Scroll listens in capture phase to
  // catch ancestor scroll containers (modal bodies), not just the window.
  // Scrolls *inside* the option list are ignored: scrollIntoView on open/keyboard
  // would otherwise re-fire updateMenuPosition → new menuStyle → a layout
  // re-scroll of the (possibly hover-moved) focusIndex, undoing the open reveal.
  useLayoutEffect(() => {
    if (!open) {
      setMenuStyle(null);
      return;
    }
    const handleScroll = (event: Event) => {
      const target = event.target;
      if (
        menuRef.current &&
        target instanceof Node &&
        (target === menuRef.current || menuRef.current.contains(target))
      ) {
        return;
      }
      updateMenuPosition();
    };
    updateMenuPosition();
    window.addEventListener("resize", updateMenuPosition);
    window.addEventListener("scroll", handleScroll, true);
    return () => {
      window.removeEventListener("resize", updateMenuPosition);
      window.removeEventListener("scroll", handleScroll, true);
    };
  }, [open, options, updateMenuPosition]);

  useEffect(() => {
    if (!open) {
      lastScrollFocusIndexRef.current = null;
      lastScrollMenuStyleRef.current = null;
      return;
    }
    const option = optionRefs.current[focusIndex];
    // preventScroll: native focus scrolling would still jump the list on hover
    // even when we skip the explicit scrollIntoView below.
    if (searchable && !optionsFocused) {
      // Only pull focus INTO the menu, never around inside it: a slot may own
      // focus (the rename field of the Abwesenheitsart select), and re-running
      // this effect must not yank it back to the search field mid-edit.
      const active = document.activeElement;
      const insideMenu =
        active instanceof Node && menuRef.current?.contains(active);
      if (!insideMenu) searchRef.current?.focus({ preventScroll: true });
    } else {
      option?.focus({ preventScroll: true });
    }
    // A long list (the calendar's 101 years) otherwise opens scrolled to the
    // top, with the current value off-screen. This waits for menuStyle, because
    // the maxHeight it carries is what makes the menu scrollable at all —
    // scrolling before that would move the page instead of the list.
    //
    // "nearest" only moves when the option is outside the visible range — never
    // re-centres an already-visible row under the pointer. Hover-driven focus
    // changes skip scroll (#2212); layout-driven menuStyle updates still reveal
    // the focused option after resize/reposition. Open uses a captured index so
    // an early mouseEnter cannot steal the one-shot reveal of the selection.
    if (!menuStyle) return;

    if (forceOpenScrollRef.current) {
      forceOpenScrollRef.current = false;
      const openIndex = openScrollIndexRef.current ?? focusIndex;
      openScrollIndexRef.current = null;
      optionRefs.current[openIndex]?.scrollIntoView({ block: "nearest" });
      lastScrollFocusIndexRef.current = focusIndex;
      lastScrollMenuStyleRef.current = menuStyle;
      return;
    }

    const focusChanged = lastScrollFocusIndexRef.current !== focusIndex;
    const layoutChanged = lastScrollMenuStyleRef.current !== menuStyle;
    lastScrollFocusIndexRef.current = focusIndex;
    lastScrollMenuStyleRef.current = menuStyle;

    // No real input change (e.g. Strict Mode re-run after a suppressed hover).
    if (!focusChanged && !layoutChanged) return;
    // Pure hover focus move: leave scroll position alone.
    if (focusChanged && suppressFocusScrollRef.current && !layoutChanged) {
      return;
    }

    option?.scrollIntoView({ block: "nearest" });
  }, [open, focusIndex, menuStyle, optionsFocused, searchable]);

  // Every fresh opening starts in the search field again.
  useEffect(() => {
    if (!open) setOptionsFocused(false);
  }, [open]);

  const resetTypeahead = useCallback(() => {
    typeaheadBufferRef.current = "";
    if (typeaheadTimerRef.current !== null) {
      clearTimeout(typeaheadTimerRef.current);
      typeaheadTimerRef.current = null;
    }
  }, []);

  useEffect(() => {
    if (!open) resetTypeahead();
  }, [open, resetTypeahead]);

  useEffect(() => resetTypeahead, [resetTypeahead]);

  const closeAndReturnFocus = useCallback(() => {
    setOpen(false);
    buttonRef.current?.focus();
  }, [setOpen]);

  const selectAt = useCallback(
    (index: number) => {
      const option = options[index];
      if (!option || option.disabled) return;
      onChange(option.value);
      // A multi-select stays open: picking a second class without reopening
      // the menu is the whole point of the mode.
      if (isMulti) return;
      closeAndReturnFocus();
    },
    [closeAndReturnFocus, isMulti, onChange, options],
  );

  const markOpenScrollTo = (index: number) => {
    forceOpenScrollRef.current = true;
    openScrollIndexRef.current = index;
  };

  const openAtSelected = () => {
    suppressFocusScrollRef.current = false;
    markOpenScrollTo(selectedIndex);
    setFocusIndex(selectedIndex);
    setOpen(true);
  };

  const handleTypeaheadKey = (
    event: KeyboardEvent<HTMLButtonElement>,
  ): boolean => {
    // A search field IS the type-to-find affordance; running both would give
    // one keystroke two meanings.
    if (searchable) return false;
    const { key } = event;
    if (key.length !== 1 || event.ctrlKey || event.metaKey || event.altKey) {
      return false;
    }
    // Space starts no search; it only extends one already in progress so
    // multi-word labels ("Anna Becker") stay reachable.
    if (key === " " && typeaheadBufferRef.current.length === 0) return false;

    event.preventDefault();
    event.stopPropagation();
    typeaheadBufferRef.current += key;
    if (typeaheadTimerRef.current !== null) {
      clearTimeout(typeaheadTimerRef.current);
    }
    typeaheadTimerRef.current = setTimeout(() => {
      typeaheadBufferRef.current = "";
      typeaheadTimerRef.current = null;
    }, TYPEAHEAD_RESET_MS);

    // While open the focused option is a real anchor; while closed only a
    // value that matches an actual option is — otherwise selectedIndex is a
    // synthesized fallback and typeahead must not skip past it.
    const hasAnchor = open || selectedOption !== undefined;
    const matchIndex = typeaheadMatchIndex(
      options,
      typeaheadBufferRef.current,
      open ? focusIndex : selectedIndex,
      hasAnchor,
    );
    suppressFocusScrollRef.current = false;
    if (matchIndex >= 0) {
      setFocusIndex(matchIndex);
    } else if (!open) {
      setFocusIndex(selectedIndex);
    }
    if (!open) {
      markOpenScrollTo(matchIndex >= 0 ? matchIndex : selectedIndex);
      setOpen(true);
    }
    return true;
  };

  const handleTriggerKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    if (disabled) return;
    if (event.key === "Escape" && open) {
      consumeEscape(event);
      setOpen(false);
      return;
    }
    // In a focus-trapped modal (Radix FocusScope) the portaled options never
    // hold DOM focus — it is yanked back to this trigger — so the option-level
    // Tab handler never runs. Close here WITHOUT preventDefault so the browser
    // continues Tab traversal to the next field and the menu does not linger
    // open with aria-expanded still true.
    if (event.key === "Tab" && open) {
      setOpen(false);
      return;
    }
    if (handleTypeaheadKey(event)) return;
    if (open) {
      if (event.key === "ArrowDown") {
        event.preventDefault();
        suppressFocusScrollRef.current = false;
        setFocusIndex((prev) => nextEnabledIndex(options, prev, 1));
        return;
      }
      if (event.key === "ArrowUp") {
        event.preventDefault();
        suppressFocusScrollRef.current = false;
        setFocusIndex((prev) => nextEnabledIndex(options, prev, -1));
        return;
      }
      if (event.key === "Enter" || event.key === " ") {
        event.preventDefault();
        selectAt(focusIndex);
        return;
      }
    }
    if (
      event.key === "ArrowDown" ||
      event.key === "ArrowUp" ||
      event.key === "Enter" ||
      event.key === " "
    ) {
      event.preventDefault();
      openAtSelected();
    }
  };

  const moveFocus = (direction: 1 | -1) => {
    suppressFocusScrollRef.current = false;
    setOptionsFocused(true);
    setFocusIndex((prev) => nextEnabledIndex(options, prev, direction));
  };

  const handleSearchKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      // Stepping out of the field moves on from the current choice, exactly
      // like the arrow keys do inside the list. With nothing chosen there is
      // nothing to move on from, so the first step lands on the first option
      // instead of skipping it.
      if (!optionsFocused && selectedOption === undefined) {
        suppressFocusScrollRef.current = false;
        setOptionsFocused(true);
        setFocusIndex(firstEnabledIndex(options));
        return;
      }
      moveFocus(event.key === "ArrowDown" ? 1 : -1);
      return;
    }
    if (event.key === "Escape") {
      event.preventDefault();
      event.stopPropagation();
      event.nativeEvent.stopImmediatePropagation();
      closeAndReturnFocus();
      return;
    }
    onSearchKeyDown?.(event);
  };

  const handleOptionKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    if (handleTypeaheadKey(event)) return;
    switch (event.key) {
      case "ArrowDown":
        event.preventDefault();
        suppressFocusScrollRef.current = false;
        setFocusIndex((prev) => nextEnabledIndex(options, prev, 1));
        break;
      case "ArrowUp":
        event.preventDefault();
        suppressFocusScrollRef.current = false;
        setFocusIndex((prev) => nextEnabledIndex(options, prev, -1));
        break;
      case "Home":
        event.preventDefault();
        suppressFocusScrollRef.current = false;
        setFocusIndex(firstEnabledIndex(options));
        break;
      case "End":
        event.preventDefault();
        suppressFocusScrollRef.current = false;
        for (let index = options.length - 1; index >= 0; index--) {
          if (!options[index]?.disabled) {
            setFocusIndex(index);
            break;
          }
        }
        break;
      case "Enter":
      case " ":
        event.preventDefault();
        selectAt(focusIndex);
        break;
      case "Escape":
        consumeEscape(event);
        closeAndReturnFocus();
        break;
      case "Tab":
        // Close and move focus back to the trigger WITHOUT cancelling the
        // default action: the browser then continues the Tab traversal
        // from the trigger, so keyboard users reach the next field instead
        // of being trapped inside the popup.
        closeAndReturnFocus();
        break;
      default:
        break;
    }
  };

  const hasPanel =
    searchable || menuHeader !== undefined || menuFooter !== undefined;

  // The option list itself. It is the whole menu for a plain select, and the
  // middle of the panel once a search field or a slot joins it.
  const optionList = (
    <ul
      ref={(node) => {
        if (!hasPanel) menuRef.current = node;
      }}
      id={listboxId}
      role="listbox"
      aria-multiselectable={isMulti || undefined}
      style={
        hasPanel
          ? undefined
          : (menuStyle ?? { position: "fixed", visibility: "hidden" })
      }
      className={hasPanel ? `min-h-0 flex-1 ${listClassName}` : menuClassName}
      // The menu is portaled to document.body, so it sits OUTSIDE the
      // modal's [data-modal-content] subtree. useScrollLock cancels every
      // wheel/touchmove outside that subtree, which would make an open
      // menu inside a scroll-locked modal impossible to scroll. Marking
      // the menu as modal content whitelists it in the scroll-lock
      // predicate so its own options can be reached.
      data-modal-content="true"
      // Vaul/Radix dismiss the dialog on a pointerdown whose target is
      // outside the dialog content node. This menu is portaled to
      // document.body, so it is NOT inside that node — without this
      // guard the first press on an option reads as an outside
      // interaction and closes the drawer BEFORE the option's click can
      // select a value. Stopping propagation keeps the press from
      // reaching the document-level dismissal listeners; the option's
      // own onClick still fires. The internal outside-press handler
      // guards with menuRef.contains, so it is unaffected.
      onPointerDown={(event) => event.stopPropagation()}
      onMouseDown={(event) => event.stopPropagation()}
      aria-label={ariaLabel}
      // Points at the field's label element, never at the trigger: a
      // combobox referenced via aria-labelledby contributes its VALUE
      // (accname step 2E), which would name the popup after the current
      // selection instead of the field.
      aria-labelledby={ariaLabelledBy}
    >
      {options.map((option, index) => {
        // In multi mode "active" means "checked"; the single-select
        // notion of one current value does not apply.
        const isActive = isMulti
          ? selectedValueSet.has(option.value)
          : option.value === value;
        const isFocused = index === focusIndex;
        const actions = renderOptionActions?.(option);
        return (
          // The option role lives on the inner button; the list item
          // is pure structure and must not surface in the a11y tree.
          <li
            key={option.value}
            role="presentation"
            className={actions ? "flex items-center gap-1" : undefined}
          >
            <button
              ref={(el) => {
                optionRefs.current[index] = el;
              }}
              id={`${listboxId}-option-${index}`}
              type="button"
              role="option"
              aria-label={option.label}
              aria-selected={isActive}
              aria-disabled={option.disabled || undefined}
              disabled={option.disabled}
              tabIndex={isFocused ? 0 : -1}
              onClick={() => selectAt(index)}
              onKeyDown={handleOptionKeyDown}
              onMouseEnter={() => {
                if (option.disabled) return;
                suppressFocusScrollRef.current = true;
                setFocusIndex(index);
              }}
              // cn, not a template literal: the class list needs a real
              // separator, and Prettier's Tailwind plugin strips a leading
              // space inside the appended string — which silently glued
              // "text-gray-700" and "min-w-0" together before.
              className={cn(
                option.disabled
                  ? disabledOptionClassName
                  : isActive || isFocused
                    ? activeOptionClassName
                    : optionClassName,
                actions && "min-w-0 flex-1",
              )}
            >
              {isMulti ? (
                <>
                  <span
                    aria-hidden="true"
                    className={`flex h-4 w-4 shrink-0 items-center justify-center rounded border transition-colors ${
                      isActive
                        ? "border-gray-900 bg-gray-900 text-white"
                        : "border-gray-300 bg-white"
                    }`}
                  >
                    {isActive ? <Check className="h-3 w-3" /> : null}
                  </span>
                  <span className="min-w-0 flex-1 truncate">
                    {option.label}
                  </span>
                </>
              ) : (
                // The label must be able to shrink and clip whether or not the
                // row carries actions: as a bare flex child its min-width stays
                // auto, so one long custom name widens the option past the menu
                // and scrolls the list sideways on narrow viewports.
                <span className="min-w-0 flex-1 truncate" title={option.label}>
                  {option.label}
                </span>
              )}
            </button>
            {actions}
          </li>
        );
      })}
    </ul>
  );

  return (
    <div
      ref={(node) => {
        containerRef.current = node;
        if (containerNodeRef) containerNodeRef.current = node;
      }}
      className={containerClassName}
      style={containerStyle}
    >
      <button
        ref={buttonRef}
        id={id}
        type="button"
        onClick={(event) => {
          event.preventDefault();
          suppressFocusScrollRef.current = false;
          // Ref write stays outside setOpen — React may re-run functional
          // updaters, so side effects inside them are invalid (react-doctor).
          if (!open) markOpenScrollTo(selectedIndex);
          setOpen(!open);
          setFocusIndex(selectedIndex);
        }}
        onKeyDown={handleTriggerKeyDown}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={open ? listboxId : undefined}
        // The menu is portaled to document.body, outside this combobox's DOM
        // subtree. aria-owns re-parents it into the combobox in the a11y tree
        // so the aria-activedescendant option is announced even when a modal
        // focus trap keeps DOM focus on the trigger.
        aria-owns={open ? listboxId : undefined}
        // Focus traps (Radix FocusScope in Modal/FormModal) yank DOM focus
        // back from the portaled options to this trigger; activedescendant
        // keeps the highlighted option announced in that trigger-focused mode.
        aria-activedescendant={
          open ? `${listboxId}-option-${focusIndex}` : undefined
        }
        aria-label={ariaLabel}
        aria-labelledby={ariaLabelledBy}
        aria-describedby={ariaDescribedBy}
        aria-required={required || undefined}
        aria-invalid={invalid || undefined}
        role={triggerRole === "combobox" ? "combobox" : undefined}
        disabled={disabled}
        className={className}
        data-testid={testId}
      >
        {renderTrigger ? (
          renderTrigger({ selectedLabel, open })
        ) : (
          <>
            <span className="min-w-0 truncate">{selectedLabel}</span>
            <ChevronDown
              className="h-4 w-4 shrink-0 text-gray-400"
              aria-hidden
            />
          </>
        )}
      </button>
      {open && portalContainer
        ? // Menus generally portal to document.body so scrollable/overflow-hidden
          // ancestors cannot clip them. Calendar navigation instead supplies its
          // own fixed panel as the portal scope to stay inside that FocusScope.
          createPortal(
            hasPanel ? (
              <div
                ref={(node) => {
                  menuRef.current = node;
                }}
                style={{
                  ...(menuStyle ?? { position: "fixed", visibility: "hidden" }),
                  display: "flex",
                  flexDirection: "column",
                }}
                className={menuClassName}
                data-modal-content="true"
                // A pure container: the search field, the listbox and the slot
                // content carry the semantics, this element only positions
                // them and keeps the dismissal handlers off the dialog.
                role="presentation"
                onPointerDown={(event) => event.stopPropagation()}
                onMouseDown={(event) => event.stopPropagation()}
              >
                {searchable ? (
                  <div className="relative p-2 pb-1">
                    <Search
                      aria-hidden="true"
                      className="pointer-events-none absolute top-1/2 left-4.5 h-4 w-4 -translate-y-1/2 text-gray-400"
                    />
                    <input
                      ref={searchRef}
                      type="text"
                      value={searchValue ?? ""}
                      onChange={(event) => onSearchChange?.(event.target.value)}
                      onKeyDown={handleSearchKeyDown}
                      placeholder={searchPlaceholder}
                      aria-label={searchPlaceholder}
                      aria-controls={listboxId}
                      className="h-9 w-full rounded-md border border-gray-200 bg-white pr-3 pl-8 text-sm text-gray-900 placeholder:text-gray-400 focus:border-gray-300 focus:outline-none"
                    />
                  </div>
                ) : null}
                {menuHeader}
                {optionList}
                {menuFooter}
              </div>
            ) : (
              optionList
            ),
            portalContainer,
          )
        : null}
    </div>
  );
}

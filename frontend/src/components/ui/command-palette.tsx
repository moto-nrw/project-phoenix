"use client";

import { ArrowRight, Search } from "lucide-react";
import {
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
  type ReactNode,
} from "react";

import { foldForSearch } from "~/lib/search-fold";
import { cn } from "~/lib/utils";

import { SpinnerIcon } from "./icons";
import { Input } from "./input";
import { Modal } from "./modal";
import { Z_LAYER } from "./overlay-styles";
import { SectionLabel } from "./section-label";

/** One entry of a command palette: a row that leads somewhere. */
export interface CommandPaletteItem {
  /** Unique within the palette; also the option's DOM id suffix. */
  readonly id: string;
  /** The main line, usually wrapped in `CommandPaletteHighlight`. */
  readonly title: ReactNode;
  /** A second, quieter line: class, group, why the row matched. */
  readonly subtitle?: ReactNode;
  /** The symbol in front, usually a `ConceptIconTile size="xs"`. */
  readonly leading?: ReactNode;
  /** The accessible name, plain text, when `title` is not. */
  readonly label: string;
  readonly onSelect: () => void;
}

/** A titled section of entries ("Kinder", "Räume"). Empty sections are skipped. */
export interface CommandPaletteGroup {
  readonly id: string;
  readonly title: string;
  readonly items: readonly CommandPaletteItem[];
}

interface CommandPaletteProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  /** Name of the dialog and of the field ("Suchen"). */
  readonly label: string;
  readonly query: string;
  readonly onQueryChange: (next: string) => void;
  readonly placeholder: string;
  readonly groups: readonly CommandPaletteGroup[];
  /** A search is running: the field shows a spinner, the list stays. */
  readonly busy?: boolean;
  /**
   * Shown instead of the list when it has no entry: a loading skeleton, an
   * empty state or a load error. The caller decides which.
   */
  readonly placeholderContent?: ReactNode;
  /** Shown under the list, before the keyboard hints (a link to a page). */
  readonly footer?: ReactNode;
  /** Announced to screen readers when the result changes ("5 Treffer"). */
  readonly status?: string;
  readonly maxLength?: number;
}

// The option itself is a bare row; the marked row's tint sits on its inner
// span, the way the sidebar rows carry theirs.
const OPTION_CLASS = "block w-full text-left";
const OPTION_ROW_CLASS =
  "flex min-h-12 items-center gap-3 rounded-xl px-3 py-2 transition-colors pointer-coarse:min-h-14";
const OPTION_IDLE_CLASS = "text-gray-700";
const OPTION_ACTIVE_CLASS = "bg-gray-100 text-gray-900";

/**
 * A search dialog with grouped results, opened by a shortcut or a button
 * (#6954). The field keeps the focus the whole time (combobox with a
 * listbox, `aria-activedescendant`): arrow keys move through every entry
 * across the groups, Home and End jump, Enter opens the marked entry, Escape
 * closes the dialog. On a phone the dialog is a sheet of almost the whole
 * height; its list has a fixed height everywhere, so typing never makes the
 * dialog jump.
 */
export function CommandPalette({
  isOpen,
  onClose,
  label,
  query,
  onQueryChange,
  placeholder,
  groups,
  busy = false,
  placeholderContent,
  footer,
  status,
  maxLength,
}: Readonly<CommandPaletteProps>) {
  const baseId = useId();
  const listId = `${baseId}-list`;
  const listRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const visibleGroups = useMemo(
    () => groups.filter((group) => group.items.length > 0),
    [groups],
  );
  const items = useMemo(
    () => visibleGroups.flatMap((group) => group.items),
    [visibleGroups],
  );
  const [activeIndex, setActiveIndex] = useState(0);
  const optionId = useCallback(
    (item: CommandPaletteItem) => `${baseId}-option-${item.id}`,
    [baseId],
  );

  // A new result marks its first entry, so Enter opens the best hit.
  const itemKey = items.map((item) => item.id).join("|");
  useEffect(() => {
    setActiveIndex(0);
  }, [itemKey]);

  const active = items[Math.min(activeIndex, items.length - 1)];
  useEffect(() => {
    if (!active) return;
    const element = document.getElementById(optionId(active));
    element?.scrollIntoView?.({ block: "nearest" });
  }, [active, optionId]);

  const handleKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (items.length === 0) return;
    const last = items.length - 1;
    const moves: Record<string, () => number> = {
      ArrowDown: () => (activeIndex >= last ? 0 : activeIndex + 1),
      ArrowUp: () => (activeIndex <= 0 ? last : activeIndex - 1),
      PageDown: () => Math.min(last, activeIndex + 5),
      PageUp: () => Math.max(0, activeIndex - 5),
    };
    const move = moves[event.key];
    if (move) {
      event.preventDefault();
      setActiveIndex(move());
      return;
    }
    if (event.key === "Enter" && active) {
      event.preventDefault();
      active.onSelect();
    }
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title=""
      ariaLabel={label}
      size="lg"
      mobileSheet
      // The dialog opens to type: the field takes the focus, not the X.
      initialFocusRef={inputRef}
    >
      {/* What a person types and finds here names children and families: a
          session recording never holds it (data-analytics-block). */}
      <div
        data-analytics-block
        className="-mx-4 -mt-4 flex flex-col md:-mx-6 md:-mt-6"
      >
        <div className="relative border-b border-gray-100 py-3 pr-14 pl-4 md:pl-6">
          <span
            aria-hidden="true"
            className={`pointer-events-none absolute top-3 left-7 ${Z_LAYER.raised} flex h-12 items-center text-gray-400 md:left-9 [&>svg]:h-5 [&>svg]:w-5`}
          >
            <Search />
          </span>
          <Input
            ref={inputRef}
            type="text"
            inputMode="search"
            aria-label={label}
            placeholder={placeholder}
            value={query}
            maxLength={maxLength}
            autoComplete="off"
            autoCorrect="off"
            spellCheck={false}
            enterKeyHint="go"
            role="combobox"
            aria-autocomplete="list"
            aria-expanded={items.length > 0}
            aria-controls={listId}
            aria-activedescendant={active ? optionId(active) : undefined}
            className="pr-10 pl-11"
            onChange={(event) => onQueryChange(event.target.value)}
            onKeyDown={handleKeyDown}
          />
          {busy ? (
            <span
              aria-hidden="true"
              className={`pointer-events-none absolute top-3 right-17 ${Z_LAYER.raised} flex h-12 items-center text-gray-400`}
            >
              <SpinnerIcon className="h-4 w-4" />
            </span>
          ) : null}
        </div>
        <div
          ref={listRef}
          className="h-[calc(100dvh-13rem)] scrollbar-thin overflow-y-auto overscroll-contain px-2 py-2 sm:h-[min(28rem,60dvh)] md:px-3"
        >
          {items.length === 0 ? (
            placeholderContent
          ) : (
            <div id={listId} role="listbox" aria-label={label}>
              {visibleGroups.map((group) => (
                <div
                  key={group.id}
                  role="group"
                  aria-labelledby={`${baseId}-group-${group.id}`}
                  className="pb-2"
                >
                  <SectionLabel
                    as="p"
                    id={`${baseId}-group-${group.id}`}
                    className="px-3 pt-2 pb-1"
                  >
                    {group.title}
                  </SectionLabel>
                  {group.items.map((item) => {
                    const isActive = item === active;
                    return (
                      <button
                        type="button"
                        key={item.id}
                        id={optionId(item)}
                        role="option"
                        // Never focused: the field keeps the focus.
                        tabIndex={-1}
                        aria-selected={isActive}
                        aria-label={item.label}
                        onMouseMove={() => {
                          if (!isActive) setActiveIndex(items.indexOf(item));
                        }}
                        onMouseDown={(event) => event.preventDefault()}
                        onClick={item.onSelect}
                        className={OPTION_CLASS}
                      >
                        <span
                          className={cn(
                            OPTION_ROW_CLASS,
                            isActive ? OPTION_ACTIVE_CLASS : OPTION_IDLE_CLASS,
                          )}
                        >
                          {item.leading}
                          <span className="min-w-0 flex-1">
                            <span className="block truncate text-sm font-medium text-gray-900">
                              {item.title}
                            </span>
                            {item.subtitle ? (
                              <span className="line-clamp-2 block text-xs text-gray-500">
                                {item.subtitle}
                              </span>
                            ) : null}
                          </span>
                          <ArrowRight
                            aria-hidden="true"
                            className={cn(
                              "h-4 w-4 shrink-0 text-gray-400 transition-opacity",
                              isActive ? "opacity-100" : "opacity-0",
                            )}
                          />
                        </span>
                      </button>
                    );
                  })}
                </div>
              ))}
            </div>
          )}
        </div>
        <div className="flex items-center justify-between gap-4 border-t border-gray-100 px-4 py-3 md:px-6">
          <div className="min-w-0 truncate text-sm">{footer}</div>
          <p className="hidden shrink-0 items-center gap-3 text-xs text-gray-500 pointer-fine:flex">
            <span>
              <CommandPaletteKey>↑</CommandPaletteKey>
              <CommandPaletteKey>↓</CommandPaletteKey> auswählen
            </span>
            <span>
              <CommandPaletteKey>↵</CommandPaletteKey> öffnen
            </span>
            <span>
              <CommandPaletteKey>Esc</CommandPaletteKey> schließen
            </span>
          </p>
        </div>
        <p className="sr-only" role="status" aria-live="polite">
          {status}
        </p>
      </div>
    </Modal>
  );
}

function CommandPaletteKey({ children }: Readonly<{ children: ReactNode }>) {
  return (
    <kbd className="mr-1 inline-flex min-w-5 items-center justify-center rounded-md border border-gray-200 px-1 font-sans text-xs text-gray-600">
      {children}
    </kbd>
  );
}

/**
 * Text with the part that matches the query in bold, compared like a name
 * search (case and accents fold: "muller" marks "Müller").
 */
export function CommandPaletteHighlight({
  text,
  query,
}: Readonly<{ text: string; query: string }>) {
  const range = matchRange(text, query);
  if (!range) return <>{text}</>;
  return (
    <>
      {text.slice(0, range[0])}
      <mark className="bg-transparent font-semibold text-gray-900">
        {text.slice(range[0], range[1])}
      </mark>
      {text.slice(range[1])}
    </>
  );
}

/**
 * The span of `text` whose folded form contains the folded query, or null.
 * Folding may change a character's length (ß → ss), so every folded
 * character remembers the original index it came from.
 */
export function matchRange(
  text: string,
  query: string,
): readonly [number, number] | null {
  const needle = foldForSearch(query.trim());
  if (!needle) return null;
  let folded = "";
  const origin: number[] = [];
  for (let index = 0; index < text.length; index++) {
    const piece = foldForSearch(text[index] ?? "");
    folded += piece;
    for (let i = 0; i < piece.length; i++) origin.push(index);
  }
  const start = folded.indexOf(needle);
  if (start < 0) return null;
  const end = origin[start + needle.length - 1];
  const from = origin[start];
  if (from === undefined || end === undefined) return null;
  return [from, end + 1];
}

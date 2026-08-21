"use client";

import { Check, ChevronDown, Pencil, Plus, X } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";

import { ListboxDropdown } from "./listbox-dropdown";
import { LOCATION_COLORS } from "~/lib/location-helper";

export interface CreatableSelectOption {
  readonly value: string;
  readonly label: string;
  /** Fixed options render without the edit/retire affordances. */
  readonly fixed?: boolean;
  /**
   * A retired option stays selectable only while it is the current value, so a
   * record filed under it keeps rendering its own name instead of silently
   * falling back to something else.
   */
  readonly inactive?: boolean;
}

interface CreatableSelectProps {
  readonly value: string;
  readonly options: readonly CreatableSelectOption[];
  readonly onChange: (value: string) => void;
  readonly id?: string;
  readonly ariaLabel?: string;
  readonly ariaLabelledBy?: string;
  readonly disabled?: boolean;
  readonly placeholder?: string;
  readonly searchPlaceholder?: string;
  /**
   * Adds the typed name and selects it. Omit to get a plain searchable select —
   * that is how the component renders for someone without the permission to
   * manage the list.
   */
  readonly onCreate?: (name: string) => Promise<string>;
  /** Renames an existing option. Omit to hide the rename affordance. */
  readonly onRename?: (value: string, name: string) => Promise<void>;
  /** Retires / restores an option. Omit to hide the affordance. */
  readonly onSetActive?: (value: string, isActive: boolean) => Promise<void>;
  /** Wording of the create row, e.g. `(n) => \`„${n}" hinzufügen\``. */
  readonly createLabel?: (name: string) => string;
  readonly testId?: string;
}

const TRIGGER_CLASS =
  "moto-content-surface flex h-10 w-full items-center justify-between gap-2 rounded-lg border px-3 text-left text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:bg-gray-100 disabled:text-gray-500 disabled:opacity-80";

const MENU_CLASS = "rounded-xl border border-gray-200 bg-white shadow-lg";
const LIST_CLASS = "scrollbar-thin overflow-y-auto px-2 pb-2";

// Module-level so the default prop keeps a stable identity across renders.
const DEFAULT_CREATE_LABEL = (name: string) => `„${name}" hinzufügen`;

const ROW_CLASS =
  "flex w-full items-center gap-2 rounded-md px-2 py-2 text-left text-sm transition-colors";
const OPTION_CLASS = `${ROW_CLASS} text-gray-700 hover:bg-gray-50`;
const ACTIVE_OPTION_CLASS = `${ROW_CLASS} bg-gray-50 font-medium text-gray-900`;
const DISABLED_OPTION_CLASS = `${ROW_CLASS} cursor-not-allowed text-gray-500 opacity-60`;
const ICON_BUTTON_CLASS =
  "rounded-md p-1.5 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700 disabled:opacity-40";

function normalize(value: string): string {
  return value.trim().toLowerCase();
}

/**
 * CreatableSelect: the kit's searchable single-select that can grow its own
 * list.
 *
 * Reach for it when the option set is data a school maintains rather than a
 * fixed enum — the missing entry is then a normal thing to hit, and sending
 * someone to a settings page to add it (and back again to pick it) loses the
 * form they were filling in. The typed text doubles as the search term and as
 * the name of the entry to add, so "look for it, then add it if it isn't there"
 * is one uninterrupted move.
 *
 * It is the same select as {@link CustomSelect} and {@link MultiSelect}: all
 * three run on ListboxDropdown (portaled menu, keyboard navigation, focus and
 * ARIA wiring) and only differ in what the menu carries — here a search field,
 * a row that adds the typed name, and per-row rename/retire controls.
 *
 * Editing lives on the row itself, not behind a separate management screen:
 * that is where the school notices the typo. Rows marked `fixed` carry no
 * affordances at all — code-owned options must not look editable.
 *
 * Pass only the callbacks the current user is allowed to use: without
 * `onCreate` this is an ordinary searchable select, which is exactly the right
 * shape for someone who may pick from the list but not change it.
 */
export function CreatableSelect({
  value,
  options,
  onChange,
  id,
  ariaLabel,
  ariaLabelledBy,
  disabled = false,
  placeholder = "Bitte wählen",
  searchPlaceholder = "Suchen oder neuen Namen eingeben",
  onCreate,
  onRename,
  onSetActive,
  createLabel = DEFAULT_CREATE_LABEL,
  testId,
}: CreatableSelectProps) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [editingValue, setEditingValue] = useState<string | null>(null);
  const [editingName, setEditingName] = useState("");
  const editRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!open) {
      setQuery("");
      setError(null);
      setEditingValue(null);
    }
  }, [open]);

  useEffect(() => {
    if (editingValue) editRef.current?.focus();
  }, [editingValue]);

  const selected = options.find((option) => option.value === value);

  // A retired option is hidden so nobody files new records under it — except
  // when it IS the current value (otherwise editing an old record would
  // silently rewrite it), and except for whoever may restore it, who has to be
  // able to find it again.
  const canManageActive = Boolean(onSetActive);
  const visible = useMemo(
    () =>
      options.filter(
        (option) =>
          !option.inactive || option.value === value || canManageActive,
      ),
    [options, value, canManageActive],
  );

  const filtered = useMemo(() => {
    const needle = normalize(query);
    if (!needle) return visible;
    return visible.filter((option) => normalize(option.label).includes(needle));
  }, [visible, query]);

  // A retired art may still be restored, but not picked: the listbox skips
  // disabled options for both the pointer and the arrow keys.
  const listOptions = useMemo(
    () =>
      filtered.map((option) => ({
        value: option.value,
        label: option.label,
        disabled: Boolean(option.inactive) && option.value !== value,
      })),
    [filtered, value],
  );

  const byValue = useMemo(
    () => new Map(options.map((option) => [option.value, option])),
    [options],
  );

  const trimmedQuery = query.trim();
  const exactExists = options.some(
    (option) => normalize(option.label) === normalize(trimmedQuery),
  );
  const canCreate = Boolean(onCreate) && trimmedQuery !== "" && !exactExists;

  const runCreate = async () => {
    if (!onCreate || !canCreate) return;
    setBusy(true);
    setError(null);
    try {
      const created = await onCreate(trimmedQuery);
      onChange(created);
      setOpen(false);
    } catch (err) {
      setError(
        err instanceof Error && err.message
          ? err.message
          : "Name konnte nicht hinzugefügt werden.",
      );
    } finally {
      setBusy(false);
    }
  };

  const runRename = async () => {
    if (!onRename || !editingValue) return;
    const name = editingName.trim();
    if (name === "") return;
    setBusy(true);
    setError(null);
    try {
      await onRename(editingValue, name);
      setEditingValue(null);
    } catch (err) {
      setError(
        err instanceof Error && err.message
          ? err.message
          : "Name konnte nicht geändert werden.",
      );
    } finally {
      setBusy(false);
    }
  };

  const runSetActive = async (option: CreatableSelectOption) => {
    if (!onSetActive) return;
    setBusy(true);
    setError(null);
    try {
      await onSetActive(option.value, Boolean(option.inactive));
    } catch (err) {
      setError(
        err instanceof Error && err.message
          ? err.message
          : "Änderung konnte nicht gespeichert werden.",
      );
    } finally {
      setBusy(false);
    }
  };

  const editing = editingValue ? byValue.get(editingValue) : undefined;

  const menuHeader = (
    <div className="flex flex-col gap-1 px-2 pt-1">
      {editing ? (
        // Named explicitly: right under the search field an unlabelled second
        // input would read as a second search.
        <div className="flex flex-col gap-1 rounded-md bg-gray-50 p-2">
          <span className="text-xs text-gray-500">
            Neuer Name für „{editing.label}"
          </span>
          <div className="flex items-center gap-1">
            <input
              ref={editRef}
              type="text"
              value={editingName}
              onChange={(event) => setEditingName(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") {
                  event.preventDefault();
                  void runRename();
                }
                if (event.key === "Escape") {
                  event.preventDefault();
                  event.stopPropagation();
                  setEditingValue(null);
                }
              }}
              aria-label={`Name von ${editing.label}`}
              className="h-8 min-w-0 flex-1 rounded-md border border-gray-200 px-2 text-sm text-gray-900 focus:border-gray-300 focus:outline-none"
            />
            <button
              type="button"
              onClick={() => void runRename()}
              disabled={busy || editingName.trim() === ""}
              aria-label="Namen speichern"
              className={ICON_BUTTON_CLASS}
            >
              <Check aria-hidden="true" className="h-4 w-4" />
            </button>
            <button
              type="button"
              onClick={() => setEditingValue(null)}
              aria-label="Umbenennen abbrechen"
              className={ICON_BUTTON_CLASS}
            >
              <X aria-hidden="true" className="h-4 w-4" />
            </button>
          </div>
        </div>
      ) : null}

      {error ? (
        <p className="py-1 text-xs" style={{ color: LOCATION_COLORS.DANGER }}>
          {error}
        </p>
      ) : null}

      {filtered.length === 0 && !canCreate ? (
        <p className="py-2 text-sm text-gray-500">Kein Treffer.</p>
      ) : null}
    </div>
  );

  const menuFooter = canCreate ? (
    <div className="px-2 pb-2">
      <button
        type="button"
        onClick={() => void runCreate()}
        disabled={busy}
        className={`${ROW_CLASS} border-t border-gray-100 pt-2.5 font-medium text-gray-900 hover:bg-gray-50 disabled:opacity-40`}
      >
        <Plus
          aria-hidden="true"
          className="h-4 w-4 flex-shrink-0"
          style={{ color: LOCATION_COLORS.GROUP_ROOM }}
        />
        <span className="min-w-0 flex-1 truncate">
          {createLabel(trimmedQuery)}
        </span>
      </button>
    </div>
  ) : null;

  return (
    <ListboxDropdown
      id={id}
      value={value}
      options={listOptions}
      onChange={onChange}
      open={open && !disabled}
      onOpenChange={setOpen}
      ariaLabel={ariaLabel}
      ariaLabelledBy={ariaLabelledBy}
      disabled={disabled}
      placeholder={placeholder}
      triggerRole="combobox"
      testId={testId}
      // The menu carries a text field, and a modal's focus trap (Radix
      // FocusScope) pulls focus straight back out of anything portaled to
      // document.body — the field could be clicked but never typed into. This
      // keeps the menu inside the trap. Outside a modal the selector matches
      // nothing and the menu portals to the body as usual.
      portalScopeSelector='[data-modal-focus-scope="true"]'
      className={TRIGGER_CLASS}
      menuClassName={MENU_CLASS}
      listClassName={LIST_CLASS}
      optionClassName={OPTION_CLASS}
      activeOptionClassName={ACTIVE_OPTION_CLASS}
      disabledOptionClassName={DISABLED_OPTION_CLASS}
      searchValue={query}
      onSearchChange={setQuery}
      searchPlaceholder={searchPlaceholder}
      onSearchKeyDown={(event) => {
        if (event.key === "Enter" && canCreate) {
          event.preventDefault();
          void runCreate();
        }
      }}
      menuHeader={menuHeader}
      menuFooter={menuFooter}
      renderOptionActions={(listOption) => {
        const option = byValue.get(listOption.value);
        if (!option) return null;
        const manageable = !option.fixed && (onRename ?? onSetActive);
        if (!option.inactive && !manageable) return null;
        return (
          <span className="flex flex-shrink-0 items-center gap-0.5 pr-1">
            {option.inactive ? (
              <span className="text-xs text-gray-400">deaktiviert</span>
            ) : null}
            {!option.fixed && onRename ? (
              <button
                type="button"
                onClick={() => {
                  setEditingValue(option.value);
                  setEditingName(option.label);
                }}
                aria-label={`${option.label} umbenennen`}
                title="Umbenennen"
                className={ICON_BUTTON_CLASS}
              >
                <Pencil aria-hidden="true" className="h-3.5 w-3.5" />
              </button>
            ) : null}
            {!option.fixed && onSetActive ? (
              <button
                type="button"
                onClick={() => void runSetActive(option)}
                disabled={busy}
                aria-label={
                  option.inactive
                    ? `${option.label} wieder aktivieren`
                    : `${option.label} deaktivieren`
                }
                title={option.inactive ? "Wieder aktivieren" : "Deaktivieren"}
                className={ICON_BUTTON_CLASS}
              >
                {option.inactive ? (
                  <Check aria-hidden="true" className="h-3.5 w-3.5" />
                ) : (
                  <X aria-hidden="true" className="h-3.5 w-3.5" />
                )}
              </button>
            ) : null}
          </span>
        );
      }}
      renderTrigger={({ open: isOpen }) => (
        <>
          {/* The selected label comes from the FULL list: while the search
              narrows the menu the trigger must keep showing what is picked,
              not fall back to the placeholder. */}
          <span
            className={`min-w-0 flex-1 truncate ${
              selected ? "text-gray-900" : "text-gray-500"
            }`}
          >
            {selected?.label ?? placeholder}
          </span>
          <ChevronDown
            aria-hidden="true"
            className={`h-4 w-4 flex-shrink-0 text-gray-400 transition-transform ${
              isOpen ? "rotate-180" : ""
            }`}
          />
        </>
      )}
    />
  );
}

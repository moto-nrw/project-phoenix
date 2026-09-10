"use client";

// Das Auswahlfeld „Art der Abwesenheit" (#2403) — eine Auswahl, mehr nicht
// (#3114).
//
// Bis dahin hingen an den Zeilen dieses Felds ein Stift zum Umbenennen und ein
// Kreuz zum Abschalten, und unter der Liste ein „… hinzufügen": eine
// Verwaltungsoberfläche in einem Auswahlfeld, geöffnet aus zwei Dialogen der
// Zeiterfassung. Wer die Arten pflegt, tut das jetzt unter
// „Datenverwaltung → Abwesenheitsarten"; das Feld daneben verlinkt sie.
//
// Bewusst weiterhin ein Hook und keine Komponente: alles Sichtbare kommt aus
// `ListboxDropdown`, lokal ist nur der getippte Suchtext.

import { ChevronDown } from "lucide-react";
import { useEffect, useMemo, useState, type ComponentProps } from "react";

import {
  useAbsenceTypeOptions,
  type AbsenceTypeOption,
} from "./use-absence-type-options";
import { ListboxDropdown } from "~/components/ui/listbox-dropdown";

type ListboxProps = ComponentProps<typeof ListboxDropdown<string>>;

const TRIGGER_CLASS =
  "moto-content-surface flex h-10 w-full items-center justify-between gap-2 rounded-lg border px-3 text-left text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:bg-gray-100 disabled:text-gray-500 disabled:opacity-80";

const MENU_CLASS = "moto-popover-surface rounded-xl border";
const LIST_CLASS = "scrollbar-thin overflow-y-auto px-2 pb-2";

const ROW_CLASS =
  "flex w-full items-center gap-2 rounded-md px-2 py-2 text-left text-sm transition-colors";
const OPTION_CLASS = `${ROW_CLASS} text-gray-700 hover:bg-gray-50`;
const ACTIVE_OPTION_CLASS = `${ROW_CLASS} bg-gray-50 font-medium text-gray-900`;
const DISABLED_OPTION_CLASS = `${ROW_CLASS} cursor-not-allowed text-gray-500 opacity-60`;

const PLACEHOLDER = "Bitte wählen";
const SEARCH_PLACEHOLDER = "Suchen";

function normalize(value: string): string {
  return value.trim().toLowerCase();
}

interface UseAbsenceTypeSelectArgs {
  readonly value: string;
  readonly onChange: (value: string) => void;
  /**
   * Mit time_tracking:manage stehen auch die Arten mit eigenem
   * Jahreskontingent zur Wahl; ohne das Recht bleiben sie außen vor, weil
   * niemand versehentlich gegen ein Kontingent buchen soll.
   */
  readonly canManage: boolean;
  readonly disabled?: boolean;
}

/**
 * useAbsenceTypeSelect liefert die Eigenschaften für ein `ListboxDropdown`,
 * das die Standardarten und die eigenen Namen der Schule in einer Liste zeigt.
 */
export function useAbsenceTypeSelect({
  value,
  onChange,
  canManage,
  disabled = false,
}: UseAbsenceTypeSelectArgs): ListboxProps {
  const { options } = useAbsenceTypeOptions(canManage);

  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");

  useEffect(() => {
    if (!open) setQuery("");
  }, [open]);

  const selected = options.find((option) => option.value === value);

  // Eine abgeschaltete Art steht nur da, solange sie der gewählte Wert ist:
  // sonst würde das Bearbeiten einer alten Abwesenheit sie still austauschen.
  const visible = useMemo(
    () =>
      options.filter(
        (option: AbsenceTypeOption) =>
          !option.inactive || option.value === value,
      ),
    [options, value],
  );

  const filtered = useMemo(() => {
    const needle = normalize(query);
    if (!needle) return visible;
    return visible.filter((option) => normalize(option.label).includes(needle));
  }, [visible, query]);

  const listOptions = useMemo(
    () =>
      filtered.map((option) => ({
        value: option.value,
        label: option.label,
      })),
    [filtered],
  );

  const menuHeader =
    filtered.length === 0 ? (
      <p className="px-2 py-2 text-sm text-gray-500">Kein Treffer.</p>
    ) : null;

  return {
    value,
    options: listOptions,
    onChange,
    open: open && !disabled,
    onOpenChange: setOpen,
    disabled,
    placeholder: PLACEHOLDER,
    triggerRole: "combobox",
    // Das Menü trägt ein Suchfeld, und die Fokusfalle eines Dialogs (Radix
    // FocusScope) zieht den Fokus aus allem heraus, was an document.body
    // hängt — das Feld ließe sich anklicken, aber nicht beschreiben. Außerhalb
    // eines Dialogs greift der Selektor nicht und das Menü hängt wie sonst am
    // Body.
    portalScopeSelector: '[data-modal-focus-scope="true"]',
    className: TRIGGER_CLASS,
    menuClassName: MENU_CLASS,
    listClassName: LIST_CLASS,
    optionClassName: OPTION_CLASS,
    activeOptionClassName: ACTIVE_OPTION_CLASS,
    disabledOptionClassName: DISABLED_OPTION_CLASS,
    searchValue: query,
    onSearchChange: setQuery,
    searchPlaceholder: SEARCH_PLACEHOLDER,
    menuHeader,
    renderTrigger: ({ open: isOpen }) => (
      <>
        {/* Die Beschriftung kommt aus der VOLLEN Liste: während die Suche das
            Menü verengt, muss der Auslöser weiter zeigen, was gewählt ist. */}
        <span
          className={`min-w-0 flex-1 truncate ${
            selected ? "text-gray-900" : "text-gray-500"
          }`}
        >
          {selected?.label ?? PLACEHOLDER}
        </span>
        <ChevronDown
          aria-hidden="true"
          className={`h-4 w-4 flex-shrink-0 text-gray-400 transition-transform ${
            isOpen ? "rotate-180" : ""
          }`}
        />
      </>
    ),
  };
}

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
// `CustomSelect` mit Suchfeld (#5580), lokal ist nur die Auswahl der Arten.

import { useMemo, type ComponentProps } from "react";

import {
  STANDARD_ABSENCE_OPTIONS,
  useAbsenceTypeOptions,
  type AbsenceTypeOption,
} from "./use-absence-type-options";
import { CustomSelect } from "~/components/ui/custom-select";

type CustomSelectProps = ComponentProps<typeof CustomSelect>;

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
 * useAbsenceTypeSelect liefert die Eigenschaften für ein `CustomSelect`, das
 * die Standardarten und die eigenen Namen der Schule in einer durchsuchbaren
 * Liste zeigt.
 */
export function useAbsenceTypeSelect({
  value,
  onChange,
  canManage,
  disabled = false,
}: UseAbsenceTypeSelectArgs): CustomSelectProps {
  const { options } = useAbsenceTypeOptions(
    canManage,
    STANDARD_ABSENCE_OPTIONS,
    value,
  );

  // Eine abgeschaltete Art steht nur da, solange sie der gewählte Wert ist:
  // sonst würde das Bearbeiten einer alten Abwesenheit sie still austauschen.
  const visible = useMemo(
    () =>
      options
        .filter(
          (option: AbsenceTypeOption) =>
            !option.inactive || option.value === value,
        )
        .map((option) => ({ value: option.value, label: option.label })),
    [options, value],
  );

  return {
    value,
    options: visible,
    onChange,
    disabled,
    searchable: true,
  };
}

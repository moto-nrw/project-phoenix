"use client";

/**
 * TimetableAddMenu: the single "+ Neu" entry point in the timetable toolbar.
 */

import { CalendarPlus, ChevronDown, Plus, Repeat } from "lucide-react";

import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";

interface TimetableAddMenuProps {
  /** Create a single one-off appointment. */
  onAddInstance: () => void;
  /** Create a recurring series. */
  onAddSeries: () => void;
  disabled?: boolean;
}

export function TimetableAddMenu({
  onAddInstance,
  onAddSeries,
  disabled = false,
}: TimetableAddMenuProps) {
  return (
    // Die Aktionen der Kopfkarte stehen unter sm in einer gemeinsamen Zeile;
    // ein Knopf, der sich die volle Breite nimmt, drängt die anderen heraus.
    <OverflowMenu
      ariaLabel="Neu anlegen"
      disabled={disabled}
      wrapperClassName="flex-1 sm:flex-none"
      triggerButton={{ variant: "primary", size: "md" }}
      triggerClassName="w-full gap-1.5"
      triggerContent={
        <>
          <Plus className="h-4 w-4" aria-hidden />
          Neu
          <ChevronDown className="h-4 w-4" aria-hidden />
        </>
      }
      items={[
        { kind: "header", label: "Termin anlegen" },
        {
          label: "Einmaliger Termin",
          description: "Findet nur an diesem Tag statt.",
          icon: <CalendarPlus className="size-4" aria-hidden />,
          onClick: onAddInstance,
        },
        {
          label: "Regelmäßiger Termin",
          description:
            "Für Angebote, die jede Woche oder alle zwei Wochen stattfinden.",
          icon: <Repeat className="size-4" aria-hidden />,
          onClick: onAddSeries,
        },
      ]}
    />
  );
}

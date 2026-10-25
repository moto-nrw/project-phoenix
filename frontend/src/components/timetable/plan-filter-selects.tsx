"use client";

import { CustomSelect } from "~/components/ui/custom-select";

export interface PlanFilterOption {
  readonly id: string;
  readonly name: string;
}

interface PlanFilterSelectsProps {
  /** OGS-Gruppen; `null`, wenn die Person die Gruppenliste nicht lesen darf. */
  readonly groups: readonly PlanFilterOption[] | null;
  readonly persons: readonly PlanFilterOption[];
  readonly groupId: string | null;
  readonly staffId: string | null;
  readonly onGroupChange: (groupId: string | null) => void;
  readonly onStaffChange: (staffId: string | null) => void;
}

/**
 * Filter „Gruppe“ und „Person“ des Betreuungsplans (#4766), in der
 * Kontextzeile wie die Personenauswahl des Dienstplans. Die erste Option
 * jeder Auswahl zeigt wieder den ganzen Plan.
 */
export function PlanFilterSelects({
  groups,
  persons,
  groupId,
  staffId,
  onGroupChange,
  onStaffChange,
}: PlanFilterSelectsProps) {
  return (
    <>
      {groups && (
        <CustomSelect
          value={groupId ?? ""}
          options={[
            { value: "", label: "Alle Gruppen" },
            ...groups.map((group) => ({ value: group.id, label: group.name })),
          ]}
          onChange={(value) => onGroupChange(value === "" ? null : value)}
          ariaLabel="Nach Gruppe filtern"
          className="w-full shrink-0 sm:w-48"
          testId="plan-filter-group"
        />
      )}
      <CustomSelect
        value={staffId ?? ""}
        options={[
          { value: "", label: "Alle Personen" },
          ...persons.map((person) => ({
            value: person.id,
            label: person.name,
          })),
        ]}
        onChange={(value) => onStaffChange(value === "" ? null : value)}
        ariaLabel="Nach Person filtern"
        className="w-full shrink-0 sm:w-48"
        testId="plan-filter-staff"
      />
    </>
  );
}

/** Sortierte Auswahl aus einer Namensliste (ID → Name). */
export function sortedFilterOptions(
  names: ReadonlyMap<string, string>,
): PlanFilterOption[] {
  return [...names.entries()]
    .map(([id, name]) => ({ id, name }))
    .sort((a, b) => a.name.localeCompare(b.name, "de"));
}

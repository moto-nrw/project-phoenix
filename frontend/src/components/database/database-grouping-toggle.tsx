"use client";

import { Layers } from "lucide-react";
import {
  ListboxChevron,
  ListboxDropdown,
} from "~/components/ui/listbox-dropdown";

interface GroupingOption<K extends string> {
  value: K;
  label: string;
}

interface DatabaseGroupingToggleProps<K extends string> {
  value: K;
  options: GroupingOption<K>[];
  onChange: (next: K) => void;
}

export function DatabaseGroupingToggle<K extends string>({
  value,
  options,
  onChange,
}: DatabaseGroupingToggleProps<K>) {
  return (
    <ListboxDropdown
      value={value}
      options={options}
      onChange={onChange}
      ariaLabel="Gruppieren"
      placeholder={options[0]?.label ?? ""}
      className="font-medium text-gray-700"
      menuAlign="end"
      menuClassName="w-44"
      renderTrigger={({ selectedLabel, open }) => (
        <>
          <Layers className="h-4 w-4 text-gray-500" aria-hidden />
          <span className="text-gray-500">Gruppieren:</span>
          <span className="font-semibold text-gray-900">{selectedLabel}</span>
          <ListboxChevron open={open} />
        </>
      )}
    />
  );
}

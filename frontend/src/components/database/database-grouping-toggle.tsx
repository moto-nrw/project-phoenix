"use client";

import { ChevronDown, Layers } from "lucide-react";
import { ListboxDropdown } from "~/components/ui/listbox-dropdown";

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
      className="flex h-10 items-center gap-2 rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 hover:bg-gray-50"
      menuAlign="end"
      menuClassName="w-44 overflow-y-auto rounded-lg border border-gray-200 bg-white shadow-lg"
      optionClassName="flex w-full items-center justify-between px-3 py-2 text-left text-sm text-gray-700 hover:bg-gray-50"
      activeOptionClassName="flex w-full items-center justify-between bg-[#DCF5C1]/60 px-3 py-2 text-left text-sm font-semibold text-gray-900"
      renderTrigger={({ selectedLabel }) => (
        <>
          <Layers className="h-4 w-4 text-gray-500" aria-hidden />
          <span className="text-gray-500">Gruppieren:</span>
          <span className="font-semibold text-gray-900">{selectedLabel}</span>
          <ChevronDown className="h-3.5 w-3.5 text-gray-500" aria-hidden />
        </>
      )}
    />
  );
}

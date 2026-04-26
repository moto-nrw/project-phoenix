"use client";

import { ChevronDown, Layers } from "lucide-react";
import { useEffect, useRef, useState } from "react";

export interface GroupingOption<K extends string> {
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
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const handleClick = (event: MouseEvent) => {
      if (
        containerRef.current &&
        !containerRef.current.contains(event.target as Node)
      ) {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, [open]);

  const activeLabel =
    options.find((option) => option.value === value)?.label ??
    options[0]?.label ??
    "";

  return (
    <div ref={containerRef} className="relative">
      <button
        type="button"
        onClick={() => setOpen((prev) => !prev)}
        aria-haspopup="listbox"
        aria-expanded={open}
        className="flex h-10 items-center gap-2 rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 hover:bg-gray-50"
      >
        <Layers className="h-4 w-4 text-gray-500" aria-hidden />
        <span className="text-gray-500">Gruppieren:</span>
        <span className="font-semibold text-gray-900">{activeLabel}</span>
        <ChevronDown className="h-3.5 w-3.5 text-gray-500" aria-hidden />
      </button>
      {open ? (
        <ul
          role="listbox"
          className="absolute right-0 z-50 mt-1 w-44 overflow-hidden rounded-lg border border-gray-200 bg-white shadow-lg"
        >
          {options.map((option) => {
            const isActive = option.value === value;
            return (
              <li key={option.value}>
                <button
                  type="button"
                  role="option"
                  aria-selected={isActive}
                  onClick={() => {
                    onChange(option.value);
                    setOpen(false);
                  }}
                  className={
                    isActive
                      ? "flex w-full items-center justify-between bg-[#DCF5C1]/60 px-3 py-2 text-left text-sm font-semibold text-gray-900"
                      : "flex w-full items-center justify-between px-3 py-2 text-left text-sm text-gray-700 hover:bg-gray-50"
                  }
                >
                  {option.label}
                </button>
              </li>
            );
          })}
        </ul>
      ) : null}
    </div>
  );
}

"use client";

import { useTranslations } from "next-intl";
import { Avatar } from "~/components/ui/avatar";

/**
 * Der Umschalter zwischen mehreren Kindern (Entscheidung E9).
 *
 * Bei einem Kind rendert er nichts: eine Auswahl mit einer Moeglichkeit ist
 * keine Auswahl. Bei mehreren steht je Kind ein Initialen-Kreis; der aktive
 * traegt einen gruenen Ring und den Namen darunter.
 */

export interface ChildSwitcherItem {
  readonly studentId: string;
  readonly name: string;
}

export function ChildSwitcher({
  items,
  activeId,
  onSelect,
}: Readonly<{
  items: readonly ChildSwitcherItem[];
  activeId: string;
  onSelect: (studentId: string) => void;
}>) {
  const t = useTranslations("parentChild");
  if (items.length < 2) return null;

  return (
    <div
      role="tablist"
      aria-label={t("switcherLabel")}
      className="-mx-1 flex gap-3 overflow-x-auto px-1 pb-1"
    >
      {items.map((item) => {
        const active = item.studentId === activeId;
        return (
          <button
            key={item.studentId}
            type="button"
            role="tab"
            aria-selected={active}
            onClick={() => onSelect(item.studentId)}
            className="flex min-h-12 w-20 shrink-0 flex-col items-center gap-1.5 rounded-xl py-1 focus-visible:ring-2 focus-visible:ring-[#5080D8] focus-visible:outline-none"
          >
            <Avatar
              name={item.name}
              decorative
              className={
                active
                  ? "ring-moto-green size-12 text-[15px] ring-2 ring-offset-2"
                  : "size-12 text-[15px] opacity-70"
              }
            />
            <span
              className={
                active
                  ? "w-full truncate text-center text-[15px] font-semibold text-gray-900"
                  : "w-full truncate text-center text-[15px] text-gray-600"
              }
            >
              {item.name}
            </span>
          </button>
        );
      })}
    </div>
  );
}

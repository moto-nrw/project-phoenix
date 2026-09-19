"use client";

import { ChevronRight } from "lucide-react";
import type { ReactNode } from "react";
import { cn } from "~/lib/utils";
import { Checkbox } from "~/components/ui/checkbox";
import NavigationLink from "~/components/ui/navigation-link";

interface DatabaseListItemProps {
  title: string;
  subtitle: ReactNode;
  isSelected: boolean;
  /**
   * Öffnet die Auswahl im Pane daneben. Entfällt, wenn die Zeile über `href`
   * auf die Objektroute führt.
   */
  onSelect?: () => void;
  /**
   * Ziel der Objektansicht. Mit `href` ist die Zeile ein Link (Mittelklick,
   * „in neuem Tab öffnen" und Tastatur funktionieren), kein Knopf: der Weg
   * zum Objekt ist portalweit die Route (BAUARTEN-SPEC Bauart 1 Regel 2).
   */
  href?: string;
  trailingAccessory?: ReactNode;
  selectionMode?: boolean;
  isChecked?: boolean;
  onToggleSelection?: () => void;
}

const ROW_CLASS =
  "flex w-full items-center gap-3 border-b border-gray-100 px-4 py-2.5 text-left transition-colors hover:bg-gray-50";

export function DatabaseListItem({
  title,
  subtitle,
  isSelected,
  onSelect,
  href,
  trailingAccessory,
  selectionMode = false,
  isChecked = false,
  onToggleSelection,
}: DatabaseListItemProps) {
  const content = (
    <>
      <div className="min-w-0 flex-1">
        <div
          className={cn(
            "truncate text-sm text-gray-900",
            isSelected || isChecked ? "font-semibold" : "font-medium",
          )}
        >
          {title}
        </div>
        {subtitle ? (
          <div className="truncate text-xs text-gray-500">{subtitle}</div>
        ) : null}
      </div>
      {trailingAccessory}
    </>
  );

  if (selectionMode) {
    return (
      <label
        className={cn(
          "cursor-pointer",
          ROW_CLASS,
          isChecked && "bg-moto-green-soft/60 hover:bg-moto-green-soft/70",
        )}
      >
        <Checkbox checked={isChecked} onChange={onToggleSelection} />
        {content}
      </label>
    );
  }

  const chevron = (
    <ChevronRight
      className={cn(
        "h-4 w-4 shrink-0",
        isSelected ? "text-moto-green" : "text-gray-400",
      )}
      aria-hidden
    />
  );
  const rowClassName = cn(
    ROW_CLASS,
    isSelected && "bg-moto-green-soft/60 hover:bg-moto-green-soft/70",
  );

  if (href) {
    return (
      <NavigationLink
        href={href}
        aria-current={isSelected ? "true" : undefined}
        className={rowClassName}
      >
        {content}
        {chevron}
      </NavigationLink>
    );
  }

  return (
    <button
      type="button"
      onClick={onSelect}
      aria-current={isSelected ? "true" : undefined}
      className={rowClassName}
    >
      {content}
      {chevron}
    </button>
  );
}

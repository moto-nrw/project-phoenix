"use client";

import type { ReactNode } from "react";
import { cn } from "~/lib/utils";
import { Checkbox } from "~/components/ui/checkbox";
import { ListRowLink } from "~/components/ui/list-row-link";

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
  /** Vor dem Titel, etwa ein Avatar. */
  leadingAccessory?: ReactNode;
  trailingAccessory?: ReactNode;
  selectionMode?: boolean;
  isChecked?: boolean;
  onToggleSelection?: () => void;
}

/** Trennlinie und Höhe der Zeile, im Auswahlmodus wie als Link oder Knopf. */
const ROW_FRAME = "border-b border-gray-100 py-2.5";

export function DatabaseListItem({
  title,
  subtitle,
  isSelected,
  onSelect,
  href,
  leadingAccessory,
  trailingAccessory,
  selectionMode = false,
  isChecked = false,
  onToggleSelection,
}: DatabaseListItemProps) {
  const content = (
    <>
      {leadingAccessory}
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
          "flex w-full cursor-pointer items-center gap-3 px-4 text-left transition-colors",
          ROW_FRAME,
          isChecked
            ? "bg-moto-green-soft/60 hover:bg-moto-green-soft/70"
            : "hover:bg-gray-50",
        )}
      >
        <Checkbox checked={isChecked} onChange={onToggleSelection} />
        {content}
      </label>
    );
  }

  // Navigationszeile aus dem Kit (#5593): Auswahl grün mit aria-current.
  const subtitleText = subtitle ? (
    <span className="block truncate">{subtitle}</span>
  ) : undefined;
  const shared = {
    icon: leadingAccessory,
    title,
    subtitle: subtitleText,
    badge: trailingAccessory,
    selected: isSelected,
    className: ROW_FRAME,
  };

  if (href) return <ListRowLink href={href} {...shared} />;
  return <ListRowLink onClick={onSelect} {...shared} />;
}

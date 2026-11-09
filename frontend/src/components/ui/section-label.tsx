import type { HTMLAttributes, ReactNode } from "react";

import { RequiredMark } from "~/components/ui/field-label";
import { cn } from "~/lib/utils";

type SectionLabelElement = "h2" | "h3" | "h4" | "legend" | "p";

interface SectionLabelProps extends HTMLAttributes<HTMLElement> {
  /** Das Element: eine Überschrift, die `legend` einer Gruppe oder ein Absatz. */
  readonly as?: SectionLabelElement;
  /** Stern nach dem Text, für die `legend` einer Pflichtgruppe. */
  readonly required?: boolean;
  readonly children: ReactNode;
}

/** Die Optik eines Gruppenkopfs, eine Stufe leiser als der Kartentitel. */
export const SECTION_LABEL_CLASS = "text-sm font-semibold text-gray-600";

/**
 * Der Kopf einer Gruppe oder eine Zwischenüberschrift in einer Karte oder einem
 * Formular (#5590, Epic #5576 Entscheidung 2): normal geschrieben, nie in
 * Versalien. `as` wählt das Element, ohne die Optik zu ändern; über mehreren
 * Feldern einer Gruppe ist es `legend` im `fieldset`. Die Beschriftung eines
 * einzelnen Felds ist `FieldLabel`, ein Kartentitel `SectionCard title`.
 * `className` trägt nur Abstand und Layout.
 */
export function SectionLabel({
  as = "h3",
  required = false,
  className,
  children,
  ...rest
}: SectionLabelProps) {
  // Die Elemente teilen dieselben Attribute; `as` wählt nur den Tag.
  const Element = as as "h3";
  return (
    <Element {...rest} className={cn(SECTION_LABEL_CLASS, className)}>
      {children}
      {required && <RequiredMark />}
    </Element>
  );
}

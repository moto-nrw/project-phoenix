import type { HTMLAttributes, ReactNode } from "react";
import { cn } from "~/lib/utils";

type InsetPanelTone = "muted" | "white" | "danger" | "warning";
type InsetPanelPadding = "sm" | "md";

type InsetPanelElement = "div" | "section" | "p" | "dl" | "ul" | "li";

interface InsetPanelProps extends HTMLAttributes<HTMLElement> {
  readonly tone?: InsetPanelTone;
  /** Rahmen um die Fläche. Standard: `muted` ohne, alle anderen Töne mit Rahmen. */
  readonly framed?: boolean;
  /** Gestrichelter Rahmen, etwa für einen leeren Bereich. */
  readonly dashed?: boolean;
  readonly padding?: InsetPanelPadding;
  readonly as?: InsetPanelElement;
  readonly children?: ReactNode;
}

const PADDING_CLASSES: Record<InsetPanelPadding, string> = {
  sm: "p-3",
  md: "p-4",
};

function toneClasses(
  tone: InsetPanelTone,
  framed: boolean,
  dashed: boolean,
): string {
  const border = framed || dashed ? "border" : "";
  const dash = dashed ? "border-dashed" : "";
  switch (tone) {
    case "muted":
      // Ohne Rahmen braucht die Fläche den vollen Grauton, sonst verschwindet
      // sie auf Weiß. Mit Rahmen trägt der Rand die Abgrenzung.
      if (!framed && !dashed) return "bg-gray-50";
      return cn(
        border,
        dash,
        dashed ? "border-gray-200" : "border-gray-100",
        "bg-gray-50/70",
      );
    case "white":
      return cn("moto-content-surface", border, dash);
    case "danger":
      return cn(border, dash, "border-moto-red/20 bg-moto-red/5");
    case "warning":
      return cn(border, dash, "border-moto-orange/20 bg-moto-orange/5");
  }
}

/**
 * Die Innenfläche auf einer Karte: Vorschau, Notiz, Summenblock oder eine innere
 * Karte. Der Radius ist fest `rounded-xl` (ADR 0149, Entscheidung 11). Eine
 * Meldung mit Symbol und Schließen ist kein InsetPanel, sondern ein `Alert`.
 * `className` trägt nur Layout (`space-y-*`, `text-sm`, `flex`), keine Fläche.
 */
export function InsetPanel({
  tone = "muted",
  framed = tone !== "muted",
  dashed = false,
  padding = "sm",
  as = "div",
  className,
  children,
  ...rest
}: InsetPanelProps) {
  // Die Elemente teilen dieselben Attribute; `as` wählt nur den Tag.
  const Element = as as "div";
  return (
    <Element
      {...(rest as HTMLAttributes<HTMLDivElement>)}
      className={cn(
        "rounded-xl",
        toneClasses(tone, framed, dashed),
        PADDING_CLASSES[padding],
        className,
      )}
    >
      {children}
    </Element>
  );
}

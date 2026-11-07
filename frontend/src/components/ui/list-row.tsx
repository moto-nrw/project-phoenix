"use client";

import type { CSSProperties, MouseEventHandler, ReactNode } from "react";
import NavigationLink from "~/components/ui/navigation-link";
import { cn } from "~/lib/utils";
import {
  listRowClasses,
  type ListRowDensity,
  type ListRowTone,
} from "./list-row-styles";

type ListRowTarget =
  | {
      /**
       * Macht die ganze Zeile zum Link. Braucht die Zeile daneben einen eigenen
       * Knopf: ohne `href` lassen und den Link in `children` setzen.
       */
      readonly href?: string;
      readonly onClick?: never;
    }
  | {
      readonly href?: never;
      readonly onClick: MouseEventHandler<HTMLButtonElement>;
    };

type ListRowProps = ListRowTarget & {
  readonly children: ReactNode;
  /** Rechts in der Zeile, etwa Uhrzeit oder Pille. Liegt in Link oder Knopf. */
  readonly trailing?: ReactNode;
  readonly tone?: ListRowTone;
  readonly density?: ListRowDensity;
  readonly className?: string;
  readonly style?: CSSProperties;
  /** Nur für Link oder Knopf; eine reine Anzeige trägt keinen eigenen Namen. */
  readonly "aria-label"?: string;
};

/**
 * Eine Zeile als eigene Fläche: die Liste der Startseite, Kinder in einem Block,
 * Personen im Panel. Ohne `href` und `onClick` bleibt sie reine Anzeige und
 * verspricht beim Überfahren nichts.
 */
export function ListRow({
  children,
  trailing,
  tone = "neutral",
  density = "default",
  href,
  onClick,
  className,
  style,
  "aria-label": ariaLabel,
}: ListRowProps) {
  const body = (
    <>
      {children}
      {trailing ? (
        <span className="flex shrink-0 items-center gap-2">{trailing}</span>
      ) : null}
    </>
  );
  const rowClass = (interactive: boolean) =>
    cn(listRowClasses({ tone, density, interactive }), className);

  if (href) {
    return (
      <NavigationLink
        href={href}
        aria-label={ariaLabel}
        className={rowClass(true)}
        style={style}
      >
        {body}
      </NavigationLink>
    );
  }
  if (onClick) {
    return (
      <button
        type="button"
        onClick={onClick}
        aria-label={ariaLabel}
        className={rowClass(true)}
        style={style}
      >
        {body}
      </button>
    );
  }
  return (
    <div className={rowClass(false)} style={style}>
      {body}
    </div>
  );
}

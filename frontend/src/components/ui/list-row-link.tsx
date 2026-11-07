"use client";

import { ChevronRight } from "lucide-react";
import type { MouseEventHandler, ReactNode } from "react";
import NavigationLink from "~/components/ui/navigation-link";
import { cn } from "~/lib/utils";

interface ListRowLinkProps {
  /** Vor dem Text: Symbol-Kachel, Avatar oder eine Zeitspalte. */
  readonly icon?: ReactNode;
  readonly title: ReactNode;
  readonly subtitle?: ReactNode;
  /** Vor dem Pfeil, etwa eine Pille. */
  readonly badge?: ReactNode;
  readonly href?: string;
  readonly onClick?: MouseEventHandler<HTMLButtonElement>;
  readonly disabled?: boolean;
  /** Als einzelne Karte statt als Zeile einer `FramedList`. */
  readonly framed?: boolean;
  /** Führt die Zeile nicht auf eine andere Seite, entfällt der Pfeil. */
  readonly chevron?: boolean;
  readonly className?: string;
  readonly "aria-label"?: string;
}

/**
 * Eine Zeile, die woanders hinführt: Symbol, Titel, Untertitel, Pille, Pfeil.
 * In einer `FramedList` oder als einzelne Karte mit `framed`.
 */
export function ListRowLink({
  icon,
  title,
  subtitle,
  badge,
  href,
  onClick,
  disabled = false,
  framed = false,
  chevron = true,
  className,
  "aria-label": ariaLabel,
}: ListRowLinkProps) {
  const rowClass = cn(
    "group flex w-full items-center gap-3 text-left transition-colors focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-gray-400",
    framed ? "rounded-2xl p-4" : "px-4 py-3",
    disabled && "cursor-not-allowed opacity-60",
    !disabled && !framed && "hover:bg-gray-50 active:bg-gray-100",
    !framed && className,
  );
  const body = (
    <>
      {icon}
      <span className="min-w-0 flex-1">
        <span
          className={cn(
            "block text-sm text-gray-900",
            framed ? "font-semibold" : "truncate font-medium",
          )}
        >
          {title}
        </span>
        {subtitle ? (
          <span
            className={cn(
              "block text-xs text-gray-500",
              framed ? "mt-1" : "truncate",
            )}
          >
            {subtitle}
          </span>
        ) : null}
      </span>
      {badge ? (
        <span className="flex shrink-0 items-center">{badge}</span>
      ) : null}
      {chevron ? (
        <ChevronRight
          className={cn(
            "h-4 w-4 shrink-0 text-gray-400",
            framed && "group-hover:text-gray-700",
          )}
          aria-hidden="true"
        />
      ) : null}
    </>
  );

  const row = href ? (
    <NavigationLink href={href} aria-label={ariaLabel} className={rowClass}>
      {body}
    </NavigationLink>
  ) : (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      aria-label={ariaLabel}
      className={rowClass}
    >
      {body}
    </button>
  );
  if (!framed) return row;
  return (
    <div className={cn("moto-content-surface rounded-2xl border", className)}>
      {row}
    </div>
  );
}

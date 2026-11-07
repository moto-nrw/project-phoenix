import type { ReactNode } from "react";

import { IconTile } from "~/components/ui/icon-tile";
import { cn } from "~/lib/utils";

type EmptyStateTone = "success" | "error" | "neutral";

const TONE_TO_TILE = {
  success: "green",
  error: "red",
  neutral: "neutral",
} as const;

interface EmptyStateBaseProps {
  readonly icon?: ReactNode;
  readonly description?: string;
  readonly action?: ReactNode;
  /** Unter dem Text: Lade- oder Weiterleitungsanzeige eines Ergebnisses. */
  readonly progress?: ReactNode;
  /**
   * `dashed` ist der gestrichelte Platzhalter für einen Bereich, der sich noch
   * füllt. `compact` steht linksbündig in Karten und Listen.
   */
  readonly variant?: "default" | "compact" | "dashed";
  /** `h1` when the empty state is the page's only content, e.g. a full-page notice. */
  readonly titleAs?: "p" | "h1" | "h2" | "h3";
  readonly className?: string;
}

// Ein Ergebnis darf ohne eigenen Titel auskommen, wenn der Seitenkopf ihn trägt.
// `tone` setzt `icon` auf das runde Medaillon (`IconTile`).
type EmptyStateProps = EmptyStateBaseProps &
  (
    | { readonly title: string; readonly tone?: EmptyStateTone }
    | { readonly title?: string; readonly tone: EmptyStateTone }
  );

// Centered empty-state block: optional muted icon, title, description, and an
// optional action slot. Extracted from the operator provisioning pages and the
// tenant list pages, which all hand-rolled this same column (#1629). Pass the
// icon as a ReactNode (an inline SVG or a lucide icon sized h-12 w-12) so
// existing screens keep their exact artwork.
export function EmptyState({
  icon,
  title,
  description,
  action,
  progress,
  variant = "default",
  tone,
  titleAs = "p",
  className = "",
}: EmptyStateProps) {
  const compact = variant === "compact";
  const visual =
    tone != null && icon != null ? (
      <IconTile
        icon={icon}
        tone={TONE_TO_TILE[tone]}
        shape="round"
        size={compact ? "xs" : "display"}
      />
    ) : (
      icon
    );
  const hasDescription = description != null && description !== "";

  if (compact) {
    return (
      <div className={cn("flex items-start gap-3 py-2 text-left", className)}>
        {visual != null ? (
          <span className="mt-0.5 shrink-0 text-gray-400" aria-hidden="true">
            {visual}
          </span>
        ) : null}
        <div className="min-w-0 flex-1">
          {title != null ? (
            <p className="text-sm font-semibold text-gray-900">{title}</p>
          ) : null}
          {hasDescription ? (
            <p className="mt-0.5 text-sm leading-6 text-gray-500">
              {description}
            </p>
          ) : null}
          {progress != null ? <div className="mt-3">{progress}</div> : null}
          {action != null ? <div className="mt-3">{action}</div> : null}
        </div>
      </div>
    );
  }

  const dashed = variant === "dashed";
  const Title = titleAs;
  return (
    // cn statt Aneinanderreihung: sonst gewinnt bei einer Kollision nicht die
    // uebergebene Klasse, sondern die, die in der erzeugten CSS-Datei spaeter
    // steht — `py-4` von aussen blieb gegen das eigene `py-12` wirkungslos.
    <div
      className={cn(
        "flex flex-col items-center text-center",
        dashed
          ? "moto-content-surface gap-2 rounded-xl border border-dashed px-4 py-6"
          : "gap-3 py-12",
        className,
      )}
    >
      {visual != null ? (
        <span className="text-gray-400" aria-hidden="true">
          {visual}
        </span>
      ) : null}
      {title != null ? (
        <Title
          className={
            dashed
              ? "text-sm font-medium text-gray-700"
              : "text-lg font-medium text-gray-900"
          }
        >
          {title}
        </Title>
      ) : null}
      {hasDescription ? (
        <p className="max-w-md text-sm leading-relaxed text-gray-500">
          {description}
        </p>
      ) : null}
      {progress != null ? <div className="mt-1">{progress}</div> : null}
      {action != null ? <div className="mt-2">{action}</div> : null}
    </div>
  );
}

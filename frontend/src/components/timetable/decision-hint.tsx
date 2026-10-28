"use client";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";

interface Decision {
  label: string;
  onClick: () => void;
}

/**
 * The two equal ways out of a planning hint, e.g. „Mit Angebot verbinden“
 * and „Bleibt von Hand gepflegt“, or „Regeltermin anlegen“ and „Kein
 * Regeltermin nötig“. Both are the same kit button: neither way is the
 * lesser one. While a decision is saved, both wait.
 */
export function DecisionActions({
  primary,
  secondary,
  busy = false,
}: Readonly<{
  primary?: Decision;
  secondary?: Decision;
  busy?: boolean;
}>) {
  if (!primary && !secondary) return null;
  return (
    <span className="flex flex-wrap gap-2">
      {[primary, secondary].map((decision) =>
        decision ? (
          <Button
            key={decision.label}
            type="button"
            variant="surface"
            size="compact"
            onClick={decision.onClick}
            disabled={busy}
          >
            {decision.label}
          </Button>
        ) : null,
      )}
    </span>
  );
}

/**
 * A planning hint with its two decisions: a warning in a list, not an
 * event, so it does not interrupt screen readers. Readers without planning
 * rights get the hint without buttons.
 */
export function DecisionHint({
  message,
  primary,
  secondary,
  busy,
  className,
}: Readonly<{
  message: string;
  primary?: Decision;
  secondary?: Decision;
  busy?: boolean;
  className?: string;
}>) {
  const hasActions = Boolean(primary ?? secondary);
  return (
    <Alert
      type="warning"
      announce="off"
      message={message}
      action={
        hasActions ? (
          <DecisionActions
            primary={primary}
            secondary={secondary}
            busy={busy}
          />
        ) : undefined
      }
      actionLayout="stacked"
      className={className}
    />
  );
}

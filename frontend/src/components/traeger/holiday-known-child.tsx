"use client";

// Der Hinweis auf ein bekanntes Kind in der Prüfung der Ferienanmeldungen
// (#6683, ADR 0214): Eine Schule des Trägers führt heute genau ein Kind mit
// gleichem Namen und Geburtsdatum. moto verknüpft nichts und zeigt keine
// Daten der Schule, nur ihren Namen. Jeder Abgleich steht im Protokoll, die
// Seite fragt deshalb nur, was sie zeigt.

import { School } from "lucide-react";
import { useEffect, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import {
  StatusBadge,
  type StatusBadgeTone,
} from "~/components/ui/status-badge";
import { createLogger } from "~/lib/logger";
import {
  fetchHolidayKnownChildren,
  type HolidayRegistrationChild,
} from "~/lib/traeger-holiday-registrations-api";

const logger = createLogger({ component: "HolidayKnownChild" });

const NO_KNOWN_CHILDREN: ReadonlyMap<string, string> = new Map();

/** What the screen says about a known child. */
export interface KnownChildView {
  /** The school the parents named is the one that knows the child. */
  readonly sameSchool: boolean;
  readonly schoolName: string;
  readonly tone: StatusBadgeTone;
  readonly label: string;
  readonly title: string;
}

/**
 * The hint of one child; null when no school of the Träger knows it, or
 * when the school is gone from the Träger's list.
 */
export function knownChildView(
  child: HolidayRegistrationChild,
  knownSchoolId: string | undefined,
  names: ReadonlyMap<string, string>,
): KnownChildView | null {
  if (!knownSchoolId) return null;
  const schoolName = names.get(knownSchoolId);
  if (!schoolName) return null;
  if (child.homeSchoolId === knownSchoolId) {
    return {
      sameSchool: true,
      schoolName,
      tone: "green",
      label: "An der Schule bekannt",
      title: `Die Schule „${schoolName}“ führt ein Kind mit gleichem Namen und Geburtsdatum.`,
    };
  }
  return {
    sameSchool: false,
    schoolName,
    // Blue like „Ferienwunsch prüfen“: a check for the office, no warning.
    tone: "blue",
    label: `Bekannt an ${schoolName}`,
    title: `${registeredAs(child)} Die Schule „${schoolName}“ führt ein Kind mit gleichem Namen und Geburtsdatum.`,
  };
}

/** How the parents registered the child, as one sentence. */
function registeredAs(child: HolidayRegistrationChild): string {
  return child.homeSchoolId === null
    ? "Angemeldet als Feriengast."
    : "In der Anmeldung steht eine andere Schule.";
}

/** The plaque of the hint, for the list and the card. */
export function KnownChildBadge({ view }: Readonly<{ view: KnownChildView }>) {
  return (
    <StatusBadge
      tone={view.tone}
      label={view.label}
      icon={School}
      title={view.title}
    />
  );
}

/**
 * The hint in the card of a child. The same school is a plaque; another
 * school asks the office to check the child's school and offers the change
 * of its details.
 */
export function KnownChildNotice({
  child,
  view,
  names,
  onEdit,
}: Readonly<{
  child: HolidayRegistrationChild;
  view: KnownChildView;
  names: ReadonlyMap<string, string>;
  onEdit?: () => void;
}>) {
  if (view.sameSchool) return <KnownChildBadge view={view} />;
  const named =
    child.homeSchoolId === null
      ? `Angemeldet ist ${child.firstName} als Feriengast.`
      : `In der Anmeldung steht die Schule „${names.get(child.homeSchoolId) ?? "nicht mehr vorhanden"}“.`;
  return (
    <Alert
      type="info"
      icon={<School aria-hidden className="size-5" />}
      message={`Die Schule „${view.schoolName}“ führt ein Kind mit gleichem Namen und Geburtsdatum. ${named} Bitte prüfen Sie die Schule.`}
      actionLayout="responsive"
      action={
        onEdit ? (
          <Button
            type="button"
            variant="ghost_tone"
            size="compact"
            aria-label={`Schule von ${child.firstName} ${child.lastName} ändern`}
            onClick={onEdit}
          >
            Schule ändern
          </Button>
        ) : undefined
      }
    />
  );
}

/**
 * The names and dates of birth of the children, the only details the
 * comparison depends on: a key that asks again after they changed, and
 * not after every other change of the registration.
 */
export function knownChildrenKey(
  children: readonly HolidayRegistrationChild[] | undefined,
): string {
  return (children ?? [])
    .map(
      (child) =>
        `${child.id}:${child.firstName}:${child.lastName}:${child.birthDate}`,
    )
    .join("|");
}

/**
 * The known children of a period, or of one registration: child ID to
 * school ID. Empty while it loads and when the comparison fails. With
 * `children` (see knownChildrenKey) it waits for the children and asks again
 * when they change.
 */
export function useHolidayKnownChildren(
  periodId: string,
  registrationId?: string,
  children?: string,
): ReadonlyMap<string, string> {
  const [known, setKnown] = useState(NO_KNOWN_CHILDREN);
  useEffect(() => {
    if (!periodId || children === "") return;
    let current = true;
    const load = async () => {
      let next = NO_KNOWN_CHILDREN;
      try {
        next = await fetchHolidayKnownChildren(periodId, registrationId);
      } catch (err) {
        // Silent on purpose: the hint only adds to the review, which works
        // without it, and the page shows its own load errors.
        logger.warn("holiday_known_children_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      }
      if (current) setKnown(next);
    };
    void load();
    return () => {
      current = false;
    };
  }, [periodId, registrationId, children]);
  return known;
}

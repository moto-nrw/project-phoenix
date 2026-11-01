"use client";

/**
 * Hinweis „Termine in den Ferien prüfen“ (#5345): In den Schulferien der
 * nächsten acht Wochen sind Regeltermine geplant, an Tagen ohne Schließtag
 * und ohne Feiertag. Ferien sind keine Schließtage, deshalb behauptet der
 * Hinweis nicht, die OGS sei zu. Die Schule entscheidet: Termine absagen
 * (die Sammelabsage aus #3594), Schließtage eintragen oder bestätigen, dass
 * Betreuung geplant ist. Die Bestätigung gilt für die ganze Schule und nur
 * für diese Ferien.
 *
 * Betreuungsplan, „Schuljahr und Kalender“ und Startseite zeigen denselben
 * Hinweis. Ohne geplante Termine ist er nicht da. Kann er nicht geladen
 * werden, fehlt nur er; der Plan und die Seite bleiben bedienbar.
 *
 * Dieser Kern hängt die Sammelabsage nicht ein: Ihr Datumswähler würde das
 * Bundle der Startseite um rund 45 kB vergrößern (Bundle-Ratchet). Ohne
 * `onCancel` führt „Termine absagen“ in den Betreuungsplan, der die
 * Sammelabsage mit diesen Ferien öffnet. `SchoolVacationHints` öffnet sie
 * direkt auf der Seite.
 */

import { type ReactNode, useCallback, useState } from "react";
import { TriangleAlert } from "lucide-react";

import { Alert } from "~/components/ui/alert";
import { Button, ButtonLink } from "~/components/ui/button";
import { ResponsiveActionList } from "~/components/ui/responsive-action-list";
import { SectionCard } from "~/components/ui/section-card";
import { useApiErrorDisplay, useToast } from "~/contexts/ToastContext";
import { formatDate } from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { createLogger } from "~/lib/logger";
import {
  acknowledgeSchoolVacation,
  getSchoolVacationHints,
  SCHOOL_VACATION_HINTS_KEY,
  type SchoolVacationHint,
} from "~/lib/school-vacations-api";
import { useSWRAuth, useTenantMutateMatching } from "~/lib/swr";
import { timetableKeys } from "~/lib/swr/portal-keys";
import { useTenantAwarePath } from "~/lib/tenant-path";

const logger = createLogger({ component: "SchoolVacationHints" });

const DECISION_TEXT =
  "Findet in dieser Zeit Betreuung statt? Wenn nicht, sagen Sie die Termine ab oder tragen Sie Schließtage ein.";

/** „vom 23.12.2026 bis 06.01.2027“, ein einzelner Tag als „am 18.05.2027“. */
function vacationRangeLabel(hint: SchoolVacationHint): string {
  if (hint.startDate === hint.endDate)
    return `am ${formatDate(hint.startDate)}`;
  return `vom ${formatDate(hint.startDate)} bis ${formatDate(hint.endDate)}`;
}

function regelterminLabel(count: number): string {
  return count === 1 ? "1 Regeltermin" : `${count} Regeltermine`;
}

/** Text des Hinweises für eine Ferienzeit. */
export function vacationHintMessage(hint: SchoolVacationHint): string {
  // Laufende Ferien zählen erst ab heute.
  const still = hint.cancelFrom > hint.startDate ? "noch " : "";
  const planned =
    hint.count === 1
      ? `ist ${still}1 Regeltermin geplant`
      : `sind ${still}${hint.count} Regeltermine geplant`;
  return `${hint.name} ${vacationRangeLabel(hint)}: In dieser Zeit ${planned}. ${DECISION_TEXT}`;
}

/**
 * Link in den Betreuungsplan, der die Sammelabsage mit diesen Ferien öffnet
 * (`absagen`/`absagenBis`, siehe `betreuungsplan-view.tsx`).
 */
export function vacationCancelPath(hint: SchoolVacationHint): string {
  const query = new URLSearchParams({
    d: hint.cancelFrom,
    absagen: hint.cancelFrom,
    absagenBis: hint.endDate,
  });
  return `/betreuungsplan?${query.toString()}`;
}

export interface SchoolVacationHintsCoreProps {
  /** Recht `schedules:manage` und Betreuungsplan eingeschaltet. */
  enabled: boolean;
  /** Auf „Schuljahr und Kalender“ selbst führt kein Link dorthin. */
  showClosingDaysLink?: boolean;
  /**
   * „Termine absagen“ auf der Seite selbst. Ohne führt der Knopf in den
   * Betreuungsplan.
   */
  onCancel?: (hint: SchoolVacationHint) => void;
}

/**
 * Lädt die Hinweise. Nur mit Hinweisen hängt der Teil mit Aktionen ein, so
 * kostet eine Seite ohne Ferien keine weiteren Hooks.
 */
export function SchoolVacationHintsCore({
  enabled,
  ...props
}: Readonly<SchoolVacationHintsCoreProps>) {
  const today = useBerlinToday();
  // A failed load leaves only this hint out (see the file comment); the
  // planning surfaces keep their own load errors.
  const { data } = useSWRAuth(
    enabled ? timetableKeys.schoolVacationHints(today) : null,
    getSchoolVacationHints,
  );
  const hints = Array.isArray(data) ? data : [];
  if (!enabled || hints.length === 0) return null;
  return <SchoolVacationHintList hints={hints} {...props} />;
}

function SchoolVacationHintList({
  hints,
  showClosingDaysLink = true,
  onCancel,
}: Readonly<
  Omit<SchoolVacationHintsCoreProps, "enabled"> & {
    hints: readonly SchoolVacationHint[];
  }
>) {
  const toast = useToast();
  const { show: showActionError } = useApiErrorDisplay();
  const revalidateHints = useTenantMutateMatching([SCHOOL_VACATION_HINTS_KEY]);
  const revalidateVacations = useTenantMutateMatching([
    timetableKeys.schoolVacationsFamily,
  ]);
  const tenantPath = useTenantAwarePath();
  const [busy, setBusy] = useState<string | null>(null);

  const confirmCare = useCallback(
    async (hint: SchoolVacationHint) => {
      setBusy(hint.startDate);
      try {
        await acknowledgeSchoolVacation(hint.startDate, hint.endDate);
        toast.success(
          `Gespeichert: Für „${hint.name}“ ist Betreuung geplant. Der Hinweis ist ausgeblendet.`,
        );
        await Promise.all([revalidateHints(), revalidateVacations()]);
      } catch (err) {
        logger.error("school_vacation_acknowledge_failed", {
          start_date: hint.startDate,
          error: err instanceof Error ? err.message : String(err),
        });
        void showActionError(err, { object: "die Bestätigung der Ferien" });
      } finally {
        setBusy(null);
      }
    },
    [revalidateHints, revalidateVacations, showActionError, toast],
  );

  const actions = (hint: SchoolVacationHint, compact: boolean): ReactNode => {
    const size = compact ? "compact" : "md";
    return (
      <>
        {onCancel ? (
          <Button
            type="button"
            variant="surface"
            size={size}
            aria-label={`Termine absagen: ${hint.name}`}
            onClick={() => onCancel(hint)}
            disabled={busy === hint.startDate}
          >
            Termine absagen
          </Button>
        ) : (
          <ButtonLink
            href={tenantPath(vacationCancelPath(hint))}
            variant="surface"
            size={size}
            aria-label={`Termine absagen: ${hint.name}`}
          >
            Termine absagen
          </ButtonLink>
        )}
        {showClosingDaysLink ? (
          <ButtonLink
            href={tenantPath("/calendar-periods")}
            variant="surface"
            size={size}
            aria-label={`Schließtage eintragen: ${hint.name}`}
          >
            Schließtage eintragen
          </ButtonLink>
        ) : null}
        <Button
          type="button"
          variant="surface"
          size={size}
          aria-label={`Betreuung ist geplant: ${hint.name}`}
          onClick={() => void confirmCare(hint)}
          disabled={busy === hint.startDate}
        >
          Betreuung ist geplant
        </Button>
      </>
    );
  };

  const [single] = hints;
  return hints.length === 1 && single ? (
    <Alert
      type="warning"
      announce="polite"
      title="Termine in den Ferien prüfen"
      message={vacationHintMessage(single)}
      actionLayout="stacked"
      action={
        <div className="flex flex-wrap gap-2">{actions(single, false)}</div>
      }
      data-testid="school-vacation-hints"
    />
  ) : (
    <SectionCard
      title="Termine in den Ferien prüfen"
      description={`In diesen Ferien sind Regeltermine geplant. ${DECISION_TEXT}`}
      leading={
        <TriangleAlert
          className="text-moto-orange-strong mt-0.5 size-5 shrink-0"
          aria-hidden="true"
        />
      }
      testId="school-vacation-hints"
    >
      <ResponsiveActionList>
        {hints.map((hint) => (
          <li
            key={`${hint.startDate}-${hint.endDate}`}
            className="flex flex-col gap-2 py-3 first:pt-0 last:pb-0 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-center"
          >
            <div className="min-w-0">
              <p className="text-sm font-semibold text-gray-900">{hint.name}</p>
              <p className="mt-0.5 text-sm text-gray-600">
                {vacationRangeLabel(hint)} · {regelterminLabel(hint.count)}
              </p>
            </div>
            <div className="flex w-full flex-col gap-2 min-[480px]:w-auto min-[480px]:flex-row sm:justify-end">
              {actions(hint, true)}
            </div>
          </li>
        ))}
      </ResponsiveActionList>
    </SectionCard>
  );
}

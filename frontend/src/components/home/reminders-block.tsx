"use client";

import { EmptyState } from "~/components/ui/empty-state";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { ListRow } from "~/components/ui/list-row";
import { SkeletonRows } from "~/components/ui/skeleton";
import { HomeCard, HomeCardIcon } from "~/components/home/home-card";
import { useReminders } from "~/lib/hooks/use-reminders";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import {
  isReminderOverdue,
  reminderKey,
  reminderRelativeLabel,
  reminderSubtitle,
  reminderToneClass,
} from "~/lib/reminders-display";
import { useTenantAwarePath } from "~/lib/tenant-path";
import {
  REMINDERS_LINK_TOOLTIP,
  HomeCardLink,
  HomeMoreRow,
  useHomeCardRows,
} from "~/components/home/home-card-rows";
import { pickupConcept } from "~/lib/concepts/pickup";

/** So viele Zeilen passen in eine Karte dieser Höhe ganz hinein. */
const MAX_ROWS = 3;

/**
 * Baustein „Erinnerungen" (#2180, Daten aus #1457): was in den nächsten
 * Minuten ansteht oder schon überfällig ist — Abholungen und Aktivitätsbeginn.
 *
 * Für eine Betreuungskraft ist das der Teil des Tages, den man verpassen kann;
 * deshalb steht er in ihrer Standardansicht und nicht nur hinter der Glocke in
 * der Kopfzeile. Überfälliges zuerst, weil es sonst untergeht.
 */
export function RemindersBlock() {
  const tenantPath = useTenantAwarePath();
  const { reminders, error, isLoading, data, retry } = useReminders();
  const loadError = useSwrLoadError(error, "die Liste der Erinnerungen", retry);

  const sorted = [...reminders].sort((a, b) => a.minutes_away - b.minutes_away);
  const { shown, hidden } = useHomeCardRows(sorted, MAX_ROWS);

  return (
    <HomeCard
      title="Erinnerungen"
      leading={<HomeCardIcon concept={pickupConcept} />}
      inlineActions
      actions={
        <HomeCardLink
          href={tenantPath("/reminders")}
          label="Erinnerungen: alle ansehen"
          tooltip={REMINDERS_LINK_TOOLTIP}
        >
          Alle ansehen
        </HomeCardLink>
      }
    >
      {(() => {
        if (loadError) return <LoadErrorAlert error={loadError} />;
        // Bis der Katalogtext des Ladefehlers da ist, bleibt das Skelett.
        if ((isLoading && data === undefined) || error) {
          return <SkeletonRows />;
        }
        if (shown.length === 0) {
          return (
            <EmptyState
              className="py-4"
              title="Nichts steht an"
              description="Anstehende Abholungen und Aktivitäten erscheinen hier."
            />
          );
        }
        return (
          <>
            <ul className="space-y-1.5">
              {shown.map((reminder) => {
                // Eine Erinnerung zu einem Kind fuehrt zu diesem Kind; eine
                // Erinnerung ohne Kind (Aktivitaetsbeginn) bleibt eine Anzeige
                // und sieht auch nicht klickbar aus.
                const subtitle = reminderSubtitle(reminder);
                const href = reminder.student_id
                  ? tenantPath(`/students/${reminder.student_id}`)
                  : undefined;
                return (
                  <li key={reminderKey(reminder)}>
                    {/* Auf dem Handy zwei Zeilen je Erinnerung; ab der
                        zweispaltigen Ansicht eine, damit drei in die feste
                        Kartenhöhe passen, ohne dass die Karte den Rest
                        abschneidet. */}
                    <ListRow
                      href={href}
                      tone={isReminderOverdue(reminder) ? "danger" : "neutral"}
                      density="compact"
                      aria-label={
                        href ? `${reminder.title}: Kind öffnen` : undefined
                      }
                      trailing={
                        <span className="flex shrink-0 flex-col items-end sm:flex-row sm:items-baseline sm:gap-1.5">
                          <span className="text-sm font-semibold text-gray-900 tabular-nums">
                            {reminder.due_time}
                          </span>
                          <span
                            className={`text-xs ${reminderToneClass(reminder)}`}
                          >
                            {reminderRelativeLabel(reminder)}
                          </span>
                        </span>
                      }
                    >
                      <span className="flex min-w-0 flex-col sm:flex-row sm:items-baseline sm:gap-1.5">
                        <span className="truncate text-sm font-medium text-gray-900 sm:shrink-0">
                          {reminder.title}
                        </span>
                        {subtitle && (
                          <span className="min-w-0 truncate text-xs text-gray-500">
                            {subtitle}
                          </span>
                        )}
                      </span>
                    </ListRow>
                  </li>
                );
              })}
            </ul>
            <HomeMoreRow
              hidden={hidden}
              href={tenantPath("/reminders")}
              label="Erinnerungen"
              singular="Erinnerung"
            />
          </>
        );
      })()}
    </HomeCard>
  );
}

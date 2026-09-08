"use client";

import { ChevronRight } from "lucide-react";

import { Alert } from "~/components/ui/alert";
import { EmptyState } from "~/components/ui/empty-state";
import Link from "~/components/ui/navigation-link";
import { SectionCard } from "~/components/ui/section-card";
import {
  HOME_CARD_BODY,
  HomeCardIcon,
} from "~/components/home/home-block-content";
import { useReminders } from "~/lib/hooks/use-reminders";
import {
  isReminderOverdue,
  reminderKey,
  reminderRelativeLabel,
  reminderToneClass,
} from "~/lib/reminders-display";
import { useTenantAwarePath } from "~/lib/tenant-path";
import { HomeMoreRow, useHomeCardRows } from "~/components/home/home-card-rows";

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
  const { reminders, error, isLoading, data } = useReminders();

  const sorted = [...reminders].sort((a, b) => a.minutes_away - b.minutes_away);
  const { shown, hidden } = useHomeCardRows(sorted, MAX_ROWS);

  return (
    <SectionCard
      title="Erinnerungen"
      className="flex h-full flex-col"
      bodyClassName={HOME_CARD_BODY}
      leading={<HomeCardIcon concept="pickup" />}
      actions={
        <Link
          href={tenantPath("/reminders")}
          aria-label="Erinnerungen: alle ansehen"
          className="flex items-center gap-1 text-sm font-medium text-gray-600 transition-colors hover:text-gray-900"
        >
          Alle ansehen
          <ChevronRight className="h-4 w-4" aria-hidden="true" />
        </Link>
      }
    >
      {(() => {
        if (error) {
          return (
            <Alert
              type="error"
              message="Die Erinnerungen konnten nicht geladen werden. Bitte die Seite neu laden."
            />
          );
        }
        if (isLoading && data === undefined) {
          return (
            <div className="space-y-2" aria-hidden="true">
              {[1, 2, 3].map((i) => (
                <div
                  key={i}
                  className="flex items-center justify-between gap-3 rounded-xl bg-gray-50/50 px-3 py-2"
                >
                  <div className="h-4 w-24 animate-pulse rounded bg-gray-200"></div>
                  <div className="h-4 flex-1 animate-pulse rounded bg-gray-200"></div>
                </div>
              ))}
            </div>
          );
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
                const href = reminder.student_id
                  ? tenantPath(`/students/${reminder.student_id}`)
                  : undefined;
                // Auf dem Handy zwei Zeilen je Erinnerung; ab der
                // zweispaltigen Ansicht eine, damit drei in die feste
                // Kartenhöhe passen, ohne dass die Karte den Rest abschneidet.
                const rowClass = `flex items-center justify-between gap-3 rounded-xl px-3 py-2 sm:py-1.5 ${
                  isReminderOverdue(reminder)
                    ? "bg-moto-red/5"
                    : "bg-gray-50/50"
                }`;
                const body = (
                  <>
                    <span className="flex min-w-0 flex-col sm:flex-row sm:items-baseline sm:gap-1.5">
                      <span className="truncate text-sm font-medium text-gray-900 sm:shrink-0">
                        {reminder.title}
                      </span>
                      {reminder.subtitle && (
                        <span className="min-w-0 truncate text-xs text-gray-500">
                          {reminder.subtitle}
                        </span>
                      )}
                    </span>
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
                  </>
                );
                return (
                  <li key={reminderKey(reminder)}>
                    {href ? (
                      <Link
                        href={href}
                        aria-label={`${reminder.title}: Kind öffnen`}
                        className={`${rowClass} transition-colors hover:bg-gray-100/70`}
                      >
                        {body}
                      </Link>
                    ) : (
                      <span className={rowClass}>{body}</span>
                    )}
                  </li>
                );
              })}
            </ul>
            <HomeMoreRow
              hidden={hidden}
              href={tenantPath("/reminders")}
              label="Erinnerungen"
            />
          </>
        );
      })()}
    </SectionCard>
  );
}

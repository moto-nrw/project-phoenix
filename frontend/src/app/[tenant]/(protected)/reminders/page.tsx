"use client";

import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import { useReminders } from "~/lib/hooks/use-reminders";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { useTenantMutate } from "~/lib/swr/hooks";
import type { Reminder } from "~/lib/reminders-api";
import {
  REMINDER_SECTIONS,
  isReminderOverdue,
  reminderKey,
  reminderRelativeLabel,
  reminderToneClass,
} from "~/lib/reminders-display";

function ReminderRow({ reminder }: { reminder: Reminder }) {
  return (
    <li className="flex items-center justify-between gap-4 px-4 py-3">
      <div className="min-w-0">
        <p className="truncate font-medium text-gray-900">{reminder.title}</p>
        {reminder.subtitle && (
          <p className="truncate text-sm text-gray-500">{reminder.subtitle}</p>
        )}
      </div>
      <div className="flex flex-shrink-0 flex-col items-end">
        <span className="text-sm font-semibold text-gray-900">
          {reminder.due_time}
        </span>
        <span className={`text-xs ${reminderToneClass(reminder)}`}>
          {reminderRelativeLabel(reminder)}
        </span>
      </div>
    </li>
  );
}

export default function RemindersPage() {
  const { reminders, count, error, isLoading, data } = useReminders();
  const tenantMutate = useTenantMutate();
  // Ladefehler vor Ort mit Katalogtext und Wiederholen (#2517).
  const loadError = useSwrLoadError(error, "die Liste der Erinnerungen", () =>
    tenantMutate("reminders"),
  );

  // Statuszeile unter dem Seitentitel, allein aus der geladenen Liste.
  const overdue = reminders.filter(isReminderOverdue).length;
  const upcoming = count - overdue;
  // Bis der Text eines Ladefehlers da ist, bleibt der Ladezustand stehen:
  // keine leere Seite ohne Daten.
  const loading =
    (isLoading && reminders.length === 0) || Boolean(error && !loadError);

  return (
    <TenantPage
      title="Erinnerungen"
      // Ohne geladene Liste keine "0 anstehend" neben dem Ladefehler (#2517).
      stats={
        data === undefined
          ? null
          : `${upcoming} anstehend · ${overdue} überfällig`
      }
      statsLoading={loading}
      loading={loading}
      error={error ? loadError : null}
      empty={
        !loading && !error && count === 0
          ? {
              title: "Keine aktiven Erinnerungen",
              description:
                data?.enabled === false
                  ? "Sobald eine Abholung näher rückt oder eine Aktivität startet, erscheint sie hier. Erinnerungstypen werden in den Einstellungen unter „Erinnerungen“ aktiviert."
                  : "Erinnerungen aktiviert. Aktuell gibt es keine aktiven Erinnerungen.",
            }
          : null
      }
    >
      {REMINDER_SECTIONS.map((section) => {
        const items = reminders.filter((r) => r.type === section.type);
        if (items.length === 0) return null;
        return (
          <SectionCard
            key={section.type}
            title={section.title}
            actions={
              <span className="flex h-6 min-w-6 items-center justify-center rounded-full bg-gray-100 px-2 text-xs font-semibold text-gray-600">
                {items.length}
              </span>
            }
          >
            <ul className="divide-y divide-gray-100">
              {items.map((reminder) => (
                <ReminderRow key={reminderKey(reminder)} reminder={reminder} />
              ))}
            </ul>
          </SectionCard>
        );
      })}
    </TenantPage>
  );
}

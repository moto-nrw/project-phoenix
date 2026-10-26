// Shared display metadata for staff reminders (issue #1457).
// Single source of truth for the per-type German section title and the
// relative time label, consumed by both the /reminders page and the header
// bell so the two never drift.
//
// Color discipline: the list stays calm and neutral like the rest of the app.
// The ONLY color is red, and only for "überfällig" — the genuine alert (a
// child is still present past its pickup time). Everything else is gray.

import type { Reminder, ReminderType } from "~/lib/reminders-api";

// Color discipline: the only accent is brand red (LOCATION_COLORS.DANGER,
// #DC2626) and only for "überfällig" — applied via arbitrary-value Tailwind
// classes (text-moto-red below, bg-moto-red on the bell badge), never a
// generic Tailwind red.

// Order = display order, most urgent first (overdue → upcoming → activities).
export const REMINDER_SECTIONS: {
  type: ReminderType;
  title: string;
}[] = [
  { type: "pickup_overdue", title: "Überfällige Abholung" },
  { type: "activity_overdue", title: "Überfällige Aktivität" },
  { type: "activity_running_over", title: "AG dauert länger" },
  { type: "pickup_upcoming", title: "Anstehende Abholung" },
  { type: "activity_start", title: "Aktivitätsbeginn" },
];

export function isReminderOverdue(reminder: Reminder): boolean {
  return reminder.minutes_away < 0;
}

export function reminderRelativeLabel(reminder: Reminder): string {
  // Für eine laufende AG ist "überfällig" falsch: sie läuft, nur länger als
  // geplant. Die Uhrzeit daneben ist das geplante Ende.
  if (reminder.type === "activity_running_over") {
    return `${Math.abs(reminder.minutes_away)} Min länger`;
  }
  if (reminder.minutes_away < 0) {
    return `${Math.abs(reminder.minutes_away)} Min überfällig`;
  }
  if (reminder.minutes_away === 0) {
    return "jetzt";
  }
  return `in ${reminder.minutes_away} Min`;
}

// Tailwind text-color class for the relative-time label: red only when
// overdue, neutral gray otherwise.
export function reminderToneClass(reminder: Reminder): string {
  return isReminderOverdue(reminder) ? "text-moto-red" : "text-gray-500";
}

/**
 * Second line under the title. A running-over block names its room and,
 * when there are any, how many children in it have reached their pickup time
 * (#4765). Every other type shows the subtitle as sent.
 */
export function reminderSubtitle(reminder: Reminder): string | undefined {
  if (reminder.type !== "activity_running_over") {
    return reminder.subtitle;
  }
  const count = reminder.pickup_due_count ?? 0;
  const pickup =
    count === 0
      ? undefined
      : `${count} ${count === 1 ? "Kind" : "Kinder"} zur Abholung`;
  const parts = [reminder.subtitle, pickup].filter(Boolean);
  return parts.length > 0 ? parts.join(" · ") : undefined;
}

/** Stable React key backed by the domain entity that produced the reminder. */
export function reminderKey(reminder: Reminder): string {
  switch (reminder.type) {
    case "pickup_upcoming":
    case "pickup_overdue":
      if (!reminder.student_id) {
        throw new Error(`${reminder.type} reminder is missing student_id`);
      }
      return `${reminder.type}-student-${reminder.student_id}`;
    case "activity_start":
    case "activity_overdue":
    case "activity_running_over":
      if (!reminder.activity_instance_id) {
        throw new Error(
          `${reminder.type} reminder is missing activity_instance_id`,
        );
      }
      return `${reminder.type}-activity-${reminder.activity_instance_id}`;
  }
}

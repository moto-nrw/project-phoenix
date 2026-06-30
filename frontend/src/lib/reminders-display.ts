// Shared display metadata for staff reminders (issue #1457).
// Single source of truth for the per-type German section title and the
// relative time label, consumed by both the /reminders page and the header
// bell so the two never drift.
//
// Color discipline: the list stays calm and neutral like the rest of the app.
// The ONLY color is red, and only for "überfällig" — the genuine alert (a
// child is still present past its pickup time). Everything else is gray.

import type { Reminder, ReminderType } from "~/lib/reminders-api";

// Brand red (LOCATION_COLORS.HOME) via arbitrary-value hex — not generic
// Tailwind red. Used for the overdue label and the bell count badge.
export const REMINDER_ALERT_HEX = "#FF3130";

// Order = display order, most urgent first (overdue → upcoming → activities).
export const REMINDER_SECTIONS: {
  type: ReminderType;
  title: string;
}[] = [
  { type: "pickup_overdue", title: "Überfällige Abholung" },
  { type: "activity_overdue", title: "Überfällige Aktivität" },
  { type: "pickup_upcoming", title: "Anstehende Abholung" },
  { type: "activity_start", title: "Aktivitätsbeginn" },
];

export function isReminderOverdue(reminder: Reminder): boolean {
  return reminder.minutes_away < 0;
}

export function reminderRelativeLabel(reminder: Reminder): string {
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
  return isReminderOverdue(reminder) ? "text-[#FF3130]" : "text-gray-500";
}

"use client";

// Soll und Ist eines Feriengasts (#5994): beginnt mit seiner Ferienwoche,
// denn außerhalb davon ist das Kind nicht in der Schule. Liest den Bogen
// über denselben Schlüssel wie die Feriengast-Karte, also ohne zweite Anfrage.

import { StudentCareTimesSection } from "~/components/students/care-times-section";
import { berlinTodayISO } from "~/lib/date-helpers";
import { fetchHolidayGuestSheet } from "~/lib/holiday-host-api";
import { useSWRAuth } from "~/lib/swr";
import { studentKeys } from "~/lib/swr/portal-keys";

export function HolidayGuestCareTimes({
  studentId,
}: Readonly<{ studentId: string }>) {
  const sheet = useSWRAuth(studentKeys.holidayGuest(studentId), () =>
    fetchHolidayGuestSheet(studentId),
  );
  // Ohne Bogen (Zugriff beendet) bleibt der laufende Monat.
  if (!sheet.data && !sheet.error) return null;
  const today = berlinTodayISO();
  const week = sheet.data;
  const initialRange =
    week && week.startsOn <= today
      ? { from: week.startsOn, to: week.endsOn < today ? week.endsOn : today }
      : undefined;
  return (
    <StudentCareTimesSection
      key={initialRange ? `${initialRange.from}/${initialRange.to}` : "month"}
      studentId={studentId}
      active
      initialRange={initialRange}
    />
  );
}

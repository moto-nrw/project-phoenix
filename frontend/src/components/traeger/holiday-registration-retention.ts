// When moto deletes the holiday registrations of a period (#6691). Each
// registration keeps the retention its consent text names, so a period can
// have more than one day; the health information goes earlier.

import { formatDate } from "~/lib/date-helpers";

interface Retained {
  readonly deletesOn: string | null;
  readonly healthDeletesOn: string | null;
}

const DOWNLOAD_HINT =
  "Laden Sie sie vorher herunter, wenn Sie sie länger brauchen.";

/** The sentence about the health information, when it goes earlier. */
function healthSentence(first: string, registrations: readonly Retained[]) {
  const days = registrations
    .map((registration) => registration.healthDeletesOn)
    .filter((day): day is string => day !== null)
    .sort((a, b) => a.localeCompare(b));
  const earliest = days[0];
  return earliest && earliest < first
    ? ` Gesundheitsangaben löscht moto schon am ${formatDate(earliest)}.`
    : "";
}

/**
 * The notice above the registrations of a period: when moto deletes them.
 * Null without a registration that has a day.
 */
export function holidayDeletionNotice(
  registrations: readonly Retained[],
): string | null {
  const days = [
    ...new Set(
      registrations
        .map((registration) => registration.deletesOn)
        .filter((day): day is string => day !== null),
    ),
  ].sort((a, b) => a.localeCompare(b));
  const first = days[0];
  const last = days.at(-1);
  if (!first || !last) return null;
  const when =
    first === last
      ? `moto löscht diese Anmeldungen am ${formatDate(first)}.`
      : `moto löscht diese Anmeldungen zwischen dem ${formatDate(first)} und dem ${formatDate(last)}. Den Tag sehen Sie in jeder Anmeldung.`;
  return `${when} ${DOWNLOAD_HINT}${healthSentence(first, registrations)}`;
}

/** The notice of one registration. Null for a period without weeks. */
export function holidayRegistrationDeletionNotice(
  registration: Retained,
): string | null {
  if (!registration.deletesOn) return null;
  return `moto löscht diese Anmeldung am ${formatDate(registration.deletesOn)}. Laden Sie sie vorher herunter, wenn Sie sie länger brauchen.${healthSentence(registration.deletesOn, [registration])}`;
}

/**
 * SWR-Schlüssel der Wartelistenansichten. Die Liste und die einzelne AG
 * brauchen getrennte Einträge, damit eine AG-Ansicht nicht die Übersicht der
 * Anfragen überschreibt.
 */
export const courseWaitlistKeys = {
  all: () => "course-waitlists",
  activity: (activityId: string) => `course-waitlist-${activityId}`,
};

/** Die heutige Teilnehmerliste ergänzt die Warteliste im AG-Fenster. */
export const activityWaitlistKeys = {
  participants: (activityId: string) => `activity-participants-${activityId}`,
};

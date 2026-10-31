/**
 * Warteliste je AG für das Team (#4767). Die Reihenfolge rechnet das Backend
 * nach derselben Regel wie den Wartelistenplatz im Elternportal: die älteste
 * offene Anfrage zuerst. Gespeichert wird nichts; das Nachrücken ist die
 * normale Freigabe der ersten Anfrage in /anfragen.
 */

export interface CourseWaitlistEntryWire {
  readonly request_id: string;
  readonly student_id: string;
  readonly student_name: string;
  readonly rank: number;
  readonly submitted_at: string;
}

export interface CourseWaitlistWire {
  readonly activity_group_id: string;
  readonly name: string;
  readonly participant_limit: number | null;
  readonly participants: number;
  readonly free_slots: number | null;
  readonly slot_free_for_next: boolean;
  readonly entries: readonly CourseWaitlistEntryWire[];
}

export interface CourseWaitlistEntry {
  readonly requestId: string;
  readonly studentId: string;
  readonly studentName: string;
  readonly rank: number;
  readonly submittedAt: string;
}

export interface CourseWaitlist {
  readonly activityGroupId: string;
  readonly name: string;
  /** null: die AG hat keine Teilnehmergrenze. */
  readonly participantLimit: number | null;
  readonly participants: number;
  readonly freeSlots: number | null;
  /** Ein Platz ist frei und ein Kind wartet: das Team ist dran. */
  readonly slotFreeForNext: boolean;
  readonly entries: readonly CourseWaitlistEntry[];
}

export function mapCourseWaitlists(
  wire: readonly CourseWaitlistWire[],
): CourseWaitlist[] {
  return wire.map((waitlist) => ({
    activityGroupId: String(waitlist.activity_group_id),
    name: waitlist.name,
    participantLimit: waitlist.participant_limit ?? null,
    participants: waitlist.participants,
    freeSlots: waitlist.free_slots ?? null,
    slotFreeForNext: waitlist.slot_free_for_next,
    entries: (waitlist.entries ?? []).map((entry) => ({
      requestId: String(entry.request_id),
      studentId: String(entry.student_id),
      studentName: entry.student_name,
      rank: entry.rank,
      submittedAt: entry.submitted_at,
    })),
  }));
}

/** Die Wartelisten-Einträge einer Anfrage, eine Zeile je AG. */
export interface RequestWaitlistPlace {
  readonly waitlist: CourseWaitlist;
  readonly entry: CourseWaitlistEntry;
}

export function waitlistPlacesByRequest(
  waitlists: readonly CourseWaitlist[],
): ReadonlyMap<string, readonly RequestWaitlistPlace[]> {
  const places = new Map<string, RequestWaitlistPlace[]>();
  for (const waitlist of waitlists) {
    for (const entry of waitlist.entries) {
      const list = places.get(entry.requestId) ?? [];
      list.push({ waitlist, entry });
      places.set(entry.requestId, list);
    }
  }
  return places;
}

/** Die AGs, in denen ein Platz frei ist und ein Kind darauf wartet. */
export function waitlistsWithFreeSlot(
  waitlists: readonly CourseWaitlist[],
): CourseWaitlist[] {
  return waitlists.filter((waitlist) => waitlist.slotFreeForNext);
}

/**
 * Die AGs mit Teilnehmergrenze, auf die gerade ein Kind wartet. Eine AG ohne
 * Grenze hat keine Warteliste, nur offene Anfragen.
 */
export function waitlistsWithQueue(
  waitlists: readonly CourseWaitlist[],
): CourseWaitlist[] {
  return waitlists.filter(
    (waitlist) =>
      waitlist.participantLimit !== null && waitlist.entries.length > 0,
  );
}

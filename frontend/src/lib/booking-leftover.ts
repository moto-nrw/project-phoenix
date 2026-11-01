/**
 * A child a hand-maintained Regeltermin keeps after a booking change (#5102):
 * the booking of these weekdays ended and no other booking carries the
 * Regeltermin. The save response of the change names it once; this hint
 * finds it again from the stored bookings.
 */
export interface BookingLeftover {
  templateId: string;
  templateName: string;
  studentId: string;
  studentName: string;
  /** ISO weekdays, 1 = Monday, ascending. */
  weekdays: number[];
  /** Day the booking ended (exclusive end, YYYY-MM-DD). */
  bookingEndedOn: string;
  /** First day „Entfernen“ changes, tomorrow at the earliest. */
  from: string;
  /** The school decided: the child stays planned. */
  kept: boolean;
}

interface BackendBookingLeftover {
  template_id: number;
  template_name: string;
  student_id: number;
  student_name: string;
  weekdays?: number[] | null;
  booking_ended_on: string;
  from: string;
  kept: boolean;
}

export interface BackendBookingLeftoversResponse {
  leftovers?: BackendBookingLeftover[] | null;
}

export function mapBookingLeftovers(
  raw: BackendBookingLeftoversResponse,
): BookingLeftover[] {
  return (raw.leftovers ?? []).map((leftover) => ({
    templateId: String(leftover.template_id),
    templateName: leftover.template_name,
    studentId: String(leftover.student_id),
    studentName: leftover.student_name,
    weekdays: leftover.weekdays ?? [],
    bookingEndedOn: leftover.booking_ended_on,
    from: leftover.from,
    kept: leftover.kept,
  }));
}

/** One row per child, Regeltermin and start; stable across reloads. */
export function bookingLeftoverKey(leftover: BookingLeftover): string {
  return `${leftover.templateId}-${leftover.studentId}-${leftover.from}`;
}

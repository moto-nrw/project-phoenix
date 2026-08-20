// Pure mapping between the Abwesenheitsart dropdown's option values and the
// two request fields (#2403).
//
// The dropdown mixes two things that look alike and are not: the five standard
// types, which are code constants on both sides, and the school's own names,
// which are rows. Both share one option value space so that picking
// "Regenerationstag" does not require knowing which kind it is — and this
// module converts back at the request boundary.
//
// Standard types keep their canonical value ("sick", "vacation", …). A
// school's own art is `custom:<id>`. Nothing else in the app has to know the
// difference.
//
// Deliberately free of React and of any component import: the option list and
// the hook that loads it live next to their consumer in
// components/staff/use-absence-type-options.

const CUSTOM_PREFIX = "custom:";

export function customOptionValue(id: string): string {
  return `${CUSTOM_PREFIX}${id}`;
}

/**
 * selectValueFor maps a stored absence back onto its option: the school's own
 * art when the row carries one, otherwise the canonical type.
 */
export function selectValueFor(
  absenceType: string,
  absenceTypeId?: string | null,
): string {
  return absenceTypeId ? customOptionValue(absenceTypeId) : absenceType;
}

/**
 * absenceRequestFor turns an option back into the two request fields. The base
 * type sent along with a custom art is only a hint — the backend overwrites it
 * from the art itself, which is what keeps a name from choosing its own
 * arithmetic.
 */
export function absenceRequestFor(selectValue: string): {
  absence_type: string;
  absence_type_id: number | null;
} {
  if (selectValue.startsWith(CUSTOM_PREFIX)) {
    return {
      absence_type: "other",
      absence_type_id: Number(selectValue.slice(CUSTOM_PREFIX.length)),
    };
  }
  return { absence_type: selectValue, absence_type_id: null };
}

/** Strips the `custom:` prefix off an option value. */
export function customIdFromOptionValue(value: string): string {
  return value.slice(CUSTOM_PREFIX.length);
}

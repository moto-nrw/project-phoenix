/**
 * The care-facility term of a school (`enrollment.care_facility_term`,
 * #6042): what the fixed texts of the enrollment flow call the school's care,
 * for example „Die OGS prüft Ihre Angaben“ or „Die GTS prüft …“. Messages
 * carry it as the ICU placeholder `{careTerm}` in every language. Pure, so
 * server code and the tenant client share it.
 */
const DEFAULT_CARE_FACILITY_TERM = "OGS";

/** The term the backend sent, or „OGS“ for missing metadata. */
export function normalizeCareFacilityTerm(value: unknown): string {
  if (typeof value !== "string") return DEFAULT_CARE_FACILITY_TERM;
  const trimmed = value.trim();
  return trimmed === "" ? DEFAULT_CARE_FACILITY_TERM : trimmed;
}

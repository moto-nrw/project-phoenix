/**
 * Client of the registration form of a holiday period (#5198): the
 * Träger-Büro builds the questions with the form builder of the online
 * registration. Every save creates a new version; a registration keeps the
 * version it was sent with. Talks to the BFF route
 * /api/traeger/holiday-periods/{id}/form.
 */

import type { HolidayFormField } from "./holiday-form-field";
import { traegerRequest } from "./traeger-api";

export {
  HOLIDAY_FORM_FIELD_TYPES,
  type HolidayFormField,
} from "./holiday-form-field";

/** One version of the form of a holiday period. */
export interface HolidayForm {
  readonly version: number;
  readonly fields: readonly HolidayFormField[];
  readonly createdAt: string;
  /** The standard template (B7); „Standardvorlage einfügen“ adds from it. */
  readonly template: readonly HolidayFormField[];
}

interface BackendHolidayForm {
  version: number;
  fields?: HolidayFormField[] | null;
  created_at: string;
  template?: HolidayFormField[] | null;
}

function formFromBackend(form: BackendHolidayForm): HolidayForm {
  return {
    version: form.version,
    fields: form.fields ?? [],
    createdAt: form.created_at,
    template: form.template ?? [],
  };
}

function formPath(periodId: string): string {
  return `/api/traeger/holiday-periods/${encodeURIComponent(periodId)}/form`;
}

/** The current version of the form of a holiday period. */
export async function fetchHolidayForm(periodId: string): Promise<HolidayForm> {
  return formFromBackend(
    await traegerRequest<BackendHolidayForm>(formPath(periodId)),
  );
}

/** Saves the questions as a new version of the form. */
export async function saveHolidayForm(
  periodId: string,
  fields: readonly HolidayFormField[],
): Promise<HolidayForm> {
  return formFromBackend(
    await traegerRequest<BackendHolidayForm>(formPath(periodId), {
      method: "PUT",
      body: { fields },
    }),
  );
}

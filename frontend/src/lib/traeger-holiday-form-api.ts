/**
 * Client of the registration form of a holiday period (#5198): the
 * Träger-Büro builds the questions with the form builder of the online
 * registration. Every save creates a new version; a registration keeps the
 * version it was sent with. Talks to the BFF route
 * /api/traeger/holiday-periods/{id}/form.
 */

import { apiErrorFromResponse, transportFetch } from "./api-error";
import type { FormLegalBlock } from "./enrollment-form-schema-api";
import {
  holidayLegalBlocks,
  type HolidayLegalBlockWire,
  type HolidayFormField,
} from "./holiday-form-field";
import { traegerRequest } from "./traeger-api";

export {
  HOLIDAY_FORM_FIELD_TYPES,
  type HolidayFormField,
} from "./holiday-form-field";

/** One version of the form of a holiday period. */
export interface HolidayForm {
  readonly legalBlocks?: readonly FormLegalBlock[];
  readonly version: number;
  readonly fields: readonly HolidayFormField[];
  readonly createdAt: string;
  /** The standard template (B7); „Standardvorlage einfügen“ adds from it. */
  readonly template: readonly HolidayFormField[];
}

interface BackendHolidayForm {
  legal_blocks?: HolidayLegalBlockWire[];
  version: number;
  fields?: HolidayFormField[] | null;
  created_at: string;
  template?: HolidayFormField[] | null;
}

function formFromBackend(form: BackendHolidayForm): HolidayForm {
  return {
    legalBlocks: holidayLegalBlocks(form.legal_blocks),
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
  legalBlocks: readonly FormLegalBlock[] = [],
): Promise<HolidayForm> {
  return formFromBackend(
    await traegerRequest<BackendHolidayForm>(formPath(periodId), {
      method: "PUT",
      body: {
        fields,
        legal_blocks: legalBlocks.map((block) => ({
          ...block,
          attachments: block.attachments?.map((a) => ({
            label: a.label,
            document_id: a.document_url,
          })),
        })),
      },
    }),
  );
}

export function holidayDocumentPath(
  periodId: string,
  documentId?: string,
): string {
  return `${formPath(periodId)}/documents${documentId ? `/${encodeURIComponent(documentId)}` : ""}`;
}
export async function uploadHolidayDocument(
  periodId: string,
  file: File,
): Promise<string> {
  const body = new FormData();
  body.append("document", file);
  const response = await transportFetch(holidayDocumentPath(periodId), {
    method: "POST",
    body,
  });
  if (!response.ok)
    throw await apiErrorFromResponse(response, "Holiday PDF request failed");
  const result = (await response.json()) as { data: { id: string } };
  return String(result.data.id);
}
export async function deleteHolidayDocument(
  periodId: string,
  id: string,
  options?: { keepalive?: boolean },
): Promise<void> {
  const response = await transportFetch(holidayDocumentPath(periodId, id), {
    method: "DELETE",
    keepalive: options?.keepalive,
  });
  if (!response.ok)
    throw await apiErrorFromResponse(response, "Holiday PDF request failed");
}

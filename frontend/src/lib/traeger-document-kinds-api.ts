/**
 * Dokumentart je Leistung (#4688): Rechnung, Gebührenbescheid oder (nur
 * Ferien) Bestätigungsschreiben als Rechnung. Die Freigabe eines Laufs hält
 * die Art auf jedem Beleg fest. Ein Gebührenbescheid braucht die
 * Rechtsbehelfsbelehrung der Vorlage „Bescheid“.
 */

import { traegerRequest } from "./traeger-api";

export type BillingService = "holiday" | "care";
export type DocumentKind = "confirmation" | "invoice" | "fee_notice";

export interface ServiceDocumentKind {
  readonly service: BillingService;
  readonly documentKind: DocumentKind;
  /** Die Arten, die die Leistung ausstellen kann; die erste ist der Standard. */
  readonly choices: readonly DocumentKind[];
  /** Zeitpunkt der Wahl; `null`, solange der Standard gilt. */
  readonly updatedAt: string | null;
}

export interface DocumentKindOverview {
  readonly services: readonly ServiceDocumentKind[];
  /** Die Vorlage „Bescheid“ hat eine Rechtsbehelfsbelehrung. */
  readonly legalRemedySet: boolean;
  /** Version der Vorlage „Bescheid“; 0 = Standardvorlage. */
  readonly feeNoticeTemplateVersion: number;
}

interface BackendOverview {
  services?:
    | {
        service: BillingService;
        document_kind: DocumentKind;
        choices?: DocumentKind[] | null;
        updated_at?: string | null;
      }[]
    | null;
  legal_remedy_set: boolean;
  fee_notice_template_version: number;
}

const PATH = "/api/traeger/billing-document-kinds";

/** Name einer Dokumentart, wie Schreiben und Übersicht sie nennen. */
export const DOCUMENT_KIND_LABEL: Readonly<Record<DocumentKind, string>> = {
  confirmation: "Bestätigung und Rechnung",
  invoice: "Rechnung",
  fee_notice: "Gebührenbescheid",
};

/** Name einer Leistung. */
export const BILLING_SERVICE_LABEL: Readonly<Record<BillingService, string>> = {
  holiday: "Ferienbetreuung",
  care: "Monatsbeitrag",
};

function overviewFromBackend(body: BackendOverview): DocumentKindOverview {
  return {
    services: (body.services ?? []).map((service) => ({
      service: service.service,
      documentKind: service.document_kind,
      choices: service.choices ?? [],
      updatedAt: service.updated_at ?? null,
    })),
    legalRemedySet: body.legal_remedy_set,
    feeNoticeTemplateVersion: body.fee_notice_template_version,
  };
}

export async function fetchDocumentKinds(): Promise<DocumentKindOverview> {
  return overviewFromBackend(await traegerRequest<BackendOverview>(PATH));
}

export async function saveDocumentKind(
  service: BillingService,
  documentKind: DocumentKind,
): Promise<DocumentKindOverview> {
  return overviewFromBackend(
    await traegerRequest<BackendOverview>(
      `${PATH}/${encodeURIComponent(service)}`,
      { method: "PUT", body: { document_kind: documentKind } },
    ),
  );
}

/**
 * Takes over only the service a card saved: two cards may save at once, and
 * the answer of one must not hide the choice the other just stored.
 */
export function mergeSavedService(
  current: DocumentKindOverview | null,
  next: DocumentKindOverview,
  saved: BillingService,
): DocumentKindOverview {
  if (!current) return next;
  return {
    ...next,
    services: current.services.map((service) =>
      service.service === saved
        ? (next.services.find((candidate) => candidate.service === saved) ??
          service)
        : service,
    ),
  };
}

/**
 * Vorlagen des Trägers je Dokumentart (#4677, ADR 0150): Betreff, Anrede,
 * Brieftext und Grußformel mit Platzhaltern wie `{{familienname}}`. Jedes
 * Speichern ist eine neue Version; ohne eigene Version gilt die
 * Standardvorlage (Version 0).
 */
import { traegerRequest } from "./traeger-api";

/** Die Dokumentarten, in der Reihenfolge des Backends. */
export type DocumentTemplateKind =
  | "bestaetigung"
  | "rechnung"
  | "bescheid"
  | "vertrag"
  | "kuendigung"
  | "mahnung"
  | "bescheinigung";

/** Der Name einer Dokumentart, wie das Träger-Büro sie kennt. */
export const DOCUMENT_TEMPLATE_KIND_LABELS: Record<
  DocumentTemplateKind,
  string
> = {
  bestaetigung: "Bestätigung der Ferienanmeldung",
  rechnung: "Rechnung",
  bescheid: "Bescheid über den Elternbeitrag",
  vertrag: "Betreuungsvertrag",
  kuendigung: "Bestätigung der Kündigung",
  mahnung: "Zahlungserinnerung",
  bescheinigung: "Bescheinigung über Betreuungskosten",
};

/** Wo die Vorlage heute schon gedruckt wird. */
export const DOCUMENT_TEMPLATE_KIND_USE: Record<DocumentTemplateKind, string> =
  {
    bestaetigung:
      "Für die Bestätigungsschreiben eines freigegebenen Ferien-Laufs.",
    rechnung: "Für Rechnungen. Noch nicht in Gebrauch.",
    bescheid: "Für Gebührenbescheide. Noch nicht in Gebrauch.",
    vertrag: "Für Betreuungsverträge. Noch nicht in Gebrauch.",
    kuendigung: "Für Kündigungsbestätigungen. Noch nicht in Gebrauch.",
    mahnung: "Für Zahlungserinnerungen. Noch nicht in Gebrauch.",
    bescheinigung:
      "Für Bescheinigungen über Betreuungskosten. Noch nicht in Gebrauch.",
  };

export function isDocumentTemplateKind(
  value: string,
): value is DocumentTemplateKind {
  return value in DOCUMENT_TEMPLATE_KIND_LABELS;
}

/** Die vier Teile einer Vorlage. */
export interface DocumentTemplateText {
  readonly subject: string;
  readonly salutation: string;
  readonly body: string;
  readonly closing: string;
}

/** Eine Version einer Vorlage; Version 0 ist die Standardvorlage. */
export interface DocumentTemplateVersion extends DocumentTemplateText {
  readonly kind: DocumentTemplateKind;
  readonly version: number;
  readonly standard: boolean;
  /** Zeitpunkt des Speicherns; `null` bei der Standardvorlage. */
  readonly createdAt: string | null;
}

/** Der Stand einer Dokumentart in der Übersicht. */
export interface DocumentTemplateSummary {
  readonly kind: DocumentTemplateKind;
  readonly version: number;
  readonly standard: boolean;
  readonly updatedAt: string | null;
}

export interface DocumentTemplatePlaceholder {
  readonly name: string;
  readonly label: string;
  /** Druckt eine Tabelle, wenn er allein in einer Zeile steht. */
  readonly table: boolean;
}

export interface DocumentTemplateDetail {
  readonly current: DocumentTemplateVersion;
  readonly standardTemplate: DocumentTemplateText;
  readonly placeholders: readonly DocumentTemplatePlaceholder[];
  /** Alle eigenen Versionen, die neueste zuerst. */
  readonly versions: readonly DocumentTemplateVersion[];
}

interface DocumentTemplatePreviewRow {
  readonly label: string;
  readonly value: string;
  readonly bold: boolean;
}

interface DocumentTemplatePreviewBlock {
  readonly heading: string;
  readonly text: string;
  readonly bold: boolean;
  readonly rows: readonly DocumentTemplatePreviewRow[];
}

/** Eine Vorlage, gefüllt mit Beispieldaten. */
export interface DocumentTemplatePreview {
  readonly subject: string;
  readonly salutation: string;
  readonly blocks: readonly DocumentTemplatePreviewBlock[];
  readonly closing: readonly string[];
}

interface BackendVersion extends DocumentTemplateText {
  kind: DocumentTemplateKind;
  version: number;
  standard: boolean;
  created_at: string | null;
}

interface BackendSummary {
  kind: DocumentTemplateKind;
  version: number;
  standard: boolean;
  updated_at: string | null;
}

const basePath = "/api/traeger/master-data/document-templates";
const kindPath = (kind: DocumentTemplateKind) =>
  `${basePath}/${encodeURIComponent(kind)}`;

function mapVersion(version: BackendVersion): DocumentTemplateVersion {
  return {
    kind: version.kind,
    version: version.version,
    standard: version.standard,
    createdAt: version.created_at ?? null,
    subject: version.subject ?? "",
    salutation: version.salutation ?? "",
    body: version.body ?? "",
    closing: version.closing ?? "",
  };
}

/** Liest den Stand jeder Dokumentart. */
export async function fetchDocumentTemplates(): Promise<
  DocumentTemplateSummary[]
> {
  const data = await traegerRequest<BackendSummary[] | null>(basePath);
  return (data ?? []).map((summary) => ({
    kind: summary.kind,
    version: summary.version,
    standard: summary.standard,
    updatedAt: summary.updated_at ?? null,
  }));
}

/** Liest die Vorlage einer Dokumentart mit Platzhaltern und Versionen. */
export async function fetchDocumentTemplate(
  kind: DocumentTemplateKind,
): Promise<DocumentTemplateDetail> {
  const data = await traegerRequest<{
    current: BackendVersion;
    standard_template: DocumentTemplateText;
    placeholders: DocumentTemplatePlaceholder[] | null;
    versions: BackendVersion[] | null;
  }>(kindPath(kind));
  return {
    current: mapVersion(data.current),
    standardTemplate: data.standard_template,
    placeholders: data.placeholders ?? [],
    versions: (data.versions ?? []).map(mapVersion),
  };
}

/** Speichert die Vorlage als neue Version. Wirft bei Ablehnung einen ApiError. */
export async function saveDocumentTemplate(
  kind: DocumentTemplateKind,
  text: DocumentTemplateText,
): Promise<DocumentTemplateVersion> {
  return mapVersion(
    await traegerRequest<BackendVersion>(`${kindPath(kind)}/versions`, {
      method: "POST",
      body: text,
    }),
  );
}

/** Füllt die Vorlage mit Beispieldaten, ohne zu speichern. */
export async function previewDocumentTemplate(
  kind: DocumentTemplateKind,
  text: DocumentTemplateText,
): Promise<DocumentTemplatePreview> {
  const data = await traegerRequest<{
    subject: string;
    salutation: string;
    blocks: Array<{
      heading: string;
      text: string;
      bold: boolean;
      rows: DocumentTemplatePreviewRow[] | null;
    }> | null;
    closing: string[] | null;
  }>(`${kindPath(kind)}/preview`, { method: "POST", body: text });
  return {
    subject: data.subject,
    salutation: data.salutation,
    blocks: (data.blocks ?? []).map((block) => ({
      ...block,
      rows: block.rows ?? [],
    })),
    closing: data.closing ?? [],
  };
}

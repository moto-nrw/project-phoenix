/**
 * Vorlagen des Trägers je Dokumentart (#4677, ADR 0152): Betreff, Anrede,
 * Brieftext und Grußformel mit Platzhaltern wie `{{familienname}}`, bei
 * Schreiben mit Rechnung auch die Darstellung der Tabelle der Beträge
 * (#5691). Jedes Speichern ist eine neue Version; ohne eigene Version gilt
 * die Standardvorlage (Version 0).
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
  bestaetigung: "Bestätigung und Rechnung",
  rechnung: "Rechnung",
  bescheid: "Gebührenbescheid",
  vertrag: "Betreuungsvertrag",
  kuendigung: "Bestätigung der Kündigung",
  mahnung: "Zahlungserinnerung",
  bescheinigung: "Bescheinigung über Betreuungskosten",
};

/** Wo die Vorlage heute schon gedruckt wird. */
export const DOCUMENT_TEMPLATE_KIND_USE: Record<DocumentTemplateKind, string> =
  {
    bestaetigung:
      "Für die Bestätigung der Ferienanmeldung mit Rechnung, die ein freigegebener Ferien-Lauf verschickt.",
    rechnung:
      "Für Rechnungen eines Laufs, wenn unter Dokumentarten die Art „Rechnung“ gewählt ist.",
    bescheid:
      "Für Gebührenbescheide eines Laufs, wenn unter Dokumentarten die Art „Gebührenbescheid“ gewählt ist.",
    vertrag: "Für die Betreuungsverträge, die Sie an Eltern senden.",
    kuendigung:
      "Für einen Brief zur Kündigung. Noch nicht in Gebrauch. Den Text der E-Mail an die Eltern legen Sie unter Allgemein › Betreuungsverträge fest.",
    mahnung: "Für Zahlungserinnerungen. Noch nicht in Gebrauch.",
    bescheinigung:
      "Für Bescheinigungen über Betreuungskosten. Noch nicht in Gebrauch.",
  };

export function isDocumentTemplateKind(
  value: string,
): value is DocumentTemplateKind {
  return value in DOCUMENT_TEMPLATE_KIND_LABELS;
}

/**
 * Wie die Tabelle der Beträge die Positionen zeigt (#5691). Alles `false` ist
 * die Standardtabelle: eine Zeile je Position mit Vorname, Woche und
 * Zeitmodell. Betrag, Ermäßigungen, Gesamtbetrag und die Woche als Zeitraum
 * der Leistung stehen immer darin; sie sind Pflichtangaben der Rechnung.
 */
export interface DocumentTemplateInvoiceLayout {
  /** Vorname einmal über den Positionen des Kindes, darunter seine Summe. */
  readonly groupByChild: boolean;
  readonly hideTimeModels: boolean;
}

const STANDARD_INVOICE_LAYOUT: DocumentTemplateInvoiceLayout = {
  groupByChild: false,
  hideTimeModels: false,
};

/**
 * Die Teile einer Vorlage. `invoice` fehlt bei der Standardtabelle; so bleibt
 * eine Vorlage ohne eigene Tabelle dieselbe wie vor #5691.
 */
export interface DocumentTemplateText {
  readonly subject: string;
  readonly salutation: string;
  readonly body: string;
  readonly closing: string;
  readonly invoice?: DocumentTemplateInvoiceLayout;
  /**
   * Die Rechtsbehelfsbelehrung des Bescheids (#4688); fehlt, solange sie leer
   * ist. Ohne sie gibt moto keinen Gebührenbescheid frei.
   */
  readonly legalRemedy?: string;
}

/** Nur der Bescheid hat eine Rechtsbehelfsbelehrung (#4688). */
export function hasLegalRemedy(kind: DocumentTemplateKind): boolean {
  return kind === "bescheid";
}

/** Die Tabelle einer Vorlage; ohne eigene die Standardtabelle. */
export function invoiceLayoutOf(
  text: DocumentTemplateText,
): DocumentTemplateInvoiceLayout {
  return text.invoice ?? STANDARD_INVOICE_LAYOUT;
}

function isStandardLayout(layout: DocumentTemplateInvoiceLayout): boolean {
  return !layout.groupByChild && !layout.hideTimeModels;
}

/** Die vier Texte und, nur wenn sie abweicht, die Tabelle. */
export function templateTextOf(
  template: DocumentTemplateText,
): DocumentTemplateText {
  const text: DocumentTemplateText = {
    subject: template.subject,
    salutation: template.salutation,
    body: template.body,
    closing: template.closing,
  };
  const layout = invoiceLayoutOf(template);
  const withLayout = isStandardLayout(layout)
    ? text
    : { ...text, invoice: layout };
  return template.legalRemedy
    ? { ...withLayout, legalRemedy: template.legalRemedy }
    : withLayout;
}

/** Gleicher Text und gleiche Tabelle. */
export function sameTemplateText(
  a: DocumentTemplateText,
  b: DocumentTemplateText,
): boolean {
  const la = invoiceLayoutOf(a);
  const lb = invoiceLayoutOf(b);
  return (
    a.subject === b.subject &&
    a.salutation === b.salutation &&
    a.body === b.body &&
    a.closing === b.closing &&
    (a.legalRemedy ?? "") === (b.legalRemedy ?? "") &&
    la.groupByChild === lb.groupByChild &&
    la.hideTimeModels === lb.hideTimeModels
  );
}

/** Schreiben mit der Tabelle der Beträge haben eine Rechnungsdarstellung. */
export function hasInvoiceTable(
  placeholders: readonly DocumentTemplatePlaceholder[],
): boolean {
  return placeholders.some((placeholder) => placeholder.name === "positionen");
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
  /** Anschriftfeld der Beispielfamilie (#5691). */
  readonly recipient?: readonly string[];
  /** Nummern und Daten neben der Anschrift, wie der Brief sie druckt. */
  readonly facts?: readonly DocumentTemplatePreviewRow[];
  readonly subject: string;
  readonly salutation: string;
  readonly blocks: readonly DocumentTemplatePreviewBlock[];
  readonly closing: readonly string[];
  /** Die Absätze der Rechtsbehelfsbelehrung eines Bescheids (#4688). */
  readonly legalRemedy?: readonly DocumentTemplatePreviewBlock[];
}

interface BackendInvoiceLayout {
  group_by_child?: boolean;
  hide_time_models?: boolean;
}

interface BackendText {
  subject: string;
  salutation: string;
  body: string;
  closing: string;
  invoice?: BackendInvoiceLayout | null;
  legal_remedy?: string | null;
}

interface BackendVersion extends BackendText {
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

function mapText(text: BackendText): DocumentTemplateText {
  return templateTextOf({
    subject: text.subject ?? "",
    salutation: text.salutation ?? "",
    body: text.body ?? "",
    closing: text.closing ?? "",
    invoice: {
      groupByChild: text.invoice?.group_by_child ?? false,
      hideTimeModels: text.invoice?.hide_time_models ?? false,
    },
    legalRemedy: text.legal_remedy ?? "",
  });
}

function mapVersion(version: BackendVersion): DocumentTemplateVersion {
  return {
    kind: version.kind,
    version: version.version,
    standard: version.standard,
    createdAt: version.created_at ?? null,
    ...mapText(version),
  };
}

function requestBody(text: DocumentTemplateText) {
  const layout = invoiceLayoutOf(text);
  return {
    subject: text.subject,
    salutation: text.salutation,
    body: text.body,
    closing: text.closing,
    invoice: {
      group_by_child: layout.groupByChild,
      hide_time_models: layout.hideTimeModels,
    },
    legal_remedy: text.legalRemedy ?? "",
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
    standard_template: BackendText;
    placeholders: DocumentTemplatePlaceholder[] | null;
    versions: BackendVersion[] | null;
  }>(kindPath(kind));
  return {
    current: mapVersion(data.current),
    standardTemplate: mapText(data.standard_template),
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
      body: requestBody(text),
    }),
  );
}

/** Füllt die Vorlage mit Beispieldaten, ohne zu speichern. */
export async function previewDocumentTemplate(
  kind: DocumentTemplateKind,
  text: DocumentTemplateText,
): Promise<DocumentTemplatePreview> {
  const data = await traegerRequest<{
    recipient: string[] | null;
    facts: DocumentTemplatePreviewRow[] | null;
    subject: string;
    salutation: string;
    blocks: Array<{
      heading: string;
      text: string;
      bold: boolean;
      rows: DocumentTemplatePreviewRow[] | null;
    }> | null;
    closing: string[] | null;
    legal_remedy?: Array<{
      heading: string;
      text: string;
      bold: boolean;
      rows: DocumentTemplatePreviewRow[] | null;
    }> | null;
  }>(`${kindPath(kind)}/preview`, { method: "POST", body: requestBody(text) });
  return {
    recipient: data.recipient ?? [],
    facts: data.facts ?? [],
    subject: data.subject,
    salutation: data.salutation,
    blocks: (data.blocks ?? []).map((block) => ({
      ...block,
      rows: block.rows ?? [],
    })),
    closing: data.closing ?? [],
    legalRemedy: (data.legal_remedy ?? []).map((block) => ({
      ...block,
      rows: block.rows ?? [],
    })),
  };
}

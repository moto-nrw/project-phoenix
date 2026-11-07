"use client";

// Eine Vorlage des Trägers (#4677, ADR 0152): Betreff, Anrede, Brieftext und
// Grußformel mit Platzhaltern, bei Schreiben mit Rechnung die Darstellung der
// Rechnungstabelle (#5691), die Vorschau des ganzen Briefs mit Beispieldaten
// im Briefkopf des Trägers und die gespeicherten Versionen. Auf breiten
// Bildschirmen steht die Vorschau neben dem Formular und bleibt beim Scrollen
// sichtbar. Speichern legt eine neue Version an; unbekannte Platzhalter lehnt
// das Backend ab.

import { useParams } from "next/navigation";
import { useCallback, useEffect, useRef, useState } from "react";
import { useSession } from "next-auth/react";
import { DocumentTemplateInvoiceFields } from "~/components/traeger/document-template-invoice-fields";
import { DocumentTemplatePlaceholdersCard } from "~/components/traeger/document-template-placeholders-card";
import { DocumentTemplatePreviewBody } from "~/components/traeger/document-template-preview-body";
import { DocumentTemplateVersionsCard } from "~/components/traeger/document-template-versions-card";
import { TraegerLetterheadPreview } from "~/components/traeger/traeger-letterhead-preview";
import { useTraegerSettingsSchema } from "~/components/traeger/use-traeger-resources";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { EditActions } from "~/components/ui/edit-actions";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import { Textarea } from "~/components/ui/textarea";
import {
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { hasPermission } from "~/lib/auth-utils";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import type { ResolvedSetting } from "~/lib/settings-api";
import {
  fetchTraegerMasterData,
  type TraegerMasterData,
  traegerLogoSrc,
} from "~/lib/traeger-api";
import { TRAEGER_BRANDING_TAB } from "~/lib/traeger-branding";
import {
  DOCUMENT_TEMPLATE_KIND_LABELS,
  DOCUMENT_TEMPLATE_KIND_USE,
  type DocumentTemplateDetail,
  type DocumentTemplateKind,
  type DocumentTemplatePlaceholder,
  type DocumentTemplatePreview,
  type DocumentTemplateText,
  fetchDocumentTemplate,
  hasInvoiceTable,
  invoiceLayoutOf,
  isDocumentTemplateKind,
  previewDocumentTemplate,
  sameTemplateText,
  saveDocumentTemplate,
  templateTextOf,
} from "~/lib/traeger-document-templates";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerVorlagePage" });

/** Das Träger-Recht, das Vorlagen ändert (ADR 0084 Abschnitt 4). */
const SETTINGS_RIGHT = "traeger_settings:manage";

interface PageData {
  readonly detail: DocumentTemplateDetail;
  readonly masterData: TraegerMasterData;
  readonly preview: DocumentTemplatePreview;
}

export default function TraegerVorlagePage() {
  const params = useParams<{ kind: string }>();
  const kind = params?.kind ?? "";
  if (!isDocumentTemplateKind(kind)) {
    return (
      <TenantPage
        title="Vorlage"
        back
        backHref={traegerPath("/traeger/einstellungen/briefkopf/vorlagen")}
        backLabel="Zurück zu den Vorlagen"
      >
        <Alert
          type="info"
          message="Diese Vorlage gibt es nicht. Wählen Sie eine aus der Liste."
        />
      </TenantPage>
    );
  }
  return <TemplateEditorPage key={kind} kind={kind} />;
}

function TemplateEditorPage({
  kind,
}: Readonly<{ kind: DocumentTemplateKind }>) {
  const { data: session } = useSession();
  const canEdit = hasPermission(session ?? null, SETTINGS_RIGHT);
  const [data, setData] = useState<PageData | null>(null);
  const [draft, setDraft] = useState<DocumentTemplateText | null>(null);
  const [previewed, setPreviewed] = useState<DocumentTemplateText | null>(null);
  const [saving, setSaving] = useState(false);
  const [previewing, setPreviewing] = useState(false);
  // A save or reload makes every older draft preview obsolete.
  const previewRequest = useRef(0);
  const {
    data: schema,
    isLoading: schemaLoading,
    loadError: schemaError,
  } = useTraegerSettingsSchema();
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;
  const formRef = useRef<HTMLFormElement>(null);
  const bodyRef = useRef<HTMLTextAreaElement>(null);
  // Die letzte Schreibmarke im Brieftext. Ein Klick auf einen Platzhalter
  // nimmt dem Feld den Fokus; ohne Marke kommt der Platzhalter ans Ende.
  const caret = useRef<{ start: number; end: number } | null>(null);
  const formErrors = useApiFormError(formRef);
  const { success } = useToast();

  const load = useCallback(async () => {
    previewRequest.current++;
    setPreviewing(false);
    try {
      const [detail, masterData] = await Promise.all([
        fetchDocumentTemplate(kind),
        fetchTraegerMasterData(),
      ]);
      const current = templateTextOf(detail.current);
      const preview = await previewDocumentTemplate(kind, current);
      setData({ detail, masterData, preview });
      setDraft(current);
      setPreviewed(current);
      clearLoadError();
    } catch (err) {
      logger.error("traeger_document_template_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Vorlage",
        retry: () => void load(),
      });
    }
  }, [kind, clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const showPreview = async () => {
    if (!draft || !data || saving || previewing) return;
    const request = ++previewRequest.current;
    formErrors.clear();
    setPreviewing(true);
    const shown = draft;
    try {
      const preview = await previewDocumentTemplate(kind, shown);
      if (request !== previewRequest.current) return;
      setData((before) => (before ? { ...before, preview } : before));
      setPreviewed(shown);
    } catch (err) {
      if (request !== previewRequest.current) return;
      logger.warn("traeger_document_template_preview_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "die Vorschau" });
    } finally {
      if (request === previewRequest.current) setPreviewing(false);
    }
  };

  const save = async () => {
    if (!draft || saving) return;
    previewRequest.current++;
    setPreviewing(false);
    formErrors.clear();
    setSaving(true);
    try {
      const saved = await saveDocumentTemplate(kind, draft);
      success(
        `Die Vorlage ist gespeichert. Neue Schreiben nutzen Version ${saved.version}.`,
      );
      await load();
    } catch (err) {
      logger.error("traeger_document_template_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "die Vorlage" });
    } finally {
      setSaving(false);
    }
  };

  const error = pageLoad.error ?? schemaError;
  const current = data ? templateTextOf(data.detail.current) : null;
  const changed =
    draft !== null && current !== null && !sameTemplateText(draft, current);
  const previewStale =
    draft !== null && previewed !== null && !sameTemplateText(draft, previewed);
  const standard = data?.detail.standardTemplate ?? null;
  const branding =
    schema?.tabs.find((tab) => tab.key === TRAEGER_BRANDING_TAB) ?? null;
  const brandingValue = (key: string): string => {
    const item: ResolvedSetting | undefined = branding?.categories
      .flatMap((category) => category.items)
      .find((candidate) => candidate.key === key);
    return typeof item?.value === "string" ? item.value : "";
  };
  const set = (
    part: "subject" | "salutation" | "body" | "closing",
    value: string,
  ) => setDraft((before) => (before ? { ...before, [part]: value } : before));
  const rememberCaret = (field: HTMLTextAreaElement) => {
    caret.current = { start: field.selectionStart, end: field.selectionEnd };
  };
  // Setzt den Platzhalter an die letzte Schreibmarke des Brieftexts, ohne
  // Marke ans Ende, und lässt die Marke hinter ihm stehen.
  const insertPlaceholder = (placeholder: DocumentTemplatePlaceholder) => {
    if (!draft) return;
    const field = bodyRef.current;
    const token = `{{${placeholder.name}}}`;
    const length = draft.body.length;
    const end = Math.min(caret.current?.end ?? length, length);
    const start = Math.min(caret.current?.start ?? length, end);
    // Eine Tabelle druckt nur allein in ihrer Zeile.
    const before =
      placeholder.table && start > 0 && draft.body[start - 1] !== "\n"
        ? "\n"
        : "";
    const after =
      placeholder.table && end < draft.body.length && draft.body[end] !== "\n"
        ? "\n"
        : "";
    const insert = before + token + after;
    const body = draft.body.slice(0, start) + insert + draft.body.slice(end);
    set("body", body);
    const next = start + insert.length;
    caret.current = { start: next, end: next };
    requestAnimationFrame(() => {
      field?.focus();
      field?.setSelectionRange(next, next);
    });
  };
  const invoice = data && hasInvoiceTable(data.detail.placeholders);

  return (
    <TenantPage
      title={DOCUMENT_TEMPLATE_KIND_LABELS[kind]}
      back
      backHref={traegerPath("/traeger/einstellungen/briefkopf/vorlagen")}
      backLabel="Zurück zu den Vorlagen"
      info={DOCUMENT_TEMPLATE_KIND_USE[kind]}
      stats={
        data ? (
          <TenantPageStats
            items={
              data.detail.current.standard
                ? [{ value: "Standardvorlage", label: "gilt" }]
                : [
                    {
                      value: `Version ${data.detail.current.version}`,
                      label: data.detail.current.createdAt
                        ? `vom ${formatDate(data.detail.current.createdAt)}`
                        : "gilt",
                    },
                  ]
            }
          />
        ) : null
      }
      statsLoading={!data && !error}
      error={error}
      loading={!error && (!data || schemaLoading)}
    >
      {data && draft && schema ? (
        <>
          {!canEdit ? (
            <Alert
              type="info"
              message="Sie können die Vorlage nur ansehen. Ändern kann, wer im Träger-Büro das Recht „Einstellungen“ hat."
            />
          ) : null}
          {/* Zwei Spalten ab xl: links bearbeiten, rechts die Vorschau, die
              beim Scrollen stehen bleibt. Auf schmalen Bildschirmen folgt
              die Vorschau direkt auf das Formular. */}
          <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_minmax(0,1fr)] xl:items-start">
            <div className="flex min-w-0 flex-col gap-6">
              <SectionCard
                title="Text der Vorlage"
                description="Das steht im Brief unter dem Briefkopf. Speichern legt eine neue Version an."
                action={
                  canEdit && standard && !sameTemplateText(draft, standard) ? (
                    <Button
                      type="button"
                      variant="outline"
                      size="md"
                      disabled={saving}
                      onClick={() => {
                        setDraft(templateTextOf(standard));
                        formErrors.clear();
                      }}
                    >
                      Standardtext laden
                    </Button>
                  ) : null
                }
              >
                <form
                  ref={formRef}
                  noValidate
                  className="space-y-4"
                  onSubmit={(event) => {
                    event.preventDefault();
                    void save();
                  }}
                >
                  <FormErrorAlert message={formErrors.error} />
                  <Input
                    id="template-subject"
                    label="Betreff"
                    name="subject"
                    required
                    value={draft.subject}
                    onChange={(e) => set("subject", e.target.value)}
                    error={formErrors.fieldError("subject")}
                    maxLength={200}
                    disabled={!canEdit || saving}
                  />
                  <Input
                    id="template-salutation"
                    label="Anrede"
                    name="salutation"
                    value={draft.salutation}
                    onChange={(e) => set("salutation", e.target.value)}
                    error={formErrors.fieldError("salutation")}
                    maxLength={200}
                    disabled={!canEdit || saving}
                  />
                  <Textarea
                    id="template-body"
                    label="Brieftext"
                    name="body"
                    required
                    rows={14}
                    ref={bodyRef}
                    onBlur={(e) => rememberCaret(e.currentTarget)}
                    value={draft.body}
                    onChange={(e) => set("body", e.target.value)}
                    error={formErrors.fieldError("body")}
                    maxLength={10000}
                    disabled={!canEdit || saving}
                    className="font-mono"
                  />
                  <Textarea
                    id="template-closing"
                    label="Grußformel"
                    name="closing"
                    rows={3}
                    value={draft.closing}
                    onChange={(e) => set("closing", e.target.value)}
                    error={formErrors.fieldError("closing")}
                    maxLength={500}
                    disabled={!canEdit || saving}
                  />
                  {invoice ? (
                    <DocumentTemplateInvoiceFields
                      layout={invoiceLayoutOf(draft)}
                      onChange={(layout) =>
                        setDraft((before) =>
                          before
                            ? templateTextOf({ ...before, invoice: layout })
                            : before,
                        )
                      }
                      disabled={!canEdit || saving}
                      error={formErrors.fieldError("invoice")}
                    />
                  ) : null}
                  <div className="flex flex-wrap items-center justify-between gap-2 border-t border-gray-100 pt-4">
                    <Button
                      type="button"
                      variant="outline"
                      size="md"
                      disabled={saving || previewing || !previewStale}
                      isLoading={previewing}
                      loadingText="Vorschau lädt…"
                      onClick={() => void showPreview()}
                    >
                      Vorschau zeigen
                    </Button>
                    {canEdit && changed ? (
                      <EditActions
                        onCancel={() => {
                          if (current) setDraft(current);
                          formErrors.clear();
                        }}
                        saving={saving}
                        saveType="submit"
                      />
                    ) : null}
                  </div>
                </form>
              </SectionCard>
            </div>
            <div className="min-w-0 xl:sticky xl:top-6 xl:row-span-3">
              <SectionCard
                title="Vorschau"
                description="So sieht der ganze Brief mit erfundenen Beispieldaten aus. Logo, Absender, Ansprechperson und Fußzeile kommen aus dem Briefkopf."
              >
                {previewStale ? (
                  <Alert
                    type="info"
                    message="Die Vorschau zeigt noch den Text vor Ihren Änderungen. Tippen Sie auf „Vorschau zeigen“."
                  />
                ) : null}
                <div className={previewStale ? "mt-4" : undefined}>
                  <TraegerLetterheadPreview
                    label="Vorschau des Briefs"
                    logoSrc={
                      data.masterData.logo
                        ? traegerLogoSrc(data.masterData.logo)
                        : null
                    }
                    letterheadText={brandingValue("branding.letterhead_text")}
                    footerText={brandingValue("branding.footer_text")}
                    color={brandingValue("branding.primary_color")}
                    contact={data.masterData.contact}
                    defaultAccount={
                      data.masterData.bankAccounts.find(
                        (account) => account.isDefault,
                      ) ?? null
                    }
                    recipient={data.preview.recipient}
                    facts={data.preview.facts}
                    body={
                      <DocumentTemplatePreviewBody preview={data.preview} />
                    }
                  />
                </div>
              </SectionCard>
            </div>
            <div className="flex min-w-0 flex-col gap-6">
              <DocumentTemplatePlaceholdersCard
                placeholders={data.detail.placeholders}
                onInsert={canEdit ? insertPlaceholder : undefined}
                disabled={saving}
              />

              <DocumentTemplateVersionsCard
                versions={data.detail.versions}
                canEdit={canEdit}
                disabled={saving}
                onLoad={(version) => {
                  setDraft(templateTextOf(version));
                  formErrors.clear();
                  formRef.current?.scrollIntoView?.({
                    behavior: "smooth",
                    block: "start",
                  });
                }}
              />
            </div>
          </div>
        </>
      ) : null}
    </TenantPage>
  );
}

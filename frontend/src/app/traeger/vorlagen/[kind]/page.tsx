"use client";

// Eine Vorlage des Trägers (#4677, ADR 0150): Betreff, Anrede, Brieftext und
// Grußformel mit Platzhaltern, die Vorschau mit Beispieldaten im Briefkopf
// des Trägers und die gespeicherten Versionen. Speichern legt eine neue
// Version an; unbekannte Platzhalter lehnt das Backend ab.

import { useParams } from "next/navigation";
import { useCallback, useEffect, useRef, useState } from "react";
import { useSession } from "next-auth/react";
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
  type DocumentTemplatePreview,
  type DocumentTemplateText,
  fetchDocumentTemplate,
  isDocumentTemplateKind,
  previewDocumentTemplate,
  saveDocumentTemplate,
} from "~/lib/traeger-document-templates";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerVorlagePage" });

/** Das Träger-Recht, das Vorlagen ändert (ADR 0084 Abschnitt 4). */
const SETTINGS_RIGHT = "traeger_settings:manage";

function textOf(template: DocumentTemplateText): DocumentTemplateText {
  return {
    subject: template.subject,
    salutation: template.salutation,
    body: template.body,
    closing: template.closing,
  };
}

function sameText(a: DocumentTemplateText, b: DocumentTemplateText): boolean {
  return (
    a.subject === b.subject &&
    a.salutation === b.salutation &&
    a.body === b.body &&
    a.closing === b.closing
  );
}

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
        backHref={traegerPath("/traeger/vorlagen")}
        backLabel="Zurück zu den Vorlagen"
      >
        <Alert
          type="info"
          message="Diese Vorlage gibt es nicht. Wählen Sie eine aus der Liste."
        />
      </TenantPage>
    );
  }
  return <TemplateEditorPage kind={kind} />;
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
  const { data: schema } = useTraegerSettingsSchema();
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { success } = useToast();

  const load = useCallback(async () => {
    try {
      const [detail, masterData] = await Promise.all([
        fetchDocumentTemplate(kind),
        fetchTraegerMasterData(),
      ]);
      const current = textOf(detail.current);
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
    if (!draft || !data) return;
    formErrors.clear();
    setPreviewing(true);
    const shown = draft;
    try {
      const preview = await previewDocumentTemplate(kind, shown);
      setData({ ...data, preview });
      setPreviewed(shown);
    } catch (err) {
      logger.warn("traeger_document_template_preview_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "die Vorschau" });
    } finally {
      setPreviewing(false);
    }
  };

  const save = async () => {
    if (!draft) return;
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

  const current = data ? textOf(data.detail.current) : null;
  const changed =
    draft !== null && current !== null && !sameText(draft, current);
  const previewStale =
    draft !== null && previewed !== null && !sameText(draft, previewed);
  const standard = data?.detail.standardTemplate ?? null;
  const branding =
    schema?.tabs.find((tab) => tab.key === TRAEGER_BRANDING_TAB) ?? null;
  const brandingValue = (key: string): string => {
    const item: ResolvedSetting | undefined = branding?.categories
      .flatMap((category) => category.items)
      .find((candidate) => candidate.key === key);
    return typeof item?.value === "string" ? item.value : "";
  };
  const set = (part: keyof DocumentTemplateText, value: string) =>
    setDraft((before) => (before ? { ...before, [part]: value } : before));

  return (
    <TenantPage
      title={DOCUMENT_TEMPLATE_KIND_LABELS[kind]}
      back
      backHref={traegerPath("/traeger/vorlagen")}
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
      statsLoading={!data && !pageLoad.error}
      error={pageLoad.error}
      loading={!pageLoad.error && !data}
    >
      {data && draft ? (
        <>
          {!canEdit ? (
            <Alert
              type="info"
              message="Sie können die Vorlage nur ansehen. Ändern kann, wer im Träger-Büro das Recht „Einstellungen“ hat."
            />
          ) : null}
          <SectionCard
            title="Text der Vorlage"
            description="Das steht im Brief unter dem Briefkopf. Speichern legt eine neue Version an."
            action={
              canEdit && standard && !sameText(draft, standard) ? (
                <Button
                  type="button"
                  variant="outline"
                  size="md"
                  onClick={() => {
                    setDraft(textOf(standard));
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
                disabled={!canEdit}
              />
              <Input
                id="template-salutation"
                label="Anrede"
                name="salutation"
                value={draft.salutation}
                onChange={(e) => set("salutation", e.target.value)}
                error={formErrors.fieldError("salutation")}
                maxLength={200}
                disabled={!canEdit}
              />
              <Textarea
                id="template-body"
                label="Brieftext"
                name="body"
                required
                rows={14}
                value={draft.body}
                onChange={(e) => set("body", e.target.value)}
                error={formErrors.fieldError("body")}
                maxLength={10000}
                disabled={!canEdit}
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
                disabled={!canEdit}
              />
              <div className="flex flex-wrap items-center justify-between gap-2">
                <Button
                  type="button"
                  variant="outline"
                  size="md"
                  disabled={previewing || !previewStale}
                  onClick={() => void showPreview()}
                >
                  {previewing ? "Vorschau lädt…" : "Vorschau zeigen"}
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

          <SectionCard
            title="Vorschau"
            description="So sieht der Brief mit erfundenen Beispieldaten aus. Logo, Absender und Fußzeile kommen aus dem Briefkopf."
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
                body={<DocumentTemplatePreviewBody preview={data.preview} />}
              />
            </div>
          </SectionCard>

          <DocumentTemplatePlaceholdersCard
            placeholders={data.detail.placeholders}
          />

          <DocumentTemplateVersionsCard
            versions={data.detail.versions}
            canEdit={canEdit}
            onLoad={(version) => {
              setDraft(textOf(version));
              formErrors.clear();
              formRef.current?.scrollIntoView?.({
                behavior: "smooth",
                block: "start",
              });
            }}
          />
        </>
      ) : null}
    </TenantPage>
  );
}

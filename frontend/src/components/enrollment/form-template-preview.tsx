"use client";

import { ExternalLink, Info } from "lucide-react";
import {
  FormPreviewFrame,
  FormPreviewHeading,
  PreviewAttachmentNames,
  PreviewFieldsSection,
  PreviewSection,
  type PreviewFieldLabel,
} from "~/components/enrollment/form-preview";
import { CORE_FIELDS } from "~/components/enrollment/form-template-core-fields";
import {
  StatusBadge,
  type StatusBadgeTone,
} from "~/components/ui/status-badge";
import type {
  CoreRequirements,
  FormField,
  FormLegalBlock,
} from "~/lib/enrollment-form-schema-api";
import { legalBlockPreviewText } from "~/lib/enrollment-legal-blocks";
import { useTenantAwarePath } from "~/lib/tenant-path";

/** The address of a template's parent preview in this tenant. */
export function useTemplatePreviewHref(): (schemaId: string) => string {
  const tenantPath = useTenantAwarePath();
  return (schemaId) =>
    tenantPath(`/anmeldung/preview?schemaId=${encodeURIComponent(schemaId)}`);
}

export function openTemplatePreview(href: string) {
  window.open(href, "_blank", "noopener,noreferrer");
}

function PreviewLinkCardContent({
  description,
}: Readonly<{ description: string }>) {
  return (
    <>
      <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-white text-gray-600 shadow-sm">
        <ExternalLink className="h-4 w-4" aria-hidden="true" />
      </span>
      <span className="min-w-0">
        <span className="block text-sm font-semibold text-gray-900">
          Vorschau öffnen
        </span>
        <span className="mt-0.5 block text-xs leading-5 text-gray-500">
          {description}
        </span>
      </span>
    </>
  );
}

/**
 * The card that opens the parent preview in a new tab: a link, a button that
 * saves first, or a placeholder while there is nothing to preview.
 */
export function PreviewLinkCard({
  href,
  onClick,
  description,
}: Readonly<{
  href?: string;
  onClick?: () => void;
  description: string;
}>) {
  if (href) {
    return (
      <a
        href={href}
        target="_blank"
        rel="noreferrer"
        className="moto-content-surface moto-hover-elevated flex items-start gap-3 rounded-2xl border p-3 text-left focus-visible:outline-2 focus-visible:outline-gray-400"
      >
        <PreviewLinkCardContent description={description} />
      </a>
    );
  }
  if (onClick) {
    return (
      <button
        type="button"
        onClick={onClick}
        className="moto-content-surface moto-hover-elevated flex w-full items-start gap-3 rounded-2xl border p-3 text-left focus-visible:outline-2 focus-visible:outline-gray-400"
      >
        <PreviewLinkCardContent description={description} />
      </button>
    );
  }
  return (
    <div className="moto-content-surface flex items-start gap-3 rounded-2xl border p-3 text-left opacity-70">
      <PreviewLinkCardContent description={description} />
    </div>
  );
}

export function FormPreview({
  fields,
  coreRequirements,
  legalBlocks,
  templateName,
  isActive,
  isSaved,
  previewHref,
  onPreviewClick,
  assignedPhaseCount = 0,
  sticky = true,
}: Readonly<{
  fields: FormField[];
  coreRequirements: CoreRequirements;
  legalBlocks: FormLegalBlock[];
  templateName: string;
  isActive: boolean;
  isSaved: boolean;
  previewHref?: string;
  onPreviewClick?: () => void;
  assignedPhaseCount?: number;
  sticky?: boolean;
}>) {
  const previewStatus = getPreviewStatus({
    assignedPhaseCount,
    isActive,
    isSaved,
  });
  const enabledLegalBlocks = legalBlocks.filter((block) => block.enabled);
  const coreFieldLabels = (group: "guardian" | "child"): PreviewFieldLabel[] =>
    CORE_FIELDS.filter((field) => field.group === group).map((field) => ({
      label: field.previewLabel,
      required: field.requirementKey
        ? coreRequirements[field.requirementKey]
        : field.required,
    }));

  return (
    <FormPreviewFrame
      sticky={sticky}
      title={templateName.trim() || "Basisformular"}
      header={
        <>
          <FormPreviewHeading
            title="Elternformular"
            hint={previewStatus.hint}
            status={
              <StatusBadge
                tone={previewStatus.tone}
                label={previewStatus.label}
              />
            }
          />

          <PreviewLinkCard
            href={previewHref}
            onClick={onPreviewClick}
            description={
              previewHref
                ? "Öffnet diese Formularvorlage in einem neuen Tab."
                : onPreviewClick
                  ? "Speichert die Vorlage und öffnet die Vorschau in einem neuen Tab."
                  : "Speichern Sie die Vorlage zuerst."
            }
          />
        </>
      }
    >
      <PreviewSection title="Elternteil" fields={coreFieldLabels("guardian")} />
      <PreviewSection title="Kind" fields={coreFieldLabels("child")} />
      <PreviewSection
        title="Betreuungsangebot"
        fields={["Gewünschtes Angebot", "Betreuungstage"]}
      />

      <PreviewFieldsSection
        title="Zusatzfragen"
        fields={fields}
        emptyTitle="Keine Zusatzfragen"
        emptyText="Eltern sehen nur das Basisformular mit den gewählten Pflichtangaben."
      />

      {enabledLegalBlocks.length > 0 ? (
        <section>
          <div className="mb-3 flex items-center justify-between gap-3">
            <h4 className="text-sm font-semibold text-gray-900">
              Zustimmungen & Hinweise
            </h4>
            <StatusBadge
              tone="gray"
              compact
              label={String(enabledLegalBlocks.length)}
            />
          </div>
          <div className="space-y-2">
            {enabledLegalBlocks.map((block) => {
              const previewText = legalBlockPreviewText(block);
              return (
                <div
                  key={block.key}
                  className="rounded-lg border border-gray-200 bg-white px-3 py-2"
                >
                  <div className="flex items-start gap-2.5">
                    <span
                      className={`mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center rounded border ${
                        block.kind === "notice"
                          ? "border-moto-blue/30 bg-moto-blue/10"
                          : "border-gray-300 bg-white"
                      }`}
                      aria-hidden="true"
                    >
                      {block.kind === "notice" ? (
                        <Info className="text-moto-blue h-3 w-3" />
                      ) : null}
                    </span>
                    <div className="min-w-0 flex-1">
                      <div className="flex items-start justify-between gap-2">
                        <span className="text-sm font-medium text-gray-900">
                          {block.title.trim() || block.label}
                        </span>
                        <span className="shrink-0">
                          <StatusBadge
                            tone="gray"
                            compact
                            label={
                              block.kind === "notice"
                                ? "Hinweis"
                                : block.required
                                  ? "Pflicht"
                                  : "Optional"
                            }
                          />
                        </span>
                      </div>
                      <p className="mt-1 line-clamp-2 text-xs leading-5 text-gray-500">
                        {block.label}
                      </p>
                      <PreviewAttachmentNames attachments={block.attachments} />
                      {previewText.trim() !== "" ? (
                        <p className="mt-1 line-clamp-2 text-sm leading-5 text-gray-400">
                          {previewText}
                        </p>
                      ) : null}
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        </section>
      ) : null}
    </FormPreviewFrame>
  );
}

function getPreviewStatus({
  assignedPhaseCount,
  isActive,
  isSaved,
}: Readonly<{
  assignedPhaseCount: number;
  isActive: boolean;
  isSaved: boolean;
}>): { label: string; hint: string; tone: StatusBadgeTone } {
  if (assignedPhaseCount > 0) {
    return {
      label: "In Phase verwendet",
      hint: `Diese Vorlage ist in ${assignedPhaseCount} Anmeldephase ausgewählt.`,
      tone: "green",
    };
  }

  if (isActive) {
    return {
      label: "Bereit zur Zuordnung",
      hint: "Eltern sehen diese Vorlage erst, wenn sie einer Anmeldephase zugeordnet ist.",
      tone: "green",
    };
  }

  if (isSaved) {
    return {
      label: "Gespeichert",
      hint: "Ordne die Vorlage in einer Anmeldephase zu, damit Eltern sie verwenden.",
      tone: "gray",
    };
  }

  return {
    label: "Entwurf",
    hint: "Speichern Sie die Vorlage zuerst. Danach können Sie sie in einer Anmeldephase auswählen.",
    tone: "gray",
  };
}

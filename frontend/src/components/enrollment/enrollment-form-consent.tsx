// The Einwilligungen and Hinweise block of the OGS-Anmeldung
// (enrollment-form.tsx): one consent row per legal block, the e-mail contact
// notice, the "Details" button that opens a legal text, and the Markdown the
// legal modal renders. Also resolves which legal blocks an admin preview shows.

import { FileText, Info } from "lucide-react";

import { Button } from "~/components/ui/button";
import { ConsentCheckbox } from "~/components/ui/consent-checkbox";
import {
  fetchPublicLegalTexts,
  type PublicFormSchema,
  type PublicLegalBlock,
  type PublicLegalTexts,
} from "~/lib/enrollment-form-schema-api";

import type { TranslationFn } from "./enrollment-form-shared";

export { LegalMarkdown } from "./legal-markdown";

/**
 * One consent a parent ticks for a legal block. When the school configured
 * a legal document, a "Details" button at the end of the row opens it; the
 * button sits outside the row's label, so reading never ticks the box.
 */
export function EnrollmentConsent({
  name,
  label,
  checked,
  onChange,
  required = false,
  error,
  legalText,
  legalTitle,
  onViewLegal,
  tr,
}: {
  readonly name: string;
  readonly label: string;
  readonly checked: boolean;
  readonly onChange: (v: boolean) => void;
  readonly required?: boolean;
  readonly error?: string;
  /**
   * Optional per-tenant legal document (Markdown). When non-empty, the
   * consent row gains a "Details" button that opens onViewLegal. When
   * empty/undefined the row renders plain — tenants that haven't
   * configured a document still get a working form.
   */
  readonly legalText?: string;
  readonly legalTitle?: string;
  readonly onViewLegal?: () => void;
  readonly tr: TranslationFn;
}) {
  const hasLink = Boolean(legalText?.trim() && onViewLegal);
  return (
    <ConsentCheckbox
      name={name}
      label={label}
      checked={checked}
      onChange={onChange}
      required={required}
      error={error}
      action={
        hasLink ? (
          <LegalDetailsButton
            title={legalTitle ?? label}
            label={tr("actions.details")}
            ariaLabel={tr("actions.detailsAria", {
              title: legalTitle ?? label,
            })}
            onClick={onViewLegal}
          />
        ) : undefined
      }
    />
  );
}

/**
 * Informational notice for the e-mail contact. Unlike the consent rows
 * this carries NO checkbox: the school needs the e-mail address to run
 * the enrollment (Rückfragen, Status-Link), which is contract
 * performance, not a consent the parent could decline and still enrol.
 * When the tenant configured an explanatory text it gains a localized
 * "show more" link that opens it in the shared legal modal.
 */
export function EmailContactNotice({
  block,
  onViewLegal,
  tr,
}: {
  readonly block: PublicLegalBlock;
  readonly onViewLegal?: () => void;
  readonly tr: TranslationFn;
}) {
  return (
    <div className="border-moto-blue/25 bg-moto-blue/5 flex items-center gap-3 rounded-lg border p-3 text-sm leading-6 text-gray-700">
      <span className="border-moto-blue/30 text-moto-blue flex h-5 w-5 shrink-0 items-center justify-center rounded-md border bg-white">
        <Info className="h-3.5 w-3.5" aria-hidden="true" />
      </span>
      <div className="min-w-0 flex-1">
        <div className="mb-1 flex flex-wrap items-center gap-2">
          <span className="text-moto-blue text-xs font-semibold tracking-wide uppercase">
            {tr("actions.notice")}
          </span>
        </div>
        <span className="font-medium">{block.label}</span>{" "}
        {onViewLegal && (
          <LegalDetailsButton
            title={block.title}
            label={tr("actions.details")}
            ariaLabel={tr("actions.detailsAria", { title: block.title })}
            onClick={onViewLegal}
          />
        )}
      </div>
    </div>
  );
}

function LegalDetailsButton({
  title,
  label,
  ariaLabel,
  onClick,
}: {
  readonly title: string;
  readonly label: string;
  readonly ariaLabel: string;
  readonly onClick?: () => void;
}) {
  return (
    <Button
      type="button"
      variant="ghost"
      size="compact"
      aria-label={ariaLabel}
      title={title}
      onClick={onClick}
      className="shrink-0"
    >
      <FileText className="h-3.5 w-3.5" aria-hidden="true" />
      <span>{label}</span>
    </Button>
  );
}

/**
 * Resolves the legal blocks an admin preview shows. Template-owned blocks
 * win; base previews and templates without enabled blocks fall back to the
 * tenant-wide legal settings — the same resolution the live form uses, so
 * the preview can never omit blocks the live form will require.
 */
export async function resolvePreviewLegalTexts(
  tenantSlug: string,
  schema: PublicFormSchema | null,
): Promise<PublicLegalTexts> {
  const fromTemplate = previewLegalTexts(schema);
  if (fromTemplate.blocks.length > 0) return fromTemplate;
  return fetchPublicLegalTexts(tenantSlug);
}

function previewLegalTexts(schema: PublicFormSchema | null): PublicLegalTexts {
  const blocks =
    schema?.legal_blocks
      ?.filter((block) => block.enabled)
      .map((block) => ({
        key: block.key,
        kind: block.kind,
        title: block.title,
        label: block.label,
        text: legalBlockText(block),
        attachments: block.attachments,
        required: block.required,
        sort_order: block.sort_order,
        source: block.source,
        // The preview reads the stored document; the resolver compares each
        // translation's source, so a PDF block's link text stays German.
        translations: block.translations,
      }))
      .sort((a, b) => a.sort_order - b.sort_order) ?? [];
  return {
    agb: blocks.find((block) => block.key === "agb")?.text ?? "",
    agb_document_url: "",
    agb_display_mode: "text",
    dsgvo: blocks.find((block) => block.key === "data_processing")?.text ?? "",
    email_contact:
      blocks.find((block) => block.key === "email_contact")?.text ?? "",
    photo: blocks.find((block) => block.key === "photo")?.text ?? "",
    terms_enabled: blocks.some((block) => block.key === "agb"),
    dsgvo_enabled: blocks.some((block) => block.key === "data_processing"),
    email_contact_enabled: blocks.some(
      (block) => block.key === "email_contact",
    ),
    photo_enabled: blocks.some((block) => block.key === "photo"),
    blocks,
  };
}

function legalBlockText(
  block: NonNullable<PublicFormSchema["legal_blocks"]>[number],
) {
  const documentURL = (block.document_url ?? "").trim();
  if (
    block.key === "agb" &&
    block.display_mode === "pdf" &&
    documentURL !== ""
  ) {
    return `Die AGB / Teilnahmebedingungen sind als PDF-Datei hinterlegt: [AGB-Dokument öffnen](${publicAGBDocumentURL(documentURL)})`;
  }
  return block.text;
}

function publicAGBDocumentURL(storedURL: string): string {
  const globalPrefix = "/uploads/enrollment-legal-documents/";
  if (storedURL.startsWith(globalPrefix)) {
    return `/api/public/enrollment-legal-documents/${storedURL.slice(globalPrefix.length)}`;
  }
  const formPrefix = "/uploads/enrollment-form-legal-documents/";
  if (storedURL.startsWith(formPrefix)) {
    return `/api/public/enrollment-form-legal-documents/${storedURL.slice(formPrefix.length)}`;
  }
  return storedURL;
}

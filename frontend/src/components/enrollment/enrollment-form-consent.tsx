// The Einwilligungen and Hinweise block of the OGS-Anmeldung
// (enrollment-form.tsx): one consent row per legal block, the e-mail contact
// notice, the "Details" button that opens a legal text, and the Markdown the
// legal modal renders. Also resolves which legal blocks an admin preview shows.

import { FileText, Info } from "lucide-react";
import ReactMarkdown, { type Components } from "react-markdown";

import { Button } from "~/components/ui/button";
import { ConsentCheckbox } from "~/components/ui/consent-checkbox";
import {
  fetchPublicLegalTexts,
  type PublicFormSchema,
  type PublicLegalBlock,
  type PublicLegalTexts,
} from "~/lib/enrollment-form-schema-api";

import type { TranslationFn } from "./enrollment-form-shared";

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
  detailsLabel,
  detailsAriaLabel,
}: {
  readonly block: PublicLegalBlock;
  readonly onViewLegal?: () => void;
  readonly detailsLabel: string;
  readonly detailsAriaLabel: string;
}) {
  return (
    <div className="border-moto-blue/25 bg-moto-blue/5 flex items-center gap-3 rounded-lg border p-3 text-sm leading-6 text-gray-700">
      <span className="border-moto-blue/30 text-moto-blue flex h-5 w-5 shrink-0 items-center justify-center rounded-md border bg-white">
        <Info className="h-3.5 w-3.5" aria-hidden="true" />
      </span>
      <div className="min-w-0 flex-1">
        <div className="mb-1 flex flex-wrap items-center gap-2">
          <span className="text-moto-blue text-xs font-semibold tracking-wide uppercase">
            Hinweis
          </span>
        </div>
        <span className="font-medium">{block.label}</span>{" "}
        {onViewLegal && (
          <LegalDetailsButton
            title={block.title}
            label={detailsLabel}
            ariaLabel={detailsAriaLabel}
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
      className="shrink-0 font-semibold"
    >
      <FileText className="h-3.5 w-3.5" aria-hidden="true" />
      <span>{label}</span>
    </Button>
  );
}

// Explicit element styling for the legal-document Markdown. Defined at
// module scope (not inline in the `components` prop) so the renderers
// keep a stable identity across re-renders. Tailwind v4 + Preflight
// strips default heading/list styling, and the project has no
// @tailwindcss/typography plugin, so each tag is styled by hand.
// Links open in a new tab; react-markdown does not emit raw HTML by
// default, so authored Markdown stays XSS-safe.
//
// Every renderer strips react-markdown's internal `node` prop before
// spreading the rest onto the DOM element — forwarding `node` produces
// an invalid HTML attribute and a React warning.
const LEGAL_MARKDOWN_COMPONENTS: Components = {
  h1: ({ node: _node, children, ...props }) => (
    <h1 className="mt-4 mb-2 text-lg font-bold text-gray-900" {...props}>
      {children}
    </h1>
  ),
  h2: ({ node: _node, children, ...props }) => (
    <h2 className="mt-4 mb-2 text-base font-bold text-gray-900" {...props}>
      {children}
    </h2>
  ),
  h3: ({ node: _node, children, ...props }) => (
    <h3 className="mt-3 mb-1.5 text-sm font-bold text-gray-900" {...props}>
      {children}
    </h3>
  ),
  p: ({ node: _node, children, ...props }) => (
    <p className="mb-3 leading-6" {...props}>
      {children}
    </p>
  ),
  ul: ({ node: _node, children, ...props }) => (
    <ul className="mb-3 list-disc space-y-1 pl-5" {...props}>
      {children}
    </ul>
  ),
  ol: ({ node: _node, children, ...props }) => (
    <ol className="mb-3 list-decimal space-y-1 pl-5" {...props}>
      {children}
    </ol>
  ),
  li: ({ node: _node, children, ...props }) => (
    <li className="leading-6" {...props}>
      {children}
    </li>
  ),
  a: ({ node: _node, children, ...props }) => (
    <a
      className="text-moto-blue hover:text-moto-blue-hover font-medium underline underline-offset-2"
      target="_blank"
      rel="noopener noreferrer"
      {...props}
    >
      {children}
    </a>
  ),
  strong: ({ node: _node, children, ...props }) => (
    <strong className="font-semibold text-gray-900" {...props}>
      {children}
    </strong>
  ),
  em: ({ node: _node, children, ...props }) => (
    <em className="italic" {...props}>
      {children}
    </em>
  ),
};

export function LegalMarkdown({ text }: { readonly text: string }) {
  return (
    <ReactMarkdown components={LEGAL_MARKDOWN_COMPONENTS}>{text}</ReactMarkdown>
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

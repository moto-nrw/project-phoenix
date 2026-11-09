"use client";

// Das Elternformular als Vorschau (#5198): Rahmen, Überschrift, feste
// Abschnitte und die Zusatzfragen. Der Schul-Editor der Online-Anmeldung und
// das Anmeldeformular eines Ferienzeitraums nutzen dieselben Bausteine.

import type { ReactNode } from "react";
import { ChevronDown, Eye, HelpCircle } from "lucide-react";
import { FilePreviewInput } from "~/components/enrollment/file-field-editor";
import { Alert } from "~/components/ui/alert";
import { buttonClassName } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { RequiredMark } from "~/components/ui/field-label";
import { InsetPanel } from "~/components/ui/inset-panel";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import type {
  FormAttachment,
  FormField,
} from "~/lib/enrollment-form-schema-api";
import { getStableObjectKey } from "~/lib/stable-object-key";

export interface PreviewFieldLabel {
  label: string;
  required?: boolean;
}

export function PreviewSection({
  title,
  description,
  fields,
}: Readonly<{
  title: string;
  /** One line under the title: where the shown answers come from. */
  description?: string;
  fields: readonly (string | PreviewFieldLabel)[];
}>) {
  return (
    <section>
      <h4 className="text-sm font-semibold text-gray-900">{title}</h4>
      {description ? (
        <p className="mt-1 text-xs leading-5 text-gray-500">{description}</p>
      ) : null}
      <div className="mt-3 grid gap-2">
        {fields.map((entry) => {
          const field = typeof entry === "string" ? { label: entry } : entry;
          return (
            <InsetPanel key={field.label} tone="white" padding="compact">
              <span className="block text-sm font-medium text-gray-600">
                {field.label}
                {field.required && <RequiredMark />}
              </span>
            </InsetPanel>
          );
        })}
      </div>
    </section>
  );
}

function ConditionalBadge({ field }: Readonly<{ field: FormField }>) {
  if (!field.visible_when) return null;
  return <StatusBadge tone="blue" compact label="bedingt" />;
}

export function PreviewAttachmentNames({
  attachments,
}: Readonly<{ attachments?: FormAttachment[] }>) {
  if (!attachments?.length) return null;
  return (
    <ul className="mt-2 space-y-1 text-sm text-gray-600">
      {attachments.map((attachment) => (
        <li key={attachment.document_url}>PDF: {attachment.label}</li>
      ))}
    </ul>
  );
}

function PreviewCustomField({
  field,
  badge,
}: Readonly<{ field: FormField; badge?: ReactNode }>) {
  if (field.type === "information") {
    return (
      <Alert
        type="info"
        size="compact"
        announce="off"
        actionLayout="inline"
        action={
          field.visible_when ? <ConditionalBadge field={field} /> : undefined
        }
        title={field.label.trim() || "Infotext"}
        message={
          field.content?.trim() ? (
            <p className="leading-5 whitespace-pre-line">{field.content}</p>
          ) : (
            <p className="leading-5 italic opacity-70">
              Noch kein Text hinterlegt.
            </p>
          )
        }
        details={<PreviewAttachmentNames attachments={field.attachments} />}
      />
    );
  }

  return (
    <InsetPanel tone="white" padding="compact">
      <div className="flex flex-wrap items-start justify-between gap-x-3 gap-y-1">
        <span className="min-w-0 text-sm font-medium break-words text-gray-900">
          {field.label.trim() || "Neue Zusatzfrage"}
        </span>
        <div className="flex flex-wrap items-center gap-1.5">
          {badge}
          <ConditionalBadge field={field} />
          {field.required ? (
            <StatusBadge tone="gray" compact label="Pflicht" />
          ) : null}
        </div>
      </div>
      {field.help_text ? (
        <p className="mt-1 text-xs leading-5 text-gray-500">
          {field.help_text}
        </p>
      ) : null}
      <PreviewInput field={field} />
    </InsetPanel>
  );
}

function PreviewInput({ field }: Readonly<{ field: FormField }>) {
  if (field.type === "boolean") {
    return (
      <div className="mt-3 grid grid-cols-2 gap-2">
        <span className="flex h-9 items-center justify-center rounded-lg border border-gray-200 bg-gray-50 text-xs font-medium text-gray-500">
          Ja
        </span>
        <span className="flex h-9 items-center justify-center rounded-lg border border-gray-200 bg-gray-50 text-xs font-medium text-gray-500">
          Nein
        </span>
      </div>
    );
  }

  if (field.type === "select") {
    const firstOption = field.options?.[0]?.label ?? "Bitte wählen";
    return (
      // oxlint-disable-next-line ui-kit/no-hand-rolled-surface -- Nachbildung eines Auswahlfelds in der Formularvorschau, keine Fläche; trägt den Radius der Bedienelemente (#6137)
      <div className="mt-3 flex h-10 items-center justify-between rounded-lg border border-gray-200 bg-white px-3 text-sm text-gray-500 shadow-sm">
        <span className="min-w-0 truncate">{firstOption}</span>
        <ChevronDown
          className="h-4 w-4 shrink-0 text-gray-400"
          aria-hidden="true"
        />
      </div>
    );
  }

  if (field.type === "file") {
    return <FilePreviewInput field={field} />;
  }

  if (field.type === "textarea") {
    return (
      <div className="mt-3 h-16 rounded-lg border border-gray-200 bg-gray-50" />
    );
  }

  return (
    <div className="mt-3 h-9 rounded-lg border border-gray-200 bg-gray-50" />
  );
}
/**
 * The parent form as a preview: a header above it, the sections in it and
 * the send button at its end. Each page decides which fixed sections and
 * which questions it shows.
 */
export function FormPreviewFrame({
  header,
  title,
  intro = "Bitte füllen Sie das Formular vollständig aus.",
  submitLabel = "Anmeldung absenden",
  sticky = true,
  children,
}: Readonly<{
  header: ReactNode;
  title: string;
  intro?: string;
  submitLabel?: string;
  sticky?: boolean;
  children: ReactNode;
}>) {
  return (
    <div className={sticky ? "sticky top-6 space-y-4" : "space-y-4"}>
      {header}
      <SectionCard padding="none">
        <div className="border-b border-gray-100 px-4 py-4">
          <h3 className="text-lg font-semibold text-gray-900">{title}</h3>
          <p className="mt-1 text-sm leading-6 text-gray-500">{intro}</p>
        </div>

        <div className="space-y-5 p-4">
          {children}
          {/* Only a picture of the parents' send button: the preview sends
              nothing, so it is neither focusable nor clickable. */}
          <p
            aria-hidden="true"
            className={buttonClassName({
              variant: "primary",
              size: "md",
              className: "pointer-events-none w-full select-none",
            })}
          >
            {submitLabel}
          </p>
        </div>
      </SectionCard>
    </div>
  );
}

/** The heading of the preview with its eye icon and an optional status. */
export function FormPreviewHeading({
  title,
  status,
  hint,
}: Readonly<{ title: string; status?: ReactNode; hint: string }>) {
  return (
    <div className="flex items-start justify-between gap-4">
      <div>
        <div className="flex flex-wrap items-center gap-2">
          <h2 className="text-base font-semibold text-gray-900">{title}</h2>
          {status}
        </div>
        <p className="mt-1 max-w-sm text-xs leading-5 text-gray-500">{hint}</p>
      </div>
      <span className="inline-flex h-9 w-9 items-center justify-center rounded-xl bg-white text-gray-600 shadow-sm">
        <Eye className="h-4 w-4" aria-hidden="true" />
      </span>
    </div>
  );
}

/** The questions of a form in the preview, or a hint when there are none. */
export function PreviewFieldsSection({
  title,
  fields,
  emptyTitle,
  emptyText,
  fieldBadge,
}: Readonly<{
  title: string;
  fields: readonly FormField[];
  emptyTitle: string;
  emptyText: string;
  fieldBadge?: (field: FormField) => ReactNode;
}>) {
  return (
    <section>
      <div className="mb-3 flex items-center justify-between gap-3">
        <h4 className="text-sm font-semibold text-gray-900">{title}</h4>
        <StatusBadge tone="gray" compact label={String(fields.length)} />
      </div>
      {fields.length === 0 ? (
        <EmptyState
          variant="dashed"
          icon={<HelpCircle className="h-5 w-5" />}
          title={emptyTitle}
          description={emptyText}
        />
      ) : (
        <div className="space-y-3">
          {fields.map((field) => (
            <PreviewCustomField
              key={getStableObjectKey(field, "preview-field")}
              field={field}
              badge={fieldBadge?.(field)}
            />
          ))}
        </div>
      )}
    </section>
  );
}

"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { AttachmentList } from "~/components/enrollment/attachment-list";
import {
  EnrollmentConsent,
  EmailContactNotice,
  LegalMarkdown,
} from "~/components/enrollment/enrollment-form-consent";
import { Modal } from "~/components/ui/modal";
import { SectionCard } from "~/components/ui/section-card";
import type { FormLegalBlock } from "~/lib/enrollment-form-schema-api";

function holidayPublicDocumentPath(token: string, id: string): string {
  return `/api/holiday-registrations/${encodeURIComponent(token)}/documents/${encodeURIComponent(id)}`;
}

/** Required holiday blocks offer an explicit acknowledgement, including
 * PDF notices. Optional notices stay read-only. Reading a document never changes its checkbox. */
export function HolidayRegistrationLegal({
  token,
  blocks,
  accepted,
  onChange,
  fieldError,
}: Readonly<{
  token: string;
  blocks: readonly FormLegalBlock[];
  accepted: Readonly<Record<string, boolean>>;
  onChange: (key: string, checked: boolean) => void;
  fieldError: (name: string) => string | undefined;
}>) {
  const t = useTranslations("holidayRegistration");
  const tr = useTranslations("enrollmentForm");
  const [open, setOpen] = useState<FormLegalBlock | null>(null);
  const visible = blocks.filter((b) => b.enabled);
  if (visible.length === 0) return null;
  return (
    <SectionCard title={t("legalTitle")}>
      <div className="space-y-4">
        {visible.map((block) => (
          <div key={block.key}>
            <h3 className="mb-2 text-sm font-semibold text-gray-900">
              {block.title}
            </h3>
            {block.kind === "notice" && !block.required ? (
              <EmailContactNotice
                block={block}
                onViewLegal={
                  block.text.trim() ? () => setOpen(block) : undefined
                }
                tr={tr}
              />
            ) : (
              <EnrollmentConsent
                name={`legal_acceptances.${block.key}`}
                label={block.label}
                required={block.required}
                checked={accepted[block.key] === true}
                onChange={(checked) => onChange(block.key, checked)}
                error={fieldError(`legal_acceptances.${block.key}`)}
                legalText={block.text}
                legalTitle={block.title}
                onViewLegal={() => setOpen(block)}
                tr={tr}
              />
            )}
            <AttachmentList
              attachments={block.attachments}
              documentURL={(id) => holidayPublicDocumentPath(token, id)}
            />
          </div>
        ))}
      </div>
      <Modal
        isOpen={open !== null}
        onClose={() => setOpen(null)}
        title={open?.title ?? ""}
      >
        <LegalMarkdown text={open?.text ?? ""} />
      </Modal>
    </SectionCard>
  );
}

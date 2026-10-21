"use client";

import { createContext, useContext, useEffect, useRef, useState } from "react";
import { Button } from "~/components/ui/button";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { Input } from "~/components/ui/input";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import {
  MAX_ENROLLMENT_DOCUMENT_BYTES,
  uploadEnrollmentLegalDocument,
  deleteEnrollmentLegalDocument,
  type FormAttachment,
} from "~/lib/enrollment-form-schema-api";

export const AttachmentUploadContext = createContext<{
  remember: (url: string) => void;
  begin: () => void;
  end: () => void;
} | null>(null);

/** Removal is staged with the form. The editor cleans unreferenced drafts
 * on save/discard, while the backend protects every saved version's files. */
const NO_ATTACHMENTS: FormAttachment[] = [];

export function AttachmentEditor({
  attachments = NO_ATTACHMENTS,
  onChange,
  disabled,
  fieldPath,
  fieldError,
}: Readonly<{
  attachments?: FormAttachment[];
  onChange: (attachments: FormAttachment[]) => void;
  disabled: boolean;
  fieldPath: string;
  fieldError: (name: string) => string | undefined;
}>) {
  const lifecycle = useContext(AttachmentUploadContext);
  const { show, showMessage } = useApiErrorDisplay();
  const [uploading, setUploading] = useState(false);
  const picker = useRef<HTMLInputElement>(null);
  const replacing = useRef<string | null>(null);
  const current = useRef({ attachments, onChange });
  const mounted = useRef(true);
  useEffect(() => {
    current.current = { attachments, onChange };
  });
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);

  async function upload(file: File | undefined) {
    if (!file || !lifecycle) return;
    if (!file.name.toLowerCase().endsWith(".pdf")) {
      showMessage("Bitte wählen Sie eine PDF-Datei aus.");
      return;
    }
    if (file.size > MAX_ENROLLMENT_DOCUMENT_BYTES) {
      showMessage(
        "Die Datei ist größer als 10 MB. Bitte wählen Sie eine kleinere Datei.",
      );
      return;
    }
    setUploading(true);
    lifecycle.begin();
    const previousURL = replacing.current;
    try {
      const document_url = await uploadEnrollmentLegalDocument(file);
      lifecycle.remember(document_url);
      if (!mounted.current) {
        await deleteEnrollmentLegalDocument(document_url, { keepalive: true });
        return;
      }
      const latest = current.current;
      const replacement = latest.attachments.find(
        (a) => a.document_url === previousURL,
      );
      const attachment = {
        label: replacement?.label ?? file.name.replace(/\.pdf$/i, ""),
        document_url,
      };
      latest.onChange(
        replacement
          ? latest.attachments.map((a) =>
              a.document_url === previousURL ? attachment : a,
            )
          : [...latest.attachments, attachment],
      );
    } catch (error) {
      void show(error, { object: "die PDF-Datei" });
    } finally {
      lifecycle.end();
      if (mounted.current) setUploading(false);
    }
  }

  return (
    <div className="space-y-3">
      <p className="text-sm text-gray-600">
        Fügen Sie PDF-Dateien zum Lesen und Herunterladen hinzu. Jede Datei darf
        höchstens 10 MB groß sein.
      </p>
      {attachments.map((attachment, index) => (
        <div
          key={attachment.document_url}
          className="space-y-2 border-l-2 border-gray-200 pl-3"
        >
          <Input
            name={`${fieldPath}.${index}.label`}
            error={fieldError(`${fieldPath}.${index}.label`)}
            label={`Bezeichnung der PDF ${index + 1}`}
            value={attachment.label}
            disabled={disabled || uploading}
            onChange={(event) =>
              onChange(
                attachments.map((a, i) =>
                  i === index ? { ...a, label: event.target.value } : a,
                ),
              )
            }
          />
          <div className="flex flex-wrap gap-2">
            {!disabled && !uploading ? (
              <OverflowMenu
                ariaLabel={`Aktionen für PDF ${index + 1}`}
                items={[
                  {
                    label: "PDF ersetzen",
                    onClick: () => {
                      replacing.current = attachment.document_url;
                      picker.current?.click();
                    },
                  },
                  {
                    label: "PDF entfernen",
                    onClick: () =>
                      onChange(attachments.filter((_, i) => i !== index)),
                  },
                ]}
              />
            ) : null}
          </div>
        </div>
      ))}
      <Button
        type="button"
        variant="outline"
        size="md"
        disabled={disabled || uploading}
        isLoading={uploading}
        onClick={() => {
          replacing.current = null;
          picker.current?.click();
        }}
      >
        PDF hinzufügen
      </Button>
      <input
        ref={picker}
        type="file"
        accept=".pdf,application/pdf"
        aria-label="PDF-Datei auswählen"
        className="sr-only"
        disabled={disabled || uploading}
        onChange={(event) => {
          const file = event.currentTarget.files?.[0];
          event.currentTarget.value = "";
          void upload(file);
        }}
      />
    </div>
  );
}

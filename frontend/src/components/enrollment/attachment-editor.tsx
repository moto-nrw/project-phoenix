"use client";

import { createContext, useContext, useEffect, useRef, useState } from "react";
import { Plus, RefreshCw, X } from "lucide-react";
import { Button } from "~/components/ui/button";
import { FileTypeIcon } from "~/components/ui/file-type-icon";
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

function selectionError(files: File[]): string | undefined {
  if (files.some((file) => !file.name.toLowerCase().endsWith(".pdf"))) {
    return "Bitte wählen Sie nur PDF-Dateien aus.";
  }
  if (files.some((file) => file.size > MAX_ENROLLMENT_DOCUMENT_BYTES)) {
    return "Eine Datei ist größer als 10 MB. Bitte wählen Sie kleinere Dateien.";
  }
  return undefined;
}

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
  const replacementPicker = useRef<HTMLInputElement>(null);
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

  function attach(
    file: File,
    document_url: string,
    previousURL: string | null,
  ) {
    const latest = current.current;
    const replacement = latest.attachments.find(
      (attachment) => attachment.document_url === previousURL,
    );
    const attachment = {
      label: replacement?.label ?? file.name.replace(/\.pdf$/i, ""),
      document_url,
    };
    const next = replacement
      ? latest.attachments.map((a) =>
          a.document_url === previousURL ? attachment : a,
        )
      : [...latest.attachments, attachment];
    // Keep fast, consecutive uploads in the same selection from overwriting
    // each other before React has committed the parent's next render.
    current.current = { ...latest, attachments: next };
    latest.onChange(next);
  }

  async function upload(files: File[], previousURL: string | null) {
    if (files.length === 0 || !lifecycle) return;
    const error = selectionError(files);
    if (error) {
      showMessage(error);
      return;
    }
    setUploading(true);
    lifecycle.begin();
    try {
      for (const file of files) {
        if (!mounted.current) break;
        try {
          const documentURL = await uploadEnrollmentLegalDocument(file);
          lifecycle.remember(documentURL);
          if (!mounted.current) {
            await deleteEnrollmentLegalDocument(documentURL, {
              keepalive: true,
            });
            break;
          }
          attach(file, documentURL, previousURL);
        } catch (uploadError) {
          void show(uploadError, {
            object: "die PDF-Datei",
            messageSuffix: `Datei: ${file.name}`,
          });
        }
      }
    } finally {
      lifecycle.end();
      if (mounted.current) setUploading(false);
    }
  }

  const busy = disabled || uploading;
  return (
    <div className="space-y-3">
      <div>
        <h4 className="text-sm font-semibold text-gray-900">PDF-Anhänge</h4>
        <p className="mt-1 text-sm text-gray-600">
          Eltern können alle PDFs öffnen und herunterladen.
        </p>
      </div>
      {attachments.length > 0 ? (
        <ol className="space-y-3">
          {attachments.map((attachment, index) => (
            <li
              key={attachment.document_url}
              className="moto-content-surface inline-flex w-full flex-col gap-3 rounded-xl border p-3 shadow-sm"
            >
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div className="flex items-center gap-2">
                  <FileTypeIcon contentType="application/pdf" />
                  <span className="text-sm font-medium text-gray-700">
                    PDF {index + 1}
                  </span>
                </div>
                <div className="flex items-center gap-2">
                  <Button
                    type="button"
                    variant="ghost"
                    size="md"
                    className="gap-2"
                    disabled={busy}
                    aria-label={`PDF ${index + 1} ersetzen`}
                    onClick={() => {
                      replacing.current = attachment.document_url;
                      replacementPicker.current?.click();
                    }}
                  >
                    <RefreshCw className="h-4 w-4" aria-hidden="true" />
                    Ersetzen
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="h-10"
                    disabled={busy}
                    aria-label={`PDF ${index + 1} entfernen`}
                    title="PDF entfernen"
                    onClick={() =>
                      onChange(attachments.filter((_, i) => i !== index))
                    }
                  >
                    <X className="h-4 w-4" aria-hidden="true" />
                  </Button>
                </div>
              </div>
              <Input
                name={`${fieldPath}.${index}.label`}
                error={fieldError(`${fieldPath}.${index}.label`)}
                label="Bezeichnung für Eltern"
                aria-label={`Bezeichnung für Eltern: PDF ${index + 1}`}
                controlSize="compact"
                className="max-sm:text-base"
                value={attachment.label}
                disabled={busy}
                onChange={(event) =>
                  onChange(
                    attachments.map((a, i) =>
                      i === index ? { ...a, label: event.target.value } : a,
                    ),
                  )
                }
              />
            </li>
          ))}
        </ol>
      ) : (
        <p className="text-sm text-gray-500">Noch keine PDFs angehängt.</p>
      )}
      <div className="space-y-2">
        <Button
          type="button"
          variant="outline"
          size="md"
          className="max-w-full gap-2"
          disabled={busy}
          isLoading={uploading}
          loadingText="PDFs werden hochgeladen…"
          onClick={() => picker.current?.click()}
        >
          <Plus className="h-4 w-4 shrink-0" aria-hidden="true" />
          {attachments.length > 0
            ? "Weitere PDFs hinzufügen"
            : "PDFs hinzufügen"}
        </Button>
        <p className="text-xs leading-5 text-gray-500">
          Sie können mehrere PDFs gleichzeitig auswählen. Höchstens 10 MB pro
          Datei.
        </p>
      </div>
      <input
        ref={picker}
        type="file"
        accept=".pdf,application/pdf"
        multiple
        aria-label="PDF-Dateien auswählen"
        hidden
        disabled={busy}
        onChange={(event) => {
          const files = Array.from(event.currentTarget.files ?? []);
          event.currentTarget.value = "";
          void upload(files, null);
        }}
      />
      <input
        ref={replacementPicker}
        type="file"
        accept=".pdf,application/pdf"
        aria-label="Ersatz-PDF auswählen"
        hidden
        disabled={busy}
        onChange={(event) => {
          const file = event.currentTarget.files?.[0];
          event.currentTarget.value = "";
          void upload(file ? [file] : [], replacing.current);
        }}
      />
    </div>
  );
}

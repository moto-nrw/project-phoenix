"use client";

// Ein vorhandenes Dokument hochladen, damit der Träger es freigeben kann
// (#4692). Freigegeben ist es danach noch nicht: das geschieht auf der Seite
// des Dokuments.

import { UploadSimpleIcon } from "@phosphor-icons/react";
import { useEffect, useRef, useState } from "react";
import { Button } from "~/components/ui/button";
import { EditActions } from "~/components/ui/edit-actions";
import { FileTypeIcon } from "~/components/ui/file-type-icon";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { useApiFormError } from "~/contexts/ToastContext";
import { formatBytes } from "~/lib/files-api";
import { createLogger } from "~/lib/logger";
import { cn } from "~/lib/utils";
import {
  TRAEGER_DOCUMENT_ACCEPT,
  TRAEGER_DOCUMENT_MAX_BYTES,
  type TraegerDocument,
  uploadTraegerDocument,
} from "~/lib/traeger-documents-api";
import { documentKindLabel } from "./traeger-document-format";

const logger = createLogger({ component: "TraegerDocumentUploadModal" });

const TITLE_MAX_LENGTH = 120;

const ADMITTED_EXTENSIONS = [
  ".pdf",
  ".docx",
  ".xlsx",
  ".pptx",
  ".png",
  ".jpg",
  ".jpeg",
];

/** Der Dateiname ohne Endung, als Vorschlag für den Titel. */
function titleFromFileName(name: string): string {
  const stem = name.replace(/\.[^.]+$/, "").replaceAll(/[_-]+/g, " ");
  return stem.trim().slice(0, TITLE_MAX_LENGTH);
}

function admitted(file: File): boolean {
  const lower = file.name.toLowerCase();
  return ADMITTED_EXTENSIONS.some((extension) => lower.endsWith(extension));
}

export function TraegerDocumentUploadModal({
  isOpen,
  onClose,
  onUploaded,
}: Readonly<{
  isOpen: boolean;
  onClose: () => void;
  onUploaded: (document: TraegerDocument) => void;
}>) {
  const [file, setFile] = useState<File | null>(null);
  const [title, setTitle] = useState("");
  const [dragging, setDragging] = useState(false);
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const picker = useRef<HTMLInputElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors } = formErrors;

  useEffect(() => {
    if (!isOpen) return;
    setFile(null);
    setTitle("");
    setDragging(false);
    clearErrors();
  }, [isOpen, clearErrors]);

  const choose = (picked: File | undefined) => {
    if (!picked) return;
    formErrors.clear();
    if (!admitted(picked)) {
      formErrors.invalid("Diese Datei geht nicht.", {
        file: "Erlaubt sind PDF, Word, Excel, PowerPoint und Bilder (PNG, JPEG).",
      });
      return;
    }
    if (picked.size > TRAEGER_DOCUMENT_MAX_BYTES) {
      formErrors.invalid("Diese Datei ist zu groß.", {
        file: "Eine Datei darf höchstens 10 MB groß sein.",
      });
      return;
    }
    setFile(picked);
    setTitle((current) => current || titleFromFileName(picked.name));
  };

  const submit = async () => {
    formErrors.clear();
    const trimmed = title.trim();
    if (!file || !trimmed) {
      formErrors.invalid("Bitte prüfen Sie Ihre Angaben.", {
        ...(file ? {} : { file: "Bitte wählen Sie eine Datei." }),
        ...(trimmed ? {} : { title: "Bitte geben Sie einen Titel ein." }),
      });
      return;
    }
    setSaving(true);
    try {
      onUploaded(await uploadTraegerDocument(trimmed, file));
    } catch (err) {
      logger.error("traeger_document_upload_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "das Dokument" });
    } finally {
      setSaving(false);
    }
  };

  const fileError = formErrors.fieldError("file");
  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      closeDisabled={saving}
      isBackdropDismissDisabled
      title="Dokument hochladen"
      size="md"
      error={formErrors.error}
      footer={
        <EditActions
          onCancel={onClose}
          onSave={() => formRef.current?.requestSubmit()}
          saving={saving}
          saveLabel="Hochladen"
        />
      }
    >
      <form
        ref={formRef}
        noValidate
        className="space-y-4"
        onSubmit={(event) => {
          event.preventDefault();
          void submit();
        }}
      >
        <input
          ref={picker}
          id="traeger-document-file"
          name="file"
          type="file"
          accept={TRAEGER_DOCUMENT_ACCEPT}
          aria-describedby="traeger-document-file-hint"
          aria-invalid={fileError ? true : undefined}
          className="peer sr-only"
          onChange={(event) => {
            const picked = event.currentTarget.files?.[0];
            event.currentTarget.value = "";
            choose(picked);
          }}
        />
        {file ? (
          <div className="flex items-center gap-3 rounded-xl border border-gray-200 px-4 py-3">
            <FileTypeIcon
              contentType={file.type}
              className="h-8 w-8 shrink-0 text-gray-500"
            />
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium text-gray-900">
                {file.name}
              </p>
              <p className="text-xs text-gray-500">
                {documentKindLabel(file.type)} · {formatBytes(file.size)}
              </p>
            </div>
            <Button
              type="button"
              variant="ghost"
              size="compact"
              onClick={() => picker.current?.click()}
            >
              Andere Datei
            </Button>
          </div>
        ) : (
          <label
            htmlFor="traeger-document-file"
            onDragOver={(event) => {
              event.preventDefault();
              setDragging(true);
            }}
            onDragLeave={() => setDragging(false)}
            onDrop={(event) => {
              event.preventDefault();
              setDragging(false);
              choose(event.dataTransfer.files[0]);
            }}
            className={cn(
              "flex w-full cursor-pointer flex-col items-center gap-2 rounded-xl border-2 border-dashed px-6 py-8 text-center transition-colors peer-focus-visible:ring-2 peer-focus-visible:ring-gray-400",
              dragging
                ? "border-gray-500 bg-gray-50"
                : "border-gray-200 hover:border-gray-400 hover:bg-gray-50",
              fileError && "border-moto-red/60",
            )}
          >
            <UploadSimpleIcon
              className="h-8 w-8 text-gray-400"
              aria-hidden="true"
            />
            <span className="text-sm font-medium text-gray-900">
              Datei hierher ziehen oder auswählen
            </span>
            <span
              id="traeger-document-file-hint"
              className="text-xs text-gray-500"
            >
              PDF, Word, Excel, PowerPoint oder Bild, höchstens 10 MB
            </span>
          </label>
        )}
        {fileError && (
          <p className="text-moto-red -mt-2 text-xs">{fileError}</p>
        )}
        <Input
          id="traeger-document-title"
          name="title"
          label="Titel"
          required
          maxLength={TITLE_MAX_LENGTH}
          value={title}
          onChange={(event) => setTitle(event.target.value)}
          error={formErrors.fieldError("title")}
          placeholder="z. B. Hausordnung 2026/27"
        />
        <p className="text-sm text-gray-600">
          Eltern und Schulen sehen das Dokument erst, wenn Sie es freigeben. Den
          Titel sehen alle, die das Dokument bekommen.
        </p>
      </form>
    </FormModal>
  );
}

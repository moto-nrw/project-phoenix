"use client";

// Ein vorhandenes Dokument hochladen, damit der Träger es freigeben kann
// (#4692). Freigegeben ist es danach noch nicht: das geschieht auf der Seite
// des Dokuments.

import { useEffect, useRef, useState } from "react";
import { Button } from "~/components/ui/button";
import { Dropzone } from "~/components/ui/dropzone";
import { EditActions } from "~/components/ui/edit-actions";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { FileTypeIcon } from "~/components/ui/file-type-icon";
import { FormModal } from "~/components/ui/form-modal";
import { useFormModalPending } from "~/components/ui/use-form-modal-pending";
import { InsetPanel } from "~/components/ui/inset-panel";
import { Input } from "~/components/ui/input";
import { useApiFormError } from "~/contexts/ToastContext";
import { formatBytes } from "~/lib/file-format";
import { createLogger } from "~/lib/logger";
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
  const save = useFormModalPending(isOpen);
  const saving = save.pending;
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors } = formErrors;

  useEffect(() => {
    if (!isOpen) return;
    setFile(null);
    setTitle("");
    clearErrors();
  }, [isOpen, clearErrors]);

  const choose = (picked: File) => {
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
    await save.run(async (op) => {
      try {
        const result = await uploadTraegerDocument(trimmed, file);
        if (op.isCurrent()) onUploaded(result);
      } catch (err) {
        logger.error("traeger_document_upload_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (!op.isCurrent()) return;
        void formErrors.show(err, { object: "das Dokument" });
      }
    });
  };

  const fileError = formErrors.fieldError("file");
  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      pending={save}
      isBackdropDismissDisabled
      title="Dokument hochladen"
      titleLevel={2}
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
        {file ? (
          <InsetPanel
            tone="white"
            padding="md"
            className="flex items-center gap-3"
          >
            <FileTypeIcon
              contentType={file.type}
              className="h-8 w-8 shrink-0 text-gray-500"
            />
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium text-gray-900">
                {file.name}
              </p>
              <p className="text-sm text-gray-500">
                {documentKindLabel(file.type)} · {formatBytes(file.size)}
              </p>
            </div>
            <Button
              type="button"
              variant="ghost"
              size="compact"
              onClick={() => setFile(null)}
            >
              Andere Datei
            </Button>
          </InsetPanel>
        ) : (
          <Dropzone
            accept={TRAEGER_DOCUMENT_ACCEPT}
            legend="Bereich zum Hochladen eines Dokuments"
            label="Datei hierher ziehen"
            dragLabel="Datei hier ablegen"
            buttonLabel="Datei auswählen"
            inputLabel="Dokument auswählen"
            hint={
              "PDF, Word, Excel, PowerPoint oder Bild, höchstens\u00A010\u00A0MB"
            }
            disabled={saving}
            onFile={choose}
          />
        )}
        {/* Die Drop-Zone besitzt ihr Dateifeld; der Grund einer abgelehnten
            Datei wird deshalb beim Erscheinen vorgelesen. */}
        <FieldErrorHint
          id="traeger-document-file-error"
          message={fileError}
          live
        />
        <Input
          controlSize="touch"
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

"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { Plus, X } from "lucide-react";
import { Button } from "~/components/ui/button";
import { FieldLabel } from "~/components/ui/field-label";
import { FileTypeIcon } from "~/components/ui/file-type-icon";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { useApiFormError } from "~/contexts/ToastContext";
import type { FormField } from "~/lib/enrollment-form-schema-api";
import {
  acceptFor,
  asUploadValues,
  effectiveFileFieldOptions,
  FILE_TYPE_LABEL,
  FILE_TYPE_MIME,
  formatUploadSize,
  maxFilesFor,
  uploadEnrollmentFile,
  uploadValueKey,
  type EnrollmentFileType,
  type FileFieldOptions,
  type NewUploadValue,
  type UploadValue,
} from "~/lib/enrollment-uploads";

type Translate = (
  key: string,
  values?: Record<string, string | number | Date>,
) => string;

/**
 * Where a file field uploads to. The enrollment form provides it; without a
 * phase (editor preview) the control shows itself but uploads nothing. A form
 * outside a school, like the holiday registration of a Träger (#5238), brings
 * its own `upload` instead of a phase.
 */
interface EnrollmentUploadTarget {
  readonly tenantSlug: string;
  readonly phaseId?: string;
  readonly lateInviteToken?: string;
  readonly statusToken?: string;
  readonly uploadEndpoint?: string;
  readonly upload?: (
    file: File,
    signal: AbortSignal,
  ) => Promise<NewUploadValue>;
  readonly locale: string;
  /** Called with +1 when an upload starts and -1 when it settles. */
  readonly onPendingChange: (delta: number) => void;
}

const EnrollmentUploadContext = createContext<EnrollmentUploadTarget | null>(
  null,
);

/**
 * The upload target of a form, plus the count of running uploads, so the
 * form can hold back the submit until every chosen file has arrived.
 */
export function useEnrollmentUploadTarget(params: {
  tenantSlug: string;
  phaseId?: string;
  lateInviteToken?: string;
  statusToken?: string;
  uploadEndpoint?: string;
  upload?: EnrollmentUploadTarget["upload"];
  locale: string;
  previewMode: boolean;
}): { pendingUploads: number; uploadTarget: EnrollmentUploadTarget } {
  const [pendingUploads, setPendingUploads] = useState(0);
  const onPendingChange = useCallback(
    (delta: number) => setPendingUploads((count) => Math.max(0, count + delta)),
    [],
  );
  const {
    tenantSlug,
    phaseId,
    lateInviteToken,
    statusToken,
    uploadEndpoint,
    upload,
    locale,
    previewMode,
  } = params;
  const uploadTarget = useMemo<EnrollmentUploadTarget>(
    () => ({
      tenantSlug,
      phaseId: previewMode ? undefined : phaseId,
      lateInviteToken,
      statusToken,
      uploadEndpoint,
      upload: previewMode ? undefined : upload,
      locale,
      onPendingChange,
    }),
    [
      tenantSlug,
      phaseId,
      lateInviteToken,
      statusToken,
      uploadEndpoint,
      upload,
      previewMode,
      locale,
      onPendingChange,
    ],
  );
  return { pendingUploads, uploadTarget };
}

/** Makes the form's upload target available to its file fields. */
export function EnrollmentUploadScope({
  target,
  children,
}: Readonly<{ target: EnrollmentUploadTarget; children: ReactNode }>) {
  return (
    <EnrollmentUploadContext.Provider value={target}>
      {children}
    </EnrollmentUploadContext.Provider>
  );
}

/** True when a required file field has no file. */
export function fileValueMissing(value: unknown): boolean {
  return asUploadValues(value).length === 0;
}

const EXTENSIONS: Record<EnrollmentFileType, readonly string[]> = {
  pdf: [".pdf"],
  jpg: [".jpg", ".jpeg"],
  png: [".png"],
};

/** The file kind of a picked file, by its type or, without one, its name. */
function fileKind(file: File): EnrollmentFileType | null {
  for (const [kind, mime] of Object.entries(FILE_TYPE_MIME) as Array<
    [EnrollmentFileType, string]
  >) {
    if (file.type === mime) return kind;
  }
  if (file.type !== "") return null;
  const name = file.name.toLowerCase();
  for (const [kind, extensions] of Object.entries(EXTENSIONS) as Array<
    [EnrollmentFileType, readonly string[]]
  >) {
    if (extensions.some((ext) => name.endsWith(ext))) return kind;
  }
  return null;
}

/** The first local reason a file cannot be uploaded, or null. */
function localFileProblem(
  file: File,
  options: FileFieldOptions,
  tr: Translate,
  typeList: string,
): string | null {
  const kind = fileKind(file);
  if (!kind || !options.allowed_types.includes(kind)) {
    return tr("upload.wrongType", { types: typeList });
  }
  if (file.size > options.max_size_mb * 1024 * 1024) {
    return tr("upload.tooLarge", { max: options.max_size_mb });
  }
  if (file.size === 0) {
    return tr("upload.empty");
  }
  return null;
}

interface PendingFile {
  readonly id: number;
  readonly name: string;
}

/**
 * The parent-facing control of a `file` field (#4783). A picked file is
 * uploaded at once; the field's answer lists the uploaded files with their
 * single-use tokens (or, when editing, the files stored before).
 */
export function FileUploadInput({
  field,
  inputName,
  value,
  onChange,
  error,
  tr,
}: Readonly<{
  field: FormField;
  inputName: string;
  value: unknown;
  onChange: (value: UploadValue[]) => void;
  error?: string;
  tr: Translate;
}>) {
  const target = useContext(EnrollmentUploadContext);
  const locale = target?.locale ?? "de";
  const options = useMemo(
    () => effectiveFileFieldOptions(field.file_options),
    [field.file_options],
  );
  const files = asUploadValues(value);
  const maxFiles = maxFilesFor(options);
  const [pending, setPending] = useState<PendingFile[]>([]);
  const picker = useRef<HTMLInputElement>(null);
  const uploadError = useApiFormError();
  // Several uploads can finish in any order: each one adds to the latest
  // answer, not to the one its render saw.
  const latest = useRef<UploadValue[]>(files);
  useEffect(() => {
    latest.current = files;
  }, [files]);
  const nextPendingId = useRef(0);
  const selectionOrder = useRef(new Map<string, number>());
  const controllers = useRef(new Set<AbortController>());

  useEffect(() => {
    const running = controllers.current;
    return () => {
      for (const controller of running) controller.abort();
    };
  }, []);

  const typeList = options.allowed_types
    .map((type) => FILE_TYPE_LABEL[type])
    .join(", ");
  const canUpload = Boolean(target?.upload ?? target?.phaseId);
  const remaining = maxFiles - files.length - pending.length;
  const hintId = `${inputName}-hint`;
  const errorId = `${inputName}-error`;

  const uploadOne = async (file: File) => {
    if (!target || !canUpload) return;
    const id = nextPendingId.current++;
    const controller = new AbortController();
    controllers.current.add(controller);
    setPending((prev) => [...prev, { id, name: file.name }]);
    target.onPendingChange(1);
    try {
      const uploaded = target.upload
        ? await target.upload(file, controller.signal)
        : await uploadEnrollmentFile({
            tenantSlug: target.tenantSlug,
            phaseId: target.phaseId ?? "",
            fieldKey: field.key,
            file,
            lateInviteToken: target.lateInviteToken,
            statusToken: target.statusToken,
            uploadEndpoint: target.uploadEndpoint,
            signal: controller.signal,
          });
      selectionOrder.current.set(uploadValueKey(uploaded), id);
      // Stored files stay first; new files follow their selection order,
      // independent of which concurrent request finished first.
      const next = [...latest.current, uploaded].sort(
        (a, b) =>
          (selectionOrder.current.get(uploadValueKey(a)) ?? -1) -
          (selectionOrder.current.get(uploadValueKey(b)) ?? -1),
      );
      latest.current = next;
      onChange(next);
    } catch (cause) {
      // The field left the form (hidden or closed); nobody waits for it.
      if (controller.signal.aborted) return;
      await uploadError.show(cause, { object: tr("upload.errorObject") });
    } finally {
      controllers.current.delete(controller);
      setPending((prev) => prev.filter((entry) => entry.id !== id));
      target.onPendingChange(-1);
    }
  };

  const handlePicked = (picked: File[]) => {
    uploadError.clear();
    if (picked.length === 0) return;
    if (picked.length > remaining) {
      uploadError.invalid(
        maxFiles === 1
          ? tr("upload.onlyOne")
          : tr("upload.tooMany", { count: maxFiles }),
      );
      return;
    }
    let firstProblem: string | null = null;
    for (const file of picked) {
      const problem = localFileProblem(file, options, tr, typeList);
      if (problem) {
        firstProblem ??= problem;
        continue;
      }
      void uploadOne(file);
    }
    if (firstProblem) uploadError.invalid(firstProblem);
  };

  const dropFile = (key: string) => {
    uploadError.clear();
    const next = latest.current.filter(
      (entry) => uploadValueKey(entry) !== key,
    );
    selectionOrder.current.delete(key);
    latest.current = next;
    onChange(next);
  };

  const hints = [
    tr("upload.allowed", { types: typeList, max: options.max_size_mb }),
    options.multiple ? tr("upload.upTo", { count: maxFiles }) : null,
    options.allowed_types.some((type) => type !== "pdf")
      ? tr("upload.camera")
      : null,
  ].filter(Boolean);

  return (
    <div>
      <FieldLabel
        required={field.required}
        invalid={Boolean(error)}
        className="mb-0"
      >
        {field.label}
      </FieldLabel>
      {field.help_text && (
        <span className="mt-1 block text-xs leading-5 text-gray-500">
          {field.help_text}
        </span>
      )}

      {files.length > 0 || pending.length > 0 ? (
        <ul className="mt-2 flex flex-wrap gap-2" aria-live="polite">
          {files.map((entry) => (
            <li
              key={uploadValueKey(entry)}
              className="inline-flex max-w-full items-center gap-2 rounded-lg border border-gray-200 bg-gray-50 py-1.5 pr-1 pl-2.5 text-sm text-gray-800"
            >
              <FileTypeIcon
                contentType={entry.content_type}
                className="h-4 w-4 shrink-0"
              />
              <span className="min-w-0 truncate">{entry.filename}</span>
              <span className="shrink-0 text-xs text-gray-500">
                {formatUploadSize(entry.size_bytes, locale)}
              </span>
              <button
                type="button"
                onClick={() => dropFile(uploadValueKey(entry))}
                aria-label={tr("upload.remove", { name: entry.filename })}
                className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-gray-500 transition-colors hover:bg-gray-200 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
              >
                <X className="h-4 w-4" aria-hidden="true" />
              </button>
            </li>
          ))}
          {pending.map((entry) => (
            <li
              key={`pending-${entry.id}`}
              className="inline-flex max-w-full items-center gap-2 rounded-lg border border-dashed border-gray-300 px-2.5 py-1.5 text-sm text-gray-500"
            >
              <span className="min-w-0 truncate">{entry.name}</span>
              <span className="shrink-0 text-xs">{tr("upload.uploading")}</span>
            </li>
          ))}
        </ul>
      ) : null}

      {/* The form's focus jump finds a control by its name; for a hidden
          input it moves on to the button beside it. */}
      <input type="hidden" name={inputName} value={String(files.length)} />
      <div className="mt-2">
        {remaining > 0 ? (
          <Button
            type="button"
            variant="outline"
            size="md"
            className="max-w-full gap-2"
            disabled={!canUpload}
            onClick={() => picker.current?.click()}
            aria-describedby={
              [hintId, error ? errorId : null].filter(Boolean).join(" ") ||
              undefined
            }
          >
            <Plus className="h-4 w-4 shrink-0" aria-hidden="true" />
            {files.length > 0 ? tr("upload.chooseMore") : tr("upload.choose")}
          </Button>
        ) : null}
      </div>
      <p id={hintId} className="mt-1 text-xs leading-5 text-gray-500">
        {canUpload ? hints.join(" ") : tr("upload.previewDisabled")}
      </p>
      <input
        ref={picker}
        type="file"
        accept={acceptFor(options)}
        multiple={options.multiple}
        aria-label={field.label}
        hidden
        disabled={!canUpload}
        onChange={(event) => {
          const picked = Array.from(event.currentTarget.files ?? []);
          event.currentTarget.value = "";
          handlePicked(picked);
        }}
      />
      <FormErrorAlert message={uploadError.error} className="mt-2" />
      {error && (
        <p id={errorId} className="text-moto-red mt-1 text-xs">
          {error}
        </p>
      )}
    </div>
  );
}

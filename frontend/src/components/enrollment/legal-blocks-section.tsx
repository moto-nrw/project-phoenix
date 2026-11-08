"use client";

import {
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type Dispatch,
  type ReactNode,
  type SetStateAction,
} from "react";
import {
  ExternalLink,
  FileText,
  FileUp,
  Pencil,
  Plus,
  Trash2,
} from "lucide-react";
import {
  AttachmentEditor,
  type AttachmentUploadAdapter,
} from "~/components/enrollment/attachment-editor";
import { useDraftDocumentLifecycle } from "~/components/enrollment/draft-documents";
import {
  FieldErrorContext,
  fieldErrorProps,
} from "~/components/enrollment/form-field-builder";
import { Alert } from "~/components/ui/alert";
import { SectionCard } from "~/components/ui/section-card";
import { Button, buttonClassName } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { EmptyState } from "~/components/ui/empty-state";
import { SpinnerIcon } from "~/components/ui/icons";
import { Input } from "~/components/ui/input";
import { InsetPanel } from "~/components/ui/inset-panel";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { StatusBadge } from "~/components/ui/status-badge";
import { StatusColorBadge } from "~/components/ui/status-color-badge";
import { Switch } from "~/components/ui/switch";
import { Textarea } from "~/components/ui/textarea";
import { useApiErrorDisplay, useToast } from "~/contexts/ToastContext";
import {
  MAX_ENROLLMENT_DOCUMENT_SIZE_LABEL,
  enrollmentDocumentProblem,
  uploadEnrollmentLegalDocument,
  type FormLegalBlock,
  type LegalBlockDisplayMode,
} from "~/lib/enrollment-form-schema-api";
import {
  LEGAL_BLOCK_DISPLAY_MODE_PDF,
  LEGAL_BLOCK_DISPLAY_MODE_TEXT,
  checkboxLegalBlockKindFor,
  customCheckboxKind,
  hasStandardLegalBlockOverride,
  isStandardLegalBlock,
  legalBlockDisplayMode,
  nextCustomLegalBlockKey,
  publicAGBDocumentURL,
} from "~/lib/enrollment-legal-blocks";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "LegalBlocksSection" });

/**
 * Props traeger/holiday-form-editor.tsx still passes. The draft-document
 * context carries the same lifecycle, so the section ignores them; they go
 * once that caller stops passing them.
 */
interface LegacyDraftDocumentProps {
  /** @deprecated Ignored; the draft-document context tracks drafts. */
  readonly draftDocumentURLs?: ReadonlySet<string>;
  /** @deprecated Ignored; see DraftDocumentLifecycle.remember. */
  readonly onDraftDocumentUploaded?: (documentURL: string) => void;
  /** @deprecated Ignored; see DraftDocumentLifecycle.discard. */
  readonly onDraftDocumentDeleted?: (documentURL: string) => void;
  /** @deprecated Ignored; see DraftDocumentLifecycle.begin. */
  readonly onUploadStart?: () => void;
  /** @deprecated Ignored; see DraftDocumentLifecycle.end. */
  readonly onUploadEnd?: () => void;
}

export function LegalBlocksSection({
  blocks,
  standardBlocks,
  customOnly = false,
  uploadAdapter,
  onChange,
  disabled,
}: Readonly<
  {
    blocks: FormLegalBlock[];
    standardBlocks: FormLegalBlock[];
    customOnly?: boolean;
    uploadAdapter?: AttachmentUploadAdapter;
    onChange: Dispatch<SetStateAction<FormLegalBlock[]>>;
    disabled: boolean;
  } & LegacyDraftDocumentProps
>) {
  const lifecycle = useDraftDocumentLifecycle();
  const toast = useToast();
  const { show: showActionError, showMessage } = useApiErrorDisplay();
  const fieldError = useContext(FieldErrorContext);
  const isStandard = (block: FormLegalBlock) =>
    isStandardLegalBlock(block, standardKeys);
  const [editingStandardKeys, setEditingStandardKeys] = useState<string[]>([]);
  const [uploadingDocumentKey, setUploadingDocumentKey] = useState<
    string | null
  >(null);
  const blocksRef = useRef(blocks);
  useEffect(() => {
    blocksRef.current = blocks;
  }, [blocks]);
  const standardByKey = useMemo(
    () => new Map(standardBlocks.map((block) => [block.key, block])),
    [standardBlocks],
  );
  const standardKeys = useMemo(
    () => new Set(standardByKey.keys()),
    [standardByKey],
  );

  const updateBlock = (index: number, patch: Partial<FormLegalBlock>) => {
    onChange((currentBlocks) =>
      currentBlocks.map((block, i) => {
        if (i !== index) return block;
        const next = { ...block, ...patch };
        if (
          patch.enabled === undefined &&
          patch.text !== undefined &&
          patch.text.trim() !== ""
        ) {
          next.enabled = true;
        }
        return next;
      }),
    );
  };
  const resetStandardBlock = (index: number, key: string) => {
    const standardBlock = standardByKey.get(key);
    if (!standardBlock) return;
    onChange((currentBlocks) =>
      currentBlocks.map((block, i) => (i === index ? standardBlock : block)),
    );
    setEditingStandardKeys((keys) => keys.filter((value) => value !== key));
  };
  const editStandardBlock = (key: string) => {
    setEditingStandardKeys((keys) =>
      keys.includes(key) ? keys : [...keys, key],
    );
  };
  const uploadAGBDocument = async (index: number, file: File | null) => {
    if (!file) return;
    const problem = enrollmentDocumentProblem(file);
    if (problem === "not_pdf") {
      showMessage("Bitte wählen Sie eine PDF-Datei aus.");
      return;
    }
    if (problem === "too_large") {
      showMessage(
        `Die Datei ist größer als ${MAX_ENROLLMENT_DOCUMENT_SIZE_LABEL}. Bitte wählen Sie eine kleinere Datei.`,
      );
      return;
    }
    setUploadingDocumentKey(blocks[index]?.key ?? null);
    lifecycle.begin();
    try {
      const documentURL = await uploadEnrollmentLegalDocument(file);
      const previousURL = (blocksRef.current[index]?.document_url ?? "").trim();
      onChange((currentBlocks) => {
        return currentBlocks.map((block, i) =>
          i === index
            ? {
                ...block,
                document_url: documentURL,
                display_mode: LEGAL_BLOCK_DISPLAY_MODE_PDF,
                enabled: true,
              }
            : block,
        );
      });
      lifecycle.remember(documentURL);
      if (previousURL && previousURL !== documentURL) {
        try {
          await lifecycle.discard?.(previousURL);
        } catch (error) {
          // The URL stays tracked: the next save or discard tries again.
          logger.warn("previous_legal_document_cleanup_failed", {
            error: error instanceof Error ? error.message : String(error),
          });
          void showActionError(error, { object: "die vorherige PDF-Datei" });
        }
      }
      toast.success("Die PDF-Datei für die AGB ist hochgeladen.");
    } catch (error) {
      logger.error("legal_document_upload_failed", {
        error: error instanceof Error ? error.message : String(error),
      });
      // Kein Wiederholen: die Datei muss neu gewählt werden.
      void showActionError(error, { object: "die PDF-Datei" });
    } finally {
      setUploadingDocumentKey(null);
      lifecycle.end();
    }
  };
  const removeAGBDocument = async (index: number) => {
    const documentURL = (blocks[index]?.document_url ?? "").trim();
    updateBlock(index, {
      document_url: "",
      display_mode: LEGAL_BLOCK_DISPLAY_MODE_TEXT,
    });
    if (!documentURL) return;
    try {
      await lifecycle.discard?.(documentURL);
    } catch (error) {
      // The URL stays tracked: the next save or discard tries again.
      logger.warn("legal_document_cleanup_failed", {
        error: error instanceof Error ? error.message : String(error),
      });
      void showActionError(error, { object: "die PDF-Datei" });
    }
  };
  const addCustomBlock = () => {
    onChange((currentBlocks) => [
      ...currentBlocks,
      {
        key: nextCustomLegalBlockKey(currentBlocks),
        kind: "consent",
        title: "Weitere Einwilligung",
        label: "Ich stimme dieser Einwilligung zu.",
        text: "",
        required: false,
        enabled: true,
        sort_order: currentBlocks.length * 10 + 10,
        source: "custom",
      },
    ]);
  };
  const removeBlock = (index: number) => {
    onChange((currentBlocks) => currentBlocks.filter((_, i) => i !== index));
  };
  const standardEntries = blocks
    .map((block, index) => ({ block, index }))
    .filter(({ block }) => isStandard(block));
  const customEntries = blocks
    .map((block, index) => ({ block, index }))
    .filter(({ block }) => !isStandard(block));
  const renderBlockEditor = ({
    block,
    index,
    helperText,
    onRemove,
    onReset,
  }: {
    block: FormLegalBlock;
    index: number;
    helperText: string;
    onRemove?: () => void;
    onReset?: () => void;
  }) => {
    const titleError = fieldErrorProps(
      fieldError,
      `legal_blocks.${index}.title`,
    );
    const labelError = fieldErrorProps(
      fieldError,
      `legal_blocks.${index}.label`,
    );
    return (
      <div
        key={`${block.key}-${index}-editor`}
        className="moto-content-surface rounded-xl border p-4"
      >
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <InlineCheckbox
            checked={block.enabled}
            disabled={disabled}
            onCheckedChange={(checked) =>
              updateBlock(index, { enabled: checked })
            }
          >
            Im Formular anzeigen
          </InlineCheckbox>
          <p className="max-w-xl text-xs leading-5 text-gray-500">
            {helperText}
          </p>
          <div className="flex flex-wrap gap-2">
            {onReset ? (
              <Button
                type="button"
                onClick={onReset}
                disabled={disabled}
                variant="ghost"
                size="compact"
                className="w-fit"
              >
                Einstellungen wieder verwenden
              </Button>
            ) : null}
            {onRemove ? (
              <Button
                type="button"
                onClick={onRemove}
                disabled={disabled}
                variant="ghost_danger"
                size="compact"
                className="w-fit gap-2"
              >
                <Trash2 className="h-4 w-4" aria-hidden="true" />
                Entfernen
              </Button>
            ) : null}
          </div>
        </div>

        <div className="mt-3">
          <Input
            id={titleError.controlId}
            name={titleError.controlProps.name}
            type="text"
            label="Titel"
            controlSize="compact"
            value={block.title}
            disabled={disabled}
            onChange={(event) =>
              updateBlock(index, { title: event.target.value })
            }
            error={titleError.message}
          />
        </div>

        <div className="mt-3">
          <Textarea
            id={`legal-block-label-${block.key}`}
            name={labelError.controlProps.name}
            error={labelError.message}
            label={
              customOnly
                ? "Text neben dem Häkchen oder Hinweis"
                : "Text neben der Checkbox oder dem Hinweis"
            }
            value={block.label}
            disabled={disabled}
            rows={2}
            onChange={(event) =>
              updateBlock(index, { label: event.target.value })
            }
          />
        </div>

        {block.key === "agb" && isStandard(block) ? (
          <AGBTemplateSourceEditor
            mode={legalBlockDisplayMode(block)}
            onModeChange={(mode) => updateBlock(index, { display_mode: mode })}
            textValue={block.text}
            onTextChange={(value) => updateBlock(index, { text: value })}
            documentURL={block.document_url ?? ""}
            documentSaving={uploadingDocumentKey === block.key}
            disabled={disabled}
            onDocumentUpload={(file) => uploadAGBDocument(index, file)}
            onDocumentRemove={() => void removeAGBDocument(index)}
          />
        ) : (
          <div className="mt-3">
            <Textarea
              id={`legal-block-text-${block.key}`}
              label="Rechtstext / Erklärung"
              value={block.text}
              disabled={disabled}
              rows={4}
              onChange={(event) =>
                updateBlock(index, { text: event.target.value })
              }
            />
          </div>
        )}

        <div className="mt-3">
          <AttachmentEditor
            uploadAdapter={uploadAdapter}
            fieldPath={`legal_blocks.${index}.attachments`}
            fieldError={fieldError}
            attachments={block.attachments}
            onChange={(attachments) =>
              updateBlock(index, {
                attachments,
                ...(customOnly &&
                block.display_mode === "pdf" &&
                attachments.length === 0
                  ? { display_mode: "text" as const }
                  : {}),
              })
            }
            disabled={disabled}
          />
        </div>
        <div className="mt-3 flex flex-wrap items-center gap-3">
          <LegalBlockModeControl
            checkboxLabel={customOnly ? "Mit Häkchen" : "Mit Checkbox"}
            isNotice={block.kind === "notice"}
            disabled={disabled}
            onModeChange={(isNotice) =>
              updateBlock(index, {
                kind: isNotice ? "notice" : checkboxLegalBlockKindFor(block),
                required: isNotice ? false : block.required,
              })
            }
          />
          {block.kind !== "notice" &&
          !(block.kind === "consent" && isStandard(block)) ? (
            <InlineCheckbox
              checked={block.required}
              disabled={disabled}
              muted
              onCheckedChange={(checked) =>
                updateBlock(
                  index,
                  isStandard(block)
                    ? { required: checked }
                    : { required: checked, kind: customCheckboxKind(checked) },
                )
              }
            >
              Muss bestätigt werden
            </InlineCheckbox>
          ) : null}
        </div>
        {block.kind === "consent" ? (
          <p className="mt-2 text-xs leading-5 text-gray-500">
            Freiwillige Einwilligung: Eltern können das Formular auch ohne
            Häkchen abschicken.
          </p>
        ) : null}
        {block.kind === "terms" && !isStandard(block) ? (
          <p className="mt-2 text-xs leading-5 text-gray-500">
            Pflicht-Bestätigung: Eltern müssen das Häkchen setzen, sonst können
            sie das Formular nicht abschicken. Das ist keine freiwillige
            Einwilligung.
          </p>
        ) : null}
      </div>
    );
  };

  return (
    <SectionCard>
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h2 className="text-base font-semibold text-gray-900">
            {customOnly
              ? "Hinweise und Einwilligungen"
              : "Rechtstexte und Einwilligungen"}
          </h2>
          <p className="mt-1 max-w-2xl text-sm leading-6 text-gray-600">
            {customOnly
              ? "Hier ergänzen Sie Hinweise, PDFs und freiwillige Einwilligungen. Die festen Zustimmungen bleiben bestehen."
              : "Die vier Standardblöcke starten mit den Texten aus den Einstellungen. Hier blenden Sie sie für diese Vorlage aus oder bearbeiten bewusst eine Abweichung. Eigene Zustimmungen bleiben direkt an diese Vorlage gebunden."}
          </p>
        </div>
      </div>

      {!customOnly &&
      blocks.some((block) => block.enabled) &&
      !blocks.some(
        (block) => block.key === "data_processing" && block.enabled,
      ) ? (
        <div className="mt-3">
          <Alert
            type="warning"
            announce="off"
            message="Die Datenschutzinformation ist in dieser Vorlage deaktiviert. Stellen Sie sicher, dass Eltern die Datenschutzhinweise auf anderem Weg erhalten, zum Beispiel über den Elternbrief."
          />
        </div>
      ) : null}

      <div className="mt-4 space-y-3">
        {standardEntries.map(({ block, index }) => {
          const standardBlock = standardByKey.get(block.key);
          const hasOverride = standardBlock
            ? hasStandardLegalBlockOverride(block, standardBlock)
            : false;
          const isEditing = editingStandardKeys.includes(block.key);
          if (isEditing) {
            return renderBlockEditor({
              block,
              index,
              helperText:
                "Abweichung nur für diese Vorlage. Mit Zurücksetzen nutzt der Block wieder die Einstellungen.",
              onReset: () => resetStandardBlock(index, block.key),
            });
          }
          return (
            <div
              key={`${block.key}-${index}`}
              className="moto-content-surface rounded-xl border p-4"
            >
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <h3 className="text-sm font-semibold text-gray-900">
                      {block.title}
                    </h3>
                    <StatusBadge
                      tone="gray"
                      compact
                      wrap
                      label="Aus Einstellungen übernommen"
                    />
                    {hasOverride ? (
                      <StatusColorBadge
                        color={LOCATION_COLORS.WARNING}
                        compact
                        label="Abweichung aktiv"
                      />
                    ) : null}
                  </div>
                  <p className="mt-1 text-sm leading-6 text-gray-600">
                    {block.enabled
                      ? "Wird in dieser Vorlage angezeigt."
                      : "Ist für diese Vorlage ausgeblendet."}
                  </p>
                  {block.label.trim() !== "" ? (
                    <p className="mt-1 line-clamp-2 text-xs leading-5 text-gray-500">
                      {block.label}
                    </p>
                  ) : null}
                </div>
                <div className="flex shrink-0 items-center justify-end gap-1.5">
                  <Switch
                    checked={block.enabled}
                    onCheckedChange={(checked) =>
                      updateBlock(index, { enabled: checked })
                    }
                    disabled={disabled}
                    ariaLabel={`${block.title} in dieser Vorlage anzeigen`}
                  />
                  <OverflowMenu
                    ariaLabel={`Aktionen für ${block.title}`}
                    triggerSize="sm"
                    items={[
                      {
                        label: "Abweichend bearbeiten",
                        icon: <Pencil className="h-4 w-4" aria-hidden="true" />,
                        onClick: () => editStandardBlock(block.key),
                        disabled,
                      },
                    ]}
                  />
                </div>
              </div>
            </div>
          );
        })}

        {customEntries.map(({ block, index }) =>
          renderBlockEditor({
            block,
            index,
            helperText: customOnly
              ? "Eigener Hinweis oder Einwilligung für diesen Zeitraum."
              : "Eigene Zustimmung für diese Vorlage. Sie erscheint als zusätzliche Checkbox im Anmeldeformular.",
            onRemove: () => removeBlock(index),
          }),
        )}

        {customEntries.length === 0 ? (
          <EmptyState
            variant="dashed"
            title={
              customOnly
                ? "Noch keine eigenen Hinweise oder Einwilligungen."
                : "Noch keine eigenen Zustimmungen."
            }
            description={
              customOnly
                ? undefined
                : "Füge hier nur zusätzliche Inhalte hinzu, die nicht in den allgemeinen Einstellungen stehen."
            }
          />
        ) : null}
      </div>

      <div className="mt-4">
        <Button
          type="button"
          onClick={addCustomBlock}
          disabled={disabled}
          variant="outline"
          size="md"
          className="inline-flex items-center justify-center gap-2"
        >
          <Plus className="h-4 w-4" aria-hidden="true" />
          Eigene Zustimmung hinzufügen
        </Button>
      </div>
    </SectionCard>
  );
}

function AGBTemplateSourceEditor({
  mode,
  onModeChange,
  textValue,
  onTextChange,
  documentURL,
  documentSaving,
  disabled,
  onDocumentUpload,
  onDocumentRemove,
}: Readonly<{
  mode: LegalBlockDisplayMode;
  onModeChange: (mode: LegalBlockDisplayMode) => void;
  textValue: string;
  onTextChange: (value: string) => void;
  documentURL: string;
  documentSaving: boolean;
  disabled: boolean;
  onDocumentUpload: (file: File | null) => void;
  onDocumentRemove: () => void;
}>) {
  const hasText = textValue.trim() !== "";
  const hasDocument = documentURL.trim() !== "";
  const documentPicker = useRef<HTMLInputElement>(null);

  return (
    <div className="mt-3 space-y-3">
      <div className="grid gap-3 sm:grid-cols-2">
        <ChoiceTile
          as="button"
          tone="blue"
          selected={mode === LEGAL_BLOCK_DISPLAY_MODE_TEXT}
          onClick={() => onModeChange(LEGAL_BLOCK_DISPLAY_MODE_TEXT)}
          disabled={disabled}
          className="block p-3"
        >
          <span className="flex items-center gap-2 text-sm font-semibold">
            <FileText className="h-4 w-4" aria-hidden="true" />
            Text eingeben
          </span>
          <span className="mt-1 block text-xs text-gray-500">
            Eltern lesen den Text direkt im Formular.
          </span>
          {hasText ? (
            <span className="mt-2 block">
              <StatusBadge tone="outline" compact label="Text gespeichert" />
            </span>
          ) : null}
        </ChoiceTile>
        <ChoiceTile
          as="button"
          tone="blue"
          selected={mode === LEGAL_BLOCK_DISPLAY_MODE_PDF}
          onClick={() => onModeChange(LEGAL_BLOCK_DISPLAY_MODE_PDF)}
          disabled={disabled}
          className="block p-3"
        >
          <span className="flex items-center gap-2 text-sm font-semibold">
            <FileUp className="h-4 w-4" aria-hidden="true" />
            PDF-Datei hochladen
          </span>
          <span className="mt-1 block text-xs text-gray-500">
            Eltern öffnen im Formular einen PDF-Link.
          </span>
          {hasDocument ? (
            <span className="mt-2 block">
              <StatusBadge tone="outline" compact label="PDF gespeichert" />
            </span>
          ) : null}
        </ChoiceTile>
      </div>

      {mode === LEGAL_BLOCK_DISPLAY_MODE_TEXT ? (
        <div>
          <Textarea
            id="agb-template-text"
            label="Rechtstext / Erklärung"
            value={textValue}
            disabled={disabled}
            rows={4}
            onChange={(event) => onTextChange(event.target.value)}
          />
        </div>
      ) : (
        <InsetPanel framed padding="sm">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <p className="text-sm font-medium text-gray-900">PDF-Datei</p>
              <p className="mt-0.5 text-xs text-gray-600">
                {hasDocument
                  ? "Diese PDF wird in dieser Formularvorlage als Link angezeigt."
                  : "Laden Sie die AGB / Teilnahmebedingungen als PDF hoch."}
              </p>
            </div>
            <div className="flex flex-wrap items-center gap-2 text-xs">
              <Button
                type="button"
                variant="surface"
                size="compact"
                disabled={disabled || documentSaving}
                onClick={() => documentPicker.current?.click()}
              >
                <FileUp className="h-3.5 w-3.5" aria-hidden="true" />
                <span>{hasDocument ? "PDF ersetzen" : "PDF hochladen"}</span>
              </Button>
              <input
                ref={documentPicker}
                type="file"
                accept="application/pdf,.pdf"
                aria-label={hasDocument ? "PDF ersetzen" : "PDF hochladen"}
                hidden
                disabled={disabled || documentSaving}
                onChange={(event) => {
                  onDocumentUpload(event.currentTarget.files?.[0] ?? null);
                  event.currentTarget.value = "";
                }}
              />
              {hasDocument ? (
                <a
                  href={publicAGBDocumentURL(documentURL)}
                  target="_blank"
                  rel="noopener noreferrer"
                  className={buttonClassName({
                    variant: "surface",
                    size: "compact",
                    className: "",
                  })}
                >
                  <ExternalLink className="h-3.5 w-3.5" aria-hidden="true" />
                  <span>Öffnen</span>
                </a>
              ) : null}
              {hasDocument ? (
                <Button
                  type="button"
                  onClick={onDocumentRemove}
                  disabled={disabled || documentSaving}
                  variant="outline_danger"
                  size="compact"
                >
                  <Trash2 className="h-3.5 w-3.5" aria-hidden="true" />
                  <span>Entfernen</span>
                </Button>
              ) : null}
            </div>
          </div>
          {documentSaving ? (
            <p className="mt-2 flex items-center gap-2 text-xs text-gray-500">
              <SpinnerIcon className="h-3 w-3" />
              PDF wird hochgeladen...
            </p>
          ) : null}
        </InsetPanel>
      )}
    </div>
  );
}

function LegalBlockModeControl({
  checkboxLabel = "Mit Checkbox",
  isNotice,
  disabled,
  onModeChange,
}: Readonly<{
  checkboxLabel?: string;
  isNotice: boolean;
  disabled: boolean;
  onModeChange: (isNotice: boolean) => void;
}>) {
  return (
    <SegmentedControl
      ariaLabel="Darstellung im Elternformular"
      value={isNotice ? "hinweis" : "checkbox"}
      onChange={(next) => onModeChange(next === "hinweis")}
      items={[
        { value: "checkbox", label: checkboxLabel, disabled },
        { value: "hinweis", label: "Nur Hinweis", disabled },
      ]}
    />
  );
}

function InlineCheckbox({
  checked,
  disabled,
  muted = false,
  onCheckedChange,
  children,
}: Readonly<{
  checked: boolean;
  disabled: boolean;
  muted?: boolean;
  onCheckedChange: (checked: boolean) => void;
  children: ReactNode;
}>) {
  return (
    <label
      className={`inline-flex min-w-0 items-center gap-2 text-left text-sm font-medium ${
        muted ? "text-gray-700" : "text-gray-800"
      } ${disabled ? "cursor-not-allowed" : "cursor-pointer"}`}
    >
      <Checkbox
        checked={checked}
        disabled={disabled}
        onChange={(event) => onCheckedChange(event.target.checked)}
      />
      <span>{children}</span>
    </label>
  );
}

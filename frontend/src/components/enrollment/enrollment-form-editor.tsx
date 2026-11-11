"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { Info, Plus } from "lucide-react";
import {
  DraftDocumentContext,
  useDraftDocuments,
  type DraftDocumentSave,
} from "~/components/enrollment/draft-documents";
import {
  EnrollmentFormsOverview,
  DeleteSchemaDialog,
  RenameSchemaDialog,
} from "~/components/enrollment/enrollment-form-overview";
import {
  createIncomeDeclarationFields,
  hasIncomeDeclaration,
  IncomeDeclarationButton,
} from "~/components/enrollment/file-field-editor";
import {
  FieldEditorRow,
  FieldErrorContext,
} from "~/components/enrollment/form-field-builder";
import {
  BuilderTemplateSummary,
  FormBuilderIntro,
  SignatureRequirementSection,
  TargetSuggestions,
  UnsavedChangesDialog,
  type PendingNavigation,
} from "~/components/enrollment/form-template-builder-sections";
import { CoreFieldsSection } from "~/components/enrollment/form-template-core-fields";
import { FormTemplateDetail } from "~/components/enrollment/form-template-detail";
import {
  coreRequirementsSignature,
  createTargetField,
  getSchemaDraftValidationMessage,
  isRenameOf,
  prepareFieldsForSave,
  type ReservedTarget,
} from "~/components/enrollment/form-template-draft";
import {
  FormPreview,
  openTemplatePreview,
  useTemplatePreviewHref,
} from "~/components/enrollment/form-template-preview";
import {
  schemaTranslationTargets,
  schemaTranslationUpdate,
} from "~/components/enrollment/form-template-translations";
import { LegalBlocksSection } from "~/components/enrollment/legal-blocks-section";
import {
  TranslationsSection,
  type TranslationTarget,
} from "~/components/enrollment/translations-section";
import { BackAction } from "~/components/ui/back-link";
import { Button } from "~/components/ui/button";
import { SectionCard } from "~/components/ui/section-card";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { FormSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
import {
  useApiErrorDisplay,
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import {
  blankField,
  blankInfoField,
  createSchema,
  deleteEnrollmentLegalDocument,
  deleteSchema,
  fetchPublicLegalTexts,
  latestSchemasByName,
  listSchemas,
  renameSchema,
  updateSchema,
  type CoreRequirementKey,
  type CoreRequirements,
  type FormField,
  type FormLegalBlock,
  type FormSchema,
} from "~/lib/enrollment-form-schema-api";
import {
  STANDARD_LEGAL_BLOCKS,
  draftDocumentURLsInLegalBlocks,
  legalBlocksForTemplate,
  legalBlocksSignature,
  mergeStandardLegalBlocks,
  prepareLegalBlocksForSave,
} from "~/lib/enrollment-legal-blocks";
import { listPhases, type Phase } from "~/lib/enrollment-phase-api";
import type { Translations } from "~/lib/enrollment-translations";
import { useRefreshAfterWrite } from "~/lib/hooks/use-refresh-after-write";
import { createLogger } from "~/lib/logger";
import {
  copyStableObjectKey,
  getStableObjectKey,
} from "~/lib/stable-object-key";
import { useTenantSlugSafe } from "~/lib/tenant-context";

const logger = createLogger({ component: "EnrollmentFormEditor" });

const NEW_SCHEMA_VALUE = "__new__";
const LEGAL_DOCUMENT_UPLOAD_PENDING_MESSAGE =
  "Bitte warten Sie, bis die PDF-Datei hochgeladen ist.";

type EditorMode = "overview" | "builder" | "detail";

export function EnrollmentFormEditor({
  onTemplateCountChange,
}: {
  /** Meldet die Zahl der Vorlagen an den Seitenkopf, damit dessen Statuszeile
   *  aus denselben Daten stammt statt aus einem zweiten Request. */
  readonly onTemplateCountChange?: (
    count: number | null,
    loadFailed: boolean,
  ) => void;
} = {}) {
  const toast = useToast();
  const tenantSlug = useTenantSlugSafe();
  const templatePreviewHref = useTemplatePreviewHref();
  const [allSchemas, setAllSchemas] = useState<FormSchema[]>([]);
  const [phases, setPhases] = useState<Phase[]>([]);
  const [selectedKey, setSelectedKey] = useState<string>(NEW_SCHEMA_VALUE);
  const [name, setName] = useState("");
  const [fields, setFields] = useState<FormField[]>([]);
  const [coreRequirements, setCoreRequirements] = useState<CoreRequirements>(
    {},
  );
  const [standardLegalBlocks, setStandardLegalBlocks] = useState<
    FormLegalBlock[]
  >(STANDARD_LEGAL_BLOCKS);
  const [legalBlocks, setLegalBlocks] = useState<FormLegalBlock[]>(
    STANDARD_LEGAL_BLOCKS,
  );
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const builderRef = useRef<HTMLDivElement>(null);
  const fieldListRef = useRef<HTMLDivElement>(null);
  const revealAddedFieldRef = useRef(false);
  const {
    error: loadError,
    show: showLoadError,
    clear: clearLoadError,
  } = useApiLoadError();
  const {
    error: saveError,
    show: showSaveError,
    invalid: markInvalid,
    fieldError,
    clear: clearSaveError,
  } = useApiFormError(builderRef);
  const deleteErrors = useApiFormError();
  const { showMessage } = useApiErrorDisplay();
  const [mode, setMode] = useState<EditorMode>("overview");
  const [pendingNavigation, setPendingNavigation] =
    useState<PendingNavigation | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<FormSchema | null>(null);
  const [deletingSchemaId, setDeletingSchemaId] = useState<string | null>(null);
  const [renameTarget, setRenameTarget] = useState<FormSchema | null>(null);
  const draftDocuments = useDraftDocuments(deleteEnrollmentLegalDocument);
  const unmountedRef = useRef(false);
  useEffect(() => {
    unmountedRef.current = false;
    return () => {
      unmountedRef.current = true;
    };
  }, []);
  const hasPendingLegalDocumentUpload = draftDocuments.uploading;
  // Saving and running uploads lock every control of the builder.
  const locked = saving || hasPendingLegalDocumentUpload;

  const latestByName = useMemo(
    () => latestSchemasByName(allSchemas),
    [allSchemas],
  );

  // The retry of a failed load closes over the latest loadAll.
  const loadAllRef = useRef<() => Promise<FormSchema[]>>(async () => []);
  const loadAll = useCallback(
    async (options: { rethrow?: boolean } = {}) => {
      setLoading(true);
      if (!options.rethrow) clearLoadError();
      try {
        // Legal texts are NOT best-effort: silently swallowing a fetch
        // failure would seed the builder with all-disabled standard blocks
        // and nudge admins into saving a template without the tenant's
        // consent contract. A failure fails the whole load instead. The
        // phases fail it too: without them every template would read as
        // "not used by any phase".
        const [list, phaseList, legalTexts] = await Promise.all([
          listSchemas(),
          listPhases(),
          tenantSlug
            ? fetchPublicLegalTexts(tenantSlug)
            : Promise.resolve(null),
        ]);
        const legalDefaults = mergeStandardLegalBlocks(legalTexts);
        setStandardLegalBlocks(legalDefaults);
        setAllSchemas(list);
        setPhases(phaseList);
        return list;
      } catch (err) {
        logger.error("schema_list_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        // Nach einem Schreiben zeigt der Aufrufer den Fehler als Nachladefehler
        // über den stehenden Vorlagen (#6652).
        if (options.rethrow) throw err;
        void showLoadError(err, {
          object: "die Übersicht der Anmeldeformulare",
          retry: () => void loadAllRef.current(),
        });
        return [];
      } finally {
        setLoading(false);
      }
    },
    [clearLoadError, showLoadError, tenantSlug],
  );
  useLayoutEffect(() => {
    loadAllRef.current = loadAll;
  });

  useEffect(() => {
    void loadAll();
  }, [loadAll]);
  // Nachladen nach Speichern, Löschen oder Umbenennen: ein Fehler dabei ist
  // ein Ladefehler, kein Fehler der Aktion (#6652). Die geladene Liste liest
  // das Speichern danach aus `reloadedSchemas`.
  const reloadedSchemas = useRef<FormSchema[]>([]);
  const schemasAfterWrite = useRefreshAfterWrite(
    "die Anzeige der Anmeldeformulare",
    async () => {
      reloadedSchemas.current = [];
      reloadedSchemas.current = await loadAll({ rethrow: true });
    },
  );

  const selectSchema = (schema: FormSchema, nextMode: EditorMode) => {
    setSelectedKey(schema.id);
    setName(schema.name);
    setFields(schema.fields);
    setCoreRequirements(schema.core_requirements ?? {});
    setLegalBlocks(
      legalBlocksForTemplate(schema.legal_blocks, standardLegalBlocks),
    );
    clearSaveError();
    setMode(nextMode);
  };

  const editSchema = (schema: FormSchema) => selectSchema(schema, "builder");

  const previewSchema = (schema: FormSchema) => selectSchema(schema, "detail");

  const requestRemoveSchema = (schema: FormSchema) => {
    setDeleteTarget(schema);
  };

  const closeDeleteDialog = () => {
    deleteErrors.clear();
    setDeleteTarget(null);
  };

  const confirmRemoveSchema = async () => {
    if (!deleteTarget) return;
    deleteErrors.clear();
    setDeletingSchemaId(deleteTarget.id);
    try {
      await deleteSchema(deleteTarget.id);
    } catch (err) {
      logger.error("schema_delete_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      // The dialog stays open with the reason; its button tries again.
      void deleteErrors.show(err, { object: "die Formularvorlage" });
      setDeletingSchemaId(null);
      return;
    }
    setDeletingSchemaId(null);
    toast.success("Die Formularvorlage ist gelöscht.");
    if (selectedKey === deleteTarget.id) {
      backToOverview();
    }
    closeDeleteDialog();
    await schemasAfterWrite.refresh();
  };

  const requestRenameSchema = (schema: FormSchema) => {
    setRenameTarget(schema);
  };

  // Rename touches every version row of the lineage in one PATCH; it
  // does NOT publish a new version. Throws on failure so the dialog can
  // surface the backend message (e.g. a name already in use).
  const confirmRenameSchema = async (newName: string) => {
    if (!renameTarget) return;
    const updated = await renameSchema(renameTarget.id, newName);
    await schemasAfterWrite.refresh();
    // Keep the builder's name field in sync if this template is open.
    if (selectedKey === renameTarget.id) {
      setName(updated.name);
    }
    toast.success(`Die Formularvorlage heißt jetzt „${updated.name}“.`);
    // Schließen übernimmt der Dialog, nur solange er derselbe ist (#6533).
  };

  const startNew = () => {
    setSelectedKey(NEW_SCHEMA_VALUE);
    setName("");
    setFields([]);
    setCoreRequirements({});
    setLegalBlocks(standardLegalBlocks);
    clearSaveError();
    setMode("builder");
  };

  const backToOverview = () => {
    clearSaveError();
    setMode("overview");
  };

  const updateField = (index: number, patch: Partial<FormField>) => {
    setFields((prev) =>
      prev.map((field, i) =>
        i === index
          ? copyStableObjectKey(field, { ...field, ...patch })
          : field,
      ),
    );
  };

  const applySchemaTranslation = (
    target: TranslationTarget,
    translations: Translations,
  ) => {
    const update = schemaTranslationUpdate(target.id, translations);
    if (update.legalBlocks) setLegalBlocks(update.legalBlocks);
    if (update.fields) setFields(update.fields);
  };

  const updateCoreRequirement = (
    key: CoreRequirementKey,
    required: boolean,
  ) => {
    setCoreRequirements((prev) => ({ ...prev, [key]: required }));
  };

  const addField = () => {
    revealAddedFieldRef.current = true;
    setFields((prev) => [...prev, blankField(prev.length)]);
  };

  const addInfoField = () => {
    revealAddedFieldRef.current = true;
    setFields((prev) => [...prev, blankInfoField(prev.length)]);
  };

  const addIncomeDeclaration = () => {
    if (hasIncomeDeclaration(fields)) return;
    revealAddedFieldRef.current = true;
    setFields((prev) =>
      hasIncomeDeclaration(prev)
        ? prev
        : [...prev, ...createIncomeDeclarationFields(prev.length)],
    );
  };

  const addTargetField = (target: ReservedTarget) => {
    if (!fields.some((field) => field.target === target)) {
      revealAddedFieldRef.current = true;
    }
    setFields((prev) => {
      if (prev.some((field) => field.target === target)) return prev;
      return [...prev, createTargetField(target, prev.length)];
    });
  };

  // A new row lands at the end of the list, often below the fold. Bring it
  // into view and put the cursor into its first text field, so the admin
  // sees what the button did (#4328).
  useEffect(() => {
    if (!revealAddedFieldRef.current) return;
    revealAddedFieldRef.current = false;
    const row = fieldListRef.current?.lastElementChild;
    if (!(row instanceof HTMLElement)) return;
    row
      .querySelector<HTMLInputElement>('input[type="text"]:not(:disabled)')
      ?.focus({ preventScroll: true });
    const reduceMotion =
      globalThis.matchMedia?.("(prefers-reduced-motion: reduce)").matches ??
      false;
    // happy-dom ships no scrollIntoView on elements.
    row.scrollIntoView?.({
      behavior: reduceMotion ? "auto" : "smooth",
      block: "center",
    });
  }, [fields]);

  const removeField = (index: number) => {
    setFields((prev) =>
      prev
        .filter((_, i) => i !== index)
        .map((field, i) =>
          copyStableObjectKey(field, { ...field, sort_order: i }),
        ),
    );
  };

  const moveField = (index: number, direction: -1 | 1) => {
    setFields((prev) => {
      const target = index + direction;
      if (target < 0 || target >= prev.length) return prev;
      const next = [...prev];
      const tmp = next[index];
      next[index] = next[target]!;
      next[target] = tmp!;
      return next.map((field, i) =>
        copyStableObjectKey(field, { ...field, sort_order: i }),
      );
    });
  };

  const isCreating = selectedKey === NEW_SCHEMA_VALUE;
  const currentSchema = isCreating
    ? null
    : (latestByName.find((s) => s.id === selectedKey) ?? null);
  const savedFieldSignature = useMemo(
    () => JSON.stringify(currentSchema?.fields ?? []),
    [currentSchema?.fields],
  );
  const currentFieldSignature = useMemo(() => JSON.stringify(fields), [fields]);
  const savedCoreRequirementSignature = useMemo(
    () => coreRequirementsSignature(currentSchema?.core_requirements ?? {}),
    [currentSchema?.core_requirements],
  );
  const currentCoreRequirementSignature = useMemo(
    () => coreRequirementsSignature(coreRequirements),
    [coreRequirements],
  );
  const savedLegalBlocksSignature = useMemo(
    // Mirror selectSchema's hydration: the backend returns [] for legacy
    // templates, which the editor displays as the standard blocks. The
    // saved signature must apply the same substitution, otherwise every
    // pre-existing template opens with phantom unsaved changes.
    () =>
      legalBlocksSignature(
        legalBlocksForTemplate(
          currentSchema?.legal_blocks,
          standardLegalBlocks,
        ),
      ),
    [currentSchema?.legal_blocks, standardLegalBlocks],
  );
  const currentLegalBlocksSignature = useMemo(
    () => legalBlocksSignature(legalBlocks),
    [legalBlocks],
  );
  const standardLegalBlocksSignature = useMemo(
    () => legalBlocksSignature(standardLegalBlocks),
    [standardLegalBlocks],
  );
  // Split the edit-mode dirty check into its two independent axes so the
  // save path can tell a pure rename (name only) from a content change.
  // A name-only edit must rename in place without publishing a redundant
  // new version (which is what updateSchema does).
  const editNameChanged = name.trim() !== (currentSchema?.name ?? "");
  const editContentChanged =
    currentFieldSignature !== savedFieldSignature ||
    savedCoreRequirementSignature !== currentCoreRequirementSignature ||
    savedLegalBlocksSignature !== currentLegalBlocksSignature;
  const hasUnsavedChanges =
    mode === "builder" &&
    (isCreating
      ? name.trim() !== "" ||
        fields.length > 0 ||
        currentCoreRequirementSignature !== "{}" ||
        currentLegalBlocksSignature !== standardLegalBlocksSignature
      : editNameChanged || editContentChanged);
  const saveBlockedMessage =
    getSchemaDraftValidationMessage({
      fields,
      legalBlocks,
      name,
    })?.message ?? null;
  const pendingUploadMessage = hasPendingLegalDocumentUpload
    ? LEGAL_DOCUMENT_UPLOAD_PENDING_MESSAGE
    : null;

  // "Wiederholen" sends the draft as it is then, not as it was when the
  // save failed.
  const saveSchemaRef = useRef<(nextMode?: EditorMode) => Promise<unknown>>(
    async () => null,
  );
  const saveSchema = async (
    nextMode: EditorMode = "detail",
  ): Promise<FormSchema | null> => {
    setSaving(true);
    clearSaveError();
    let draftSave: DraftDocumentSave | null = null;
    try {
      const validation = getSchemaDraftValidationMessage({
        fields,
        legalBlocks,
        name,
      });
      if (validation) {
        markInvalid(
          validation.message,
          validation.field && validation.hint
            ? { [validation.field]: validation.hint }
            : {},
        );
        return null;
      }
      if (hasPendingLegalDocumentUpload) {
        markInvalid(LEGAL_DOCUMENT_UPLOAD_PENDING_MESSAGE);
        return null;
      }

      const fieldsForSave = prepareFieldsForSave(fields);
      const legalBlocksForSave = prepareLegalBlocksForSave(legalBlocks);
      const drafts = draftDocuments.drafts();
      const referencedDraftDocumentURLs = draftDocumentURLsInLegalBlocks(
        legalBlocksForSave,
        drafts,
      );
      for (const field of fieldsForSave) {
        for (const attachment of field.attachments ?? []) {
          if (drafts.has(attachment.document_url))
            referencedDraftDocumentURLs.add(attachment.document_url);
        }
      }
      draftSave = await draftDocuments.startSave(referencedDraftDocumentURLs);
      let result: FormSchema;
      if (isCreating) {
        result = await createSchema(
          name.trim(),
          fieldsForSave,
          coreRequirements,
          legalBlocksForSave,
        );
      } else if (isRenameOf(currentSchema, name) && !editContentChanged) {
        // Name-only change: rename the lineage in place. We deliberately
        // skip updateSchema here so a pure rename doesn't publish a
        // redundant identical version (and re-point bound phases). This
        // matches the standalone "Umbenennen" dialog's semantics.
        result = await renameSchema(selectedKey, name.trim());
      } else {
        // Combined "rename + edit" save: pass the new name to updateSchema so
        // the backend renames the lineage AND publishes the new version in ONE
        // transaction. A failed publish rolls the rename back, so there is no
        // partial "renamed but content unchanged" state and the local baseline
        // can never drift from the database. renameTo is undefined when only
        // the content changed, leaving the lineage name untouched.
        const renameTo = isRenameOf(currentSchema, name)
          ? name.trim()
          : undefined;
        result = await updateSchema(
          selectedKey,
          fieldsForSave,
          coreRequirements,
          legalBlocksForSave,
          renameTo,
        );
      }
      if (unmountedRef.current) {
        draftSave.commit();
        return result;
      }
      await schemasAfterWrite.refresh();
      const refreshed = reloadedSchemas.current;
      const stillThere = refreshed.find((s) => s.id === result.id);
      const savedSchema = stillThere ?? result;
      draftSave.commit();
      selectSchema(savedSchema, nextMode);
      toast.success(
        isCreating
          ? `Die Formularvorlage „${savedSchema.name}“ ist angelegt.`
          : `Die Formularvorlage „${savedSchema.name}“ ist gespeichert.`,
      );
      return savedSchema;
    } catch (err) {
      // After an unmount nobody is left to read the error; fail() then
      // deletes the uploads of this save.
      draftSave?.fail();
      if (!unmountedRef.current) {
        logger.error("schema_save_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        void showSaveError(err, {
          object: "die Formularvorlage",
          retry: () => void saveSchemaRef.current(nextMode),
        });
      }
      return null;
    } finally {
      if (!unmountedRef.current) setSaving(false);
    }
  };

  useLayoutEffect(() => {
    saveSchemaRef.current = saveSchema;
  });

  const handleSave = async () => {
    await saveSchema("detail");
  };

  const requestBackToOverview = () => {
    if (hasPendingLegalDocumentUpload) {
      showMessage(LEGAL_DOCUMENT_UPLOAD_PENDING_MESSAGE);
      return;
    }
    if (hasUnsavedChanges) {
      setPendingNavigation("overview");
      return;
    }
    backToOverview();
  };

  const requestStartNew = () => {
    if (hasPendingLegalDocumentUpload) {
      showMessage(LEGAL_DOCUMENT_UPLOAD_PENDING_MESSAGE);
      return;
    }
    if (hasUnsavedChanges) {
      setPendingNavigation("new");
      return;
    }
    startNew();
  };

  const requestExternalPreview = () => {
    if (hasPendingLegalDocumentUpload) {
      showMessage(LEGAL_DOCUMENT_UPLOAD_PENDING_MESSAGE);
      return;
    }
    if (!hasUnsavedChanges && currentSchema) {
      openTemplatePreview(templatePreviewHref(currentSchema.id));
      return;
    }
    setPendingNavigation("preview");
  };

  const discardPendingNavigation = async () => {
    const pending = pendingNavigation;
    setPendingNavigation(null);
    await draftDocuments.cleanup();
    if (pending === "overview") {
      backToOverview();
      return;
    }
    if (pending === "new") startNew();
  };

  const savePendingNavigation = async () => {
    const pending = pendingNavigation;
    if (!pending) return;

    let previewWindow: Window | null = null;
    if (pending === "preview") {
      previewWindow = window.open("about:blank", "_blank");
      if (previewWindow) {
        previewWindow.opener = null;
        previewWindow.document.title = "Vorschau wird geöffnet";
        previewWindow.document.body.textContent = "Vorschau wird geöffnet…";
      }
    }

    const savedSchema = await saveSchema(
      pending === "overview" ? "overview" : "detail",
    );
    if (!savedSchema) {
      previewWindow?.close();
      // The reason stands in the builder's alert; the dialog would cover it.
      setPendingNavigation(null);
      return;
    }

    setPendingNavigation(null);
    if (pending === "new") {
      startNew();
      return;
    }
    if (pending === "preview") {
      const href = templatePreviewHref(savedSchema.id);
      if (previewWindow) {
        previewWindow.location.href = href;
      } else {
        openTemplatePreview(href);
      }
    }
  };

  const loadFailed = loadError !== null;
  useLayoutEffect(() => {
    if (!onTemplateCountChange) return;
    onTemplateCountChange(
      loading || loadFailed ? null : latestByName.length,
      loadFailed,
    );
  }, [loading, loadFailed, latestByName, onTemplateCountChange]);

  if (loading) {
    return (
      <SkeletonRegion label="Anmeldeformulare werden geladen">
        <FormSkeleton fields={6} />
      </SkeletonRegion>
    );
  }

  if (mode === "overview" && loadError) {
    // No list without data: an empty overview would read as "this school
    // has no templates".
    return <LoadErrorAlert error={loadError} />;
  }

  if (mode === "overview") {
    return (
      <>
        <LoadErrorAlert error={schemasAfterWrite.error} />
        <EnrollmentFormsOverview
          templates={latestByName}
          phases={phases}
          onCreate={startNew}
          onEdit={editSchema}
          onPreview={previewSchema}
          onRename={requestRenameSchema}
          onDelete={requestRemoveSchema}
        />
        <DeleteSchemaDialog
          schema={deleteTarget}
          deleting={deletingSchemaId === deleteTarget?.id}
          error={deleteErrors.error}
          onClose={closeDeleteDialog}
          onConfirm={confirmRemoveSchema}
        />
        <RenameSchemaDialog
          schema={renameTarget}
          onClose={() => setRenameTarget(null)}
          onConfirm={confirmRenameSchema}
        />
      </>
    );
  }

  if (mode === "detail" && currentSchema) {
    return (
      <FormTemplateDetail
        schema={currentSchema}
        onBack={backToOverview}
        onEdit={() => editSchema(currentSchema)}
        assignedPhases={phases.filter(
          (phase) => phase.form_schema_id === currentSchema.id,
        )}
      />
    );
  }

  return (
    // Flex-Spalte (wie die beiden anderen Ansichten dieser Seite): als
    // Editor-Wurzel einer Tenant-Seite reicht sie den Platz an die letzte
    // Fläche weiter, die bis zur Unterkante wächst (`.moto-tenant-body`).
    <DraftDocumentContext.Provider value={draftDocuments.lifecycle}>
      <FieldErrorContext.Provider value={fieldError}>
        <div ref={builderRef} className="flex flex-col space-y-5">
          <SectionCard padding="none">
            <div className="compact:px-4 border-b border-gray-100 px-5 py-3 max-sm:px-4">
              <BackAction
                onClick={requestBackToOverview}
                disabled={locked}
                label="Zurück zur Übersicht"
              />
            </div>
            <div className="grid gap-0 lg:grid-cols-[minmax(0,1fr)_380px] xl:grid-cols-[minmax(0,1fr)_430px]">
              <div className="compact:p-4 space-y-6 p-5 max-sm:p-4">
                {/* Ein Neuladen nach dem Speichern kann scheitern; der Entwurf
                bleibt dann stehen. */}
                <LoadErrorAlert error={loadError} />
                <LoadErrorAlert error={schemasAfterWrite.error} />
                {/* Speicher- und Prüffehler stehen oben im Bearbeiten-Bereich
                (Bauart 2 Regel 5); der Alert holt sich selbst ins Bild. */}
                <FormErrorAlert message={saveError} />

                <FormBuilderIntro />

                <BuilderTemplateSummary
                  name={name}
                  isCreating={isCreating}
                  saving={saving}
                  currentSchema={currentSchema}
                  onNameChange={setName}
                  fields={fields}
                />

                <CoreFieldsSection
                  coreRequirements={coreRequirements}
                  onRequirementChange={updateCoreRequirement}
                  disabled={locked}
                />

                <LegalBlocksSection
                  blocks={legalBlocks}
                  standardBlocks={standardLegalBlocks}
                  onChange={setLegalBlocks}
                  disabled={locked}
                />

                <SignatureRequirementSection
                  required={Boolean(coreRequirements.signature)}
                  onChange={(checked) =>
                    updateCoreRequirement("signature", checked)
                  }
                  disabled={locked}
                />

                <section className="space-y-4">
                  <div className="flex flex-col gap-3 border-t border-gray-100 pt-5 sm:flex-row sm:items-end sm:justify-between">
                    <div>
                      <h2 className="text-base font-semibold text-gray-900">
                        Was Eltern zusätzlich beantworten sollen
                      </h2>
                      <p className="mt-1 max-w-2xl text-sm text-gray-600">
                        Wählen Sie feste Vorschläge, wenn die Antwort später in
                        den Stammdaten stehen soll. Freie Zusatzfragen bleiben
                        nur bei der Anmeldung.
                      </p>
                    </div>
                    <div className="flex flex-wrap gap-2">
                      <Button
                        type="button"
                        onClick={addInfoField}
                        disabled={locked}
                        variant="outline"
                        size="md"
                        className="inline-flex items-center justify-center gap-2"
                      >
                        <Info className="h-4 w-4" aria-hidden="true" />
                        Infotext
                      </Button>
                      <Button
                        type="button"
                        onClick={addField}
                        disabled={locked}
                        variant="outline"
                        size="md"
                        className="inline-flex items-center justify-center gap-2"
                      >
                        <Plus className="h-4 w-4" aria-hidden="true" />
                        Freie Zusatzfrage
                      </Button>
                      <IncomeDeclarationButton
                        fields={fields}
                        onAdd={addIncomeDeclaration}
                        disabled={locked}
                      />
                    </div>
                  </div>

                  <TargetSuggestions
                    fields={fields}
                    onAdd={addTargetField}
                    disabled={locked}
                  />

                  {fields.length > 0 ? (
                    <div ref={fieldListRef} className="space-y-3">
                      {fields.map((field, index) => (
                        <FieldEditorRow
                          key={getStableObjectKey(field, "custom-field")}
                          field={field}
                          index={index}
                          total={fields.length}
                          allFields={fields}
                          onChange={(patch) => updateField(index, patch)}
                          onRemove={() => removeField(index)}
                          onMoveUp={() => moveField(index, -1)}
                          onMoveDown={() => moveField(index, 1)}
                          disabled={locked}
                        />
                      ))}
                    </div>
                  ) : null}

                  <TranslationsSection
                    targets={schemaTranslationTargets(fields, legalBlocks)}
                    onChange={applySchemaTranslation}
                    disabled={locked}
                  />

                  <div className="flex flex-wrap items-center justify-end gap-2 border-t border-gray-100 pt-4">
                    <Button
                      type="button"
                      onClick={requestStartNew}
                      disabled={locked}
                      variant="outline"
                      size="md"
                      className="inline-flex items-center justify-center"
                    >
                      Zurücksetzen
                    </Button>
                    <Button
                      type="button"
                      onClick={handleSave}
                      disabled={locked}
                      variant="primary"
                      size="md"
                      className="inline-flex items-center justify-center"
                    >
                      {saving
                        ? "Speichert…"
                        : isCreating
                          ? "Formularvorlage erstellen"
                          : "Änderungen speichern"}
                    </Button>
                  </div>
                </section>
              </div>

              <aside className="moto-dotted-background moto-dotted-background--split compact:p-4 border-t border-gray-100 p-5 max-sm:p-4 lg:border-t-0 lg:border-l">
                <FormPreview
                  fields={fields}
                  coreRequirements={coreRequirements}
                  legalBlocks={legalBlocks}
                  templateName={name}
                  isActive={currentSchema?.is_active ?? false}
                  isSaved={currentSchema !== null}
                  previewHref={
                    currentSchema && !hasUnsavedChanges
                      ? templatePreviewHref(currentSchema.id)
                      : undefined
                  }
                  onPreviewClick={requestExternalPreview}
                  assignedPhaseCount={
                    currentSchema
                      ? phases.filter(
                          (phase) => phase.form_schema_id === currentSchema.id,
                        ).length
                      : 0
                  }
                />
              </aside>
            </div>
          </SectionCard>
          <UnsavedChangesDialog
            pendingNavigation={pendingNavigation}
            saving={locked}
            saveValidationMessage={saveBlockedMessage}
            pendingUploadMessage={pendingUploadMessage}
            onCancel={() => setPendingNavigation(null)}
            onDiscard={discardPendingNavigation}
            onSave={savePendingNavigation}
          />
        </div>
      </FieldErrorContext.Provider>
    </DraftDocumentContext.Provider>
  );
}

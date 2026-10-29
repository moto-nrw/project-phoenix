"use client";

import {
  useCallback,
  useContext,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type Dispatch,
  type ReactNode,
  type SetStateAction,
} from "react";
import {
  Check,
  Eye,
  ExternalLink,
  FileText,
  FileUp,
  Info,
  Lock,
  Pencil,
  Plus,
  TextCursorInput,
  Trash2,
} from "lucide-react";
import {
  AttachmentEditor,
  AttachmentUploadContext,
} from "~/components/enrollment/attachment-editor";
import {
  applyFileFieldRules,
  createIncomeDeclarationFields,
  fileFieldDraftProblem,
  hasIncomeDeclaration,
  IncomeDeclarationButton,
} from "~/components/enrollment/file-field-editor";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import {
  useApiErrorDisplay,
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { Modal } from "~/components/ui/modal";
import type { FormErrorInput } from "~/components/ui/form-error";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { FormModal } from "~/components/ui/form-modal";
import { Alert } from "~/components/ui/alert";
import { SectionCard } from "~/components/ui/section-card";
import { Textarea } from "~/components/ui/textarea";
import { BackAction } from "~/components/ui/back-link";
import { Button, ButtonLink } from "~/components/ui/button";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { FormSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { Input } from "~/components/ui/input";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { BooleanField } from "~/components/settings/fields/boolean-field";
import {
  TranslationsSection,
  type TranslationTarget,
} from "~/components/enrollment/translations-section";
import type { Translations } from "~/lib/enrollment-translations";
import {
  MAX_ENROLLMENT_DOCUMENT_BYTES,
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
  uploadEnrollmentLegalDocument,
  RESERVED_TARGETS,
  type FormField,
  type FormFieldTarget,
  type FormFieldType,
  type FormLegalBlock,
  type FormSchema,
  type LegalBlockDisplayMode,
  type CoreRequirementKey,
  type CoreRequirements,
  type PublicLegalTexts,
} from "~/lib/enrollment-form-schema-api";
import {
  FieldEditorRow,
  FieldErrorContext,
  FormPreview as SharedFormPreview,
  FormPreviewHeading,
  PreviewAttachmentNames,
  PreviewFieldsSection,
  PreviewSection,
  type PreviewFieldLabel,
  fieldErrorProps,
  fieldTypeLabels,
  normalizeFieldKey,
} from "~/components/enrollment/form-field-builder";
import { listPhases, type Phase } from "~/lib/enrollment-phase-api";
import { createLogger } from "~/lib/logger";
import { useTenantSlugSafe } from "~/lib/tenant-context";
import { useTenantAwarePath } from "~/lib/tenant-path";
import {
  copyStableObjectKey,
  getStableObjectKey,
} from "~/lib/stable-object-key";

const logger = createLogger({ component: "EnrollmentFormEditor" });

const structuredFieldTypes = new Set<FormFieldType>([
  "phone_list",
  "weekday_schedule",
  "weekday_boolean",
  "weekday_mode",
  "weekday_multi_mode",
  "contact_list",
]);

// Labels for the answer storage picker. Admins can still rename the
// displayed question; the target decides whether the answer is copied
// into student data, schedule data, or contacts at approval time.
const targetPickerLabels: Record<Exclude<FormFieldTarget, "">, string> = {
  "student.health_info": "Gesundheitsinformationen beim Kind speichern",
  "student.extra_info": "Hinweise für die Betreuung beim Kind speichern",
  "student.allowed_departure_modes": "Erlaubte Heimwege beim Kind speichern",
  "student.departure": "Geh- und Abholregelung beim Kind speichern",
  "student.bus_days": "Buskind beim Kind speichern",
  "student.bus": "Buskind beim Kind speichern",
  "student.pickup_status": "Abholregelung beim Kind speichern",
  "schedule.pickup": "Abholzeiten im Betreuungsplan speichern",
  "schedule.arrival": "Ankunftszeiten im Betreuungsplan speichern",
  "student.contacts":
    "Weitere Kontakte, Abholberechtigte und Notfallkontakte speichern",
};

// Targets sorted alphabetically by label for the picker, keeps the
// dropdown stable across renders even if the underlying map order
// changes. student.bus / student.bus_days / student.pickup_status are legacy
// targets superseded by the unified student.departure (#1610); they are
// excluded so new fields can only pick the canonical unified target.
const LEGACY_PICKER_TARGETS = new Set<Exclude<FormFieldTarget, "">>([
  "student.departure",
  "student.bus",
  "student.bus_days",
  "student.pickup_status",
]);
const TARGET_PICKER_ORDER: Array<Exclude<FormFieldTarget, "">> = (
  Object.keys(targetPickerLabels) as Array<Exclude<FormFieldTarget, "">>
)
  .filter((target) => !LEGACY_PICKER_TARGETS.has(target))
  .sort((a, b) =>
    targetPickerLabels[a].localeCompare(targetPickerLabels[b], "de"),
  );

const targetSuggestionDescriptions: Record<
  Exclude<FormFieldTarget, "">,
  string
> = {
  "student.health_info":
    "Für Allergien, Medikamente oder andere Gesundheitsangaben.",
  "student.extra_info":
    "Für wichtige Hinweise, die im Alltag der Betreuung sichtbar sein sollen.",
  "student.allowed_departure_modes":
    "Für Betreuungstage festlegen, welche Heimwege erlaubt sind: zu Fuß, Bus, Abholung oder mit anderem Kind. Mit Betreuungsangeboten sehen Eltern nur die gewählten Betreuungstage; als Pflichtfrage ist pro Betreuungstag mindestens ein Heimweg nötig.",
  "student.departure":
    "Für die Wochentage festlegen, wie das Kind nach Hause geht: geht alleine, fährt Bus oder wird abgeholt.",
  "student.bus_days":
    "Für die Information, an welchen Wochentagen ein Kind mit dem Bus fährt.",
  "student.bus": "Für die Information, ob ein Kind mit dem Bus fährt.",
  "student.pickup_status":
    "Für die Wochentage, an denen ein Kind abgeholt wird. Nicht gewählte Tage bedeuten, dass es alleine nach Hause geht.",
  "schedule.pickup": "Für regelmäßige Abholzeiten je Wochentag.",
  "schedule.arrival": "Für regelmäßige Ankunftszeiten je Wochentag.",
  "student.contacts":
    "Für weitere Kontakte, Abholberechtigte oder Notfallkontakte.",
};

const NEW_SCHEMA_VALUE = "__new__";
const LEGAL_DOCUMENT_UPLOAD_PENDING_MESSAGE =
  "Bitte warten Sie, bis die PDF-Datei hochgeladen ist.";

type EditorMode = "overview" | "builder" | "detail";
type PendingNavigation = "overview" | "new" | "preview";

const STANDARD_LEGAL_BLOCKS: FormLegalBlock[] = [
  {
    key: "agb",
    kind: "terms",
    title: "AGB / Teilnahmebedingungen",
    label:
      "Ich akzeptiere die AGB / Teilnahmebedingungen / den Ganztag Info-Brief.",
    text: "",
    required: true,
    enabled: false,
    sort_order: 10,
    source: "standard",
  },
  {
    key: "data_processing",
    kind: "privacy_notice",
    title: "Datenschutzinformation",
    label:
      "Ich habe die Datenschutzinformation der Schule zur Kenntnis genommen.",
    text: "",
    required: true,
    enabled: false,
    sort_order: 20,
    source: "standard",
  },
  {
    key: "photo",
    kind: "consent",
    title: "Fotoeinwilligung",
    label:
      "Mein Kind darf bei Schulveranstaltungen fotografiert werden. Diese Einwilligung ist freiwillig und jederzeit mit Wirkung für die Zukunft widerrufbar.",
    text: "",
    required: false,
    enabled: false,
    sort_order: 30,
    source: "standard",
  },
  {
    key: "email_contact",
    kind: "notice",
    title: "E-Mail-Kontakt",
    label:
      "Die Schule nutzt Ihre E-Mail-Adresse für Rückfragen und Status-Benachrichtigungen zu dieser Anmeldung.",
    text: "",
    required: false,
    enabled: false,
    sort_order: 40,
    source: "standard",
  },
];

const LEGAL_BLOCK_DISPLAY_MODE_TEXT = "text" satisfies LegalBlockDisplayMode;
const LEGAL_BLOCK_DISPLAY_MODE_PDF = "pdf" satisfies LegalBlockDisplayMode;

interface CoreField {
  readonly key: string;
  readonly label: string;
  readonly type: FormFieldType;
  readonly required: boolean;
  readonly group: "guardian" | "child";
  readonly requirementKey?: CoreRequirementKey;
  readonly requirementLabel?: string;
  readonly requirementHint?: string;
}

const CORE_FIELDS: ReadonlyArray<CoreField> = [
  {
    key: "guardian_first_name",
    label: "Vorname (Elternteil)",
    type: "text",
    required: true,
    group: "guardian",
  },
  {
    key: "guardian_last_name",
    label: "Nachname (Elternteil)",
    type: "text",
    required: true,
    group: "guardian",
  },
  {
    key: "guardian_email",
    label: "E-Mail (Elternteil)",
    type: "text",
    required: true,
    group: "guardian",
  },
  {
    key: "guardian_phone",
    label: "Telefonnummer (Elternteil)",
    type: "text",
    required: false,
    group: "guardian",
    requirementKey: "guardian_phone",
    requirementLabel: "Telefonnummer verpflichtend",
    requirementHint: "Eltern müssen eine gültige Telefonnummer angeben.",
  },
  {
    key: "first_name",
    label: "Vorname (Kind)",
    type: "text",
    required: true,
    group: "child",
  },
  {
    key: "last_name",
    label: "Nachname (Kind)",
    type: "text",
    required: true,
    group: "child",
  },
  {
    key: "date_of_birth",
    label: "Geburtsdatum (Kind)",
    type: "date",
    required: true,
    group: "child",
  },
  {
    key: "target_grade_level",
    label: "Klassenstufe (Kind)",
    type: "number",
    required: true,
    group: "child",
  },
];

// True when newName is a non-empty rename of schema's current name. The
// single source of truth for "is this a rename?" shared by both entry
// points: the builder's inline name field and the standalone rename dialog.
function isRenameOf(
  schema: FormSchema | null | undefined,
  newName: string,
): boolean {
  const trimmed = newName.trim();
  return Boolean(schema) && trimmed.length > 0 && trimmed !== schema?.name;
}

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
  const tenantPath = useTenantAwarePath();
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
  const { show: showActionError, showMessage } = useApiErrorDisplay();
  const [mode, setMode] = useState<EditorMode>("overview");
  const [pendingNavigation, setPendingNavigation] =
    useState<PendingNavigation | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<FormSchema | null>(null);
  const [deletingSchemaId, setDeletingSchemaId] = useState<string | null>(null);
  const [renameTarget, setRenameTarget] = useState<FormSchema | null>(null);
  const uploadedDraftDocumentURLsRef = useRef<Set<string>>(new Set());
  const savingDraftDocumentURLsRef = useRef<Set<string>>(new Set());
  const unmountedRef = useRef(false);
  const [uploadedDraftDocumentURLs, setUploadedDraftDocumentURLs] = useState<
    Set<string>
  >(() => new Set());
  const [uploadingLegalDocumentCount, setUploadingLegalDocumentCount] =
    useState(0);
  const hasPendingLegalDocumentUpload = uploadingLegalDocumentCount > 0;

  const latestByName = useMemo(
    () => latestSchemasByName(allSchemas),
    [allSchemas],
  );

  // The retry of a failed load closes over the latest loadAll.
  const loadAllRef = useRef<() => Promise<FormSchema[]>>(async () => []);
  const loadAll = useCallback(async () => {
    setLoading(true);
    clearLoadError();
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
        tenantSlug ? fetchPublicLegalTexts(tenantSlug) : Promise.resolve(null),
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
      void showLoadError(err, {
        object: "die Übersicht der Anmeldeformulare",
        retry: () => void loadAllRef.current(),
      });
      return [];
    } finally {
      setLoading(false);
    }
  }, [clearLoadError, showLoadError, tenantSlug]);
  useLayoutEffect(() => {
    loadAllRef.current = loadAll;
  });

  useEffect(() => {
    void loadAll();
  }, [loadAll]);

  const setDraftDocumentURLs = useCallback(
    (updater: (urls: Set<string>) => Set<string>) => {
      const next = updater(new Set(uploadedDraftDocumentURLsRef.current));
      uploadedDraftDocumentURLsRef.current = next;
      setUploadedDraftDocumentURLs(next);
    },
    [],
  );

  const rememberDraftDocumentURL = useCallback(
    (documentURL: string) => {
      const trimmed = documentURL.trim();
      if (!trimmed) return;
      setDraftDocumentURLs((urls) => {
        urls.add(trimmed);
        return urls;
      });
    },
    [setDraftDocumentURLs],
  );

  const forgetDraftDocumentURL = useCallback(
    (documentURL: string) => {
      setDraftDocumentURLs((urls) => {
        urls.delete(documentURL);
        return urls;
      });
    },
    [setDraftDocumentURLs],
  );

  const forgetDraftDocumentURLs = useCallback(
    (documentURLs: Iterable<string>) => {
      setDraftDocumentURLs((urls) => {
        for (const documentURL of documentURLs) {
          urls.delete(documentURL);
        }
        return urls;
      });
    },
    [setDraftDocumentURLs],
  );

  const beginLegalDocumentUpload = useCallback(() => {
    setUploadingLegalDocumentCount((count) => count + 1);
  }, []);

  const endLegalDocumentUpload = useCallback(() => {
    setUploadingLegalDocumentCount((count) => Math.max(0, count - 1));
  }, []);

  const attachmentLifecycle = useMemo(
    () => ({
      remember: rememberDraftDocumentURL,
      begin: beginLegalDocumentUpload,
      end: endLegalDocumentUpload,
    }),
    [
      rememberDraftDocumentURL,
      beginLegalDocumentUpload,
      endLegalDocumentUpload,
    ],
  );

  const cleanupDraftDocumentURLs = useCallback(
    async ({
      keepalive = false,
      notify = true,
      urlsToKeep = new Set<string>(),
    }: {
      keepalive?: boolean;
      notify?: boolean;
      urlsToKeep?: ReadonlySet<string>;
    } = {}) => {
      const urls = Array.from(uploadedDraftDocumentURLsRef.current).filter(
        (url) => !urlsToKeep.has(url),
      );
      if (urls.length === 0) return;
      const results = await Promise.allSettled(
        urls.map((url) => deleteEnrollmentLegalDocument(url, { keepalive })),
      );
      const deletedURLs = urls.filter(
        (_, index) => results[index]?.status === "fulfilled",
      );
      if (deletedURLs.length > 0) {
        forgetDraftDocumentURLs(deletedURLs);
      }
      const failure = results.find(
        (result): result is PromiseRejectedResult =>
          result.status === "rejected",
      );
      if (notify && failure) {
        logger.warn("draft_legal_document_cleanup_failed", {
          error:
            failure.reason instanceof Error
              ? failure.reason.message
              : String(failure.reason),
        });
        // The URLs stay tracked: the next save or discard tries again.
        void showActionError(failure.reason, {
          object: "eine ungespeicherte PDF-Datei",
        });
      }
    },
    [forgetDraftDocumentURLs, showActionError],
  );

  useEffect(() => {
    unmountedRef.current = false;

    return () => {
      unmountedRef.current = true;
      const urls = Array.from(uploadedDraftDocumentURLsRef.current).filter(
        (url) => !savingDraftDocumentURLsRef.current.has(url),
      );
      uploadedDraftDocumentURLsRef.current = new Set(
        Array.from(uploadedDraftDocumentURLsRef.current).filter((url) =>
          savingDraftDocumentURLsRef.current.has(url),
        ),
      );
      for (const url of urls) {
        void deleteEnrollmentLegalDocument(url, { keepalive: true }).catch(
          (error: unknown) => {
            logger.warn("draft_document_cleanup_failed_after_unmount", {
              error: error instanceof Error ? error.message : String(error),
            });
          },
        );
      }
    };
  }, []);

  const selectSchema = (schema: FormSchema, nextMode: EditorMode) => {
    setSelectedKey(schema.id);
    setName(schema.name);
    setFields(schema.fields);
    setCoreRequirements(schema.core_requirements ?? {});
    setLegalBlocks(
      schema.legal_blocks && schema.legal_blocks.length > 0
        ? mergeSavedLegalBlocks(schema.legal_blocks, standardLegalBlocks)
        : standardLegalBlocks,
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
    await loadAll();
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
    await loadAll();
    // Keep the builder's name field in sync if this template is open.
    if (selectedKey === renameTarget.id) {
      setName(updated.name);
    }
    toast.success(`Die Formularvorlage heißt jetzt „${updated.name}“.`);
    setRenameTarget(null);
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

  // Routes an edit from the translations block back to the object that owns
  // the text; see schemaTranslationTargets for the id shape.
  const applySchemaTranslation = (
    target: TranslationTarget,
    translations: Translations,
  ) => {
    const [owner, first, second] = target.id.split("|");
    const index = Number(first);
    if (owner === "legal") {
      setLegalBlocks((prev) =>
        prev.map((block, i) =>
          i === index
            ? copyStableObjectKey(block, { ...block, translations })
            : block,
        ),
      );
      return;
    }
    const field = fields[index];
    if (!field) return;
    if (owner === "field") {
      updateField(index, { translations });
      return;
    }
    const optionIndex = Number(second);
    updateField(index, {
      options: field.options?.map((option, i) =>
        i === optionIndex ? { ...option, translations } : option,
      ),
    });
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

  const addTargetField = (target: Exclude<FormFieldTarget, "">) => {
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
        currentSchema?.legal_blocks && currentSchema.legal_blocks.length > 0
          ? mergeSavedLegalBlocks(
              currentSchema.legal_blocks,
              standardLegalBlocks,
            )
          : standardLegalBlocks,
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
  const effectiveSaveBlockedMessage =
    saveBlockedMessage ?? pendingUploadMessage;

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
      const referencedDraftDocumentURLs = draftDocumentURLsInLegalBlocks(
        legalBlocksForSave,
        uploadedDraftDocumentURLsRef.current,
      );
      for (const field of fieldsForSave) {
        for (const attachment of field.attachments ?? []) {
          if (uploadedDraftDocumentURLsRef.current.has(attachment.document_url))
            referencedDraftDocumentURLs.add(attachment.document_url);
        }
      }
      savingDraftDocumentURLsRef.current = referencedDraftDocumentURLs;
      await cleanupDraftDocumentURLs({
        urlsToKeep: referencedDraftDocumentURLs,
      });
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
        uploadedDraftDocumentURLsRef.current = new Set();
        savingDraftDocumentURLsRef.current = new Set();
        return result;
      }
      const refreshed = await loadAll();
      const stillThere = refreshed.find((s) => s.id === result.id);
      const savedSchema = stillThere ?? result;
      forgetDraftDocumentURLs(referencedDraftDocumentURLs);
      savingDraftDocumentURLsRef.current = new Set();
      selectSchema(savedSchema, nextMode);
      toast.success(
        isCreating
          ? `Die Formularvorlage „${savedSchema.name}“ ist angelegt.`
          : `Die Formularvorlage „${savedSchema.name}“ ist gespeichert.`,
      );
      return savedSchema;
    } catch (err) {
      if (unmountedRef.current) {
        // Nobody is left to read an error: the editor is gone. Only the
        // uploads of this save are cleaned up.
        const urls = Array.from(savingDraftDocumentURLsRef.current);
        for (const url of urls) {
          void deleteEnrollmentLegalDocument(url, { keepalive: true }).catch(
            (error: unknown) => {
              logger.warn("draft_document_cleanup_failed_after_unmount", {
                error: error instanceof Error ? error.message : String(error),
              });
            },
          );
        }
        savingDraftDocumentURLsRef.current = new Set();
      } else {
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
      if (!unmountedRef.current) {
        savingDraftDocumentURLsRef.current = new Set();
        setSaving(false);
      }
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

  const openPreviewWindow = (schemaId: string) => {
    window.open(
      tenantPath(`/anmeldung/preview?schemaId=${encodeURIComponent(schemaId)}`),
      "_blank",
      "noopener,noreferrer",
    );
  };

  const requestExternalPreview = () => {
    if (hasPendingLegalDocumentUpload) {
      showMessage(LEGAL_DOCUMENT_UPLOAD_PENDING_MESSAGE);
      return;
    }
    if (!hasUnsavedChanges && currentSchema) {
      openPreviewWindow(currentSchema.id);
      return;
    }
    setPendingNavigation("preview");
  };

  const discardPendingNavigation = async () => {
    const pending = pendingNavigation;
    setPendingNavigation(null);
    await cleanupDraftDocumentURLs();
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
      const href = tenantPath(
        `/anmeldung/preview?schemaId=${encodeURIComponent(savedSchema.id)}`,
      );
      if (previewWindow) {
        previewWindow.location.href = href;
      } else {
        window.open(href, "_blank", "noopener,noreferrer");
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
    <AttachmentUploadContext.Provider value={attachmentLifecycle}>
      <FieldErrorContext.Provider value={fieldError}>
        <div ref={builderRef} className="flex flex-col space-y-5">
          <section className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm backdrop-blur-md">
            <div className="border-b border-gray-100 px-5 py-3 sm:px-6">
              <BackAction
                onClick={requestBackToOverview}
                disabled={saving || hasPendingLegalDocumentUpload}
                label="Zurück zur Übersicht"
              />
            </div>
            <div className="grid gap-0 lg:grid-cols-[minmax(0,1fr)_380px] xl:grid-cols-[minmax(0,1fr)_430px]">
              <div className="space-y-6 p-5 sm:p-6">
                {/* Ein Neuladen nach dem Speichern kann scheitern; der Entwurf
                bleibt dann stehen. */}
                <LoadErrorAlert error={loadError} />
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
                  disabled={saving || hasPendingLegalDocumentUpload}
                />

                <LegalBlocksSection
                  blocks={legalBlocks}
                  standardBlocks={standardLegalBlocks}
                  onChange={setLegalBlocks}
                  disabled={saving || hasPendingLegalDocumentUpload}
                  draftDocumentURLs={uploadedDraftDocumentURLs}
                  onDraftDocumentUploaded={rememberDraftDocumentURL}
                  onDraftDocumentDeleted={forgetDraftDocumentURL}
                  onUploadStart={beginLegalDocumentUpload}
                  onUploadEnd={endLegalDocumentUpload}
                />

                <SignatureRequirementSection
                  required={Boolean(coreRequirements.signature)}
                  onChange={(checked) =>
                    updateCoreRequirement("signature", checked)
                  }
                  disabled={saving || hasPendingLegalDocumentUpload}
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
                        disabled={saving || hasPendingLegalDocumentUpload}
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
                        disabled={saving || hasPendingLegalDocumentUpload}
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
                        disabled={saving || hasPendingLegalDocumentUpload}
                      />
                    </div>
                  </div>

                  <TargetSuggestions
                    fields={fields}
                    onAdd={addTargetField}
                    disabled={saving || hasPendingLegalDocumentUpload}
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
                          disabled={saving || hasPendingLegalDocumentUpload}
                        />
                      ))}
                    </div>
                  ) : null}

                  <TranslationsSection
                    targets={schemaTranslationTargets(fields, legalBlocks)}
                    onChange={applySchemaTranslation}
                    disabled={saving || hasPendingLegalDocumentUpload}
                  />

                  <div className="flex flex-wrap items-center justify-end gap-2 border-t border-gray-100 pt-4">
                    <Button
                      type="button"
                      onClick={requestStartNew}
                      disabled={saving || hasPendingLegalDocumentUpload}
                      variant="outline"
                      size="md"
                      className="inline-flex items-center justify-center"
                    >
                      Zurücksetzen
                    </Button>
                    <Button
                      type="button"
                      onClick={handleSave}
                      disabled={saving || hasPendingLegalDocumentUpload}
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

              <aside className="moto-dotted-background moto-dotted-background--split border-t border-gray-100 p-5 sm:p-6 lg:border-t-0 lg:border-l">
                <FormPreview
                  fields={fields}
                  coreRequirements={coreRequirements}
                  legalBlocks={legalBlocks}
                  templateName={name}
                  isActive={currentSchema?.is_active ?? false}
                  isSaved={currentSchema !== null}
                  previewHref={
                    currentSchema && !hasUnsavedChanges
                      ? tenantPath(
                          `/anmeldung/preview?schemaId=${encodeURIComponent(currentSchema.id)}`,
                        )
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
          </section>
          <UnsavedChangesDialog
            pendingNavigation={pendingNavigation}
            saving={saving || hasPendingLegalDocumentUpload}
            saveBlockedMessage={effectiveSaveBlockedMessage}
            onCancel={() => setPendingNavigation(null)}
            onDiscard={discardPendingNavigation}
            onSave={savePendingNavigation}
          />
        </div>
      </FieldErrorContext.Provider>
    </AttachmentUploadContext.Provider>
  );
}

function EnrollmentFormsOverview({
  templates,
  phases,
  onCreate,
  onEdit,
  onPreview,
  onRename,
  onDelete,
}: Readonly<{
  templates: FormSchema[];
  phases: Phase[];
  onCreate: () => void;
  onEdit: (schema: FormSchema) => void;
  onPreview: (schema: FormSchema) => void;
  onRename: (schema: FormSchema) => void;
  onDelete: (schema: FormSchema) => void;
}>) {
  const assignedTemplateCount = templates.filter((schema) =>
    phases.some((phase) => phase.form_schema_id === schema.id),
  ).length;

  return (
    // Flex-Spalte: als Editor-Wurzel einer Tenant-Seite reicht sie den
    // Platz an die Fläche weiter, die bis zur Unterkante wächst
    // (`.moto-tenant-body`).
    <div className="flex flex-col space-y-5">
      {/* Karte als Flex-Spalte und das Raster darin `flex-1`: wächst die
          Karte bis zur Unterkante, läuft die graue Seitenspalte mit bis
          dorthin, statt über dem Kartenrand zu enden. */}
      <section className="moto-content-surface flex flex-col overflow-hidden rounded-2xl border shadow-sm backdrop-blur-md">
        <div className="grid flex-1 gap-0 lg:grid-cols-[minmax(0,1fr)_360px] xl:grid-cols-[minmax(0,1fr)_400px]">
          <div className="space-y-6 p-5 sm:p-6">
            <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
              <div>
                <h2 className="text-base font-semibold text-gray-900">
                  Anmeldeformulare verwalten
                </h2>
                <p className="mt-2 max-w-3xl text-sm leading-6 text-gray-600">
                  Das Basisformular ist immer vorhanden. Eigene Vorlagen nutzen
                  Sie, wenn eine Anmeldephase abweichende Pflichtangaben oder
                  Zusatzfragen braucht.
                </p>
              </div>
              <Button
                type="button"
                onClick={onCreate}
                variant="primary"
                size="md"
                className="inline-flex items-center justify-center gap-2 whitespace-nowrap"
              >
                <Plus className="h-4 w-4" aria-hidden="true" />
                Neue Vorlage
              </Button>
            </div>

            <section className="space-y-3">
              <div className="flex items-end justify-between gap-3">
                <div>
                  <h3 className="text-base font-semibold text-gray-900">
                    Formularvorlagen
                  </h3>
                  <p className="mt-1 text-sm text-gray-600">
                    Jede Anmeldephase nutzt entweder das Basisformular oder eine
                    eigene Vorlage mit angepassten Pflichtangaben und
                    Zusatzfragen.
                  </p>
                </div>
              </div>

              <TemplateOverviewList
                templates={templates}
                phases={phases}
                onEdit={onEdit}
                onPreview={onPreview}
                onRename={onRename}
                onDelete={onDelete}
              />
            </section>
          </div>

          <aside className="moto-dotted-background moto-dotted-background--split border-t border-gray-100 p-5 sm:p-6 lg:border-t-0 lg:border-l">
            <OverviewGuide
              templateCount={templates.length}
              assignedTemplateCount={assignedTemplateCount}
            />
          </aside>
        </div>
      </section>
    </div>
  );
}

function TemplateOverviewList({
  templates,
  phases,
  onEdit,
  onPreview,
  onRename,
  onDelete,
}: Readonly<{
  templates: FormSchema[];
  phases: Phase[];
  onEdit: (schema: FormSchema) => void;
  onPreview: (schema: FormSchema) => void;
  onRename: (schema: FormSchema) => void;
  onDelete: (schema: FormSchema) => void;
}>) {
  return (
    <div className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
      <div className="divide-y divide-gray-100">
        <BaseTemplateOverviewRow />
        {templates.map((schema) => (
          <TemplateOverviewRow
            key={schema.id}
            schema={schema}
            onEdit={() => onEdit(schema)}
            onPreview={() => onPreview(schema)}
            onRename={() => onRename(schema)}
            onDelete={() => onDelete(schema)}
            isAssigned={phases.some(
              (phase) => phase.form_schema_id === schema.id,
            )}
          />
        ))}
      </div>
    </div>
  );
}

function BaseTemplateOverviewRow() {
  const tenantPath = useTenantAwarePath();

  return (
    <article className="grid gap-4 px-4 py-4 md:grid-cols-[minmax(0,1fr)_auto] md:items-center">
      <div className="min-w-0">
        <div className="flex items-center gap-2">
          <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-gray-100 text-gray-500">
            <Lock className="h-4 w-4" aria-hidden="true" />
          </span>
          <h4 className="truncate text-sm font-semibold text-gray-900">
            Basisformular
          </h4>
          <span className="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600">
            System
          </span>
        </div>
        <p className="mt-1 text-xs leading-5 text-gray-500">
          Elternteil, Kind, Klassenstufe und gewünschtes Betreuungsangebot.
        </p>
        <div className="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-xs text-gray-500">
          <UsageLine title="Standard" status="base" />
          <span>Systemformular</span>
          <span>Keine Anpassungen</span>
        </div>
      </div>
      <div className="flex justify-start gap-2 md:justify-end">
        <a
          href={tenantPath("/anmeldung/preview?base=1")}
          target="_blank"
          rel="noreferrer"
          className="inline-flex h-9 items-center justify-center gap-2 rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
        >
          <ExternalLink className="h-4 w-4" aria-hidden="true" />
          Vorschau
        </a>
      </div>
    </article>
  );
}

function TemplateOverviewRow({
  schema,
  onEdit,
  onPreview,
  onRename,
  onDelete,
  isAssigned,
}: Readonly<{
  schema: FormSchema;
  onEdit: () => void;
  onPreview: () => void;
  onRename: () => void;
  onDelete: () => void;
  isAssigned: boolean;
}>) {
  const tenantPath = useTenantAwarePath();
  const requiredCount = schema.fields.filter((field) =>
    Boolean(field.required),
  ).length;
  const usageStatus = isAssigned ? "assigned" : "ready";
  const questionLabel =
    schema.fields.length === 1 ? "1 Frage" : `${schema.fields.length} Fragen`;
  const usageTitle = isAssigned ? "In Phase verwendet" : "Nicht verwendet";

  return (
    <article className="grid gap-4 px-4 py-4 md:grid-cols-[minmax(0,1fr)_auto] md:items-center">
      <div className="min-w-0">
        <div className="flex items-center gap-2">
          <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-gray-100">
            <MotoConceptIcon concept="enrollments" size={16} />
          </span>
          <h4 className="truncate text-sm font-semibold text-gray-900">
            {schema.name}
          </h4>
        </div>
        <p className="mt-1 text-xs leading-5 text-gray-500">
          Eigene Vorlage für Pflichtangaben und Zusatzfragen.
        </p>
        <div className="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-xs text-gray-500">
          <UsageLine title={usageTitle} status={usageStatus} />
          <span>Erstellt {formatSchemaDate(schema.created_at)}</span>
          <span>{questionLabel}</span>
          {requiredCount > 0 ? <span>{requiredCount} Pflicht</span> : null}
        </div>
      </div>
      <div className="flex justify-start gap-2 md:justify-end">
        <OverflowMenu
          ariaLabel={`Aktionen für ${schema.name}`}
          items={[
            {
              label: "Prüfen",
              icon: <Eye className="h-4 w-4" aria-hidden />,
              onClick: onPreview,
            },
            {
              label: "Vorschau",
              icon: <ExternalLink className="h-4 w-4" aria-hidden />,
              onClick: () => {
                window.open(
                  tenantPath(
                    `/anmeldung/preview?schemaId=${encodeURIComponent(schema.id)}`,
                  ),
                  "_blank",
                  "noopener,noreferrer",
                );
              },
            },
            {
              label: "Bearbeiten",
              icon: <Pencil className="h-4 w-4" aria-hidden />,
              onClick: onEdit,
            },
            {
              label: "Umbenennen",
              icon: <TextCursorInput className="h-4 w-4" aria-hidden />,
              onClick: onRename,
            },
            {
              label: "Löschen",
              icon: <Trash2 className="h-4 w-4" aria-hidden />,
              onClick: onDelete,
              destructive: true,
            },
          ]}
        />
      </div>
    </article>
  );
}

type TemplateStatus = "base" | "assigned" | "ready";

function UsageLine({
  title,
  status,
}: Readonly<{
  title: string;
  status: TemplateStatus;
}>) {
  const statusMap: Record<
    TemplateStatus,
    { dotClassName: string; textClassName: string }
  > = {
    base: {
      dotClassName: "bg-gray-300",
      textClassName: "text-gray-700",
    },
    assigned: {
      dotClassName: "bg-moto-green",
      textClassName: "text-moto-green-vivid",
    },
    ready: {
      dotClassName: "bg-gray-300",
      textClassName: "text-gray-700",
    },
  };
  const config = statusMap[status];

  return (
    <span
      className={`inline-flex items-center gap-2 font-medium ${config.textClassName}`}
    >
      <span
        className={`h-2 w-2 shrink-0 rounded-full ${config.dotClassName}`}
        aria-hidden="true"
      />
      {title}
    </span>
  );
}

function OverviewGuide({
  templateCount,
  assignedTemplateCount,
}: Readonly<{
  templateCount: number;
  assignedTemplateCount: number;
}>) {
  return (
    <div className="sticky top-6 space-y-4">
      <div>
        <h2 className="text-base font-semibold text-gray-900">
          Erst prüfen, dann erweitern
        </h2>
        <p className="mt-2 text-sm leading-6 text-gray-600">
          Legen Sie nur dann eine eigene Vorlage an, wenn das Basisformular
          nicht reicht. So bleibt die Elternansicht kurz und verständlich.
        </p>
      </div>

      {/* Der Erklärtext stand als freier Absatz unter der Karte und ist jetzt
          ihre description. */}
      <SectionCard
        title="Nächste Schritte"
        description="Für eine neue Halbjahresanmeldung brauchen Sie oft keine eigene Formularvorlage. Anmeldephase und Betreuungsangebote steuern den eigentlichen Ablauf."
        headingLevel={3}
      >
        <div className="space-y-3">
          <GuideStep
            icon={<MotoConceptIcon concept="permissions" size={18} />}
            title="Basisformular prüfen"
            done
          />
          <GuideStep
            icon={<MotoConceptIcon concept="enrollments" size={18} />}
            title="Vorlage nur bei Bedarf"
            done={templateCount > 0}
          />
          <GuideStep
            icon={<MotoConceptIcon concept="calendarPeriods" size={18} />}
            title="Vorlage in Anmeldephase wählen"
            done={assignedTemplateCount > 0}
          />
        </div>
      </SectionCard>
    </div>
  );
}

function FormTemplateDetail({
  schema,
  onBack,
  onEdit,
  assignedPhases,
}: Readonly<{
  schema: FormSchema;
  onBack: () => void;
  onEdit: () => void;
  assignedPhases: Phase[];
}>) {
  const tenantPath = useTenantAwarePath();
  const requiredCount = schema.fields.filter((field) =>
    Boolean(field.required),
  ).length;
  const childFieldCount = schema.fields.filter((field) =>
    Boolean(field.applies_to_child),
  ).length;

  return (
    // Flex-Spalte, siehe die Übersicht derselben Seite.
    <div className="flex flex-col space-y-5">
      <section className="moto-content-surface flex flex-col overflow-hidden rounded-2xl border shadow-sm backdrop-blur-md">
        <div className="grid flex-1 gap-0 lg:grid-cols-[minmax(0,1fr)_360px] xl:grid-cols-[minmax(0,1fr)_400px]">
          <div>
            <div className="border-b border-gray-100 px-5 py-3 sm:px-6">
              <BackAction onClick={onBack} label="Zurück zur Übersicht" />
            </div>
            <div className="space-y-5 p-5 sm:p-6">
              <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                <div>
                  <h2 className="text-xl font-semibold text-gray-900">
                    {schema.name}
                  </h2>
                  <p className="mt-2 max-w-3xl text-sm leading-6 text-gray-600">
                    Prüfen Sie die Formularvorschau und ordnen Sie diese Vorlage
                    einer Anmeldephase zu, wenn Eltern die angepassten
                    Pflichtangaben oder Zusatzfragen sehen sollen.
                  </p>
                </div>
                <Button
                  type="button"
                  onClick={onEdit}
                  variant="primary"
                  size="md"
                  className="inline-flex items-center justify-center gap-2 whitespace-nowrap"
                >
                  <Pencil className="h-4 w-4" aria-hidden="true" />
                  Bearbeiten
                </Button>
              </div>

              <div className="grid grid-cols-2 gap-3 xl:grid-cols-4">
                <FormMetric
                  icon={<FileText className="h-4 w-4" aria-hidden="true" />}
                  value={formatSchemaDate(schema.created_at)}
                  label="Zuletzt gespeichert"
                />
                <FormMetric
                  icon={<MotoConceptIcon concept="enrollments" size={16} />}
                  value={schema.fields.length.toString()}
                  label="Zusatzfragen"
                />
                <FormMetric
                  icon={<Check className="h-4 w-4" aria-hidden="true" />}
                  value={requiredCount.toString()}
                  label="Pflicht-Zusatzfragen"
                />
                <FormMetric
                  icon={<MotoConceptIcon concept="children" size={16} />}
                  value={childFieldCount.toString()}
                  label="Pro Kind"
                />
              </div>

              <FormPreview
                fields={schema.fields}
                coreRequirements={schema.core_requirements ?? {}}
                legalBlocks={schema.legal_blocks ?? []}
                templateName={schema.name}
                isActive={schema.is_active}
                isSaved
                previewHref={tenantPath(
                  `/anmeldung/preview?schemaId=${encodeURIComponent(schema.id)}`,
                )}
                assignedPhaseCount={assignedPhases.length}
                sticky={false}
              />
            </div>
          </div>

          <aside className="moto-dotted-background moto-dotted-background--split border-t border-gray-100 p-5 sm:p-6 lg:border-t-0 lg:border-l">
            <div className="sticky top-6 space-y-4">
              <div>
                <h2 className="text-base font-semibold text-gray-900">
                  {assignedPhases.length > 0
                    ? "In Anmeldephase verwendet"
                    : "In Anmeldephase verwenden"}
                </h2>
                <p className="mt-2 text-sm leading-6 text-gray-600">
                  {assignedPhases.length > 0
                    ? `Diese Vorlage ist in ${assignedPhases.length} Anmeldephase ausgewählt.`
                    : "Eine Vorlage wird erst für Eltern relevant, wenn sie in einer Anmeldephase ausgewählt ist. Ohne Auswahl nutzt die Phase das Basisformular."}
                </p>
              </div>

              <ButtonLink
                href={
                  assignedPhases.length > 0
                    ? "/enrollment-phases"
                    : `/enrollment-phases?assignForm=${encodeURIComponent(schema.id)}`
                }
                size="md"
                className="w-full gap-2"
              >
                <MotoConceptIcon
                  concept="calendarPeriods"
                  colorMode="inherit"
                  size={18}
                />
                {assignedPhases.length > 0
                  ? "Anmeldephasen öffnen"
                  : "In Anmeldephase auswählen"}
              </ButtonLink>

              <a
                href={tenantPath(
                  `/anmeldung/preview?schemaId=${encodeURIComponent(schema.id)}`,
                )}
                target="_blank"
                rel="noreferrer"
                className="moto-content-surface flex items-start gap-3 rounded-2xl border p-3 text-left shadow-sm transition-colors hover:border-gray-300 hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
              >
                <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-white text-gray-600 shadow-sm">
                  <ExternalLink className="h-4 w-4" aria-hidden="true" />
                </span>
                <span className="min-w-0">
                  <span className="block text-sm font-semibold text-gray-900">
                    Vorschau öffnen
                  </span>
                  <span className="mt-0.5 block text-xs leading-5 text-gray-500">
                    Öffnet die Formularvorschau in einem neuen Tab.
                  </span>
                </span>
              </a>

              <div className="moto-content-surface rounded-2xl border p-4 shadow-sm">
                <h3 className="text-sm font-semibold text-gray-900">Status</h3>
                <div className="mt-4 space-y-3">
                  <GuideStep
                    icon={<Check className="h-4 w-4" aria-hidden="true" />}
                    title="Vorlage gespeichert"
                    done
                  />
                  <GuideStep
                    icon={<Eye className="h-4 w-4" aria-hidden="true" />}
                    title="Vorschau prüfen"
                    done
                  />
                  <GuideStep
                    icon={
                      <MotoConceptIcon concept="calendarPeriods" size={18} />
                    }
                    title="In Anmeldephase auswählen"
                    done={assignedPhases.length > 0}
                  />
                </div>
              </div>
            </div>
          </aside>
        </div>
      </section>
    </div>
  );
}

function GuideStep({
  icon,
  title,
  done,
}: Readonly<{ icon: ReactNode; title: string; done: boolean }>) {
  return (
    <div className="flex items-center gap-3">
      <span
        className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-xl ${
          done
            ? "bg-moto-green/15 text-moto-green-vivid"
            : "bg-gray-100 text-gray-500"
        }`}
      >
        {icon}
      </span>
      <span className="min-w-0 flex-1 text-sm font-medium text-gray-700">
        {title}
      </span>
      <span
        className={`h-2 w-2 rounded-full ${
          done ? "bg-moto-green" : "bg-gray-300"
        }`}
        aria-hidden="true"
      />
    </div>
  );
}

function DeleteSchemaDialog({
  schema,
  deleting,
  error,
  onClose,
  onConfirm,
}: Readonly<{
  schema: FormSchema | null;
  deleting: boolean;
  error: FormErrorInput;
  onClose: () => void;
  onConfirm: () => void;
}>) {
  return (
    <ConfirmDeleteModal
      isOpen={schema !== null}
      title="Formularvorlage löschen"
      description={
        <div className="space-y-3 leading-6">
          <p>
            Die Vorlage{" "}
            <span className="font-semibold text-gray-900">{schema?.name}</span>{" "}
            wird dauerhaft gelöscht.
          </p>
          <p>
            Dabei werden alle Versionen dieser Vorlage entfernt. Bereits
            verwendete Vorlagen können nicht gelöscht werden.
          </p>
        </div>
      }
      gate={{ mode: "twoStep" }}
      onConfirm={onConfirm}
      onClose={onClose}
      loading={deleting}
      error={error}
    />
  );
}

function RenameSchemaDialog({
  schema,
  onClose,
  onConfirm,
}: Readonly<{
  schema: FormSchema | null;
  onClose: () => void;
  onConfirm: (newName: string) => Promise<void>;
}>) {
  const [value, setValue] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const {
    error,
    show: showError,
    fieldError,
    clear: clearError,
  } = useApiFormError(formRef);

  // Reset the field to the current name each time the dialog opens for a
  // schema so the admin edits from the existing value.
  useEffect(() => {
    if (schema) {
      setValue(schema.name);
      clearError();
      setSubmitting(false);
    }
  }, [schema, clearError]);

  const isOpen = schema !== null;
  const trimmed = value.trim();
  const canSubmit = isRenameOf(schema, value) && !submitting;

  // "Wiederholen" sends the name as it is then.
  const submitRef = useRef<() => Promise<void>>(async () => undefined);
  const submit = async () => {
    if (!canSubmit) return;
    setSubmitting(true);
    clearError();
    try {
      await onConfirm(trimmed);
    } catch (err) {
      logger.warn("schema_rename_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void showError(err, {
        object: "die Formularvorlage",
        retry: () => void submitRef.current(),
      });
      setSubmitting(false);
    }
  };
  useLayoutEffect(() => {
    submitRef.current = submit;
  });

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title="Formular umbenennen"
      size="sm"
      error={error}
      footer={
        <div className="flex justify-end gap-2">
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={onClose}
            disabled={submitting}
          >
            Abbrechen
          </Button>
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={() => void submit()}
            disabled={!canSubmit}
            isLoading={submitting}
          >
            Speichern
          </Button>
        </div>
      }
    >
      <form
        ref={formRef}
        onSubmit={(event) => {
          event.preventDefault();
          void submit();
        }}
        className="space-y-3"
      >
        <Input
          name="name"
          label="Name"
          type="text"
          value={value}
          onChange={(event) => setValue(event.target.value)}
          error={fieldError("name")}
          placeholder="z. B. Ferienbetreuung Sommer 2026"
          autoFocus
        />
        <p className="text-xs leading-5 text-gray-500">
          Der neue Name gilt für alle Versionen dieser Vorlage. Bereits
          abgeschickte Anmeldungen bleiben unverändert.
        </p>
      </form>
    </FormModal>
  );
}

function UnsavedChangesDialog({
  pendingNavigation,
  saving,
  saveBlockedMessage,
  onCancel,
  onDiscard,
  onSave,
}: Readonly<{
  pendingNavigation: PendingNavigation | null;
  saving: boolean;
  saveBlockedMessage: string | null;
  onCancel: () => void;
  onDiscard: () => void;
  onSave: () => void;
}>) {
  if (!pendingNavigation) return null;

  const isPreview = pendingNavigation === "preview";
  const footer = (
    <>
      <Button
        type="button"
        onClick={onCancel}
        disabled={saving}
        variant="outline"
        size="md"
        className="inline-flex items-center justify-center"
      >
        Abbrechen
      </Button>
      {!isPreview ? (
        <Button
          type="button"
          onClick={onDiscard}
          disabled={saving}
          variant="outline"
          size="md"
          className="inline-flex items-center justify-center"
        >
          Verwerfen
        </Button>
      ) : null}
      <Button
        type="button"
        onClick={onSave}
        disabled={saving || Boolean(saveBlockedMessage)}
        variant="primary"
        size="md"
        className="inline-flex items-center justify-center"
      >
        {saving
          ? "Speichert…"
          : isPreview
            ? "Speichern und Vorschau öffnen"
            : "Speichern und fortfahren"}
      </Button>
    </>
  );

  return (
    <Modal
      isOpen
      onClose={onCancel}
      title="Änderungen speichern?"
      widthClass="mx-4 w-[calc(100%-2rem)] max-w-lg"
      isDismissDisabled={saving}
      footer={footer}
    >
      <p className="text-sm leading-6 text-gray-600">
        {isPreview
          ? "Für die externe Vorschau muss die Vorlage zuerst gespeichert werden."
          : "Sie haben Änderungen an dieser Vorlage. Speichern Sie sie, bevor Sie den Bereich verlassen, oder verwerfen Sie sie bewusst."}
      </p>
      {saveBlockedMessage ? (
        <div className="border-moto-red/20 bg-moto-red/10 text-moto-red-strong mt-4 rounded-lg border px-3 py-2 text-sm font-medium">
          {saveBlockedMessage}
        </div>
      ) : null}
    </Modal>
  );
}

function FormBuilderIntro() {
  return (
    <header>
      <h2 className="text-xl font-semibold text-gray-900">
        Formularvorlage bearbeiten
      </h2>
      <p className="mt-2 max-w-3xl text-sm leading-6 text-gray-600">
        Legen Sie fest, welche optionalen Basisfelder verpflichtend sind, und
        ergänzen Sie bei Bedarf Zusatzfragen. Die Vorlage wird später in einer
        Anmeldephase ausgewählt.
      </p>
    </header>
  );
}

function FormMetric({
  icon,
  value,
  label,
}: Readonly<{
  icon: ReactNode;
  value: string;
  label: string;
}>) {
  return (
    <div className="moto-content-surface flex items-center gap-3 rounded-2xl border px-4 py-3 shadow-sm">
      <span className="flex h-9 w-9 items-center justify-center rounded-xl bg-white text-gray-600 shadow-sm">
        {icon}
      </span>
      <span>
        <span className="block text-lg leading-none font-semibold text-gray-900">
          {value}
        </span>
        <span className="mt-1 block text-xs font-medium text-gray-500">
          {label}
        </span>
      </span>
    </div>
  );
}

function BuilderTemplateSummary({
  name,
  isCreating,
  saving,
  currentSchema,
  onNameChange,
  fields,
}: Readonly<{
  name: string;
  isCreating: boolean;
  saving: boolean;
  currentSchema: FormSchema | null;
  onNameChange: (value: string) => void;
  fields: FormField[];
}>) {
  const nameError = fieldErrorProps(useContext(FieldErrorContext), "name");
  const requiredCount = fields.filter((field) =>
    Boolean(field.required),
  ).length;
  const childFieldCount = fields.filter((field) =>
    Boolean(field.applies_to_child),
  ).length;

  return (
    <section className="moto-content-surface rounded-2xl border p-4 shadow-sm">
      <div className="grid gap-4 md:grid-cols-[minmax(0,1fr)_280px]">
        <div className="min-w-0">
          <h2 className="text-base font-semibold text-gray-900">
            {isCreating ? "Neue Formularvorlage" : name}
          </h2>
          <p className="mt-1 max-w-2xl text-sm text-gray-600">
            {isCreating
              ? "Geben Sie der Vorlage einen eindeutigen Namen. Danach können Sie Pflichtangaben und Zusatzfragen festlegen."
              : "Der Name lässt sich hier ändern und gilt für alle Versionen. Bestehende Anmeldungen bleiben nachvollziehbar."}
          </p>
        </div>

        <div className="space-y-3">
          <Input
            id={nameError.controlId}
            name={nameError.controlProps.name}
            type="text"
            label="Name der Vorlage"
            controlSize="compact"
            value={name}
            onChange={(event) => onNameChange(event.target.value)}
            placeholder="z. B. Ferienbetreuung Sommer 2026"
            disabled={saving}
            error={nameError.message}
          />
          {!isCreating ? (
            <div className="flex flex-wrap content-start gap-2 text-xs text-gray-600">
              <span className="rounded-full bg-gray-100 px-2.5 py-1 font-medium text-gray-700">
                {fields.length} Zusatzfragen
              </span>
              <span className="rounded-full bg-gray-100 px-2.5 py-1 font-medium text-gray-700">
                {requiredCount} Pflicht-Zusatzfragen
              </span>
              <span className="rounded-full bg-gray-100 px-2.5 py-1 font-medium text-gray-700">
                {childFieldCount} pro Kind
              </span>
            </div>
          ) : null}
        </div>
      </div>

      {currentSchema ? (
        <div className="mt-4 flex flex-wrap gap-2 text-xs text-gray-600">
          <span className="rounded-full bg-gray-100 px-2.5 py-1 font-medium text-gray-700">
            {new Date(currentSchema.created_at).toLocaleString("de-DE", {
              timeZone: "Europe/Berlin",
            })}
          </span>
        </div>
      ) : null}
    </section>
  );
}

function CoreFieldsSection({
  coreRequirements,
  onRequirementChange,
  disabled,
}: Readonly<{
  coreRequirements: CoreRequirements;
  onRequirementChange: (key: CoreRequirementKey, required: boolean) => void;
  disabled: boolean;
}>) {
  const guardian = CORE_FIELDS.filter((f) => f.group === "guardian");
  const child = CORE_FIELDS.filter((f) => f.group === "child");
  const groups = [
    { title: "Eltern", fields: guardian },
    { title: "Kind", fields: child },
  ] satisfies ReadonlyArray<{ title: string; fields: CoreField[] }>;

  return (
    <section className="moto-content-surface rounded-2xl border p-5 shadow-sm">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h2 className="text-base font-semibold text-gray-900">
            Pflichtstatus der Basisfelder
          </h2>
          <p className="mt-1 max-w-2xl text-sm leading-6 text-gray-600">
            Fest gesetzte Felder bleiben immer Pflicht. Optionale Basisfelder
            können Sie hier verpflichtend machen.
          </p>
        </div>
        <span className="inline-flex w-fit items-center gap-1.5 rounded-full bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-600">
          <Lock className="h-3.5 w-3.5" aria-hidden="true" />
          Basisfelder
        </span>
      </div>

      <div className="mt-4 overflow-hidden rounded-xl border border-gray-200">
        {groups.map((group) => (
          <CoreFieldGroup
            key={group.title}
            title={group.title}
            fields={group.fields}
            coreRequirements={coreRequirements}
            onRequirementChange={onRequirementChange}
            disabled={disabled}
          />
        ))}
      </div>
    </section>
  );
}

// SignatureRequirementSection switches the parent's signature on for the
// whole form (#4786): the form then ends with a signature field after the
// legal texts, and every later edit by the parents asks for it again.
function SignatureRequirementSection({
  required,
  onChange,
  disabled,
}: Readonly<{
  required: boolean;
  onChange: (checked: boolean) => void;
  disabled: boolean;
}>) {
  return (
    <section className="moto-content-surface rounded-2xl border p-5 shadow-sm">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h2 className="text-base font-semibold text-gray-900">
            Unterschrift
          </h2>
          <p className="mt-1 max-w-2xl text-sm leading-6 text-gray-600">
            {required
              ? "Eltern unterschreiben am Ende des Formulars mit einer einfachen elektronischen Signatur mit gezeichneter Unterschrift. Ohne Unterschrift lässt sich die Anmeldung nicht absenden. Auch jede spätere Änderung durch die Eltern braucht eine neue Unterschrift."
              : "Eltern senden die Anmeldung ohne Unterschrift ab."}
          </p>
        </div>
        <div className="flex items-center justify-between gap-3 sm:justify-end">
          <span className="text-xs font-medium text-gray-600">
            {required ? "Erforderlich" : "Aus"}
          </span>
          <BooleanField
            value={required}
            onChange={onChange}
            disabled={disabled}
            ariaLabel="Unterschrift erforderlich"
          />
        </div>
      </div>
    </section>
  );
}

export function LegalBlocksSection({
  blocks,
  standardBlocks,
  customOnly = false,
  uploadAdapter,
  onChange,
  disabled,
  draftDocumentURLs,
  onDraftDocumentUploaded,
  onDraftDocumentDeleted,
  onUploadStart,
  onUploadEnd,
}: Readonly<{
  blocks: FormLegalBlock[];
  standardBlocks: FormLegalBlock[];
  customOnly?: boolean;
  uploadAdapter?: import("./attachment-editor").AttachmentUploadAdapter;
  onChange: Dispatch<SetStateAction<FormLegalBlock[]>>;
  disabled: boolean;
  draftDocumentURLs: ReadonlySet<string>;
  onDraftDocumentUploaded: (documentURL: string) => void;
  onDraftDocumentDeleted: (documentURL: string) => void;
  onUploadStart: () => void;
  onUploadEnd: () => void;
}>) {
  const toast = useToast();
  const { show: showActionError, showMessage } = useApiErrorDisplay();
  const fieldError = useContext(FieldErrorContext);
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
    if (
      file.type !== "application/pdf" &&
      !file.name.toLowerCase().endsWith(".pdf")
    ) {
      showMessage("Bitte wählen Sie eine PDF-Datei aus.");
      return;
    }
    if (file.size > MAX_ENROLLMENT_DOCUMENT_BYTES) {
      showMessage(
        "Die Datei ist größer als 10 MB. Bitte wählen Sie eine kleinere Datei.",
      );
      return;
    }
    setUploadingDocumentKey(blocks[index]?.key ?? null);
    onUploadStart();
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
      onDraftDocumentUploaded(documentURL);
      if (
        previousURL &&
        previousURL !== documentURL &&
        draftDocumentURLs.has(previousURL)
      ) {
        try {
          await deleteEnrollmentLegalDocument(previousURL);
          onDraftDocumentDeleted(previousURL);
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
      onUploadEnd();
    }
  };
  const removeAGBDocument = async (index: number) => {
    const documentURL = (blocks[index]?.document_url ?? "").trim();
    updateBlock(index, {
      document_url: "",
      display_mode: LEGAL_BLOCK_DISPLAY_MODE_TEXT,
    });
    if (!documentURL || !draftDocumentURLs.has(documentURL)) return;
    try {
      await deleteEnrollmentLegalDocument(documentURL);
      onDraftDocumentDeleted(documentURL);
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
    .filter(
      ({ block }) =>
        block.source === "standard" || standardByKey.has(block.key),
    );
  const customEntries = blocks
    .map((block, index) => ({ block, index }))
    .filter(
      ({ block }) =>
        block.source !== "standard" && !standardByKey.has(block.key),
    );
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
        className="moto-content-surface rounded-xl border p-4 shadow-sm"
      >
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <StyledCheckboxButton
            checked={block.enabled}
            disabled={disabled}
            onCheckedChange={(checked) =>
              updateBlock(index, { enabled: checked })
            }
          >
            Im Formular anzeigen
          </StyledCheckboxButton>
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
                className="w-fit text-sm"
              >
                Einstellungen wieder verwenden
              </Button>
            ) : null}
            {onRemove ? (
              <Button
                type="button"
                onClick={onRemove}
                disabled={disabled}
                variant="ghost"
                size="compact"
                className="text-moto-red-strong hover:bg-moto-red/10 hover:text-moto-red-strong w-fit gap-2 text-sm"
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

        {block.key === "agb" && block.source === "standard" ? (
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
          !(
            block.kind === "consent" &&
            (block.source === "standard" || standardByKey.has(block.key))
          ) ? (
            <StyledCheckboxButton
              checked={block.required}
              disabled={disabled}
              muted
              onCheckedChange={(checked) =>
                updateBlock(
                  index,
                  block.source === "standard" || standardByKey.has(block.key)
                    ? { required: checked }
                    : // Eine Pflicht-Checkbox ist eine Bestätigung (terms),
                      // eine freiwillige Checkbox eine Einwilligung (consent).
                      {
                        required: checked,
                        kind: checked ? "terms" : "consent",
                      },
                )
              }
            >
              Muss bestätigt werden
            </StyledCheckboxButton>
          ) : null}
        </div>
        {block.kind === "consent" ? (
          <p className="mt-2 text-xs leading-5 text-gray-500">
            Freiwillige Einwilligung: Eltern können das Formular auch ohne
            Häkchen abschicken.
          </p>
        ) : null}
        {block.kind === "terms" &&
        block.source !== "standard" &&
        !standardByKey.has(block.key) ? (
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
    <section className="moto-content-surface rounded-2xl border p-5 shadow-sm">
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
              className="moto-content-surface rounded-xl border p-4 shadow-sm"
            >
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <h3 className="text-sm font-semibold text-gray-900">
                      {block.title}
                    </h3>
                    <span className="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600">
                      Aus Einstellungen übernommen
                    </span>
                    {hasOverride ? (
                      <span className="bg-moto-amber/10 text-moto-amber-strong rounded-full px-2 py-0.5 text-xs font-medium">
                        Abweichung aktiv
                      </span>
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
                  <CompactSwitchButton
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
          <div className="rounded-xl border border-dashed border-gray-200 bg-white/70 p-4 text-sm leading-6 text-gray-600">
            {customOnly
              ? "Noch keine eigenen Hinweise oder Einwilligungen."
              : "Noch keine eigenen Zustimmungen. Füge hier nur zusätzliche Inhalte hinzu, die nicht in den allgemeinen Einstellungen stehen."}
          </div>
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
    </section>
  );
}

function hasStandardLegalBlockOverride(
  block: FormLegalBlock,
  standardBlock: FormLegalBlock,
): boolean {
  return (
    block.kind !== standardBlock.kind ||
    block.title !== standardBlock.title ||
    block.label !== standardBlock.label ||
    block.text !== standardBlock.text ||
    block.required !== standardBlock.required ||
    legalBlockDisplayMode(block) !== legalBlockDisplayMode(standardBlock) ||
    (block.document_url ?? "") !== (standardBlock.document_url ?? "") ||
    JSON.stringify(block.attachments ?? []) !==
      JSON.stringify(standardBlock.attachments ?? [])
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
            <span className="mt-2 inline-flex rounded-full bg-white px-2 py-0.5 text-xs font-medium text-gray-600 ring-1 ring-gray-200">
              Text gespeichert
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
            <span className="mt-2 inline-flex rounded-full bg-white px-2 py-0.5 text-xs font-medium text-gray-600 ring-1 ring-gray-200">
              PDF gespeichert
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
        <div className="border-moto-blue/20 bg-moto-blue/5 rounded-xl border p-3">
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
              <label
                className={`border-moto-blue/25 hover:bg-moto-blue/10 text-moto-blue-hover inline-flex cursor-pointer items-center gap-1.5 rounded-lg border bg-white px-2.5 py-1.5 font-medium shadow-sm transition-colors ${
                  disabled || documentSaving
                    ? "pointer-events-none opacity-50"
                    : ""
                }`}
              >
                <FileUp className="h-3.5 w-3.5" aria-hidden="true" />
                <span>{hasDocument ? "PDF ersetzen" : "PDF hochladen"}</span>
                <input
                  type="file"
                  accept="application/pdf,.pdf"
                  className="sr-only"
                  disabled={disabled || documentSaving}
                  onChange={(event) => {
                    onDocumentUpload(event.currentTarget.files?.[0] ?? null);
                    event.currentTarget.value = "";
                  }}
                />
              </label>
              {hasDocument ? (
                <a
                  href={publicAGBDocumentURL(documentURL)}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-2.5 py-1.5 font-medium text-gray-600 shadow-sm transition-colors hover:bg-gray-50 hover:text-gray-900"
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
            <p className="mt-2 text-xs text-gray-500">
              PDF wird hochgeladen...
            </p>
          ) : null}
        </div>
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

function StyledCheckboxButton({
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
    <button
      type="button"
      role="checkbox"
      aria-checked={checked}
      disabled={disabled}
      onClick={() => onCheckedChange(!checked)}
      className={`inline-flex min-w-0 items-center gap-2 rounded-xl text-left focus:outline-none focus-visible:ring-2 focus-visible:ring-gray-300 disabled:cursor-not-allowed disabled:opacity-50 ${
        muted
          ? "text-sm font-medium text-gray-700"
          : "text-sm font-medium text-gray-800"
      }`}
    >
      <span
        className={`flex h-5 w-5 shrink-0 items-center justify-center rounded-md border shadow-sm transition-all ${
          checked ? "border-gray-900 bg-gray-900" : "border-gray-300 bg-white"
        }`}
        aria-hidden="true"
      >
        <Check
          className={`h-3.5 w-3.5 text-white transition-opacity ${
            checked ? "opacity-100" : "opacity-0"
          }`}
        />
      </span>
      <span>{children}</span>
    </button>
  );
}

function CompactSwitchButton({
  checked,
  disabled,
  ariaLabel,
  onCheckedChange,
}: Readonly<{
  checked: boolean;
  disabled: boolean;
  ariaLabel: string;
  onCheckedChange: (checked: boolean) => void;
}>) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={ariaLabel}
      disabled={disabled}
      onClick={() => onCheckedChange(!checked)}
      className={`relative inline-flex h-5 w-9 shrink-0 items-center rounded-full transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50 ${
        checked ? "bg-gray-900" : "bg-gray-200"
      }`}
    >
      <span
        className={`pointer-events-none inline-block h-4 w-4 rounded-full bg-white shadow-sm transition-transform ${
          checked ? "translate-x-[18px]" : "translate-x-0.5"
        }`}
        aria-hidden="true"
      />
    </button>
  );
}

function checkboxLegalBlockKindFor(
  block: FormLegalBlock,
): FormLegalBlock["kind"] {
  if (block.key === "agb") return "terms";
  if (block.key === "data_processing") return "privacy_notice";
  return block.required ? "terms" : "consent";
}

function CoreFieldGroup({
  title,
  fields,
  coreRequirements,
  onRequirementChange,
  disabled,
}: Readonly<{
  title: string;
  fields: CoreField[];
  coreRequirements: CoreRequirements;
  onRequirementChange: (key: CoreRequirementKey, required: boolean) => void;
  disabled: boolean;
}>) {
  return (
    <div className="border-b border-gray-100 last:border-b-0">
      <div className="bg-gray-50/80 px-4 py-2">
        <h3 className="text-sm font-semibold text-gray-900">{title}</h3>
      </div>
      <ul className="divide-y divide-gray-100">
        {fields.map((field) => (
          <CoreFieldRow
            key={field.key}
            field={field}
            required={Boolean(
              field.requirementKey
                ? coreRequirements[field.requirementKey]
                : field.required,
            )}
            onRequirementChange={onRequirementChange}
            disabled={disabled}
          />
        ))}
      </ul>
    </div>
  );
}

function CoreFieldRow({
  field,
  required,
  onRequirementChange,
  disabled,
}: {
  readonly field: CoreField;
  readonly required: boolean;
  readonly onRequirementChange: (
    key: CoreRequirementKey,
    required: boolean,
  ) => void;
  readonly disabled: boolean;
}) {
  return (
    <li className="grid gap-3 bg-white px-4 py-3 sm:grid-cols-[minmax(0,1fr)_220px] sm:items-center">
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-2">
          <p className="text-sm font-medium text-gray-900">{field.label}</p>
          <span className="rounded bg-gray-100 px-1.5 py-0.5 text-xs text-gray-500">
            {fieldTypeLabels[field.type]}
          </span>
        </div>
        <p className="mt-0.5 text-xs leading-5 text-gray-500">
          {field.requirementKey
            ? required
              ? (field.requirementHint ?? "Diese Angabe ist verpflichtend.")
              : "Kann verpflichtend gemacht werden."
            : "Immer erforderlich und deshalb nicht änderbar."}
        </p>
      </div>
      {field.requirementKey ? (
        <div className="flex items-center justify-between gap-3 sm:justify-end">
          <span className="text-xs font-medium text-gray-600">
            {required ? "Pflicht" : "Optional"}
          </span>
          <BooleanField
            value={required}
            onChange={(checked) =>
              onRequirementChange(field.requirementKey!, checked)
            }
            disabled={disabled}
            ariaLabel={field.requirementLabel ?? `${field.label} Pflichtfeld`}
          />
        </div>
      ) : (
        <div className="flex justify-start sm:justify-end">
          <span className="inline-flex items-center gap-1.5 rounded-full bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-600">
            <Lock className="h-3.5 w-3.5" aria-hidden="true" />
            Immer Pflicht
          </span>
        </div>
      )}
    </li>
  );
}

function TargetSuggestions({
  fields,
  onAdd,
  disabled,
}: Readonly<{
  fields: FormField[];
  onAdd: (target: Exclude<FormFieldTarget, "">) => void;
  disabled: boolean;
}>) {
  const selectedTargets = useMemo(
    () => new Set(fields.map((field) => field.target).filter(Boolean)),
    [fields],
  );

  return (
    <section className="rounded-2xl border border-gray-200 bg-gray-50/70 p-4">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h3 className="text-sm font-semibold text-gray-900">
            Stammdaten-Vorschläge
          </h3>
          <p className="mt-1 max-w-2xl text-xs leading-5 text-gray-600">
            Diese Fragen sind mit vorhandenen Stammdaten verbunden. Sie können
            sie hinzufügen oder entfernen; Label und Typ sind fest vorgegeben.
          </p>
        </div>
        <span className="inline-flex w-fit items-center gap-1.5 rounded-full bg-white px-2.5 py-1 text-xs font-medium text-gray-600 shadow-sm">
          <Lock className="h-3.5 w-3.5" aria-hidden="true" />
          Stammdaten
        </span>
      </div>

      <div className="mt-4 grid gap-2 md:grid-cols-2">
        {TARGET_PICKER_ORDER.map((target) => {
          const selected = selectedTargets.has(target);
          return (
            <ChoiceTile
              key={target}
              as="button"
              selected={selected}
              disabled={disabled || selected}
              onClick={() => onAdd(target)}
              className="min-h-24 items-start p-3 shadow-sm"
            >
              <span
                className={`mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-lg ${
                  selected
                    ? "bg-gray-100 text-gray-500"
                    : "bg-gray-900 text-white"
                }`}
              >
                {selected ? (
                  <Check className="h-4 w-4" aria-hidden="true" />
                ) : (
                  <Plus className="h-4 w-4" aria-hidden="true" />
                )}
              </span>
              <span className="min-w-0">
                <span className="block text-sm font-semibold text-gray-900">
                  {RESERVED_TARGETS[target].label}
                </span>
                <span className="mt-1 block text-xs leading-5 text-gray-500">
                  {targetSuggestionDescriptions[target]}
                </span>
                <span className="mt-2 inline-flex rounded-full bg-gray-100 px-2 py-0.5 text-sm font-medium text-gray-600">
                  {selected ? "Ist drin" : "Hinzufügen"}
                </span>
              </span>
            </ChoiceTile>
          );
        })}
      </div>
    </section>
  );
}

function FormPreview({
  fields,
  coreRequirements,
  legalBlocks,
  templateName,
  isActive,
  isSaved,
  previewHref,
  onPreviewClick,
  assignedPhaseCount = 0,
  sticky = true,
}: Readonly<{
  fields: FormField[];
  coreRequirements: CoreRequirements;
  legalBlocks: FormLegalBlock[];
  templateName: string;
  isActive: boolean;
  isSaved: boolean;
  previewHref?: string;
  onPreviewClick?: () => void;
  assignedPhaseCount?: number;
  sticky?: boolean;
}>) {
  const previewStatus = getPreviewStatus({
    assignedPhaseCount,
    isActive,
    isSaved,
  });
  const enabledLegalBlocks = legalBlocks.filter((block) => block.enabled);
  const guardianFields: PreviewFieldLabel[] = [
    { label: "Vorname", required: true },
    { label: "Nachname", required: true },
    { label: "E-Mail", required: true },
    { label: "Telefonnummer", required: coreRequirements.guardian_phone },
  ];

  return (
    <SharedFormPreview
      sticky={sticky}
      title={templateName.trim() || "Basisformular"}
      header={
        <>
          <FormPreviewHeading
            title="Elternformular"
            hint={previewStatus.hint}
            status={
              <span
                className={`inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-medium ${previewStatus.className}`}
              >
                <span
                  className={`h-1.5 w-1.5 rounded-full ${previewStatus.dotClassName}`}
                  aria-hidden="true"
                />
                {previewStatus.label}
              </span>
            }
          />

          {previewHref ? (
            <a
              href={previewHref}
              target="_blank"
              rel="noreferrer"
              className="moto-content-surface flex items-start gap-3 rounded-2xl border p-3 text-left shadow-sm transition-colors hover:border-gray-300 hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
            >
              <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-white text-gray-600 shadow-sm">
                <ExternalLink className="h-4 w-4" aria-hidden="true" />
              </span>
              <span className="min-w-0">
                <span className="block text-sm font-semibold text-gray-900">
                  Vorschau öffnen
                </span>
                <span className="mt-0.5 block text-xs leading-5 text-gray-500">
                  Öffnet diese Formularvorlage in einem neuen Tab.
                </span>
              </span>
            </a>
          ) : onPreviewClick ? (
            <button
              type="button"
              onClick={onPreviewClick}
              className="moto-content-surface flex w-full items-start gap-3 rounded-2xl border p-3 text-left shadow-sm transition-colors hover:border-gray-300 hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
            >
              <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-white text-gray-600 shadow-sm">
                <ExternalLink className="h-4 w-4" aria-hidden="true" />
              </span>
              <span className="min-w-0">
                <span className="block text-sm font-semibold text-gray-900">
                  Vorschau öffnen
                </span>
                <span className="mt-0.5 block text-xs leading-5 text-gray-500">
                  Speichert die Vorlage und öffnet die Vorschau in einem neuen
                  Tab.
                </span>
              </span>
            </button>
          ) : (
            <div className="moto-content-surface flex items-start gap-3 rounded-2xl border p-3 text-left opacity-70 shadow-sm">
              <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-white text-gray-600 shadow-sm">
                <ExternalLink className="h-4 w-4" aria-hidden="true" />
              </span>
              <span className="min-w-0">
                <span className="block text-sm font-semibold text-gray-900">
                  Vorschau öffnen
                </span>
                <span className="mt-0.5 block text-xs leading-5 text-gray-500">
                  Speichern Sie die Vorlage zuerst.
                </span>
              </span>
            </div>
          )}
        </>
      }
    >
      <PreviewSection title="Elternteil" fields={guardianFields} />
      <PreviewSection
        title="Kind"
        fields={[
          { label: "Vorname", required: true },
          { label: "Nachname", required: true },
          { label: "Geburtsdatum", required: true },
          { label: "Klassenstufe", required: true },
        ]}
      />
      <PreviewSection
        title="Betreuungsangebot"
        fields={["Gewünschtes Angebot", "Betreuungstage"]}
      />

      <PreviewFieldsSection
        title="Zusatzfragen"
        fields={fields}
        emptyTitle="Keine Zusatzfragen"
        emptyText="Eltern sehen nur das Basisformular mit den gewählten Pflichtangaben."
      />

      {enabledLegalBlocks.length > 0 ? (
        <section>
          <div className="mb-3 flex items-center justify-between gap-3">
            <h4 className="text-sm font-semibold text-gray-900">
              Zustimmungen & Hinweise
            </h4>
            <span className="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600">
              {enabledLegalBlocks.length}
            </span>
          </div>
          <div className="space-y-2">
            {enabledLegalBlocks.map((block) => {
              const previewText = legalBlockPreviewText(block);
              return (
                <div
                  key={block.key}
                  className="rounded-lg border border-gray-200 bg-white px-3 py-2"
                >
                  <div className="flex items-start gap-2.5">
                    <span
                      className={`mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center rounded border ${
                        block.kind === "notice"
                          ? "border-moto-blue/30 bg-moto-blue/10"
                          : "border-gray-300 bg-white"
                      }`}
                      aria-hidden="true"
                    >
                      {block.kind === "notice" ? (
                        <Info className="text-moto-blue h-3 w-3" />
                      ) : null}
                    </span>
                    <div className="min-w-0 flex-1">
                      <div className="flex items-start justify-between gap-2">
                        <span className="text-sm font-medium text-gray-900">
                          {block.title.trim() || block.label}
                        </span>
                        <span className="shrink-0 rounded-full bg-gray-100 px-2 py-0.5 text-sm font-medium text-gray-600">
                          {block.kind === "notice"
                            ? "Hinweis"
                            : block.required
                              ? "Pflicht"
                              : "Optional"}
                        </span>
                      </div>
                      <p className="mt-1 line-clamp-2 text-xs leading-5 text-gray-500">
                        {block.label}
                      </p>
                      <PreviewAttachmentNames attachments={block.attachments} />
                      {previewText.trim() !== "" ? (
                        <p className="mt-1 line-clamp-2 text-sm leading-5 text-gray-400">
                          {previewText}
                        </p>
                      ) : null}
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        </section>
      ) : null}
    </SharedFormPreview>
  );
}

function getPreviewStatus({
  assignedPhaseCount,
  isActive,
  isSaved,
}: Readonly<{
  assignedPhaseCount: number;
  isActive: boolean;
  isSaved: boolean;
}>) {
  if (assignedPhaseCount > 0) {
    return {
      label: "In Phase verwendet",
      hint: `Diese Vorlage ist in ${assignedPhaseCount} Anmeldephase ausgewählt.`,
      className: "bg-moto-green/10 text-moto-green-vivid",
      dotClassName: "bg-moto-green",
    };
  }

  if (isActive) {
    return {
      label: "Bereit zur Zuordnung",
      hint: "Eltern sehen diese Vorlage erst, wenn sie einer Anmeldephase zugeordnet ist.",
      className: "bg-moto-green/10 text-moto-green-vivid",
      dotClassName: "bg-moto-green",
    };
  }

  if (isSaved) {
    return {
      label: "Gespeichert",
      hint: "Ordne die Vorlage in einer Anmeldephase zu, damit Eltern sie verwenden.",
      className: "bg-gray-100 text-gray-600",
      dotClassName: "bg-gray-300",
    };
  }

  return {
    label: "Entwurf",
    hint: "Speichern Sie die Vorlage zuerst. Danach können Sie sie in einer Anmeldephase auswählen.",
    className: "bg-gray-100 text-gray-600",
    dotClassName: "bg-gray-300",
  };
}

function createTargetField(
  target: Exclude<FormFieldTarget, "">,
  sortOrder: number,
  existing?: FormField,
): FormField {
  const spec = RESERVED_TARGETS[target];
  return {
    key: target.replace(/\./g, "_"),
    label: spec.label,
    type: spec.type,
    required: Boolean(existing?.required),
    help_text: "",
    options: getTargetOptions(target),
    sort_order: sortOrder,
    applies_to_child: spec.appliesToChild,
    target,
  };
}

function getTargetOptions(
  _target: Exclude<FormFieldTarget, "">,
): FormField["options"] {
  // No reserved target uses static select options anymore. Abholregelung is
  // a weekday_boolean (the parent picks the pickup weekdays, just like the
  // Buskind field); structured types carry no options.
  return [];
}

/**
 * Every text of the template that parents read, in form order. The id names
 * the owner (`field|3|label`, `option|3|1|label`, `legal|0|title`) so
 * applySchemaTranslation can route the edit back.
 */
function schemaTranslationTargets(
  fields: FormField[],
  legalBlocks: FormLegalBlock[],
): TranslationTarget[] {
  const targets: TranslationTarget[] = [];
  const add = (target: TranslationTarget) => {
    if (target.german.trim() !== "") targets.push(target);
  };
  legalBlocks.forEach((block, index) => {
    if (!block.enabled) return;
    const { translations } = block;
    add({
      id: `legal|${index}|title`,
      caption: "Zustimmung: Titel",
      german: block.title,
      attr: "title",
      translations,
    });
    add({
      id: `legal|${index}|label`,
      caption: "Zustimmung: Text neben der Checkbox",
      german: block.label,
      attr: "label",
      translations,
    });
    // A PDF block shows a link to the file instead of its text.
    if (legalBlockDisplayMode(block) !== LEGAL_BLOCK_DISPLAY_MODE_PDF) {
      add({
        id: `legal|${index}|text`,
        caption: "Zustimmung: Rechtstext / Erklärung",
        german: block.text,
        attr: "text",
        translations,
        multiline: true,
      });
    }
  });
  fields.forEach((field, index) => {
    const { translations } = field;
    const isInfo = field.type === "information";
    add({
      id: `field|${index}|label`,
      caption: isInfo ? "Infotext: Überschrift" : "Frage",
      german: field.label,
      attr: "label",
      translations,
    });
    add({
      id: `field|${index}|content`,
      caption: "Infotext: Inhalt",
      german: isInfo ? (field.content ?? "") : "",
      attr: "content",
      translations,
      multiline: true,
    });
    add({
      id: `field|${index}|help_text`,
      caption: "Hinweis zur Frage",
      german: isInfo ? "" : (field.help_text ?? ""),
      attr: "help_text",
      translations,
    });
    field.options?.forEach((option, optionIndex) =>
      add({
        id: `option|${index}|${optionIndex}|label`,
        caption: `Antwort zu „${field.label}“`,
        german: option.label,
        attr: "label",
        translations: option.translations,
      }),
    );
  });
  return targets;
}

export function prepareFieldsForSave(fields: FormField[]): FormField[] {
  return fields.map((field, index) => {
    if (field.target) {
      // Suggested field: type/options/scope are fixed by the spec, but
      // the admin-edited label + help text + required flag + visibility
      // ride along.
      const base = createTargetField(field.target, index, field);
      return {
        ...base,
        label: field.label.trim() || base.label,
        help_text: field.help_text?.trim() ?? "",
        required: Boolean(field.required),
        visible_when: field.visible_when ?? undefined,
        translations: field.translations,
        // Carry the admin-configured fixed pickup times through; the
        // rebuilt base field drops them otherwise. Only the pickup-times
        // field may carry this (arrival stays free-entry, and the backend
        // rejects allowed_times on any other target), so anything else is
        // forced to undefined and omitted on serialize.
        allowed_times:
          field.target === "schedule.pickup" ? field.allowed_times : undefined,
        // Heimweg-Beschränkung (#2381): only the allowed-departure-modes
        // field may carry grade restrictions; the backend rejects the key
        // on any other target.
        single_mode_grades:
          field.target === "student.allowed_departure_modes" &&
          (field.single_mode_grades?.length ?? 0) > 0
            ? field.single_mode_grades
            : undefined,
      };
    }
    if (field.type === "information") {
      return {
        key:
          field.key.trim() ||
          normalizeFieldKey(field.label) ||
          `infotext_${index + 1}`,
        label: field.label.trim(),
        type: "information",
        content: field.content?.trim() ?? "",
        sort_order: index,
        applies_to_child: Boolean(field.applies_to_child),
        visible_when: field.visible_when ?? undefined,
        translations: field.translations,
        attachments: field.attachments,
      };
    }
    return applyFileFieldRules({
      ...field,
      key: field.key.trim() || normalizeFieldKey(field.label),
      label: field.label.trim(),
      help_text: field.help_text?.trim() ?? "",
      sort_order: index,
      visible_when: field.visible_when ?? undefined,
    });
  });
}

function mergeStandardLegalBlocks(
  legalTexts: PublicLegalTexts | null,
): FormLegalBlock[] {
  const byKey = new Map(
    (legalTexts?.blocks ?? []).map((block) => [block.key, block]),
  );
  return STANDARD_LEGAL_BLOCKS.map((standard) => {
    const configured = byKey.get(standard.key);
    const inheritedText = standardLegalBlockText(standard.key, legalTexts);
    const inheritedEnabled = standardLegalBlockEnabled(
      standard.key,
      legalTexts,
    );
    if (!configured) {
      return {
        ...standard,
        text: inheritedText,
        enabled: inheritedEnabled,
        display_mode: standardLegalBlockDisplayMode(standard.key, legalTexts),
        document_url: standardLegalBlockDocumentURL(standard.key, legalTexts),
      };
    }
    return {
      ...standard,
      kind: configured.kind,
      title: configured.title,
      label: configured.label,
      text:
        standard.key === "agb"
          ? inheritedText
          : configured.text || inheritedText,
      required: configured.required,
      enabled: inheritedEnabled,
      sort_order: configured.sort_order ?? standard.sort_order,
      source: "standard",
      display_mode: standardLegalBlockDisplayMode(standard.key, legalTexts),
      document_url: standardLegalBlockDocumentURL(standard.key, legalTexts),
    };
  });
}

function standardLegalBlockText(
  key: string,
  legalTexts: PublicLegalTexts | null,
): string {
  if (!legalTexts) return "";
  switch (key) {
    case "agb":
      return standardAGBText(legalTexts);
    case "data_processing":
      return legalTexts.dsgvo;
    case "photo":
      return legalTexts.photo;
    case "email_contact":
      return legalTexts.email_contact;
    default:
      return "";
  }
}

function standardAGBText(legalTexts: PublicLegalTexts): string {
  if (legalTexts.agb_display_mode === LEGAL_BLOCK_DISPLAY_MODE_PDF) {
    return "";
  }
  return legalTexts.agb;
}

function standardLegalBlockDisplayMode(
  key: string,
  legalTexts: PublicLegalTexts | null,
): LegalBlockDisplayMode {
  if (key === "agb" && legalTexts?.agb_display_mode === "pdf") {
    return LEGAL_BLOCK_DISPLAY_MODE_PDF;
  }
  return LEGAL_BLOCK_DISPLAY_MODE_TEXT;
}

function standardLegalBlockDocumentURL(
  key: string,
  legalTexts: PublicLegalTexts | null,
): string {
  if (key !== "agb") return "";
  if (legalTexts?.agb_display_mode !== LEGAL_BLOCK_DISPLAY_MODE_PDF) return "";
  return (legalTexts?.agb_document_url ?? "").trim();
}

function legalBlockDisplayMode(block: FormLegalBlock): LegalBlockDisplayMode {
  return block.display_mode === LEGAL_BLOCK_DISPLAY_MODE_PDF
    ? LEGAL_BLOCK_DISPLAY_MODE_PDF
    : LEGAL_BLOCK_DISPLAY_MODE_TEXT;
}

function legalBlockPreviewText(block: FormLegalBlock): string {
  if (
    block.key === "agb" &&
    legalBlockDisplayMode(block) === LEGAL_BLOCK_DISPLAY_MODE_PDF
  ) {
    const documentURL = (block.document_url ?? "").trim();
    if (documentURL === "") return "";
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

function standardLegalBlockEnabled(
  key: string,
  legalTexts: PublicLegalTexts | null,
): boolean {
  if (!legalTexts) return false;
  switch (key) {
    case "agb":
      if (!legalTexts.terms_enabled) return false;
      if (
        standardLegalBlockDisplayMode(key, legalTexts) ===
        LEGAL_BLOCK_DISPLAY_MODE_PDF
      ) {
        return standardLegalBlockDocumentURL("agb", legalTexts) !== "";
      }
      return standardAGBText(legalTexts).trim() !== "";
    case "data_processing":
      return legalTexts.dsgvo_enabled && legalTexts.dsgvo.trim() !== "";
    case "photo":
      return legalTexts.photo_enabled && legalTexts.photo.trim() !== "";
    case "email_contact":
      return (
        legalTexts.email_contact_enabled &&
        legalTexts.email_contact.trim() !== ""
      );
    default:
      return false;
  }
}

function mergeSavedLegalBlocks(
  savedBlocks: FormLegalBlock[],
  standardBlocks: FormLegalBlock[],
): FormLegalBlock[] {
  const standardKeys = new Set(standardBlocks.map((block) => block.key));
  const savedByKey = new Map(savedBlocks.map((block) => [block.key, block]));
  const mergedStandardBlocks = standardBlocks.map(
    (standard) => savedByKey.get(standard.key) ?? standard,
  );
  const customBlocks = savedBlocks.filter(
    (block) => block.source !== "standard" && !standardKeys.has(block.key),
  );
  return [...mergedStandardBlocks, ...customBlocks];
}

function prepareLegalBlocksForSave(blocks: FormLegalBlock[]): FormLegalBlock[] {
  return blocks.map((block, index) => {
    const displayMode = legalBlockDisplayMode(block);
    const documentURL =
      displayMode === LEGAL_BLOCK_DISPLAY_MODE_PDF
        ? (block.document_url ?? "").trim()
        : "";
    // Einwilligungen sind immer freiwillig (Art. 7 Abs. 4 DSGVO): eine als
    // Pflicht markierte Custom-Checkbox ist eine Bestätigung (terms), keine
    // Einwilligung. Legacy-Blöcke mit consent+required werden beim nächsten
    // Speichern entsprechend umgeschrieben; das Backend lehnt consent+required ab.
    const kind =
      (block.source ?? "custom") !== "standard" &&
      block.kind === "consent" &&
      block.required
        ? "terms"
        : block.kind;
    return {
      key: normalizeFieldKey(block.key) || `custom_consent_${index + 1}`,
      kind,
      title: block.title.trim(),
      label: block.label.trim(),
      text: block.text.trim(),
      required:
        kind === "notice" || kind === "consent"
          ? false
          : Boolean(block.required),
      enabled: Boolean(block.enabled),
      sort_order: index * 10 + 10,
      source: block.source ?? "custom",
      display_mode: displayMode,
      document_url: documentURL,
      translations: block.translations,
      attachments: block.attachments?.map((attachment) => ({
        label: attachment.label.trim(),
        document_url: attachment.document_url,
      })),
    };
  });
}

function draftDocumentURLsInLegalBlocks(
  blocks: FormLegalBlock[],
  draftURLs: ReadonlySet<string>,
): Set<string> {
  const urls = new Set<string>();
  for (const block of blocks) {
    for (const attachment of block.attachments ?? []) {
      if (draftURLs.has(attachment.document_url))
        urls.add(attachment.document_url);
    }
    const documentURL = (block.document_url ?? "").trim();
    if (
      block.display_mode === LEGAL_BLOCK_DISPLAY_MODE_PDF &&
      documentURL &&
      draftURLs.has(documentURL)
    ) {
      urls.add(documentURL);
    }
  }
  return urls;
}

function nextCustomLegalBlockKey(blocks: FormLegalBlock[]): string {
  const usedKeys = new Set(blocks.map((block) => block.key));
  let nextIndex = Math.max(
    blocks.length + 1,
    ...blocks.map((block) => {
      const match = /^custom_consent_(\d+)$/.exec(block.key);
      return match ? Number(match[1]) + 1 : 0;
    }),
  );

  while (usedKeys.has(`custom_consent_${nextIndex}`)) {
    nextIndex += 1;
  }

  return `custom_consent_${nextIndex}`;
}

function legalBlocksSignature(blocks: FormLegalBlock[]): string {
  return JSON.stringify(prepareLegalBlocksForSave(blocks));
}

function coreRequirementsSignature(value: CoreRequirements): string {
  const enabled = Object.entries(value)
    .filter(([, required]) => required)
    .map(([key]) => key)
    .sort((a, b) => a.localeCompare(b));
  return JSON.stringify(Object.fromEntries(enabled.map((key) => [key, true])));
}

/** A check before saving: the sentence for the alert and the control to mark. */
interface SchemaDraftProblem {
  readonly message: string;
  readonly field?: string;
  readonly hint?: string;
}

const FILL_IN_HINT = "Bitte füllen Sie dieses Feld aus.";

function getSchemaDraftValidationMessage({
  fields,
  legalBlocks,
  name,
}: Readonly<{
  fields: FormField[];
  legalBlocks: FormLegalBlock[];
  name: string;
}>): SchemaDraftProblem | null {
  if (name.trim() === "") {
    return {
      message: "Bitte geben Sie zuerst einen Namen für die Vorlage ein.",
      field: "name",
      hint: FILL_IN_HINT,
    };
  }

  for (const [prefix, entries] of [
    ["fields", fields],
    ["legal_blocks", legalBlocks],
  ] as const) {
    for (const [index, entry] of entries.entries()) {
      const attachmentIndex =
        entry.attachments?.findIndex((a) => a.label.trim() === "") ?? -1;
      if (attachmentIndex >= 0)
        return {
          message: "Bitte geben Sie jeder PDF-Datei eine Bezeichnung.",
          field: `${prefix}.${index}.attachments.${attachmentIndex}.label`,
          hint: FILL_IN_HINT,
        };
    }
  }

  const seenKeys = new Set<string>();
  for (const [index, field] of fields.entries()) {
    const position = index + 1;

    if (field.type === "information") {
      if ((field.content ?? "").trim() === "") {
        return {
          message: `Bitte geben Sie für den Infotext ${position} einen Text ein.`,
          field: `fields.${index}.content`,
          hint: FILL_IN_HINT,
        };
      }
      const infoKey =
        field.key.trim() ||
        normalizeFieldKey(field.label) ||
        `infotext_${position}`;
      if (seenKeys.has(infoKey)) {
        return {
          message: `Bitte ändern Sie den Titel von Infotext ${position}. Ein anderer Eintrag hat schon denselben Titel.`,
          field: `fields.${index}.label`,
          hint: "Bitte wählen Sie einen anderen Titel.",
        };
      }
      seenKeys.add(infoKey);
      continue;
    }

    if (field.label.trim() === "") {
      return {
        message: `Bitte geben Sie für Frage ${position} einen Fragetext ein.`,
        field: `fields.${index}.label`,
        hint: FILL_IN_HINT,
      };
    }

    const key = field.key.trim() || normalizeFieldKey(field.label);
    if (key === "") {
      return {
        message: `Bitte ändern Sie den Fragetext von Frage ${position}. Er braucht mindestens einen Buchstaben oder eine Zahl.`,
        field: `fields.${index}.label`,
        hint: "Bitte verwenden Sie Buchstaben oder Zahlen.",
      };
    }
    if (seenKeys.has(key)) {
      return {
        message: `Bitte ändern Sie den Fragetext von Frage ${position}. Eine andere Zusatzfrage hat denselben oder einen sehr ähnlichen Text.`,
        field: `fields.${index}.label`,
        hint: "Bitte wählen Sie einen anderen Fragetext.",
      };
    }
    seenKeys.add(key);

    if (!field.target && structuredFieldTypes.has(field.type)) {
      return {
        message: `Bitte wählen Sie für Frage ${position} einen einfachen Typ. Telefonlisten, Wochenzeiten und Kontaktlisten gibt es nur als feste Vorschläge.`,
      };
    }

    const fileProblem = fileFieldDraftProblem(field, index);
    if (fileProblem) return fileProblem;

    if (field.type === "select" && (field.options ?? []).length === 0) {
      return {
        message: `Bitte ergänzen Sie für Frage ${position} mindestens eine Auswahloption.`,
        field: `fields.${index}.options`,
        hint: "Bitte tragen Sie mindestens eine Option ein.",
      };
    }
  }

  const seenLegalKeys = new Set<string>();
  for (const [index, block] of prepareLegalBlocksForSave(
    legalBlocks,
  ).entries()) {
    const position = index + 1;
    if (seenLegalKeys.has(block.key)) {
      // The key is never shown; only removing the duplicate helps.
      return {
        message: `Bitte entfernen Sie Zustimmung ${position} und legen Sie sie neu an. Sie ist doppelt vorhanden.`,
      };
    }
    seenLegalKeys.add(block.key);
    if (!block.enabled) continue;
    if (block.title === "") {
      return {
        message: `Bitte geben Sie für Zustimmung ${position} einen Titel ein.`,
        field: `legal_blocks.${index}.title`,
        hint: FILL_IN_HINT,
      };
    }
    if (block.label === "") {
      return {
        message: `Bitte geben Sie für Zustimmung ${position} einen Text neben der Checkbox ein.`,
        field: `legal_blocks.${index}.label`,
        hint: FILL_IN_HINT,
      };
    }
    if (
      block.key === "agb" &&
      block.display_mode === LEGAL_BLOCK_DISPLAY_MODE_PDF &&
      (block.document_url ?? "").trim() === ""
    ) {
      return {
        message:
          "Bitte laden Sie für die AGB eine PDF-Datei hoch. Oder wählen Sie wieder „Text eingeben“.",
      };
    }
  }

  return null;
}

function formatSchemaDate(value: string): string {
  return new Date(value).toLocaleDateString("de-DE", {
    timeZone: "Europe/Berlin",
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}

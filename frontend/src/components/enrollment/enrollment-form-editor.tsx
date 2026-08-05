"use client";

import Link from "next/link";
import {
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type Dispatch,
  type ReactNode,
  type SetStateAction,
} from "react";
import {
  ArrowLeft,
  Check,
  ChevronDown,
  Eye,
  ExternalLink,
  FileText,
  FileUp,
  GripVertical,
  HelpCircle,
  Info,
  ListPlus,
  Lock,
  Pencil,
  Plus,
  TextCursorInput,
  Trash2,
} from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { useToast } from "~/contexts/ToastContext";
import { ConfirmationModal, Modal } from "~/components/ui/modal";
import { FormModal } from "~/components/ui/form-modal";
import { Button } from "~/components/ui/button";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { Input } from "~/components/ui/input";
import { CustomSelect } from "~/components/ui/custom-select";
import { BooleanField } from "~/components/settings/fields/boolean-field";
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
  uploadEnrollmentLegalDocument,
  RESERVED_TARGETS,
  type ConditionOperator,
  type ConditionSource,
  type FormField,
  type FormFieldTarget,
  type FormFieldType,
  type FormLegalBlock,
  type FormSchema,
  type LegalBlockDisplayMode,
  type CoreRequirementKey,
  type CoreRequirements,
  type PublicLegalTexts,
  type VisibilityCondition,
} from "~/lib/enrollment-form-schema-api";
import { listPhases, type Phase } from "~/lib/enrollment-phase-api";
import { createLogger } from "~/lib/logger";
import { useTenantSlugSafe } from "~/lib/tenant-context";
import {
  copyStableObjectKey,
  getStableObjectKey,
} from "~/lib/stable-object-key";

const logger = createLogger({ component: "EnrollmentFormEditor" });

const fieldTypeLabels: Record<FormFieldType, string> = {
  boolean: "Ja/Nein",
  number: "Zahl",
  text: "Text",
  textarea: "Mehrzeiliger Text",
  date: "Datum",
  select: "Auswahl",
  information: "Infotext",
  phone_list: "Telefonliste",
  weekday_schedule: "Wochenzeiten",
  weekday_boolean: "Wochentage",
  weekday_mode: "Geh- und Abholregelung",
  weekday_multi_mode: "Erlaubte Heimwege",
  contact_list: "Kontaktliste",
};

// German labels for the visibility-condition editor.
const conditionSourceLabels: Record<ConditionSource, string> = {
  field: "Antwort auf eine andere Frage",
  grade_level: "Klassenstufe des Kindes",
  care_offering: "Gewähltes Betreuungsangebot",
};

const conditionOperatorLabels: Record<ConditionOperator, string> = {
  eq: "ist gleich",
  neq: "ist nicht gleich",
  not_empty: "ist ausgefüllt",
  includes: "enthält",
};

const freeFieldTypes = [
  "boolean",
  "number",
  "text",
  "textarea",
  "date",
  "select",
] satisfies FormFieldType[];

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
  "Bitte warte, bis der PDF-Upload abgeschlossen ist.";
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

export function EnrollmentFormEditor() {
  const toast = useToast();
  const tenantSlug = useTenantSlugSafe();
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
  const [error, setError] = useState<string | null>(null);
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

  const loadAll = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      // Legal texts are NOT best-effort: silently swallowing a fetch
      // failure would seed the builder with all-disabled standard blocks
      // and nudge admins into saving a template without the tenant's
      // consent contract. A failure fails the whole load instead.
      const [list, phaseList, legalTexts] = await Promise.all([
        listSchemas(),
        listPhases().catch(() => [] as Phase[]),
        tenantSlug ? fetchPublicLegalTexts(tenantSlug) : Promise.resolve(null),
      ]);
      const legalDefaults = mergeStandardLegalBlocks(legalTexts);
      setStandardLegalBlocks(legalDefaults);
      setAllSchemas(list);
      setPhases(phaseList);
      return list;
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("schema_list_failed", { error: message });
      setError(message);
      return [];
    } finally {
      setLoading(false);
    }
  }, [tenantSlug]);

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
      if (notify && results.some((result) => result.status === "rejected")) {
        toast.error(
          "Nicht alle ungespeicherten PDF-Dateien konnten bereinigt werden.",
        );
      }
    },
    [forgetDraftDocumentURLs, toast],
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
        void deleteEnrollmentLegalDocument(url, { keepalive: true });
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
    setError(null);
    setMode(nextMode);
  };

  const editSchema = (schema: FormSchema) => selectSchema(schema, "builder");

  const previewSchema = (schema: FormSchema) => selectSchema(schema, "detail");

  const requestRemoveSchema = (schema: FormSchema) => {
    setDeleteTarget(schema);
  };

  const confirmRemoveSchema = async () => {
    if (!deleteTarget) return;
    setError(null);
    setDeletingSchemaId(deleteTarget.id);
    try {
      await deleteSchema(deleteTarget.id);
      await loadAll();
      toast.success("Formularvorlage gelöscht.");
      if (selectedKey === deleteTarget.id) {
        backToOverview();
      }
      setDeleteTarget(null);
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Löschen fehlgeschlagen";
      logger.error("schema_delete_failed", { error: message });
      setError(message);
      toast.error(message);
    } finally {
      setDeletingSchemaId(null);
    }
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
    toast.success(`Vorlage in „${updated.name}" umbenannt.`);
    setRenameTarget(null);
  };

  const startNew = () => {
    setSelectedKey(NEW_SCHEMA_VALUE);
    setName("");
    setFields([]);
    setCoreRequirements({});
    setLegalBlocks(standardLegalBlocks);
    setError(null);
    setMode("builder");
  };

  const backToOverview = () => {
    setError(null);
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

  const updateCoreRequirement = (
    key: CoreRequirementKey,
    required: boolean,
  ) => {
    setCoreRequirements((prev) => ({ ...prev, [key]: required }));
  };

  const addField = () => {
    setFields((prev) => [...prev, blankField(prev.length)]);
  };

  const addInfoField = () => {
    setFields((prev) => [...prev, blankInfoField(prev.length)]);
  };

  const addTargetField = (target: Exclude<FormFieldTarget, "">) => {
    setFields((prev) => {
      if (prev.some((field) => field.target === target)) return prev;
      return [...prev, createTargetField(target, prev.length)];
    });
  };

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
  const saveBlockedMessage = getSchemaDraftValidationMessage({
    fields,
    legalBlocks,
    name,
  });
  const pendingUploadMessage = hasPendingLegalDocumentUpload
    ? LEGAL_DOCUMENT_UPLOAD_PENDING_MESSAGE
    : null;
  const effectiveSaveBlockedMessage =
    saveBlockedMessage ?? pendingUploadMessage;

  const saveSchema = async (
    nextMode: EditorMode = "detail",
  ): Promise<FormSchema | null> => {
    setSaving(true);
    setError(null);
    try {
      const validationMessage = getSchemaDraftValidationMessage({
        fields,
        legalBlocks,
        name,
      });
      if (validationMessage) {
        setError(validationMessage);
        toast.error(validationMessage);
        return null;
      }
      if (hasPendingLegalDocumentUpload) {
        setError(LEGAL_DOCUMENT_UPLOAD_PENDING_MESSAGE);
        toast.error(LEGAL_DOCUMENT_UPLOAD_PENDING_MESSAGE);
        return null;
      }

      const fieldsForSave = prepareFieldsForSave(fields);
      const legalBlocksForSave = prepareLegalBlocksForSave(legalBlocks);
      const referencedDraftDocumentURLs = draftDocumentURLsInLegalBlocks(
        legalBlocksForSave,
        uploadedDraftDocumentURLsRef.current,
      );
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
        isCreating ? "Formularvorlage erstellt." : "Änderungen gespeichert.",
      );
      return savedSchema;
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Speichern fehlgeschlagen";
      if (unmountedRef.current) {
        const urls = Array.from(savingDraftDocumentURLsRef.current);
        for (const url of urls) {
          void deleteEnrollmentLegalDocument(url, { keepalive: true });
        }
        savingDraftDocumentURLsRef.current = new Set();
      } else {
        logger.error("schema_save_failed", { error: message });
        setError(message);
        toast.error(message);
      }
      return null;
    } finally {
      if (!unmountedRef.current) {
        savingDraftDocumentURLsRef.current = new Set();
        setSaving(false);
      }
    }
  };

  const handleSave = async () => {
    await saveSchema("detail");
  };

  const requestBackToOverview = () => {
    if (hasPendingLegalDocumentUpload) {
      toast.error(LEGAL_DOCUMENT_UPLOAD_PENDING_MESSAGE);
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
      toast.error(LEGAL_DOCUMENT_UPLOAD_PENDING_MESSAGE);
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
      `/enroll/preview?schemaId=${encodeURIComponent(schemaId)}`,
      "_blank",
      "noopener,noreferrer",
    );
  };

  const requestExternalPreview = () => {
    if (hasPendingLegalDocumentUpload) {
      toast.error(LEGAL_DOCUMENT_UPLOAD_PENDING_MESSAGE);
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
        previewWindow.document.body.textContent = "Vorschau wird geöffnet...";
      }
    }

    const savedSchema = await saveSchema(
      pending === "overview" ? "overview" : "detail",
    );
    if (!savedSchema) {
      previewWindow?.close();
      return;
    }

    setPendingNavigation(null);
    if (pending === "new") {
      startNew();
      return;
    }
    if (pending === "preview") {
      const href = `/enroll/preview?schemaId=${encodeURIComponent(savedSchema.id)}`;
      if (previewWindow) {
        previewWindow.location.href = href;
      } else {
        window.open(href, "_blank", "noopener,noreferrer");
      }
    }
  };

  if (loading) {
    return <p className="text-sm text-gray-500">Wird geladen...</p>;
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
          error={error}
        />
        <DeleteSchemaDialog
          schema={deleteTarget}
          deleting={deletingSchemaId === deleteTarget?.id}
          onClose={() => setDeleteTarget(null)}
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
    <div className="space-y-5">
      <section className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm backdrop-blur-md">
        <div className="border-b border-gray-100 px-5 py-3 sm:px-6">
          <button
            type="button"
            onClick={requestBackToOverview}
            disabled={saving || hasPendingLegalDocumentUpload}
            className="inline-flex h-8 items-center gap-2 rounded-lg px-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-50 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
          >
            <ArrowLeft className="h-4 w-4" aria-hidden="true" />
            Zurück zur Übersicht
          </button>
        </div>
        <div className="grid gap-0 lg:grid-cols-[minmax(0,1fr)_380px] xl:grid-cols-[minmax(0,1fr)_430px]">
          <div className="space-y-6 p-5 sm:p-6">
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
              disabled={saving}
            />

            <LegalBlocksSection
              blocks={legalBlocks}
              standardBlocks={standardLegalBlocks}
              onChange={setLegalBlocks}
              disabled={saving}
              draftDocumentURLs={uploadedDraftDocumentURLs}
              onDraftDocumentUploaded={rememberDraftDocumentURL}
              onDraftDocumentDeleted={forgetDraftDocumentURL}
              onUploadStart={beginLegalDocumentUpload}
              onUploadEnd={endLegalDocumentUpload}
            />

            <section className="space-y-4">
              <div className="flex flex-col gap-3 border-t border-gray-100 pt-5 sm:flex-row sm:items-end sm:justify-between">
                <div>
                  <p className="text-moto-blue text-xs font-semibold tracking-wide uppercase">
                    Zusatzfragen
                  </p>
                  <h2 className="mt-1 text-base font-semibold text-gray-900">
                    Was Eltern zusätzlich beantworten sollen
                  </h2>
                  <p className="mt-1 max-w-2xl text-sm text-gray-600">
                    Wähle feste Vorschläge, wenn die Antwort später in den
                    Stammdaten stehen soll. Freie Zusatzfragen bleiben nur bei
                    der Anmeldung.
                  </p>
                </div>
                <div className="flex flex-wrap gap-2">
                  <button
                    type="button"
                    onClick={addInfoField}
                    disabled={saving}
                    className="inline-flex h-9 items-center justify-center gap-2 rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    <Info className="h-4 w-4" aria-hidden="true" />
                    Infotext
                  </button>
                  <button
                    type="button"
                    onClick={addField}
                    disabled={saving}
                    className="inline-flex h-9 items-center justify-center gap-2 rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    <Plus className="h-4 w-4" aria-hidden="true" />
                    Freie Zusatzfrage
                  </button>
                </div>
              </div>

              {error ? (
                <div className="border-moto-red/20 bg-moto-red/10 text-moto-red-strong rounded-lg border p-3 text-sm">
                  {error}
                </div>
              ) : null}

              <TargetSuggestions
                fields={fields}
                onAdd={addTargetField}
                disabled={saving}
              />

              {fields.length > 0 ? (
                <div className="space-y-3">
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
                      disabled={saving}
                    />
                  ))}
                </div>
              ) : null}

              <div className="flex flex-wrap items-center justify-end gap-2 border-t border-gray-100 pt-4">
                <button
                  type="button"
                  onClick={requestStartNew}
                  disabled={saving || hasPendingLegalDocumentUpload}
                  className="inline-flex h-9 items-center justify-center rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50"
                >
                  Zurücksetzen
                </button>
                <button
                  type="button"
                  onClick={handleSave}
                  disabled={saving || hasPendingLegalDocumentUpload}
                  className="inline-flex h-9 items-center justify-center rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {saving
                    ? "Speichert..."
                    : isCreating
                      ? "Formularvorlage erstellen"
                      : "Änderungen speichern"}
                </button>
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
                  ? `/enroll/preview?schemaId=${encodeURIComponent(currentSchema.id)}`
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
  error,
}: Readonly<{
  templates: FormSchema[];
  phases: Phase[];
  onCreate: () => void;
  onEdit: (schema: FormSchema) => void;
  onPreview: (schema: FormSchema) => void;
  onRename: (schema: FormSchema) => void;
  onDelete: (schema: FormSchema) => void;
  error: string | null;
}>) {
  const assignedTemplateCount = templates.filter((schema) =>
    phases.some((phase) => phase.form_schema_id === schema.id),
  ).length;

  return (
    <div className="space-y-5">
      <section className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm backdrop-blur-md">
        <div className="grid gap-0 lg:grid-cols-[minmax(0,1fr)_360px] xl:grid-cols-[minmax(0,1fr)_400px]">
          <div className="space-y-6 p-5 sm:p-6">
            <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
              <div>
                <p className="text-moto-blue text-xs font-semibold tracking-wide uppercase">
                  Formularübersicht
                </p>
                <h2 className="mt-1 text-xl font-semibold text-gray-900">
                  Anmeldeformulare verwalten
                </h2>
                <p className="mt-2 max-w-3xl text-sm leading-6 text-gray-600">
                  Das Basisformular ist immer vorhanden. Eigene Vorlagen nutzt
                  du, wenn eine Anmeldephase abweichende Pflichtangaben oder
                  Zusatzfragen braucht.
                </p>
              </div>
              <button
                type="button"
                onClick={onCreate}
                className="inline-flex h-9 items-center justify-center gap-2 rounded-lg bg-gray-900 px-3 text-sm font-medium whitespace-nowrap text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
              >
                <Plus className="h-4 w-4" aria-hidden="true" />
                Neue Vorlage
              </button>
            </div>

            {error ? (
              <div className="border-moto-red/20 bg-moto-red/10 text-moto-red-strong rounded-lg border p-3 text-sm">
                {error}
              </div>
            ) : null}

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
          <span className="rounded-full bg-gray-100 px-2 py-0.5 text-[11px] font-medium text-gray-600">
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
          href="/enroll/preview?base=1"
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
          <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-gray-100 text-gray-500">
            <FileText className="h-4 w-4" aria-hidden="true" />
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
                  `/enroll/preview?schemaId=${encodeURIComponent(schema.id)}`,
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
      textClassName: "text-[#5F9F20]",
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
        <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
          Startpunkt
        </p>
        <h2 className="mt-1 text-base font-semibold text-gray-900">
          Erst prüfen, dann erweitern
        </h2>
        <p className="mt-2 text-sm leading-6 text-gray-600">
          Lege nur dann eine eigene Vorlage an, wenn das Basisformular nicht
          reicht. So bleibt die Elternansicht kurz und verständlich.
        </p>
      </div>

      <div className="moto-content-surface rounded-2xl border p-4 shadow-sm">
        <h3 className="text-sm font-semibold text-gray-900">
          Nächste Schritte
        </h3>
        <div className="mt-4 space-y-3">
          <GuideStep
            icon={<MotoConceptIcon concept="permissions" size={18} />}
            title="Basisformular prüfen"
            done
          />
          <GuideStep
            icon={<ListPlus className="h-4 w-4" aria-hidden="true" />}
            title="Vorlage nur bei Bedarf"
            done={templateCount > 0}
          />
          <GuideStep
            icon={<MotoConceptIcon concept="calendarPeriods" size={18} />}
            title="Vorlage in Anmeldephase wählen"
            done={assignedTemplateCount > 0}
          />
        </div>
      </div>

      <p className="text-sm leading-6 text-gray-500">
        Für eine neue Halbjahresanmeldung brauchst du oft keine eigene
        Formularvorlage. Anmeldephase und Betreuungsangebote steuern den
        eigentlichen Ablauf.
      </p>
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
  const requiredCount = schema.fields.filter((field) =>
    Boolean(field.required),
  ).length;
  const childFieldCount = schema.fields.filter((field) =>
    Boolean(field.applies_to_child),
  ).length;

  return (
    <div className="space-y-5">
      <section className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm backdrop-blur-md">
        <div className="grid gap-0 lg:grid-cols-[minmax(0,1fr)_360px] xl:grid-cols-[minmax(0,1fr)_400px]">
          <div>
            <div className="border-b border-gray-100 px-5 py-3 sm:px-6">
              <button
                type="button"
                onClick={onBack}
                className="inline-flex h-8 items-center gap-2 rounded-lg px-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-50 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
              >
                <ArrowLeft className="h-4 w-4" aria-hidden="true" />
                Zurück zur Übersicht
              </button>
            </div>
            <div className="space-y-5 p-5 sm:p-6">
              <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                <div>
                  <p className="text-moto-blue text-xs font-semibold tracking-wide uppercase">
                    Formular prüfen
                  </p>
                  <h2 className="mt-1 text-xl font-semibold text-gray-900">
                    {schema.name}
                  </h2>
                  <p className="mt-2 max-w-3xl text-sm leading-6 text-gray-600">
                    Prüfe die Formularvorschau und ordne diese Vorlage einer
                    Anmeldephase zu, wenn Eltern die angepassten Pflichtangaben
                    oder Zusatzfragen sehen sollen.
                  </p>
                </div>
                <button
                  type="button"
                  onClick={onEdit}
                  className="inline-flex h-9 items-center justify-center gap-2 rounded-lg bg-gray-900 px-3 text-sm font-medium whitespace-nowrap text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                >
                  <Pencil className="h-4 w-4" aria-hidden="true" />
                  Bearbeiten
                </button>
              </div>

              <div className="grid grid-cols-2 gap-3 xl:grid-cols-4">
                <FormMetric
                  icon={<FileText className="h-4 w-4" aria-hidden="true" />}
                  value={formatSchemaDate(schema.created_at)}
                  label="Zuletzt gespeichert"
                />
                <FormMetric
                  icon={<ListPlus className="h-4 w-4" aria-hidden="true" />}
                  value={schema.fields.length.toString()}
                  label="Zusatzfragen"
                />
                <FormMetric
                  icon={<Check className="h-4 w-4" aria-hidden="true" />}
                  value={requiredCount.toString()}
                  label="Pflicht-Zusatzfragen"
                />
                <FormMetric
                  icon={<FileText className="h-4 w-4" aria-hidden="true" />}
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
                previewHref={`/enroll/preview?schemaId=${encodeURIComponent(schema.id)}`}
                assignedPhaseCount={assignedPhases.length}
                sticky={false}
              />
            </div>
          </div>

          <aside className="moto-dotted-background moto-dotted-background--split border-t border-gray-100 p-5 sm:p-6 lg:border-t-0 lg:border-l">
            <div className="sticky top-6 space-y-4">
              <div>
                <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                  Nächster Schritt
                </p>
                <h2 className="mt-1 text-base font-semibold text-gray-900">
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

              <Link
                href={
                  assignedPhases.length > 0
                    ? "/enrollment-phases"
                    : `/enrollment-phases?assignForm=${encodeURIComponent(schema.id)}`
                }
                className="inline-flex h-10 w-full items-center justify-center gap-2 rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
              >
                <MotoConceptIcon concept="calendarPeriods" size={18} />
                {assignedPhases.length > 0
                  ? "Anmeldephasen öffnen"
                  : "In Anmeldephase auswählen"}
              </Link>

              <a
                href={`/enroll/preview?schemaId=${encodeURIComponent(schema.id)}`}
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
          done ? "bg-moto-green/15 text-[#5F9F20]" : "bg-gray-100 text-gray-500"
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
  onClose,
  onConfirm,
}: Readonly<{
  schema: FormSchema | null;
  deleting: boolean;
  onClose: () => void;
  onConfirm: () => void;
}>) {
  return (
    <ConfirmationModal
      isOpen={schema !== null}
      onClose={onClose}
      onConfirm={onConfirm}
      title="Formularvorlage löschen?"
      confirmText="Löschen"
      cancelText="Abbrechen"
      isConfirmLoading={deleting}
      confirmButtonClass="bg-moto-red hover:bg-moto-red-strong"
    >
      <div className="space-y-3 text-sm leading-6 text-gray-600">
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
    </ConfirmationModal>
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
  const [error, setError] = useState<string | null>(null);

  // Reset the field to the current name each time the dialog opens for a
  // schema so the admin edits from the existing value.
  useEffect(() => {
    if (schema) {
      setValue(schema.name);
      setError(null);
      setSubmitting(false);
    }
  }, [schema]);

  const isOpen = schema !== null;
  const trimmed = value.trim();
  const canSubmit = isRenameOf(schema, value) && !submitting;

  const submit = async () => {
    if (!canSubmit) return;
    setSubmitting(true);
    setError(null);
    try {
      await onConfirm(trimmed);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Umbenennen fehlgeschlagen",
      );
      setSubmitting(false);
    }
  };

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title="Formular umbenennen"
      size="sm"
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
        onSubmit={(event) => {
          event.preventDefault();
          void submit();
        }}
        className="space-y-3"
      >
        <Input
          name="schema-name"
          label="Name"
          type="text"
          value={value}
          onChange={(event) => setValue(event.target.value)}
          placeholder="z. B. Ferienbetreuung Sommer 2026"
          error={error ?? undefined}
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
      <button
        type="button"
        onClick={onCancel}
        disabled={saving}
        className="inline-flex h-9 items-center justify-center rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
      >
        Abbrechen
      </button>
      {!isPreview ? (
        <button
          type="button"
          onClick={onDiscard}
          disabled={saving}
          className="inline-flex h-9 items-center justify-center rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
        >
          Verwerfen
        </button>
      ) : null}
      <button
        type="button"
        onClick={onSave}
        disabled={saving || Boolean(saveBlockedMessage)}
        className="inline-flex h-9 items-center justify-center rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
      >
        {saving
          ? "Speichert..."
          : isPreview
            ? "Speichern und Vorschau öffnen"
            : "Speichern und fortfahren"}
      </button>
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
      <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
        Ungespeicherte Änderungen
      </p>
      <p className="mt-2 text-sm leading-6 text-gray-600">
        {isPreview
          ? "Für die externe Vorschau muss die Vorlage zuerst gespeichert werden."
          : "Du hast Änderungen an dieser Vorlage. Speichere sie, bevor du den Bereich verlässt, oder verwirf sie bewusst."}
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
      <p className="text-moto-blue text-xs font-semibold tracking-wide uppercase">
        Formular-Konfigurator
      </p>
      <h2 className="mt-1 text-xl font-semibold text-gray-900">
        Formularvorlage bearbeiten
      </h2>
      <p className="mt-2 max-w-3xl text-sm leading-6 text-gray-600">
        Lege fest, welche optionalen Basisfelder verpflichtend sind, und ergänze
        bei Bedarf Zusatzfragen. Die Vorlage wird später in einer Anmeldephase
        ausgewählt.
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
          <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
            Vorlage
          </p>
          <h2 className="mt-1 text-base font-semibold text-gray-900">
            {isCreating ? "Neue Formularvorlage" : name}
          </h2>
          <p className="mt-1 max-w-2xl text-sm text-gray-600">
            {isCreating
              ? "Gib der Vorlage einen eindeutigen Namen. Danach kannst du Pflichtangaben und Zusatzfragen festlegen."
              : "Der Name lässt sich hier ändern und gilt für alle Versionen. Bestehende Anmeldungen bleiben nachvollziehbar."}
          </p>
        </div>

        <div className="space-y-3">
          <label className="block">
            <span className="text-xs font-medium text-gray-700">
              Name der Vorlage
            </span>
            <input
              type="text"
              value={name}
              onChange={(event) => onNameChange(event.target.value)}
              placeholder="z. B. Ferienbetreuung Sommer 2026"
              disabled={saving}
              className="mt-1 h-10 w-full rounded-lg border border-gray-200 px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:bg-gray-100 disabled:text-gray-600"
            />
          </label>
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
          <p className="text-moto-blue text-xs font-semibold tracking-wide uppercase">
            Basisformular
          </p>
          <h2 className="mt-1 text-base font-semibold text-gray-900">
            Pflichtstatus der Basisfelder
          </h2>
          <p className="mt-1 max-w-2xl text-sm leading-6 text-gray-600">
            Fest gesetzte Felder bleiben immer Pflicht. Optionale Basisfelder
            kannst du hier verpflichtend machen.
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

function LegalBlocksSection({
  blocks,
  standardBlocks,
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
  onChange: Dispatch<SetStateAction<FormLegalBlock[]>>;
  disabled: boolean;
  draftDocumentURLs: ReadonlySet<string>;
  onDraftDocumentUploaded: (documentURL: string) => void;
  onDraftDocumentDeleted: (documentURL: string) => void;
  onUploadStart: () => void;
  onUploadEnd: () => void;
}>) {
  const toast = useToast();
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
        } catch {
          toast.error("Die vorherige PDF-Datei konnte nicht bereinigt werden.");
        }
      }
      toast.success("AGB-PDF hochgeladen.");
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : "PDF-Datei konnte nicht hochgeladen werden.",
      );
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
    } catch {
      toast.error("PDF-Datei konnte nicht bereinigt werden.");
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
  }) => (
    <div
      key={`${block.key}-${index}-editor`}
      className="rounded-xl border border-gray-200 bg-white p-4"
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
        <p className="max-w-xl text-xs leading-5 text-gray-500">{helperText}</p>
        <div className="flex flex-wrap gap-2">
          {onReset ? (
            <button
              type="button"
              onClick={onReset}
              disabled={disabled}
              className="inline-flex h-8 w-fit items-center rounded-lg px-2 text-sm font-medium text-gray-600 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50"
            >
              Einstellungen wieder verwenden
            </button>
          ) : null}
          {onRemove ? (
            <button
              type="button"
              onClick={onRemove}
              disabled={disabled}
              className="text-moto-red-strong hover:bg-moto-red/10 inline-flex h-8 w-fit items-center gap-2 rounded-lg px-2 text-sm font-medium disabled:cursor-not-allowed disabled:opacity-50"
            >
              <Trash2 className="h-4 w-4" aria-hidden="true" />
              Entfernen
            </button>
          ) : null}
        </div>
      </div>

      <div className="mt-3">
        <label className="block">
          <span className="text-xs font-medium text-gray-700">Titel</span>
          <input
            type="text"
            value={block.title}
            disabled={disabled}
            onChange={(event) =>
              updateBlock(index, { title: event.target.value })
            }
            className="mt-1 h-10 w-full rounded-lg border border-gray-200 px-3 text-sm shadow-sm focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:bg-gray-100"
          />
        </label>
      </div>

      <label className="mt-3 block">
        <span className="text-xs font-medium text-gray-700">
          Text neben der Checkbox oder dem Hinweis
        </span>
        <textarea
          value={block.label}
          disabled={disabled}
          rows={2}
          onChange={(event) =>
            updateBlock(index, { label: event.target.value })
          }
          className="mt-1 w-full rounded-lg border border-gray-200 px-3 py-2 text-sm shadow-sm focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:bg-gray-100"
        />
      </label>

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
        <label className="mt-3 block">
          <span className="text-xs font-medium text-gray-700">
            Rechtstext / Erklärung
          </span>
          <textarea
            value={block.text}
            disabled={disabled}
            rows={4}
            onChange={(event) =>
              updateBlock(index, { text: event.target.value })
            }
            className="mt-1 w-full rounded-lg border border-gray-200 px-3 py-2 text-sm shadow-sm focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:bg-gray-100"
          />
        </label>
      )}

      <div className="mt-3 flex flex-wrap items-center gap-3">
        <LegalBlockModeControl
          isNotice={block.kind === "notice"}
          disabled={disabled}
          onModeChange={(isNotice) =>
            updateBlock(index, {
              kind: isNotice ? "notice" : checkboxLegalBlockKindFor(block),
              required: isNotice ? false : block.required,
            })
          }
        />
        {block.kind !== "notice" ? (
          <StyledCheckboxButton
            checked={block.required}
            disabled={disabled}
            muted
            onCheckedChange={(checked) =>
              updateBlock(index, { required: checked })
            }
          >
            Muss bestätigt werden
          </StyledCheckboxButton>
        ) : null}
      </div>
    </div>
  );

  return (
    <section className="moto-content-surface rounded-2xl border p-5 shadow-sm">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <p className="text-moto-blue text-xs font-semibold tracking-wide uppercase">
            Zustimmungen
          </p>
          <h2 className="mt-1 text-base font-semibold text-gray-900">
            Rechtstexte und Einwilligungen
          </h2>
          <p className="mt-1 max-w-2xl text-sm leading-6 text-gray-600">
            Die vier Standardblöcke starten mit den Texten aus den
            Einstellungen. Hier blendest du sie für diese Vorlage aus oder
            bearbeitest bewusst eine Abweichung. Eigene Zustimmungen bleiben
            direkt an diese Vorlage gebunden.
          </p>
        </div>
      </div>

      {blocks.some((block) => block.enabled) &&
      !blocks.some(
        (block) => block.key === "data_processing" && block.enabled,
      ) ? (
        <p className="mt-3 rounded-lg border border-[#EAB308]/30 bg-[#EAB308]/10 p-3 text-sm leading-6 text-gray-700">
          Hinweis: Die Datenschutzinformation ist in dieser Vorlage deaktiviert.
          Stelle sicher, dass Eltern die Datenschutzhinweise auf anderem Weg
          erhalten, zum Beispiel über den Elternbrief.
        </p>
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
              className="rounded-xl border border-gray-200 bg-white p-4"
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
                      <span className="rounded-full bg-[#EAB308]/10 px-2 py-0.5 text-xs font-medium text-[#92400E]">
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
                  <button
                    type="button"
                    aria-label={`${block.title} abweichend bearbeiten`}
                    title="Abweichend bearbeiten"
                    onClick={() => editStandardBlock(block.key)}
                    disabled={disabled}
                    className="inline-flex h-7 w-7 items-center justify-center rounded-md text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    <Pencil className="h-3.5 w-3.5" aria-hidden="true" />
                  </button>
                </div>
              </div>
            </div>
          );
        })}

        {customEntries.map(({ block, index }) =>
          renderBlockEditor({
            block,
            index,
            helperText:
              "Eigene Zustimmung für diese Vorlage. Sie erscheint als zusätzliche Checkbox im Anmeldeformular.",
            onRemove: () => removeBlock(index),
          }),
        )}

        {customEntries.length === 0 ? (
          <div className="rounded-xl border border-dashed border-gray-200 bg-white/70 p-4 text-sm leading-6 text-gray-600">
            Noch keine eigenen Zustimmungen. Füge hier nur zusätzliche Inhalte
            hinzu, die nicht in den allgemeinen Einstellungen stehen.
          </div>
        ) : null}
      </div>

      <div className="mt-4">
        <button
          type="button"
          onClick={addCustomBlock}
          disabled={disabled}
          className="inline-flex h-9 items-center justify-center gap-2 rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
        >
          <Plus className="h-4 w-4" aria-hidden="true" />
          Eigene Zustimmung hinzufügen
        </button>
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
    (block.document_url ?? "") !== (standardBlock.document_url ?? "")
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
        <button
          type="button"
          onClick={() => onModeChange(LEGAL_BLOCK_DISPLAY_MODE_TEXT)}
          disabled={disabled}
          className={`rounded-xl border p-3 text-left transition-colors disabled:cursor-not-allowed disabled:opacity-50 ${
            mode === LEGAL_BLOCK_DISPLAY_MODE_TEXT
              ? "border-moto-blue bg-moto-blue/10 text-gray-950 shadow-sm"
              : "hover:border-moto-blue/40 hover:bg-moto-blue/5 border-gray-200 bg-white text-gray-700"
          }`}
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
        </button>
        <button
          type="button"
          onClick={() => onModeChange(LEGAL_BLOCK_DISPLAY_MODE_PDF)}
          disabled={disabled}
          className={`rounded-xl border p-3 text-left transition-colors disabled:cursor-not-allowed disabled:opacity-50 ${
            mode === LEGAL_BLOCK_DISPLAY_MODE_PDF
              ? "border-moto-blue bg-moto-blue/10 text-gray-950 shadow-sm"
              : "hover:border-moto-blue/40 hover:bg-moto-blue/5 border-gray-200 bg-white text-gray-700"
          }`}
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
        </button>
      </div>

      {mode === LEGAL_BLOCK_DISPLAY_MODE_TEXT ? (
        <label className="block">
          <span className="text-xs font-medium text-gray-700">
            Rechtstext / Erklärung
          </span>
          <textarea
            value={textValue}
            disabled={disabled}
            rows={4}
            onChange={(event) => onTextChange(event.target.value)}
            className="mt-1 w-full rounded-lg border border-gray-200 px-3 py-2 text-sm shadow-sm focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:bg-gray-100"
          />
        </label>
      ) : (
        <div className="border-moto-blue/20 bg-moto-blue/5 rounded-xl border p-3">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <p className="text-sm font-medium text-gray-900">PDF-Datei</p>
              <p className="mt-0.5 text-xs text-gray-600">
                {hasDocument
                  ? "Diese PDF wird in dieser Formularvorlage als Link angezeigt."
                  : "Lade die AGB / Teilnahmebedingungen als PDF hoch."}
              </p>
            </div>
            <div className="flex flex-wrap items-center gap-2 text-xs">
              <label
                className={`border-moto-blue/25 hover:bg-moto-blue/10 inline-flex cursor-pointer items-center gap-1.5 rounded-lg border bg-white px-2.5 py-1.5 font-medium text-[#4070C8] shadow-sm transition-colors ${
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
                <button
                  type="button"
                  onClick={onDocumentRemove}
                  disabled={disabled || documentSaving}
                  className="border-moto-red/20 text-moto-red-strong hover:bg-moto-red/10 inline-flex items-center gap-1.5 rounded-lg border bg-white px-2.5 py-1.5 font-medium shadow-sm transition-colors disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <Trash2 className="h-3.5 w-3.5" aria-hidden="true" />
                  <span>Entfernen</span>
                </button>
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
  isNotice,
  disabled,
  onModeChange,
}: Readonly<{
  isNotice: boolean;
  disabled: boolean;
  onModeChange: (isNotice: boolean) => void;
}>) {
  return (
    <div
      className="inline-flex rounded-xl border border-gray-200 bg-gray-50 p-1"
      aria-label="Darstellung im Elternformular"
    >
      {[
        { label: "Mit Checkbox", isNotice: false },
        { label: "Nur Hinweis", isNotice: true },
      ].map((option) => {
        const selected = option.isNotice === isNotice;
        return (
          <button
            key={option.label}
            type="button"
            disabled={disabled}
            aria-pressed={selected}
            onClick={() => onModeChange(option.isNotice)}
            className={`h-8 rounded-lg px-3 text-sm font-medium transition-colors focus-visible:ring-2 focus-visible:ring-gray-300 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50 ${
              selected
                ? "bg-white text-gray-900 shadow-sm"
                : "text-gray-600 hover:text-gray-900"
            }`}
          >
            {option.label}
          </button>
        );
      })}
    </div>
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
  return "consent";
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
        <h3 className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
          {title}
        </h3>
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
            Diese Fragen sind mit vorhandenen Stammdaten verbunden. Du kannst
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
            <button
              key={target}
              type="button"
              onClick={() => onAdd(target)}
              disabled={disabled || selected}
              className={`flex min-h-24 items-start gap-3 rounded-xl border p-3 text-left shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-default ${
                selected
                  ? "border-gray-200 bg-white text-gray-500"
                  : "border-gray-200 bg-white text-gray-700 hover:border-gray-300 hover:bg-gray-50"
              }`}
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
                <span className="mt-2 inline-flex rounded-full bg-gray-100 px-2 py-0.5 text-[11px] font-medium text-gray-600">
                  {selected ? "Ist drin" : "Hinzufügen"}
                </span>
              </span>
            </button>
          );
        })}
      </div>
    </section>
  );
}

interface FieldEditorRowProps {
  readonly field: FormField;
  readonly index: number;
  readonly total: number;
  readonly allFields: FormField[];
  readonly onChange: (patch: Partial<FormField>) => void;
  readonly onRemove: () => void;
  readonly onMoveUp: () => void;
  readonly onMoveDown: () => void;
  readonly disabled: boolean;
}

interface AllowedTimeRow {
  readonly id: string;
  readonly value: string;
}

function createAllowedTimeRows(
  values: readonly string[],
  idPrefix: string,
  sequence: { current: number },
): AllowedTimeRow[] {
  return values.map((value) => {
    sequence.current += 1;
    return { id: `${idPrefix}-${sequence.current}`, value };
  });
}

function FieldEditorRow({
  field,
  index,
  total,
  allFields,
  onChange,
  onRemove,
  onMoveUp,
  onMoveDown,
  disabled,
}: FieldEditorRowProps) {
  const allowedTimeIdPrefix = useId();
  const allowedTimeSequence = useRef(0);
  const target = field.target || null;
  const isTargetField = target !== null;
  const isInfo = field.type === "information";
  const displayLabel = isInfo
    ? field.label.trim() || "Infotext"
    : target
      ? field.label.trim() || RESERVED_TARGETS[target].label
      : field.label.trim() || "Neue Zusatzfrage";
  const optionSignature = useMemo(
    () => (field.options ?? []).map((option) => option.label).join("\n"),
    [field.options],
  );
  const [optionsDraft, setOptionsDraft] = useState(optionSignature);

  useEffect(() => {
    setOptionsDraft(optionSignature);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [field.key, field.type]);

  const updateOptions = (value: string) => {
    setOptionsDraft(value);
    const options = value
      .split("\n")
      .map((line) => line.trim())
      .filter(Boolean)
      .map((line) => ({
        label: line,
        value: normalizeFieldKey(line),
      }));
    onChange({ options });
  };

  // Local editing rows for the fixed pickup times: each row is one HH:MM
  // time picker. Kept separate from field.allowed_times so a half-filled
  // (empty) row can stay on screen while the admin picks a time; only the
  // trimmed, de-duplicated, non-empty values are pushed up to the field.
  const [allowedTimesRows, setAllowedTimesRows] = useState<AllowedTimeRow[]>(
    () =>
      createAllowedTimeRows(
        field.allowed_times ?? [],
        allowedTimeIdPrefix,
        allowedTimeSequence,
      ),
  );
  // Signature of the allowed_times we last pushed up via onChange. Used to
  // tell an *external* change (a schema/template switch reusing this row at
  // the same index — the row is keyed by index, not field identity, so the
  // component instance is not remounted) from a *self-induced* one (our own
  // commit echoing back through field.allowed_times). Only the former resets
  // the local rows; the latter must not, or a half-filled empty row would be
  // wiped on every keystroke.
  const lastCommittedSig = useRef<string>(
    (field.allowed_times ?? []).join("\n"),
  );

  useEffect(() => {
    const incoming = field.allowed_times ?? [];
    const incomingSig = incoming.join("\n");
    if (incomingSig !== lastCommittedSig.current) {
      lastCommittedSig.current = incomingSig;
      setAllowedTimesRows(
        createAllowedTimeRows(
          incoming,
          allowedTimeIdPrefix,
          allowedTimeSequence,
        ),
      );
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [field.key, field.type, field.allowed_times]);

  const commitAllowedTimes = (rows: AllowedTimeRow[]) => {
    setAllowedTimesRows(rows);
    const allowed_times = Array.from(
      new Set(rows.map((row) => row.value.trim()).filter(Boolean)),
    );
    lastCommittedSig.current = allowed_times.join("\n");
    onChange({ allowed_times });
  };

  const updateAllowedTime = (index: number, value: string) => {
    const rows = [...allowedTimesRows];
    const row = rows[index];
    if (!row) return;
    rows[index] = { ...row, value };
    commitAllowedTimes(rows);
  };

  const addAllowedTime = () => {
    const [row] = createAllowedTimeRows(
      [""],
      allowedTimeIdPrefix,
      allowedTimeSequence,
    );
    if (row) commitAllowedTimes([...allowedTimesRows, row]);
  };

  const removeAllowedTime = (index: number) =>
    commitAllowedTimes(allowedTimesRows.filter((_, i) => i !== index));

  return (
    <article className="moto-content-surface rounded-2xl border p-4 shadow-sm">
      <div className="flex items-start gap-3">
        <span className="mt-1 hidden text-gray-300 sm:block">
          <GripVertical className="h-5 w-5" aria-hidden="true" />
        </span>
        <div className="min-w-0 flex-1 space-y-4">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <div className="flex flex-wrap items-center gap-2">
                <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                  {isInfo ? "Infotext" : "Frage"} {index + 1}
                </p>
                {isInfo ? (
                  <span className="bg-moto-blue/10 inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium text-[#3D63B0]">
                    <Info className="h-3 w-3" aria-hidden="true" />
                    Hinweis
                  </span>
                ) : isTargetField ? (
                  <span className="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2 py-0.5 text-[11px] font-medium text-gray-600">
                    <Lock className="h-3 w-3" aria-hidden="true" />
                    Fester Vorschlag
                  </span>
                ) : (
                  <span className="rounded-full bg-gray-100 px-2 py-0.5 text-[11px] font-medium text-gray-600">
                    Freie Zusatzfrage
                  </span>
                )}
              </div>
              <h3 className="mt-1 text-sm font-semibold text-gray-900">
                {displayLabel}
              </h3>
            </div>
            <div className="flex gap-1.5">
              <button
                type="button"
                onClick={onMoveUp}
                disabled={disabled || index === 0}
                className="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-gray-200 bg-white text-sm font-medium text-gray-500 shadow-sm transition-colors hover:bg-gray-50 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-40"
                aria-label="Frage nach oben verschieben"
              >
                ↑
              </button>
              <button
                type="button"
                onClick={onMoveDown}
                disabled={disabled || index === total - 1}
                className="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-gray-200 bg-white text-sm font-medium text-gray-500 shadow-sm transition-colors hover:bg-gray-50 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-40"
                aria-label="Frage nach unten verschieben"
              >
                ↓
              </button>
              <button
                type="button"
                onClick={onRemove}
                disabled={disabled}
                className="hover:border-moto-red/30 hover:bg-moto-red/10 hover:text-moto-red-strong inline-flex h-8 w-8 items-center justify-center rounded-lg border border-gray-200 bg-white text-gray-500 shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-40"
                aria-label="Frage entfernen"
              >
                <Trash2 className="h-4 w-4" aria-hidden="true" />
              </button>
            </div>
          </div>

          {isInfo ? (
            <div className="border-moto-blue/20 bg-moto-blue/5 rounded-xl border px-3 py-2 text-xs leading-5 text-gray-600">
              <p className="font-medium text-[#3D63B0]">
                Wird Eltern als Hinweis angezeigt.
              </p>
              <p className="mt-0.5">
                Ein Infotext sammelt keine Antwort. Nutze ihn für Erklärungen,
                Hinweise oder Zwischenüberschriften.
              </p>
            </div>
          ) : isTargetField ? (
            <div className="rounded-xl border border-gray-200 bg-gray-50/80 px-3 py-2 text-xs leading-5 text-gray-600">
              <p className="font-medium text-gray-800">
                Wird bei bestätigter Anmeldung in die Stammdaten übernommen.
              </p>
              <p className="mt-0.5">
                Du kannst die angezeigte Frage umbenennen. Inhalt und Typ sind
                fest vorgegeben. Entferne den Vorschlag, wenn diese Angabe nicht
                abgefragt werden soll.
              </p>
            </div>
          ) : (
            <div className="rounded-xl border border-gray-200 bg-gray-50/80 px-3 py-2 text-xs leading-5 text-gray-600">
              <p className="font-medium text-gray-800">
                Bleibt nur als Zusatzangabe bei der Anmeldung.
              </p>
              <p className="mt-0.5">
                Für Angaben, die nicht automatisch in die Stammdaten übernommen
                werden sollen.
              </p>
            </div>
          )}

          {isInfo ? (
            <>
              <label className="block">
                <span className="text-xs font-medium text-gray-700">
                  Titel (optional)
                </span>
                <input
                  type="text"
                  value={field.label}
                  onChange={(event) => onChange({ label: event.target.value })}
                  placeholder="z. B. Wichtiger Hinweis"
                  disabled={disabled}
                  className="mt-1 h-10 w-full rounded-lg border border-gray-200 px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                />
              </label>
              <label className="block">
                <span className="text-xs font-medium text-gray-700">
                  Infotext für Eltern
                </span>
                <textarea
                  value={field.content ?? ""}
                  onChange={(event) =>
                    onChange({ content: event.target.value })
                  }
                  placeholder="Dieser Text wird Eltern im Formular angezeigt."
                  disabled={disabled}
                  rows={3}
                  className="mt-1 w-full rounded-lg border border-gray-200 px-3 py-2 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                />
              </label>
              <div className="grid gap-2 sm:grid-cols-2">
                <FormChoice
                  checked={Boolean(field.applies_to_child)}
                  onChange={(checked) =>
                    onChange({ applies_to_child: checked })
                  }
                  label="Pro Kind anzeigen"
                  hint="Der Hinweis erscheint bei jedem angemeldeten Kind."
                  disabled={disabled}
                />
              </div>
            </>
          ) : (
            <>
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <label className="block">
                  <span className="text-xs font-medium text-gray-700">
                    Frage im Elternformular
                  </span>
                  <input
                    type="text"
                    value={field.label}
                    onChange={(event) => {
                      const nextLabel = event.target.value;
                      if (isTargetField) {
                        // Suggested fields keep their fixed key + type; only
                        // the displayed question text is editable.
                        onChange({ label: nextLabel });
                        return;
                      }
                      const currentAutoKey = normalizeFieldKey(field.label);
                      const shouldUpdateKey =
                        field.key.trim() === "" || field.key === currentAutoKey;
                      onChange({
                        label: nextLabel,
                        key: shouldUpdateKey
                          ? normalizeFieldKey(nextLabel)
                          : field.key,
                      });
                    }}
                    placeholder={
                      isTargetField && target
                        ? RESERVED_TARGETS[target].label
                        : "z. B. Allergien oder Hinweise"
                    }
                    disabled={disabled}
                    className="mt-1 h-10 w-full rounded-lg border border-gray-200 px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                  />
                </label>
                <label className="block">
                  <span className="text-xs font-medium text-gray-700">
                    Typ
                    {isTargetField ? (
                      <span className="ml-1 text-[11px] font-normal text-gray-500">
                        (automatisch festgelegt)
                      </span>
                    ) : null}
                  </span>
                  <CustomSelect
                    ariaLabel="Typ"
                    value={field.type}
                    onChange={(value) =>
                      onChange({ type: value as FormFieldType })
                    }
                    disabled={disabled || isTargetField}
                    className="mt-1"
                    options={(isTargetField
                      ? [field.type]
                      : freeFieldTypes
                    ).map((value) => ({
                      value,
                      label: fieldTypeLabels[value],
                    }))}
                  />
                </label>
              </div>

              <label className="block">
                <span className="text-xs font-medium text-gray-700">
                  Hilfetext für Eltern
                </span>
                <input
                  type="text"
                  value={field.help_text ?? ""}
                  onChange={(event) =>
                    onChange({ help_text: event.target.value })
                  }
                  placeholder="Optionaler kurzer Hinweis unter der Frage"
                  disabled={disabled}
                  className="mt-1 h-10 w-full rounded-lg border border-gray-200 px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                />
              </label>

              {field.type === "select" ? (
                <label className="block">
                  <span className="text-xs font-medium text-gray-700">
                    Auswahloptionen
                  </span>
                  <textarea
                    value={optionsDraft}
                    onChange={(event) => updateOptions(event.target.value)}
                    placeholder={"Eine Option pro Zeile\nz. B. Ja\nz. B. Nein"}
                    disabled={disabled || isTargetField}
                    rows={3}
                    className="mt-1 w-full rounded-lg border border-gray-200 px-3 py-2 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                  />
                </label>
              ) : null}

              {field.target === "schedule.pickup" ? (
                <div>
                  <span className="text-xs font-medium text-gray-700">
                    Feste Auswahlzeiten (optional)
                  </span>
                  <p className="mt-1 text-xs leading-5 text-gray-500">
                    Ohne Zeiten geben Eltern die Uhrzeit frei ein. Sobald Zeiten
                    hinterlegt sind, wählen Eltern pro Wochentag nur aus dieser
                    Liste.
                  </p>
                  <button
                    type="button"
                    onClick={addAllowedTime}
                    disabled={disabled}
                    className="mt-2 inline-flex h-9 items-center justify-center gap-2 rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    <Plus className="h-4 w-4" aria-hidden="true" />
                    Zeit hinzufügen
                  </button>
                  {allowedTimesRows.length > 0 ? (
                    <ul className="mt-3 grid grid-cols-1 gap-2 sm:grid-cols-2">
                      {allowedTimesRows.map((row, index) => (
                        <li key={row.id} className="flex items-center gap-2">
                          <input
                            type="time"
                            value={row.value}
                            onChange={(event) =>
                              updateAllowedTime(index, event.target.value)
                            }
                            disabled={disabled}
                            aria-label={`Auswahlzeit ${index + 1}`}
                            className="h-10 w-full min-w-0 flex-1 rounded-lg border border-gray-200 bg-white px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                          />
                          <button
                            type="button"
                            onClick={() => removeAllowedTime(index)}
                            disabled={disabled}
                            className="border-moto-red/20 text-moto-red-strong hover:bg-moto-red/10 focus-visible:ring-moto-red/30 inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border bg-white shadow-sm transition-colors focus-visible:ring-2 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-40"
                            aria-label={`Auswahlzeit ${index + 1} entfernen`}
                          >
                            <Trash2 className="h-4 w-4" aria-hidden="true" />
                          </button>
                        </li>
                      ))}
                    </ul>
                  ) : null}
                </div>
              ) : null}

              <div className="grid gap-2 sm:grid-cols-2">
                <FormChoice
                  checked={Boolean(field.required)}
                  onChange={(checked) => onChange({ required: checked })}
                  label="Pflichtfrage"
                  hint={getRequiredHint(field)}
                  disabled={disabled}
                />
                <FormChoice
                  checked={Boolean(field.applies_to_child)}
                  onChange={(checked) =>
                    onChange({ applies_to_child: checked })
                  }
                  label="Pro Kind abfragen"
                  hint="Die Frage erscheint für jedes angemeldete Kind."
                  disabled={disabled || isTargetField}
                />
              </div>
            </>
          )}

          <ConditionEditor
            field={field}
            index={index}
            allFields={allFields}
            onChange={onChange}
            disabled={disabled}
          />
        </div>
      </div>
    </article>
  );
}

function FormChoice({
  checked,
  onChange,
  label,
  hint,
  disabled,
}: Readonly<{
  checked: boolean;
  onChange: (checked: boolean) => void;
  label: string;
  hint: string;
  disabled: boolean;
}>) {
  return (
    <button
      type="button"
      role="checkbox"
      aria-checked={checked}
      onClick={() => onChange(!checked)}
      disabled={disabled}
      className={`flex cursor-pointer items-start gap-3 rounded-lg border p-3 transition-colors ${
        checked
          ? "border-moto-green/40 bg-moto-green/10"
          : "border-gray-200 bg-white hover:bg-gray-50"
      } ${disabled ? "cursor-not-allowed opacity-60" : ""}`}
    >
      <span
        className={`mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-md border ${
          checked
            ? "border-moto-green bg-moto-green text-gray-950"
            : "border-gray-300 bg-white"
        }`}
        aria-hidden="true"
      >
        {checked ? <Check className="h-3.5 w-3.5" /> : null}
      </span>
      <span className="min-w-0">
        <span className="block text-sm font-medium text-gray-900">{label}</span>
        <span className="mt-0.5 block text-xs text-gray-500">{hint}</span>
      </span>
    </button>
  );
}

const conditionInputClass =
  "moto-content-surface mt-1 h-10 w-full rounded-lg border px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:bg-gray-100 disabled:text-gray-600";

// Field-source operators offered in the condition editor. "includes" is
// reserved for the care-offering source and is not listed here.
const FIELD_CONDITION_OPERATORS: ConditionOperator[] = [
  "eq",
  "neq",
  "not_empty",
];

function defaultConditionForController(
  controller: FormField,
): VisibilityCondition {
  if (controller.type === "select") {
    return {
      source: "field",
      field: controller.key,
      operator: "eq",
      value: controller.options?.[0]?.value ?? "",
    };
  }
  return {
    source: "field",
    field: controller.key,
    operator: "eq",
    value: true,
  };
}

function defaultConditionForSource(
  source: ConditionSource,
  controllers: FormField[],
): VisibilityCondition {
  if (source === "grade_level") {
    return { source: "grade_level", operator: "eq", value: 1 };
  }
  if (source === "care_offering") {
    return { source: "care_offering", operator: "includes", value: "" };
  }
  const controller = controllers[0];
  return controller
    ? defaultConditionForController(controller)
    : { source: "field", field: "", operator: "eq", value: "" };
}

/**
 * Per-field visibility editor. Lets an admin gate a question or info
 * block behind another answer, the child's grade level, or a chosen
 * care offering. Eligible controlling questions are limited to yes/no
 * and selection fields observable from this field's scope.
 */
function ConditionEditor({
  field,
  index,
  allFields,
  onChange,
  disabled,
}: Readonly<{
  field: FormField;
  index: number;
  allFields: FormField[];
  onChange: (patch: Partial<FormField>) => void;
  disabled: boolean;
}>) {
  const condition = field.visible_when ?? null;

  const controllers = allFields.filter(
    (f, i) =>
      i !== index &&
      f.key.trim() !== "" &&
      (f.type === "boolean" || f.type === "select") &&
      (field.applies_to_child || !f.applies_to_child),
  );

  const sources: ConditionSource[] = [];
  if (controllers.length > 0) sources.push("field");
  if (field.applies_to_child) {
    sources.push("grade_level", "care_offering");
  }
  const canEnable = sources.length > 0;

  const setCondition = (next: VisibilityCondition | null) =>
    onChange({ visible_when: next });
  const patchCondition = (patch: Partial<VisibilityCondition>) => {
    if (condition) setCondition({ ...condition, ...patch });
  };

  if (!canEnable && !condition) {
    return (
      <p className="rounded-lg border border-dashed border-gray-200 bg-gray-50/60 px-3 py-2 text-xs leading-5 text-gray-500">
        Eine Sichtbarkeitsregel ist möglich, sobald es eine Ja/Nein- oder
        Auswahlfrage gibt
        {field.applies_to_child
          ? "."
          : " (oder dieses Feld pro Kind angezeigt wird)."}
      </p>
    );
  }

  return (
    <div className="rounded-xl border border-gray-200 bg-white p-3">
      <FormChoice
        checked={condition !== null}
        onChange={(checked) =>
          setCondition(
            checked
              ? defaultConditionForSource(sources[0]!, controllers)
              : null,
          )
        }
        label="Nur unter einer Bedingung anzeigen"
        hint="Das Feld erscheint nur, wenn die Bedingung erfüllt ist."
        disabled={disabled || !canEnable}
      />

      {condition ? (
        <div className="mt-3 space-y-2">
          <label
            className="block"
            htmlFor={`condition-${index}-source`}
            id={`condition-${index}-source-label`}
          >
            <span className="text-xs font-medium text-gray-700">
              Sichtbar wenn
            </span>
            <CustomSelect
              ariaLabelledBy={`condition-${index}-source-label`}
              id={`condition-${index}-source`}
              value={condition.source}
              onChange={(value) =>
                setCondition(
                  defaultConditionForSource(
                    value as ConditionSource,
                    controllers,
                  ),
                )
              }
              disabled={disabled}
              className={conditionInputClass}
              options={sources.map((source) => ({
                value: source,
                label: conditionSourceLabels[source],
              }))}
            />
          </label>

          {condition.source === "field" ? (
            <ConditionFieldControls
              condition={condition}
              controllers={controllers}
              onPatch={patchCondition}
              onReplace={setCondition}
              disabled={disabled}
              idPrefix={`condition-${index}-field`}
            />
          ) : condition.source === "grade_level" ? (
            <ConditionGradeControls
              condition={condition}
              onPatch={patchCondition}
              disabled={disabled}
              idPrefix={`condition-${index}-grade`}
            />
          ) : (
            <ConditionOfferingControls
              condition={condition}
              onPatch={patchCondition}
              disabled={disabled}
            />
          )}
        </div>
      ) : null}
    </div>
  );
}

function ConditionFieldControls({
  condition,
  controllers,
  onPatch,
  onReplace,
  disabled,
  idPrefix,
}: Readonly<{
  condition: VisibilityCondition;
  controllers: FormField[];
  onPatch: (patch: Partial<VisibilityCondition>) => void;
  onReplace: (next: VisibilityCondition) => void;
  disabled: boolean;
  idPrefix: string;
}>) {
  const controller = controllers.find((c) => c.key === condition.field);

  const changeController = (key: string) => {
    const next = controllers.find((c) => c.key === key);
    if (!next) return;
    const base = defaultConditionForController(next);
    onReplace(
      condition.operator === "not_empty"
        ? { ...base, operator: "not_empty", value: undefined }
        : base,
    );
  };

  const changeOperator = (operator: ConditionOperator) => {
    if (operator === "not_empty") {
      onPatch({ operator, value: undefined });
      return;
    }
    const fallback = controller
      ? defaultConditionForController(controller).value
      : "";
    onPatch({ operator, value: condition.value ?? fallback });
  };

  return (
    <>
      <label
        className="block"
        htmlFor={`${idPrefix}-question`}
        id={`${idPrefix}-question-label`}
      >
        <span className="text-xs font-medium text-gray-700">Frage</span>
        <CustomSelect
          ariaLabelledBy={`${idPrefix}-question-label`}
          id={`${idPrefix}-question`}
          value={condition.field ?? ""}
          onChange={changeController}
          disabled={disabled}
          className={conditionInputClass}
          options={controllers.map((controller) => ({
            value: controller.key,
            label: controller.label.trim() || controller.key,
          }))}
        />
      </label>

      <label
        className="block"
        htmlFor={`${idPrefix}-operator`}
        id={`${idPrefix}-operator-label`}
      >
        <span className="text-xs font-medium text-gray-700">Vergleich</span>
        <CustomSelect
          ariaLabelledBy={`${idPrefix}-operator-label`}
          id={`${idPrefix}-operator`}
          value={condition.operator}
          onChange={(value) => changeOperator(value as ConditionOperator)}
          disabled={disabled}
          className={conditionInputClass}
          options={FIELD_CONDITION_OPERATORS.map((operator) => ({
            value: operator,
            label: conditionOperatorLabels[operator],
          }))}
        />
      </label>

      {condition.operator !== "not_empty" && controller ? (
        <label
          className="block"
          htmlFor={`${idPrefix}-value`}
          id={`${idPrefix}-value-label`}
        >
          <span className="text-xs font-medium text-gray-700">Wert</span>
          {controller.type === "boolean" ? (
            <CustomSelect
              ariaLabelledBy={`${idPrefix}-value-label`}
              id={`${idPrefix}-value`}
              value={condition.value === true ? "true" : "false"}
              onChange={(value) => onPatch({ value: value === "true" })}
              disabled={disabled}
              className={conditionInputClass}
              options={[
                { value: "true", label: "Ja" },
                { value: "false", label: "Nein" },
              ]}
            />
          ) : (
            <CustomSelect
              ariaLabelledBy={`${idPrefix}-value-label`}
              id={`${idPrefix}-value`}
              value={String(condition.value ?? "")}
              onChange={(value) => onPatch({ value })}
              disabled={disabled}
              className={conditionInputClass}
              options={(controller.options ?? []).map((option) => ({
                value: option.value,
                label: option.label,
              }))}
            />
          )}
        </label>
      ) : null}
    </>
  );
}

function ConditionGradeControls({
  condition,
  onPatch,
  disabled,
  idPrefix,
}: Readonly<{
  condition: VisibilityCondition;
  onPatch: (patch: Partial<VisibilityCondition>) => void;
  disabled: boolean;
  idPrefix: string;
}>) {
  return (
    <>
      <label
        className="block"
        htmlFor={`${idPrefix}-operator`}
        id={`${idPrefix}-operator-label`}
      >
        <span className="text-xs font-medium text-gray-700">Vergleich</span>
        <CustomSelect
          ariaLabelledBy={`${idPrefix}-operator-label`}
          id={`${idPrefix}-operator`}
          value={condition.operator}
          onChange={(value) =>
            onPatch({ operator: value as ConditionOperator })
          }
          disabled={disabled}
          className={conditionInputClass}
          options={[
            { value: "eq", label: conditionOperatorLabels.eq },
            { value: "neq", label: conditionOperatorLabels.neq },
          ]}
        />
      </label>
      <label className="block">
        <span className="text-xs font-medium text-gray-700">Klassenstufe</span>
        <input
          type="number"
          min={1}
          value={String(condition.value ?? "")}
          onChange={(event) =>
            onPatch({
              value:
                event.target.value === "" ? "" : Number(event.target.value),
            })
          }
          disabled={disabled}
          className={conditionInputClass}
        />
      </label>
    </>
  );
}

function ConditionOfferingControls({
  condition,
  onPatch,
  disabled,
}: Readonly<{
  condition: VisibilityCondition;
  onPatch: (patch: Partial<VisibilityCondition>) => void;
  disabled: boolean;
}>) {
  return (
    <label className="block">
      <span className="text-xs font-medium text-gray-700">
        Angebotsname (enthält)
      </span>
      <input
        type="text"
        value={String(condition.value ?? "")}
        onChange={(event) => onPatch({ value: event.target.value })}
        placeholder="z. B. Mittagessen"
        disabled={disabled}
        className={conditionInputClass}
      />
      <span className="mt-1 block text-[11px] leading-4 text-gray-500">
        Sichtbar, wenn ein gewähltes Betreuungsangebot diesen Namen trägt.
      </span>
    </label>
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
  const guardianFields = [
    "Vorname *",
    "Nachname *",
    "E-Mail *",
    coreRequirements.guardian_phone ? "Telefonnummer *" : "Telefonnummer",
  ];

  return (
    <div className={sticky ? "sticky top-6 space-y-4" : "space-y-4"}>
      <div className="flex items-start justify-between gap-4">
        <div>
          <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
            Vorschau
          </p>
          <div className="mt-1 flex flex-wrap items-center gap-2">
            <h2 className="text-base font-semibold text-gray-900">
              Elternformular
            </h2>
            <span
              className={`inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-medium ${previewStatus.className}`}
            >
              <span
                className={`h-1.5 w-1.5 rounded-full ${previewStatus.dotClassName}`}
                aria-hidden="true"
              />
              {previewStatus.label}
            </span>
          </div>
          <p className="mt-1 max-w-sm text-xs leading-5 text-gray-500">
            {previewStatus.hint}
          </p>
        </div>
        <span className="inline-flex h-9 w-9 items-center justify-center rounded-xl bg-white text-gray-600 shadow-sm">
          <Eye className="h-4 w-4" aria-hidden="true" />
        </span>
      </div>

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
              Speichert die Vorlage und öffnet die Vorschau in einem neuen Tab.
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
              Speichere die Vorlage zuerst.
            </span>
          </span>
        </div>
      )}

      <div className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
        <div className="border-b border-gray-100 px-4 py-4">
          <p className="text-moto-blue text-xs font-semibold tracking-wide uppercase">
            Online-Anmeldung
          </p>
          <h3 className="mt-1 text-lg font-semibold text-gray-900">
            {templateName.trim() || "Basisformular"}
          </h3>
          <p className="mt-1 text-sm leading-6 text-gray-500">
            Bitte füllen Sie das Formular vollständig aus.
          </p>
        </div>

        <div className="space-y-5 p-4">
          <PreviewSection title="Elternteil" fields={guardianFields} />
          <PreviewSection
            title="Kind"
            fields={[
              "Vorname *",
              "Nachname *",
              "Geburtsdatum *",
              "Klassenstufe *",
            ]}
          />
          <PreviewSection
            title="Betreuungsangebot"
            fields={["Gewünschtes Angebot", "Betreuungstage"]}
          />

          <section>
            <div className="mb-3 flex items-center justify-between gap-3">
              <h4 className="text-sm font-semibold text-gray-900">
                Zusatzfragen
              </h4>
              <span className="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600">
                {fields.length}
              </span>
            </div>
            {fields.length === 0 ? (
              <div className="rounded-xl border border-dashed border-gray-200 bg-gray-50/80 px-4 py-5 text-center">
                <HelpCircle className="mx-auto h-5 w-5 text-gray-400" />
                <p className="mt-2 text-sm font-medium text-gray-700">
                  Keine Zusatzfragen
                </p>
                <p className="mt-1 text-xs leading-5 text-gray-500">
                  Eltern sehen nur das Basisformular mit den gewählten
                  Pflichtangaben.
                </p>
              </div>
            ) : (
              <div className="space-y-3">
                {fields.map((field) => (
                  <PreviewCustomField
                    key={getStableObjectKey(field, "preview-field")}
                    field={field}
                  />
                ))}
              </div>
            )}
          </section>

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
                            <span className="shrink-0 rounded-full bg-gray-100 px-2 py-0.5 text-[10px] font-medium text-gray-600">
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
                          {previewText.trim() !== "" ? (
                            <p className="mt-1 line-clamp-2 text-[11px] leading-4 text-gray-400">
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

          <button
            type="button"
            className="h-9 w-full rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm"
          >
            Anmeldung absenden
          </button>
        </div>
      </div>
    </div>
  );
}

function PreviewSection({
  title,
  fields,
}: Readonly<{ title: string; fields: string[] }>) {
  return (
    <section>
      <h4 className="text-sm font-semibold text-gray-900">{title}</h4>
      <div className="mt-3 grid gap-2">
        {fields.map((field) => (
          <div
            key={field}
            className="rounded-lg border border-gray-200 bg-white px-3 py-2"
          >
            <span className="block text-[11px] font-medium text-gray-500">
              {field}
            </span>
            <span className="mt-1 block h-2 w-2/3 rounded-full bg-gray-100" />
          </div>
        ))}
      </div>
    </section>
  );
}

function ConditionalBadge({ field }: Readonly<{ field: FormField }>) {
  if (!field.visible_when) return null;
  return (
    <span className="bg-moto-blue/10 inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-medium text-[#3D63B0]">
      bedingt
    </span>
  );
}

function PreviewCustomField({ field }: Readonly<{ field: FormField }>) {
  if (field.type === "information") {
    return (
      <div className="border-moto-blue/20 bg-moto-blue/5 rounded-lg border px-3 py-2">
        <div className="flex items-start justify-between gap-3">
          {field.label.trim() ? (
            <span className="text-sm font-semibold text-gray-900">
              {field.label}
            </span>
          ) : (
            <span className="text-xs font-medium text-[#3D63B0]">Infotext</span>
          )}
          <ConditionalBadge field={field} />
        </div>
        {field.content?.trim() ? (
          <p className="mt-1 text-xs leading-5 whitespace-pre-line text-gray-600">
            {field.content}
          </p>
        ) : (
          <p className="mt-1 text-xs leading-5 text-gray-400 italic">
            Noch kein Text hinterlegt.
          </p>
        )}
      </div>
    );
  }

  return (
    <div className="rounded-lg border border-gray-200 bg-white px-3 py-2">
      <div className="flex items-start justify-between gap-3">
        <span className="text-sm font-medium text-gray-900">
          {field.label.trim() || "Neue Zusatzfrage"}
        </span>
        <div className="flex shrink-0 items-center gap-1.5">
          <ConditionalBadge field={field} />
          {field.required ? (
            <span className="rounded-full bg-gray-100 px-2 py-0.5 text-[10px] font-medium text-gray-600">
              Pflicht
            </span>
          ) : null}
        </div>
      </div>
      {field.help_text ? (
        <p className="mt-1 text-xs leading-5 text-gray-500">
          {field.help_text}
        </p>
      ) : null}
      <PreviewInput field={field} />
    </div>
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
      <div className="mt-3 flex h-10 items-center justify-between rounded-lg border border-gray-200 bg-white px-3 text-sm text-gray-500 shadow-sm">
        <span className="min-w-0 truncate">{firstOption}</span>
        <ChevronDown
          className="h-4 w-4 shrink-0 text-gray-400"
          aria-hidden="true"
        />
      </div>
    );
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
      className: "bg-moto-green/10 text-[#5F9F20]",
      dotClassName: "bg-moto-green",
    };
  }

  if (isActive) {
    return {
      label: "Bereit zur Zuordnung",
      hint: "Eltern sehen diese Vorlage erst, wenn sie einer Anmeldephase zugeordnet ist.",
      className: "bg-moto-green/10 text-[#5F9F20]",
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
    hint: "Speichere die Vorlage zuerst. Danach kannst du sie in einer Anmeldephase auswählen.",
    className: "bg-gray-100 text-gray-600",
    dotClassName: "bg-gray-300",
  };
}

// Snake-case-ify a free-form label into a schema-safe field key.
//
// Implementation is character-loop based on purpose: SonarCloud
// (rule S5852) flags `+` / `*` quantifiers on character classes as
// potentially super-linear under backtracking. The patterns here
// aren't actually catastrophic (single char classes, no overlapping
// alternation), but the loop has the same O(n) cost and avoids the
// scanner warning. Inputs are also capped \u2014 admin labels never
// legitimately exceed a few hundred chars.
function normalizeFieldKey(value: string): string {
  const MAX_INPUT = 256;
  const source = value
    .slice(0, MAX_INPUT)
    .trim()
    .toLowerCase()
    .normalize("NFD");

  let out = "";
  let pendingUnderscore = false;
  for (let i = 0; i < source.length; i++) {
    const code = source.charCodeAt(i);
    // Combining diacritical marks introduced by NFD \u2014 skip.
    if (code >= 0x0300 && code <= 0x036f) continue;
    const isLower = code >= 0x61 && code <= 0x7a; // a-z
    const isDigit = code >= 0x30 && code <= 0x39; // 0-9
    if (isLower || isDigit) {
      if (pendingUnderscore && out.length > 0) {
        out += "_";
      }
      out += source[i];
      pendingUnderscore = false;
    } else {
      // Any other character \u2192 collapse into a single underscore
      // (deferred so we don't leave a trailing one).
      pendingUnderscore = true;
    }
  }
  return out;
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
        // Carry the admin-configured fixed pickup times through; the
        // rebuilt base field drops them otherwise. Only the pickup-times
        // field may carry this (arrival stays free-entry, and the backend
        // rejects allowed_times on any other target), so anything else is
        // forced to undefined and omitted on serialize.
        allowed_times:
          field.target === "schedule.pickup" ? field.allowed_times : undefined,
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
      };
    }
    return {
      ...field,
      key: field.key.trim() || normalizeFieldKey(field.label),
      label: field.label.trim(),
      help_text: field.help_text?.trim() ?? "",
      sort_order: index,
      visible_when: field.visible_when ?? undefined,
    };
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
    return {
      key: normalizeFieldKey(block.key) || `custom_consent_${index + 1}`,
      kind: block.kind,
      title: block.title.trim(),
      label: block.label.trim(),
      text: block.text.trim(),
      required: block.kind === "notice" ? false : Boolean(block.required),
      enabled: Boolean(block.enabled),
      sort_order: index * 10 + 10,
      source: block.source ?? "custom",
      display_mode: displayMode,
      document_url: documentURL,
    };
  });
}

function draftDocumentURLsInLegalBlocks(
  blocks: FormLegalBlock[],
  draftURLs: ReadonlySet<string>,
): Set<string> {
  const urls = new Set<string>();
  for (const block of blocks) {
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

function getRequiredHint(field: FormField): string {
  if (field.type === "boolean") {
    return "Eltern müssen Ja oder Nein auswählen.";
  }
  if (field.type === "weekday_schedule") {
    return "Eltern müssen für jeden Betreuungstag eine Uhrzeit angeben (ohne Betreuungsangebote: mindestens eine).";
  }
  if (field.type === "weekday_boolean") {
    // Legacy pickup (student.pickup_status) accepts an empty selection ("geht
    // alleine nach Hause") -- required only forces parents to confirm the
    // field once, not to tick a day. Every other weekday_boolean (Buskind)
    // genuinely needs at least one day.
    if (field.target === "student.pickup_status") {
      return "Eltern müssen die Abholregelung bestätigen (Tage auswählen oder leer lassen).";
    }
    return "Eltern müssen mindestens einen Wochentag auswählen.";
  }
  if (field.type === "weekday_multi_mode") {
    return "Eltern müssen pro Betreuungstag mindestens einen Heimweg auswählen.";
  }
  if (field.type === "contact_list") {
    return "Eltern müssen mindestens einen vollständigen Kontakt angeben.";
  }
  if (field.type === "phone_list") {
    return "Eltern müssen mindestens eine Telefonnummer angeben.";
  }
  return "Eltern müssen diese Frage beantworten.";
}

function getSchemaDraftValidationMessage({
  fields,
  legalBlocks,
  name,
}: Readonly<{
  fields: FormField[];
  legalBlocks: FormLegalBlock[];
  name: string;
}>): string | null {
  if (name.trim() === "") {
    return "Bitte gib zuerst einen Namen für die Vorlage ein.";
  }

  const seenKeys = new Set<string>();
  for (const [index, field] of fields.entries()) {
    const position = index + 1;

    if (field.type === "information") {
      if ((field.content ?? "").trim() === "") {
        return `Bitte gib für den Infotext ${position} einen Text ein.`;
      }
      const infoKey =
        field.key.trim() ||
        normalizeFieldKey(field.label) ||
        `infotext_${position}`;
      if (seenKeys.has(infoKey)) {
        return `Bitte ändere Infotext ${position}. Es entsteht ein doppeltes internes Feld.`;
      }
      seenKeys.add(infoKey);
      continue;
    }

    if (field.label.trim() === "") {
      return `Bitte gib für Frage ${position} einen Fragetext ein.`;
    }

    const key = field.key.trim() || normalizeFieldKey(field.label);
    if (key === "") {
      return `Bitte ändere Frage ${position}. Aus dem Fragetext konnte kein internes Feld erzeugt werden.`;
    }
    if (seenKeys.has(key)) {
      return `Bitte ändere Frage ${position}. Zwei Zusatzfragen haben denselben oder einen zu ähnlichen Fragetext.`;
    }
    seenKeys.add(key);

    if (!field.target && structuredFieldTypes.has(field.type)) {
      return `Bitte wähle für Frage ${position} einen einfachen Typ. Telefonlisten, Wochenzeiten und Kontaktlisten sind nur als feste Vorschläge verfügbar.`;
    }

    if (field.type === "select" && (field.options ?? []).length === 0) {
      return `Bitte ergänze für Frage ${position} mindestens eine Auswahloption.`;
    }
  }

  const seenLegalKeys = new Set<string>();
  for (const [index, block] of prepareLegalBlocksForSave(
    legalBlocks,
  ).entries()) {
    const position = index + 1;
    if (seenLegalKeys.has(block.key)) {
      return `Bitte ändere Zustimmung ${position}. Zwei Zustimmungen haben denselben internen Schlüssel.`;
    }
    seenLegalKeys.add(block.key);
    if (!block.enabled) continue;
    if (block.title === "") {
      return `Bitte gib für Zustimmung ${position} einen Titel ein.`;
    }
    if (block.label === "") {
      return `Bitte gib für Zustimmung ${position} einen Text neben der Checkbox ein.`;
    }
    if (
      block.key === "agb" &&
      block.display_mode === LEGAL_BLOCK_DISPLAY_MODE_PDF &&
      (block.document_url ?? "").trim() === ""
    ) {
      return "Bitte lade für die AGB eine PDF-Datei hoch oder wähle wieder Text eingeben.";
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

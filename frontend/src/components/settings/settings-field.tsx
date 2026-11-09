"use client";

import {
  useState,
  useCallback,
  useEffect,
  useEffectEvent,
  useLayoutEffect,
  useRef,
  useId,
} from "react";
import {
  ExternalLink,
  FileText,
  FileUp,
  Pencil,
  RotateCcw,
  TextAlignStart,
  Trash2,
} from "lucide-react";
import { IconTile } from "~/components/ui/icon-tile";
import { StatusBadge } from "~/components/ui/status-badge";
import type { ResolvedSetting } from "~/lib/settings-api";
import {
  useApiErrorDisplay,
  useApiFormError,
  useToast,
} from "~/contexts/ToastContext";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { InsetPanel } from "~/components/ui/inset-panel";
import {
  ApiError,
  apiErrorFromResponse,
  transportFetch,
} from "~/lib/api-error";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { ConfirmationModal } from "~/components/ui/modal";
import { Button, buttonClassName } from "~/components/ui/button";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { Textarea } from "~/components/ui/textarea";
import { MODULE_SETTING_KEYS } from "~/lib/modules";
import {
  BUILT_IN_PAYMENT_TEXT,
  PAYMENT_TEXT_KEY,
  paymentTextProblem,
} from "~/lib/traeger-payment-text";
import { textTooLong } from "~/lib/text-length";
import { Switch } from "~/components/ui/switch";
import { NumberField } from "./fields/number-field";
import { SettingsTimeField } from "./fields/time-field";
import { DateField } from "./fields/date-field";
import { TextField } from "./fields/text-field";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { type ColorPreset, ColorField } from "./fields/color-field";
import { TextareaField } from "./fields/textarea-field";
import { PasswordField } from "./fields/password-field";
import { SelectField } from "./fields/select-field";
import {
  isOperatorManaged,
  retentionShorteningText,
  shortenedRetentionDays,
} from "./retention-shortening";
import {
  type SettingsAudience,
  followsTraegerAfterReset,
  resetRestoresTraegerValue,
  settingOriginNote,
  showsStandardBadge,
} from "./settings-origin";

/** Settings keys that require a confirmation dialog before enabling. */
const CONFIRM_ON_ENABLE: Record<
  string,
  { title: string; body: string; confirmText?: string }
> = {
  "gdpr.attendance_log_enabled": {
    title: "Anwesenheitsprotokoll einschalten?",
    body: "Das Team sieht Anwesenheitszeiten und Raumverläufe der Kinder. Ausgeschaltet bleiben frühere Raumbesuche nur für den heutigen Tag. Ältere abgeschlossene Raumbesuche werden beim Ausschalten und nachts gelöscht. Der aktuelle Raum bleibt sichtbar. Vor dem Einschalten prüft die Schule die Rechtsgrundlage.",
    confirmText: "Einschalten",
  },
};

/**
 * Settings keys that require a confirmation dialog before disabling.
 * Triggered when a destructive side effect on the backend is gated on
 * the toggle (e.g. clearing already-stored student photos).
 */
const CONFIRM_ON_DISABLE: Record<
  string,
  { title: string; body: string; confirmText?: string }
> = {
  "operations.student_photos_enabled": {
    title: "Kinderfotos deaktivieren?",
    body: "Beim Ausschalten werden ALLE bereits hochgeladenen Kinderfotos sofort und unwiderruflich gelöscht. Die elterliche Einwilligung bleibt dokumentiert. Diese Aktion kann nicht rückgängig gemacht werden.",
    confirmText: "Deaktivieren und Fotos löschen",
  },
};

// operator_managed settings (ADR 0048) say who decides them: the moto team,
// by contract. Module switches (#4892) name the booking, the federal state
// (#4775) the holidays, every other one is a retention window.
function operatorManagedNote(key: string, audience: SettingsAudience): string {
  if (key === "operations.federal_state") {
    return audience === "operator"
      ? "Bundesland der Schule. Die Schule sieht es, kann es aber nicht ändern."
      : "Das Bundesland legt moto mit Ihrer Schule fest. Stimmt es nicht, wenden Sie sich an das moto-Team.";
  }
  if (MODULE_SETTING_KEYS.includes(key)) {
    return audience === "operator"
      ? "Gebuchtes Modul aus dem Vertrag. Die Schule sieht es, kann es aber nicht ändern."
      : "Welche Module Ihre Schule hat, steht im Vertrag mit moto. Für eine Änderung wenden Sie sich an das moto-Team.";
  }
  return audience === "operator"
    ? "Löschfrist aus dem Vertrag. Die Schule sieht sie, kann sie aber nicht ändern."
    : "Diese Frist legt moto mit Ihrer Schule fest. Für eine Änderung wenden Sie sich an moto.";
}

function toStr(v: unknown): string {
  if (v == null) return "";
  if (typeof v === "string") return v;
  if (typeof v === "number" || typeof v === "boolean") return v.toString();
  return JSON.stringify(v);
}

/**
 * What an optional time setting shows while it is empty. „Jederzeit“ fits a
 * time that limits something; a lesson without an end time (#3372) or a care
 * time preset (#3371) limits nothing, it is just not maintained.
 */
const NOT_ENTERED_TIME_PREFIXES = ["school_periods.", "care_times."];

function emptyTimeLabel(setting: ResolvedSetting): string | undefined {
  if (setting.default !== "") return undefined;
  return NOT_ENTERED_TIME_PREFIXES.some((prefix) =>
    setting.key.startsWith(prefix),
  )
    ? "Nicht eingetragen"
    : "Jederzeit";
}

const REQUIRED_ENROLLMENT_LEGAL_TEXT_ERROR =
  "Dieser Text oder eine PDF-Datei ist erforderlich, solange der Block im Anmeldeformular angezeigt wird.";

const ENROLLMENT_LEGAL_AGB_TEXT_KEY = "enrollment.legal_agb_text";
const ENROLLMENT_LEGAL_AGB_DOCUMENT_URL_KEY =
  "enrollment.legal_agb_document_url";
const ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_KEY =
  "enrollment.legal_agb_display_mode";
const ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_TEXT = "text";
const ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_PDF = "pdf";

type LegalAGBDisplayMode =
  | typeof ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_TEXT
  | typeof ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_PDF;

const ENROLLMENT_LEGAL_TEXT_KEYS = new Set([
  ENROLLMENT_LEGAL_AGB_TEXT_KEY,
  "enrollment.legal_dsgvo_text",
  "enrollment.legal_photo_text",
  "enrollment.legal_email_contact_text",
]);

function isEnrollmentLegalTextKey(key: string) {
  return ENROLLMENT_LEGAL_TEXT_KEYS.has(key);
}

const ENROLLMENT_LEGAL_TOGGLE_TO_TEXT_KEY: Record<string, string> = {
  "enrollment.legal_terms_enabled": ENROLLMENT_LEGAL_AGB_TEXT_KEY,
  "enrollment.legal_dsgvo_enabled": "enrollment.legal_dsgvo_text",
  "enrollment.legal_photo_enabled": "enrollment.legal_photo_text",
  "enrollment.legal_email_contact_enabled":
    "enrollment.legal_email_contact_text",
};

const ENROLLMENT_LEGAL_TEXT_TO_TOGGLE_KEY: Record<string, string> = {
  [ENROLLMENT_LEGAL_AGB_TEXT_KEY]: "enrollment.legal_terms_enabled",
  "enrollment.legal_dsgvo_text": "enrollment.legal_dsgvo_enabled",
  "enrollment.legal_photo_text": "enrollment.legal_photo_enabled",
  "enrollment.legal_email_contact_text":
    "enrollment.legal_email_contact_enabled",
};

const CARE_FACILITY_TERM_KEY = "enrollment.care_facility_term";

/**
 * The care-facility term (#6042) stands in sentences for parents, so the
 * form names what it accepts before the server refuses it.
 */
function careFacilityTermProblem(
  value: unknown,
  pattern: string | undefined,
): string | null {
  const text = typeof value === "string" ? value : "";
  if (text.trim() === "") return "Bitte geben Sie einen Namen ein.";
  if (!pattern) return null;
  let accepted: RegExp;
  try {
    accepted = new RegExp(pattern, "u");
  } catch {
    return null;
  }
  return accepted.test(text)
    ? null
    : "Erlaubt sind Buchstaben, Ziffern, Leerzeichen und . - / ( ). Am Anfang und Ende steht kein Leerzeichen.";
}

function validateLocally(
  setting: ResolvedSetting,
  value: unknown,
  categoryItems: ResolvedSetting[],
): string | null {
  const legalToggleKey = ENROLLMENT_LEGAL_TEXT_TO_TOGGLE_KEY[setting.key];
  if (legalToggleKey) {
    const toggleSetting = categoryItems.find(
      (item) => item.key === legalToggleKey,
    );
    if (
      toggleSetting?.value === true &&
      !hasEnrollmentLegalContent(setting.key, value, categoryItems)
    ) {
      return REQUIRED_ENROLLMENT_LEGAL_TEXT_ERROR;
    }
  }

  // The payment text of the Träger takes only its placeholders (#5622).
  if (setting.key === PAYMENT_TEXT_KEY) {
    return paymentTextProblem(typeof value === "string" ? value : "");
  }

  if (setting.key === CARE_FACILITY_TERM_KEY) {
    const problem = careFacilityTermProblem(value, setting.validation?.pattern);
    if (problem) return problem;
  }

  // A text limit counts characters like the backend (#5622).
  const maxLength = setting.validation?.max_length;
  if (maxLength != null && typeof value === "string") {
    const tooLong = textTooLong(value, maxLength);
    if (tooLong) return tooLong;
  }

  if (setting.type === "number") {
    const num = Number(value);
    if (isNaN(num)) return "Bitte eine Zahl eingeben.";
    if (setting.validation?.min != null && num < setting.validation.min) {
      return `Minimum: ${setting.validation.min}`;
    }
    if (setting.validation?.max != null && num > setting.validation.max) {
      return `Maximum: ${setting.validation.max}`;
    }
  }
  return null;
}

const AUTO_SAVE_DELAY_MS = 3000;
const HIGHLIGHT_FLASH_MS = 2200;
const EMPTY_CATEGORY_ITEMS: ResolvedSetting[] = [];

interface AttendanceScopeChange {
  /** Written in this order before the chosen value. */
  readonly changes: readonly { key: string; value: string }[];
  readonly writable: boolean;
  readonly title: string;
  readonly body: string;
  readonly confirmText: string;
}

const VISIBILITY_SCOPE_KEY = "operations.operational_overview_scope";

// Settings whose "all_staff" needs the school-wide view; the backend rejects
// any other order (#3180, #3622). `expand` answers widening one of them while
// the view is personal, `restrict` narrowing the view while only it is wide.
const DEPENDENT_SCOPES: Record<
  string,
  {
    readonly name: string;
    readonly expand: string;
    readonly restrict: { readonly title: string; readonly body: string };
  }
> = {
  "operations.attendance_edit_scope": {
    name: "An- und Abmelden",
    expand:
      "Überall an- und abmelden geht nur mit schulweitem Überblick. Das Team sieht danach alle Gruppen und Blöcke.",
    restrict: {
      title: "Auch die Bearbeitung begrenzen?",
      body: "Eigene Zuständigkeiten als Sichtbereich erfordern denselben Bereich für An- und Abmelden. Vorhandene Sonderzugänge bleiben erhalten.",
    },
  },
  "operations.block_start_scope": {
    name: "Blöcke starten",
    expand:
      "Das ganze Team darf nur starten, wenn es alle Blöcke sieht. Das Team sieht danach alle Gruppen und Blöcke.",
    restrict: {
      title: "Auch das Starten begrenzen?",
      body: "Dann dürfen nur eingeplante Kräfte Blöcke starten.",
    },
  },
  "operations.block_complete_scope": {
    name: "Blöcke beenden",
    expand:
      "Das ganze Team darf nur beenden, wenn es alle Blöcke sieht. Das Team sieht danach alle Gruppen und Blöcke.",
    restrict: {
      title: "Auch das Beenden begrenzen?",
      body: "Dann dürfen nur eingeplante Kräfte Blöcke beenden.",
    },
  },
};

function attendanceScopePrerequisite(
  key: string,
  value: unknown,
  items: ResolvedSetting[],
): AttendanceScopeChange | null {
  const visibility = items.find((item) => item.key === VISIBILITY_SCOPE_KEY);
  const dependent = DEPENDENT_SCOPES[key];
  if (dependent && value === "all_staff" && visibility?.value === "own") {
    return {
      changes: [{ key: visibility.key, value: "all_staff" }],
      writable: visibility.writable,
      title: "Auch den Sichtbereich erweitern?",
      body: dependent.expand,
      confirmText: "Beides erweitern",
    };
  }
  if (key !== visibility?.key || value !== "own") return null;
  const widened = items.flatMap((item) => {
    const scope = DEPENDENT_SCOPES[item.key];
    return scope && item.value === "all_staff" ? [{ item, scope }] : [];
  });
  const [first] = widened;
  if (!first) return null;
  const restriction = {
    changes: widened.map(({ item }) => ({ key: item.key, value: "own" })),
    writable: widened.every(({ item }) => item.writable),
  };
  if (widened.length > 1) {
    const names = widened.map(({ scope }) => scope.name).join(", ");
    return {
      ...restriction,
      title: "Auch diese Rechte begrenzen?",
      body: `Dann gelten nur noch eigene Zuständigkeiten für: ${names}. Vorhandene Sonderzugänge bleiben erhalten.`,
      confirmText: "Alles begrenzen",
    };
  }
  return {
    ...restriction,
    ...first.scope.restrict,
    confirmText: "Beides begrenzen",
  };
}

interface SettingsFieldProps {
  readonly setting: ResolvedSetting;
  readonly categoryItems?: ResolvedSetting[];
  readonly highlighted?: boolean;
  /**
   * Saves one value. Throws the ApiError of a failed save, which the field
   * shows on the shared error path (#2517, #2519).
   */
  readonly onSave: (key: string, value: unknown) => Promise<void>;
  /** Same contract as onSave. */
  readonly onReset: (key: string) => Promise<void>;
  readonly onSchemaRefresh?: () => void;
  readonly onBookingAuthorityEnable?: () => Promise<void>;
  // audience controls the "auch von {other side} änderbar" hint shown
  // on shared settings and the origin of a value (default, Träger, school).
  // Defaults to "admin" (tenant settings page); "traeger" is the Träger
  // portal, which sets values for all its schools (#4652).
  readonly audience?: SettingsAudience;
  // revealFn overrides how PasswordField fetches the unmasked value.
  // Defaults to the tenant reveal endpoint; the operator page passes a
  // school-bound function so reveal hits the operator endpoint instead.
  readonly revealFn?: (key: string) => Promise<string | null>;
}

interface PendingSettingSave {
  readonly setting: ResolvedSetting;
  readonly categoryItems: ResolvedSetting[];
  readonly localValue: unknown;
  readonly isDirty: boolean;
}

export function SettingsField({
  setting,
  categoryItems = EMPTY_CATEGORY_ITEMS,
  highlighted = false,
  onSave,
  onReset,
  onSchemaRefresh,
  onBookingAuthorityEnable,
  audience = "admin",
  revealFn,
}: SettingsFieldProps) {
  const { success: toastSuccess } = useToast();
  const [localValue, setLocalValue] = useState<unknown>(setting.value);
  const [isDirty, setIsDirty] = useState(false);
  const [serverValue, setServerValue] = useState<unknown>(setting.value);
  // Hint from a check before sending (range, missing legal text, a setting
  // that must change first). Server errors go through the shared path.
  const [hint, setHint] = useState<string | null>(null);
  const hintId = useId();
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [historyDeletion, setHistoryDeletion] = useState<
    "save" | "reset" | null
  >(null);
  const [historyDeletionSaving, setHistoryDeletionSaving] = useState(false);
  const historyDeletionRef = useRef<HTMLDivElement | null>(null);
  const historyDeletionErrors = useApiFormError(historyDeletionRef);
  const deletesRoomHistory = setting.key === "gdpr.attendance_log_enabled";
  // A shorter retention window waits here for the operator's confirmation
  // (ADR 0048): the next nightly run deletes everything older for good.
  const [pendingShortening, setPendingShortening] = useState<{
    readonly days: number;
    readonly value: unknown;
    readonly reset: boolean;
  } | null>(null);
  const [showHighlight, setShowHighlight] = useState(false);
  const [legalActivationOpen, setLegalActivationOpen] = useState(false);
  const [legalActivationText, setLegalActivationText] = useState("");
  const [legalActivationDocumentURL, setLegalActivationDocumentURL] =
    useState("");
  const [legalActivationMode, setLegalActivationMode] =
    useState<LegalAGBDisplayMode>(ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_TEXT);
  const [legalActivationSaving, setLegalActivationSaving] = useState(false);
  const [legalTextEditOpen, setLegalTextEditOpen] = useState(false);
  const [legalTextEditDraft, setLegalTextEditDraft] = useState("");
  const [legalTextEditSaving, setLegalTextEditSaving] = useState(false);
  const [legalDocumentDraftURL, setLegalDocumentDraftURL] = useState("");
  const [legalTextEditMode, setLegalTextEditMode] =
    useState<LegalAGBDisplayMode>(ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_TEXT);
  const [legalDocumentSaving, setLegalDocumentSaving] = useState(false);
  const fieldRef = useRef<HTMLDivElement | null>(null);
  const activationRef = useRef<HTMLDivElement | null>(null);
  const textEditRef = useRef<HTMLDivElement | null>(null);
  // A field saves on its own (no form): a failed save is a toast with the
  // catalog text (#2517). Errors from an open dialog stay in the dialog.
  const { show: showFieldError } = useApiErrorDisplay(fieldRef);
  const activationErrors = useApiFormError(activationRef);
  const textEditErrors = useApiFormError(textEditRef);
  const latestDoSaveRef = useRef<(value: unknown) => void>(() => undefined);
  const latestResetRef = useRef<() => void>(() => undefined);
  const latestActivationRef = useRef<() => void>(() => undefined);
  const latestTextEditRef = useRef<() => void>(() => undefined);
  const latestUploadRef = useRef<(file: File) => void>(() => undefined);
  const latestDeleteRef = useRef<() => void>(() => undefined);
  const settingObject = `die Einstellung „${setting.label}“`;
  const restoresTraegerValue = resetRestoresTraegerValue(setting, audience);
  const originNote = settingOriginNote(setting, audience);
  const legalActivationTextKey =
    ENROLLMENT_LEGAL_TOGGLE_TO_TEXT_KEY[setting.key] ?? null;
  const legalActivationTextSetting = legalActivationTextKey
    ? categoryItems.find((item) => item.key === legalActivationTextKey)
    : undefined;
  const isEnrollmentLegalTextSetting = isEnrollmentLegalTextKey(setting.key);
  const isAGBLegalContext =
    setting.key === ENROLLMENT_LEGAL_AGB_TEXT_KEY ||
    legalActivationTextKey === ENROLLMENT_LEGAL_AGB_TEXT_KEY;
  const legalDocumentSetting = isAGBLegalContext
    ? categoryItems.find(
        (item) => item.key === ENROLLMENT_LEGAL_AGB_DOCUMENT_URL_KEY,
      )
    : undefined;
  const legalDocumentURL = toStr(legalDocumentSetting?.value).trim();
  const legalAGBDisplayMode = legalAGBDisplayModeFromItems(categoryItems);
  const legalTermsEnabled = legalTermsEnabledFromItems(setting, categoryItems);
  const legalDocumentManagementDisabledReason =
    audience === "operator"
      ? "PDF-Dateien können nur im Schulportal hochgeladen oder entfernt werden."
      : undefined;
  const legalDocumentDeleteDisabledReason =
    audience !== "operator" &&
    legalTermsEnabled &&
    legalAGBDisplayMode === ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_PDF
      ? "Diese PDF kann nicht entfernt werden, solange die AGB aktiv sind und als PDF angezeigt werden. Wechsle zuerst auf Text oder deaktiviere den Block."
      : undefined;
  const legalTextWarning = getEnrollmentLegalTextWarning(
    setting,
    categoryItems,
  );
  const legalTextStoredStatus = getEnrollmentLegalStoredTextStatus(
    setting,
    categoryItems,
  );

  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const writeQueueRef = useRef<Promise<unknown>>(Promise.resolve());
  const pendingSaveRef = useRef<PendingSettingSave>({
    setting,
    categoryItems,
    localValue,
    isDirty,
  });

  // Keep the current field's complete save context together. On a setting-key
  // change React runs the previous passive-effect cleanup before installing
  // this render's effects, so cleanup still sees the old field and value.
  useEffect(() => {
    pendingSaveRef.current = {
      setting,
      categoryItems,
      localValue,
      isDirty,
    };
  }, [categoryItems, isDirty, localValue, setting]);

  // A schema revalidation may arrive while this field is being edited.
  // Observe each server value once; only pristine fields adopt it. Clearing
  // isDirty after a save must not restore an older server value.
  if (!Object.is(serverValue, setting.value)) {
    setServerValue(setting.value);
    if (!isDirty) setLocalValue(setting.value);
  }

  const cleanupPendingSave = useEffectEvent((pending: PendingSettingSave) => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    // Fire-and-forget save if user switches tab with unsaved changes.
    if (!pending.isDirty) return;
    const localErr = validateLocally(
      pending.setting,
      pending.localValue,
      pending.categoryItems,
    );
    const shortening = shortenedRetentionDays(
      pending.setting,
      pending.localValue,
      audience,
    );
    if (!localErr && shortening === null) {
      // The field is gone: nothing left to show an error at. The page
      // reloads the schema, so a lost value shows up as the old one.
      const run = writeQueueRef.current.then(() =>
        persistSetting(onSave, pending.setting.key, pending.localValue),
      );
      writeQueueRef.current = run.catch(() => undefined);
    }
  });

  // Save dirty values on unmount (e.g., tab change) + cleanup timers.
  useEffect(() => {
    const savedSettingKey = setting.key;
    return () => {
      const pending = pendingSaveRef.current;
      if (pending.setting.key === savedSettingKey) {
        cleanupPendingSave(pending);
      }
    };
  }, [setting.key]);

  /** Shows a failed field save on the shared error path. */
  const reportFieldFailure = useCallback(
    (outcome: SaveFailure, retry?: () => void) => {
      void showFieldError(outcome.error, { object: settingObject, retry });
    },
    [settingObject, showFieldError],
  );

  const doSave = useCallback(
    async (value: unknown): Promise<boolean> => {
      const localError = validateLocally(setting, value, categoryItems);
      if (localError) {
        setHint(localError);
        return false;
      }
      // Saves and resets of one field run in order, so the last action wins.
      const run = writeQueueRef.current.then(() =>
        persistSetting(onSave, setting.key, value),
      );
      writeQueueRef.current = run.catch(() => undefined);
      const outcome = await run;
      if (!outcome.ok) {
        reportFieldFailure(outcome, () => latestDoSaveRef.current(value));
        return false;
      }
      setHint(null);
      setIsDirty(false);
      toastSuccess(`Die Einstellung „${setting.label}“ ist gespeichert.`);
      return true;
    },
    [setting, categoryItems, onSave, reportFieldFailure, toastSuccess],
  );

  /** Saves, unless the save shortens a retention window: then it asks first. */
  const saveOrConfirm = useCallback(
    async (value: unknown): Promise<boolean> => {
      // A value outside the range gets its hint first; only a valid shorter
      // window is worth the question.
      const days = validateLocally(setting, value, categoryItems)
        ? null
        : shortenedRetentionDays(setting, value, audience);
      if (days === null) return doSave(value);
      setPendingShortening({ days, value, reset: false });
      return false;
    },
    [audience, categoryItems, doSave, setting],
  );

  /** A failed save inside an open dialog stays in that dialog. */
  const reportDialogFailure = useCallback(
    (
      errors: ReturnType<typeof useApiFormError>,
      outcome: SaveFailure,
      retry: () => void,
    ) => {
      void errors.show(outcome.error, { object: settingObject, retry });
    },
    [settingObject],
  );

  const handleOpenLegalTextEdit = useCallback(() => {
    setLegalTextEditDraft(toStr(localValue));
    setLegalDocumentDraftURL(legalDocumentURL);
    setLegalTextEditMode(legalAGBDisplayMode);
    textEditErrors.clear();
    setLegalTextEditOpen(true);
  }, [legalAGBDisplayMode, legalDocumentURL, localValue, textEditErrors]);

  const handleLegalTextEditConfirm = useCallback(async () => {
    const trimmedText = legalTextEditDraft.trim();
    const retry = () => latestTextEditRef.current();
    textEditErrors.clear();
    if (setting.key !== ENROLLMENT_LEGAL_AGB_TEXT_KEY) {
      if (!hasEnrollmentLegalTextContent(trimmedText)) {
        textEditErrors.invalid(REQUIRED_ENROLLMENT_LEGAL_TEXT_ERROR);
        return;
      }
      setLegalTextEditSaving(true);
      try {
        const outcome = await persistSetting(onSave, setting.key, trimmedText);
        if (!outcome.ok) {
          reportDialogFailure(textEditErrors, outcome, retry);
          return;
        }
        setLocalValue(trimmedText);
        setIsDirty(false);
        setHint(null);
        toastSuccess(`Die Einstellung „${setting.label}“ ist gespeichert.`);
        setLegalTextEditOpen(false);
      } finally {
        setLegalTextEditSaving(false);
      }
      return;
    }

    if (
      !hasEnrollmentLegalAGBSourceContent(
        legalTextEditMode,
        trimmedText,
        legalDocumentDraftURL,
      )
    ) {
      textEditErrors.invalid(selectedAGBSourceError(legalTextEditMode));
      return;
    }
    setLegalTextEditSaving(true);
    try {
      if (legalTextEditMode !== legalAGBDisplayMode) {
        const outcome = await persistSetting(
          onSave,
          ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_KEY,
          legalTextEditMode,
        );
        if (!outcome.ok) {
          reportDialogFailure(textEditErrors, outcome, retry);
          return;
        }
      }
      if (
        legalTextEditMode === ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_TEXT &&
        toStr(localValue) !== trimmedText
      ) {
        const outcome = await persistSetting(onSave, setting.key, trimmedText);
        if (!outcome.ok) {
          reportDialogFailure(textEditErrors, outcome, retry);
          return;
        }
        setLocalValue(trimmedText);
      }
      setIsDirty(false);
      setHint(null);
      toastSuccess(`Die Einstellung „${setting.label}“ ist gespeichert.`);
      setLegalTextEditOpen(false);
    } finally {
      setLegalTextEditSaving(false);
    }
  }, [
    legalAGBDisplayMode,
    legalDocumentDraftURL,
    legalTextEditDraft,
    legalTextEditMode,
    localValue,
    onSave,
    reportDialogFailure,
    setting.key,
    setting.label,
    textEditErrors,
    toastSuccess,
  ]);

  // Immediate save — for booleans and selects.
  // If the setting requires confirmation on enable OR disable (some
  // toggles trigger destructive backend side effects), show the modal
  // first and only persist the new value once the user confirms.
  const enableConfig = CONFIRM_ON_ENABLE[setting.key];
  const disableConfig = CONFIRM_ON_DISABLE[setting.key];
  const pendingValueRef = useRef<unknown>(null);
  const pendingScopeChangeRef = useRef<AttendanceScopeChange | null>(null);
  const activeConfirmConfig =
    pendingScopeChangeRef.current ??
    (enableConfig && pendingValueRef.current === true
      ? enableConfig
      : disableConfig && pendingValueRef.current === false
        ? disableConfig
        : null);

  const handleImmediateSave = useCallback(
    async (value: unknown) => {
      const prerequisite = attendanceScopePrerequisite(
        setting.key,
        value,
        categoryItems,
      );
      if (prerequisite) {
        if (!prerequisite.writable) {
          setHint(
            "Dafür muss zuerst die andere Einstellung geändert werden. Bitte wenden Sie sich an die OGS-Leitung.",
          );
          return;
        }
        pendingScopeChangeRef.current = prerequisite;
        pendingValueRef.current = value;
        setConfirmOpen(true);
        return;
      }
      if (legalActivationTextKey && value === true) {
        setLegalActivationText(toStr(legalActivationTextSetting?.value));
        setLegalActivationDocumentURL(legalDocumentURL);
        setLegalActivationMode(legalAGBDisplayMode);
        activationErrors.clear();
        setLegalActivationOpen(true);
        return;
      }
      if (deletesRoomHistory && value === false) {
        historyDeletionErrors.clear();
        setHistoryDeletion("save");
        return;
      }
      if (enableConfig && value === true) {
        pendingValueRef.current = value;
        setConfirmOpen(true);
        return;
      }
      if (disableConfig && value === false) {
        pendingValueRef.current = value;
        setConfirmOpen(true);
        return;
      }
      if (
        setting.key === "enrollment.bookings_authoritative" &&
        value === true &&
        onBookingAuthorityEnable
      ) {
        await onBookingAuthorityEnable();
        return;
      }
      await saveOrConfirm(value);
    },
    [
      activationErrors,
      saveOrConfirm,
      categoryItems,
      deletesRoomHistory,
      historyDeletionErrors,
      enableConfig,
      disableConfig,
      legalDocumentURL,
      legalAGBDisplayMode,
      legalActivationTextKey,
      legalActivationTextSetting?.value,
      onBookingAuthorityEnable,
      setting.key,
    ],
  );

  const handleLegalActivationConfirm = useCallback(async () => {
    if (!legalActivationTextKey) return;
    const trimmedText = legalActivationText.trim();
    const isAGBActivation =
      legalActivationTextKey === ENROLLMENT_LEGAL_AGB_TEXT_KEY;
    const retry = () => latestActivationRef.current();
    activationErrors.clear();
    if (
      isAGBActivation
        ? !hasEnrollmentLegalAGBSourceContent(
            legalActivationMode,
            trimmedText,
            legalActivationDocumentURL,
          )
        : !hasEnrollmentLegalTextContent(trimmedText)
    ) {
      activationErrors.invalid(
        isAGBActivation
          ? selectedAGBSourceError(legalActivationMode)
          : "Bitte tragen Sie zuerst einen Text ein.",
      );
      return;
    }
    setLegalActivationSaving(true);
    try {
      if (isAGBActivation && legalActivationMode !== legalAGBDisplayMode) {
        const outcome = await persistSetting(
          onSave,
          ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_KEY,
          legalActivationMode,
        );
        if (!outcome.ok) {
          reportDialogFailure(activationErrors, outcome, retry);
          return;
        }
      }
      if (
        !isAGBActivation ||
        legalActivationMode === ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_TEXT
      ) {
        const outcome = await persistSetting(
          onSave,
          legalActivationTextKey,
          trimmedText,
        );
        if (!outcome.ok) {
          reportDialogFailure(activationErrors, outcome, retry);
          return;
        }
      }
      const outcome = await persistSetting(onSave, setting.key, true);
      if (!outcome.ok) {
        reportDialogFailure(activationErrors, outcome, retry);
        return;
      }
      setIsDirty(false);
      setHint(null);
      toastSuccess(`Die Einstellung „${setting.label}“ ist gespeichert.`);
      setLegalActivationOpen(false);
    } finally {
      setLegalActivationSaving(false);
    }
  }, [
    activationErrors,
    legalAGBDisplayMode,
    legalActivationDocumentURL,
    legalActivationMode,
    legalActivationText,
    legalActivationTextKey,
    onSave,
    reportDialogFailure,
    setting.key,
    setting.label,
    toastSuccess,
  ]);

  // The PDF is managed from inside the open legal dialog; its errors and
  // checks stay in that dialog.
  const documentErrors = legalActivationOpen
    ? activationErrors
    : textEditErrors;

  const handleLegalDocumentUpload = useCallback(
    async (file: File | null) => {
      if (!file) return;
      documentErrors.clear();
      const fileProblem = legalDocumentFileProblem(file);
      if (fileProblem) {
        documentErrors.invalid(fileProblem);
        return;
      }
      setLegalDocumentSaving(true);
      try {
        const uploadedURL = await uploadEnrollmentLegalAGBDocument(file);
        setLegalDocumentDraftURL(uploadedURL);
        setLegalActivationDocumentURL(uploadedURL);
        toastSuccess("Die AGB-Datei ist gespeichert.");
        onSchemaRefresh?.();
      } catch (uploadError) {
        void documentErrors.show(uploadError, {
          object: "das Hochladen der AGB-Datei",
          retry: () => latestUploadRef.current(file),
        });
      } finally {
        setLegalDocumentSaving(false);
      }
    },
    [documentErrors, onSchemaRefresh, toastSuccess],
  );

  const handleLegalDocumentDelete = useCallback(async () => {
    documentErrors.clear();
    setLegalDocumentSaving(true);
    try {
      await deleteEnrollmentLegalAGBDocument();
      setLegalDocumentDraftURL("");
      setLegalActivationDocumentURL("");
      toastSuccess("Die AGB-Datei ist entfernt.");
      onSchemaRefresh?.();
    } catch (deleteError) {
      void documentErrors.show(deleteError, {
        object: "das Entfernen der AGB-Datei",
        retry: () => latestDeleteRef.current(),
      });
    } finally {
      setLegalDocumentSaving(false);
    }
  }, [documentErrors, onSchemaRefresh, toastSuccess]);

  const handleConfirm = useCallback(async () => {
    setConfirmOpen(false);
    const value = pendingValueRef.current;
    const prerequisite = pendingScopeChangeRef.current;
    pendingValueRef.current = null;
    pendingScopeChangeRef.current = null;
    if (value != null) {
      for (const change of prerequisite?.changes ?? []) {
        const outcome = await persistSetting(onSave, change.key, change.value);
        if (!outcome.ok) {
          // The dialog is closed already: the field shows the failure.
          reportFieldFailure(outcome);
          return;
        }
      }
      await doSave(value);
    }
  }, [doSave, onSave, reportFieldFailure]);

  // Local change — for text/number/time (debounce auto-save)
  const handleLocalChange = useCallback(
    (value: unknown) => {
      setLocalValue(value);
      setIsDirty(true);
      setHint(null);

      // Reset debounce timer
      if (debounceRef.current) clearTimeout(debounceRef.current);
      debounceRef.current = setTimeout(() => {
        void saveOrConfirm(value);
      }, AUTO_SAVE_DELAY_MS);
    },
    [saveOrConfirm],
  );

  // Save on blur — cancel debounce and save immediately
  const handleBlur = useCallback(async () => {
    if (debounceRef.current) {
      clearTimeout(debounceRef.current);
      debounceRef.current = null;
    }
    if (isDirty) {
      await saveOrConfirm(localValue);
    }
  }, [isDirty, localValue, saveOrConfirm]);

  // A picked preset replaces any typed draft: the pending draft save stops
  // and the field shows the preset at once, also when the server value does
  // not change (#6042).
  const handlePresetPick = useCallback(
    async (value: unknown) => {
      if (debounceRef.current) {
        clearTimeout(debounceRef.current);
        debounceRef.current = null;
      }
      setLocalValue(value);
      setIsDirty(true);
      setHint(null);
      await handleImmediateSave(value);
    },
    [handleImmediateSave],
  );

  const performReset = useCallback(async () => {
    if (debounceRef.current) {
      clearTimeout(debounceRef.current);
      debounceRef.current = null;
    }
    const run = writeQueueRef.current.then(async (): Promise<SaveOutcome> => {
      try {
        await onReset(setting.key);
        return { ok: true };
      } catch (error) {
        return { ok: false, error };
      }
    });
    writeQueueRef.current = run.catch(() => undefined);
    const outcome = await run;
    if (!outcome.ok) {
      reportFieldFailure(outcome, () => latestResetRef.current());
      return;
    }
    setHint(null);
    setIsDirty(false);
    toastSuccess(
      restoresTraegerValue
        ? `Für „${setting.label}“ gilt wieder der Wert des Trägers.`
        : `Die Einstellung „${setting.label}“ steht wieder auf dem Standard.`,
    );
  }, [
    setting.key,
    setting.label,
    onReset,
    reportFieldFailure,
    restoresTraegerValue,
    toastSuccess,
  ]);

  const handleHistoryDeletionConfirm = useCallback(async () => {
    if (!historyDeletion || historyDeletionSaving) return;
    historyDeletionErrors.clear();
    setHistoryDeletionSaving(true);
    try {
      if (historyDeletion === "reset") await onReset(setting.key);
      else await onSave(setting.key, false);
      setLocalValue(false);
      setIsDirty(false);
      setHint(null);
      setHistoryDeletion(null);
      toastSuccess(`Die Einstellung „${setting.label}“ ist ausgeschaltet.`);
    } catch (error) {
      void historyDeletionErrors.show(error, { object: settingObject });
    } finally {
      setHistoryDeletionSaving(false);
    }
  }, [
    historyDeletion,
    historyDeletionSaving,
    historyDeletionErrors,
    onReset,
    onSave,
    setting.key,
    setting.label,
    settingObject,
    toastSuccess,
  ]);

  /** Destructive defaults follow the same confirmation as an explicit save. */
  const handleReset = useCallback(async () => {
    if (deletesRoomHistory && setting.default === false) {
      historyDeletionErrors.clear();
      setHistoryDeletion("reset");
      return;
    }
    const days = shortenedRetentionDays(setting, setting.default, audience);
    if (days !== null) {
      if (debounceRef.current) {
        clearTimeout(debounceRef.current);
        debounceRef.current = null;
      }
      setPendingShortening({ days, value: setting.default, reset: true });
      return;
    }
    await performReset();
  }, [
    audience,
    deletesRoomHistory,
    historyDeletionErrors,
    performReset,
    setting,
  ]);

  const handleShorteningConfirm = useCallback(async () => {
    const pending = pendingShortening;
    setPendingShortening(null);
    if (!pending) return;
    if (pending.reset) {
      await performReset();
      return;
    }
    await doSave(pending.value);
  }, [doSave, pendingShortening, performReset]);

  const handleShorteningCancel = useCallback(() => {
    if (debounceRef.current) {
      clearTimeout(debounceRef.current);
      debounceRef.current = null;
    }
    setPendingShortening(null);
    setLocalValue(setting.value);
    setIsDirty(false);
  }, [setting.value]);

  // „Wiederholen“ sendet den Stand, der dann gilt.
  useLayoutEffect(() => {
    latestDoSaveRef.current = (value) => void doSave(value);
    latestResetRef.current = () => void performReset();
    latestActivationRef.current = () => void handleLegalActivationConfirm();
    latestTextEditRef.current = () => void handleLegalTextEditConfirm();
    latestUploadRef.current = (file) => void handleLegalDocumentUpload(file);
    latestDeleteRef.current = () => void handleLegalDocumentDelete();
  });

  useEffect(() => {
    if (!highlighted) return;
    setShowHighlight(true);
    const scrollTimeout = window.setTimeout(() => {
      fieldRef.current?.scrollIntoView({ block: "center", behavior: "smooth" });
    }, 100);
    const hideTimeout = window.setTimeout(() => {
      setShowHighlight(false);
    }, HIGHLIGHT_FLASH_MS);
    return () => {
      window.clearTimeout(scrollTimeout);
      window.clearTimeout(hideTimeout);
    };
  }, [highlighted]);

  if (!setting.visible) {
    return null;
  }

  const hasColorPresets =
    setting.type === "color" && colorPresets(setting).length > 0;
  const fieldLayout = hasColorPresets ? "" : "sm:flex-row sm:justify-between";
  const fieldAlignment =
    setting.type === "boolean" ? "sm:items-center" : "sm:items-start";

  return (
    // oxlint-disable-next-line ui-kit/no-hand-rolled-notice -- kurze grüne Hervorhebung der angesprungenen Einstellung, kein Hinweis
    <div
      ref={fieldRef}
      id={settingsFieldId(setting.key)}
      className={`flex scroll-mt-24 flex-col gap-3 rounded-xl border px-3 py-4 transition-[background-color,border-color,box-shadow] sm:gap-4 ${fieldLayout} ${fieldAlignment} ${
        showHighlight
          ? "ring-moto-green/35 bg-moto-green/10 shadow-sm ring-1 duration-500"
          : "border-transparent"
      }`}
    >
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <h4 className="text-sm font-medium text-gray-900">{setting.label}</h4>
          {showsStandardBadge(setting, audience) && (
            <span className="rounded bg-gray-100 px-1.5 py-0.5 text-xs text-gray-600">
              Standard
            </span>
          )}
          {!setting.writable && (
            <span className="bg-moto-amber-soft text-moto-amber-strong rounded px-1.5 py-0.5 text-xs">
              Nur Lesen
            </span>
          )}
        </div>
        {setting.description && (
          <p className="mt-0.5 text-sm text-gray-500">{setting.description}</p>
        )}
        {originNote && (
          <p className="mt-0.5 text-xs text-gray-500">{originNote}</p>
        )}
        {setting.access_policy === "shared" && audience === "operator" && (
          <p className="mt-0.5 text-xs text-gray-400 italic">
            Kann auch vom Schul-Admin geändert werden.
          </p>
        )}
        {isOperatorManaged(setting) && (
          <p className="mt-0.5 text-xs text-gray-500">
            {operatorManagedNote(setting.key, audience)}
          </p>
        )}
        {legalTextWarning && (
          <p className="text-moto-amber-strong mt-1 text-xs font-medium">
            {legalTextWarning}
          </p>
        )}
        {legalTextStoredStatus && (
          <p className="mt-1 text-xs text-gray-500">{legalTextStoredStatus}</p>
        )}
        {hint && (
          <p id={hintId} className="text-moto-red mt-1 text-xs">
            {hint}
          </p>
        )}
      </div>

      <div
        className={`flex min-w-0 flex-wrap items-center gap-3 ${
          hasColorPresets ? "w-full" : "shrink-0 sm:flex-nowrap"
        }`}
      >
        {isEnrollmentLegalTextSetting
          ? renderEnrollmentLegalTextEditor(
              localValue,
              setting.writable,
              handleOpenLegalTextEdit,
              setting.key,
              legalDocumentURL,
              legalAGBDisplayMode,
            )
          : renderField(
              setting,
              localValue,
              handleImmediateSave,
              handleLocalChange,
              handleBlur,
              revealFn,
              hint ? hintId : undefined,
              handlePresetPick,
            )}

        {offersReset(setting, audience, deletesRoomHistory) && (
          <Button
            type="button"
            variant="surface"
            size="compact"
            onClick={handleReset}
            title={
              restoresTraegerValue
                ? "Auf den Wert des Trägers zurücksetzen"
                : "Auf Standard zurücksetzen"
            }
          >
            <RotateCcw className="h-3.5 w-3.5" aria-hidden="true" />
            <span className="hidden sm:inline">Zurücksetzen</span>
          </Button>
        )}
      </div>

      <ConfirmDeleteModal
        isOpen={historyDeletion !== null}
        title="Anwesenheitsprotokoll ausschalten?"
        description={
          <div ref={historyDeletionRef}>
            <p>
              Alle abgeschlossenen Raumbesuche vor heute werden sofort gelöscht.
            </p>
            <p>
              Das lässt sich nicht rückgängig machen. Auch erneutes Einschalten
              stellt nichts wieder her.
            </p>
            <p>
              Heutige Raumbesuche bleiben bis zum nächtlichen Löschen erhalten.
              Der aktuelle Raum und die Anwesenheitszeiten bleiben erhalten.
            </p>
            {historyDeletion === "reset" && (
              <p>Der Standard ist ausgeschaltet.</p>
            )}
          </div>
        }
        gate={{
          mode: "textConfirm",
          expected: "LÖSCHEN",
          inputId: `history-deletion-${audience}`,
          label:
            "Geben Sie LÖSCHEN ein, um ältere Raumbesuche endgültig zu löschen.",
        }}
        onClose={() => {
          if (!historyDeletionSaving) setHistoryDeletion(null);
        }}
        onConfirm={handleHistoryDeletionConfirm}
        loading={historyDeletionSaving}
        error={historyDeletionErrors.error}
        confirmLabel="Ausschalten und löschen"
      />

      {activeConfirmConfig && (
        <ConfirmationModal
          isOpen={confirmOpen}
          onClose={() => {
            setConfirmOpen(false);
            pendingValueRef.current = null;
            pendingScopeChangeRef.current = null;
          }}
          onConfirm={handleConfirm}
          title={activeConfirmConfig.title}
          confirmText={
            pendingScopeChangeRef.current?.confirmText ??
            (pendingValueRef.current === false
              ? (disableConfig?.confirmText ?? "Deaktivieren")
              : (enableConfig?.confirmText ?? "Aktivieren"))
          }
          cancelText="Abbrechen"
          confirmVariant={
            pendingValueRef.current === false ? "danger" : "primary"
          }
        >
          <p className="text-sm text-gray-600">{activeConfirmConfig.body}</p>
        </ConfirmationModal>
      )}

      <ConfirmationModal
        isOpen={pendingShortening !== null}
        onClose={handleShorteningCancel}
        onConfirm={() => {
          void handleShorteningConfirm();
        }}
        title="Löschfrist verkürzen?"
        confirmText="Frist verkürzen"
        cancelText="Abbrechen"
        confirmVariant="danger"
      >
        <p className="text-sm text-gray-600">
          {pendingShortening
            ? retentionShorteningText(setting, pendingShortening.days)
            : null}
        </p>
      </ConfirmationModal>

      <ConfirmationModal
        isOpen={legalActivationOpen}
        onClose={() => {
          if (legalActivationSaving) return;
          setLegalActivationOpen(false);
          activationErrors.clear();
        }}
        onConfirm={() => {
          void handleLegalActivationConfirm();
        }}
        title={legalActivationTitle(setting.label)}
        confirmText="Aktivieren"
        cancelText="Abbrechen"
        isConfirmLoading={legalActivationSaving}
        isConfirmDisabled={
          legalActivationTextKey === ENROLLMENT_LEGAL_AGB_TEXT_KEY
            ? !hasEnrollmentLegalAGBSourceContent(
                legalActivationMode,
                legalActivationText,
                legalActivationDocumentURL,
              )
            : !hasEnrollmentLegalTextContent(legalActivationText)
        }
      >
        <div ref={activationRef} className="space-y-3">
          <FormErrorAlert message={activationErrors.error} />
          <p className="text-sm text-gray-600">
            {legalActivationTextKey === ENROLLMENT_LEGAL_AGB_TEXT_KEY
              ? "Dieser Block erscheint im Anmeldeformular, sobald er aktiviert ist. Wähle aus, ob Eltern den AGB-Text direkt lesen oder eine PDF-Datei öffnen sollen."
              : "Dieser Block erscheint im Anmeldeformular, sobald er aktiviert ist und ein Text hinterlegt wurde."}
          </p>
          {legalActivationTextKey === ENROLLMENT_LEGAL_AGB_TEXT_KEY ? (
            renderAGBSourceEditor({
              mode: legalActivationMode,
              onModeChange: (mode) => {
                setLegalActivationMode(mode);
                activationErrors.clear();
              },
              textValue: legalActivationText,
              onTextChange: (value) => {
                setLegalActivationText(value);
                activationErrors.clear();
              },
              documentURL: legalActivationDocumentURL,
              documentSaving: legalDocumentSaving,
              writable: setting.writable,
              documentManagementDisabledReason:
                legalDocumentManagementDisabledReason,
              documentDeleteDisabledReason: legalDocumentDeleteDisabledReason,
              onDocumentUpload: handleLegalDocumentUpload,
              onDocumentDelete: handleLegalDocumentDelete,
              textOnly: audience === "traeger",
            })
          ) : (
            <Textarea
              label="Rechtstext"
              value={legalActivationText}
              onChange={(event) => {
                setLegalActivationText(event.target.value);
                activationErrors.clear();
              }}
              rows={8}
              className="resize-y py-2.5 font-mono leading-6"
            />
          )}
        </div>
      </ConfirmationModal>

      <ConfirmationModal
        isOpen={legalTextEditOpen}
        onClose={() => {
          if (legalTextEditSaving) return;
          setLegalTextEditOpen(false);
          textEditErrors.clear();
        }}
        onConfirm={() => {
          void handleLegalTextEditConfirm();
        }}
        title={
          setting.key === ENROLLMENT_LEGAL_AGB_TEXT_KEY
            ? "AGB / Teilnahmebedingungen überarbeiten"
            : `${setting.label} bearbeiten`
        }
        confirmText="Speichern"
        cancelText="Abbrechen"
        isConfirmLoading={legalTextEditSaving}
        isConfirmDisabled={
          setting.key === ENROLLMENT_LEGAL_AGB_TEXT_KEY
            ? !hasEnrollmentLegalAGBSourceContent(
                legalTextEditMode,
                legalTextEditDraft,
                legalDocumentDraftURL,
              )
            : !hasEnrollmentLegalTextContent(legalTextEditDraft)
        }
      >
        <div ref={textEditRef} className="space-y-3">
          <FormErrorAlert message={textEditErrors.error} />
          {setting.key === ENROLLMENT_LEGAL_AGB_TEXT_KEY ? (
            renderAGBSourceEditor({
              mode: legalTextEditMode,
              onModeChange: (mode) => {
                setLegalTextEditMode(mode);
                textEditErrors.clear();
              },
              textValue: legalTextEditDraft,
              onTextChange: (value) => {
                setLegalTextEditDraft(value);
                textEditErrors.clear();
              },
              documentURL: legalDocumentDraftURL,
              documentSaving: legalDocumentSaving,
              writable: setting.writable,
              documentManagementDisabledReason:
                legalDocumentManagementDisabledReason,
              documentDeleteDisabledReason: legalDocumentDeleteDisabledReason,
              onDocumentUpload: handleLegalDocumentUpload,
              onDocumentDelete: handleLegalDocumentDelete,
              textOnly: audience === "traeger",
            })
          ) : (
            <Textarea
              label="Rechtstext"
              value={legalTextEditDraft}
              onChange={(event) => {
                setLegalTextEditDraft(event.target.value);
                textEditErrors.clear();
              }}
              rows={10}
              className="resize-y py-2.5 font-mono leading-6"
            />
          )}
          {/* Was noch fehlt, steht dauerhaft da: Speichern ist bis dahin
              gesperrt. */}
          {(setting.key === ENROLLMENT_LEGAL_AGB_TEXT_KEY
            ? !hasEnrollmentLegalAGBSourceContent(
                legalTextEditMode,
                legalTextEditDraft,
                legalDocumentDraftURL,
              )
            : !hasEnrollmentLegalTextContent(legalTextEditDraft)) && (
            <p className="text-moto-red text-xs font-medium">
              {setting.key === ENROLLMENT_LEGAL_AGB_TEXT_KEY
                ? selectedAGBSourceError(legalTextEditMode)
                : REQUIRED_ENROLLMENT_LEGAL_TEXT_ERROR}
            </p>
          )}
        </div>
      </ConfirmationModal>
    </div>
  );
}

/**
 * Whether the field offers „Zurücksetzen“. A school's toggle and legal text
 * normally have none: a toggle switches back, a legal text is edited. Back to
 * the Träger value is offered for every type (see resetRestoresTraegerValue),
 * and so is removing the Träger value in the Träger portal.
 */
function offersReset(
  setting: ResolvedSetting,
  audience: SettingsAudience,
  deletesRoomHistory: boolean,
): boolean {
  if (!setting.writable) return false;
  if (followsTraegerAfterReset(setting, audience)) return true;
  if (setting.is_default) return false;
  if (audience === "traeger") return true;
  return (
    (setting.type !== "boolean" || deletesRoomHistory) &&
    !isEnrollmentLegalTextKey(setting.key)
  );
}

function settingsFieldId(key: string) {
  return `setting-${key.replaceAll(".", "-")}`;
}

function legalActivationTitle(label: string) {
  return `${label
    .replace(" im Anmeldeformular anzeigen", "")
    .replace(" anzeigen", "")} aktivieren`;
}

function legalDocumentURLFromItems(categoryItems: ResolvedSetting[]): string {
  return toStr(
    categoryItems.find(
      (item) => item.key === ENROLLMENT_LEGAL_AGB_DOCUMENT_URL_KEY,
    )?.value,
  ).trim();
}

function legalAGBDisplayModeFromItems(
  categoryItems: ResolvedSetting[],
): LegalAGBDisplayMode {
  const value = toStr(
    categoryItems.find(
      (item) => item.key === ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_KEY,
    )?.value,
  ).trim();
  return value === ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_PDF
    ? ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_PDF
    : ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_TEXT;
}

function legalTermsEnabledFromItems(
  setting: ResolvedSetting,
  categoryItems: ResolvedSetting[],
): boolean {
  if (setting.key === "enrollment.legal_terms_enabled") {
    return setting.value === true;
  }
  return (
    categoryItems.find((item) => item.key === "enrollment.legal_terms_enabled")
      ?.value === true
  );
}

function hasEnrollmentLegalContent(
  textKey: string,
  textValue: unknown,
  categoryItems: ResolvedSetting[],
): boolean {
  if (textKey !== ENROLLMENT_LEGAL_AGB_TEXT_KEY) {
    return hasEnrollmentLegalTextContent(textValue);
  }
  return hasEnrollmentLegalAGBSourceContent(
    legalAGBDisplayModeFromItems(categoryItems),
    textValue,
    legalDocumentURLFromItems(categoryItems),
  );
}

function hasEnrollmentLegalTextContent(textValue: unknown): boolean {
  return toStr(textValue).trim() !== "";
}

function hasEnrollmentLegalAGBSourceContent(
  mode: LegalAGBDisplayMode,
  textValue: unknown,
  documentURL: string,
): boolean {
  if (mode === ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_PDF) {
    return documentURL.trim() !== "";
  }
  return hasEnrollmentLegalTextContent(textValue);
}

function selectedAGBSourceError(mode: LegalAGBDisplayMode): string {
  return mode === ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_PDF
    ? "Bitte laden Sie zuerst eine PDF-Datei hoch."
    : "Bitte tragen Sie zuerst einen AGB-Text ein.";
}

function publicLegalDocumentURL(storedURL: string): string {
  const prefix = "/uploads/enrollment-legal-documents/";
  if (storedURL.startsWith(prefix)) {
    return `/api/public/enrollment-legal-documents/${storedURL.slice(prefix.length)}`;
  }
  return storedURL;
}

interface AGBSourceEditorProps {
  readonly mode: LegalAGBDisplayMode;
  readonly onModeChange: (mode: LegalAGBDisplayMode) => void;
  readonly textValue: string;
  readonly onTextChange: (value: string) => void;
  readonly documentURL: string;
  readonly documentSaving: boolean;
  readonly writable: boolean;
  readonly documentManagementDisabledReason?: string;
  readonly documentDeleteDisabledReason?: string;
  readonly onDocumentUpload: (file: File | null) => void;
  readonly onDocumentDelete: () => void;
  /**
   * Only the text source. The Träger portal sets the AGB text for all its
   * schools; the PDF and the choice between both stay with each school.
   */
  readonly textOnly?: boolean;
}

function renderAGBSourceEditor({
  mode,
  onModeChange,
  textValue,
  onTextChange,
  documentURL,
  documentSaving,
  writable,
  documentManagementDisabledReason,
  documentDeleteDisabledReason,
  onDocumentUpload,
  onDocumentDelete,
  textOnly = false,
}: AGBSourceEditorProps) {
  const hasText = textValue.trim() !== "";
  const hasDocument = documentURL.trim() !== "";
  const canManageDocument =
    writable && documentManagementDisabledReason == null;
  if (textOnly) {
    return (
      <Textarea
        id="traeger-agb-text"
        label="AGB-Text"
        value={textValue}
        onChange={(event) => onTextChange(event.target.value)}
        rows={10}
        className="resize-y font-mono leading-6"
      />
    );
  }
  return (
    <div className="space-y-4">
      <div className="grid gap-3 sm:grid-cols-2">
        <ChoiceTile
          as="button"
          tone="blue"
          selected={mode === ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_TEXT}
          onClick={() => onModeChange(ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_TEXT)}
          className="block p-4"
        >
          <span className="flex items-center gap-2 text-sm font-semibold">
            <FileText className="h-4 w-4" aria-hidden="true" />
            Text eingeben
          </span>
          <span className="mt-1 block text-xs text-gray-500">
            Eltern lesen den Text direkt im Formular.
          </span>
          {hasText && (
            <span className="mt-2 block">
              <StatusBadge label="Text gespeichert" tone="outline" compact />
            </span>
          )}
        </ChoiceTile>
        <ChoiceTile
          as="button"
          tone="blue"
          selected={mode === ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_PDF}
          onClick={() => onModeChange(ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_PDF)}
          className="block p-4"
        >
          <span className="flex items-center gap-2 text-sm font-semibold">
            <FileUp className="h-4 w-4" aria-hidden="true" />
            PDF-Datei hochladen
          </span>
          <span className="mt-1 block text-xs text-gray-500">
            Eltern öffnen im Formular einen PDF-Link.
          </span>
          {hasDocument && (
            <span className="mt-2 block">
              <StatusBadge label="PDF gespeichert" tone="outline" compact />
            </span>
          )}
        </ChoiceTile>
      </div>

      {mode === ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_TEXT ? (
        <div className="space-y-2">
          <Textarea
            label="AGB-Text"
            value={textValue}
            onChange={(event) => onTextChange(event.target.value)}
            rows={10}
            className="resize-y py-2.5 font-mono leading-6"
          />
          {hasDocument && (
            <p className="text-xs text-gray-500">
              Eine PDF ist gespeichert, wird aber aktuell nicht angezeigt.
            </p>
          )}
        </div>
      ) : (
        <InsetPanel framed>
          <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <p className="text-sm font-medium text-gray-900">PDF-Datei</p>
              <p className="mt-0.5 text-xs text-gray-600">
                {hasDocument
                  ? "Diese PDF wird im Anmeldeformular als Link angezeigt."
                  : "Lade die AGB / Teilnahmebedingungen als PDF hoch."}
              </p>
            </div>
            <div className="flex flex-wrap items-center gap-2 text-xs">
              {documentManagementDisabledReason == null && (
                <label
                  className={buttonClassName({
                    variant: "surface",
                    size: "compact",
                    className:
                      !canManageDocument || documentSaving
                        ? "pointer-events-none cursor-pointer opacity-50"
                        : "cursor-pointer",
                  })}
                >
                  <FileUp className="h-3.5 w-3.5" aria-hidden="true" />
                  <span>{hasDocument ? "PDF ersetzen" : "PDF hochladen"}</span>
                  <input
                    type="file"
                    accept="application/pdf,.pdf"
                    className="sr-only"
                    disabled={!canManageDocument || documentSaving}
                    onChange={(event) => {
                      onDocumentUpload(event.currentTarget.files?.[0] ?? null);
                      event.currentTarget.value = "";
                    }}
                  />
                </label>
              )}
              {hasDocument && (
                <a
                  href={publicLegalDocumentURL(documentURL)}
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
              )}
              {hasDocument &&
                canManageDocument &&
                documentDeleteDisabledReason == null && (
                  <Button
                    type="button"
                    variant="outline_danger"
                    size="compact"
                    onClick={onDocumentDelete}
                    disabled={documentSaving}
                  >
                    <Trash2 className="h-3.5 w-3.5" aria-hidden="true" />
                    <span>Entfernen</span>
                  </Button>
                )}
            </div>
          </div>
          {documentManagementDisabledReason && (
            <p className="mt-2 text-xs font-medium text-gray-600">
              {documentManagementDisabledReason}
            </p>
          )}
          {hasDocument && documentDeleteDisabledReason && (
            <p className="mt-2 text-xs font-medium text-gray-600">
              {documentDeleteDisabledReason}
            </p>
          )}
          {hasText && (
            <p className="mt-2 text-xs text-gray-500">
              Ein Text ist gespeichert, wird aber aktuell nicht angezeigt.
            </p>
          )}
        </InsetPanel>
      )}
    </div>
  );
}

function getEnrollmentLegalTextWarning(
  setting: ResolvedSetting,
  categoryItems: ResolvedSetting[],
): string | null {
  if (setting.value !== true) return null;
  const textKey = ENROLLMENT_LEGAL_TOGGLE_TO_TEXT_KEY[setting.key];
  if (!textKey) return null;
  const textSetting = categoryItems.find((item) => item.key === textKey);
  if (hasEnrollmentLegalContent(textKey, textSetting?.value, categoryItems)) {
    return null;
  }
  if (textKey !== ENROLLMENT_LEGAL_AGB_TEXT_KEY) {
    return "Wird erst im Anmeldeformular angezeigt, wenn der passende Text hinterlegt ist.";
  }
  return "Wird erst im Anmeldeformular angezeigt, wenn der passende Text oder eine PDF-Datei hinterlegt ist.";
}

function getEnrollmentLegalStoredTextStatus(
  setting: ResolvedSetting,
  categoryItems: ResolvedSetting[],
): string | null {
  if (setting.value !== false) return null;
  const textKey = ENROLLMENT_LEGAL_TOGGLE_TO_TEXT_KEY[setting.key];
  if (!textKey) return null;
  const textSetting = categoryItems.find((item) => item.key === textKey);
  if (!hasEnrollmentLegalContent(textKey, textSetting?.value, categoryItems)) {
    return null;
  }
  return textKey === ENROLLMENT_LEGAL_AGB_TEXT_KEY
    ? "Text oder Datei ist gespeichert, wird aber nicht angezeigt."
    : "Text ist gespeichert, wird aber nicht angezeigt.";
}

function renderEnrollmentLegalTextEditor(
  value: unknown,
  writable: boolean,
  onEdit: () => void,
  settingKey: string,
  documentURL: string,
  agbDisplayMode: LegalAGBDisplayMode,
) {
  const preview = toStr(value).trim();
  const isAGB = settingKey === ENROLLMENT_LEGAL_AGB_TEXT_KEY;
  const hasDocument = documentURL !== "";
  const hasContent = isAGB
    ? hasEnrollmentLegalAGBSourceContent(agbDisplayMode, preview, documentURL)
    : preview !== "";
  const status = legalContentStatusText(
    preview,
    hasDocument,
    isAGB,
    agbDisplayMode,
  );
  const isPDF =
    isAGB && agbDisplayMode === ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_PDF;
  const SourceIcon = isPDF ? FileText : TextAlignStart;
  let sourceLabel = hasContent ? "Text hinterlegt" : "Noch kein Text";
  if (isAGB) sourceLabel = isPDF ? "PDF-Datei" : "Text";
  return (
    <div className="moto-content-surface flex w-full flex-col gap-3 rounded-xl border p-3 sm:w-[28rem] sm:flex-row sm:items-center">
      <div className="flex min-w-0 flex-1 items-center gap-3">
        <IconTile
          size="xs"
          tone={hasContent ? "green" : "orange"}
          icon={<SourceIcon aria-hidden="true" />}
        />
        <div className="min-w-0">
          <p className="flex flex-wrap items-center gap-x-2 gap-y-1 text-sm font-medium text-gray-900">
            {isAGB && (
              <span className="-mr-1 font-normal text-gray-500">Quelle: </span>
            )}
            {sourceLabel}
            {!hasContent && (
              <StatusBadge tone="orange" size="xs" label="Fehlt" />
            )}
          </p>
          <p className="mt-0.5 text-xs text-gray-500">{status}</p>
        </div>
      </div>
      <Button
        type="button"
        variant={hasContent ? "outline" : "primary"}
        size="compact"
        onClick={onEdit}
        disabled={!writable}
        className="shrink-0"
      >
        <Pencil className="h-3.5 w-3.5" aria-hidden="true" />
        <span>{isAGB ? "AGB überarbeiten" : "Rechtstext bearbeiten"}</span>
      </Button>
    </div>
  );
}

function legalContentStatusText(
  text: string,
  hasDocument: boolean,
  isAGB: boolean,
  agbDisplayMode: LegalAGBDisplayMode,
): string {
  if (isAGB && agbDisplayMode === ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_PDF) {
    return hasDocument
      ? "Eltern öffnen die AGB als PDF-Datei."
      : "Noch keine PDF-Datei hochgeladen.";
  }
  if (text === "") return "Noch kein Text eingetragen.";
  return isAGB
    ? "Eltern lesen die AGB direkt im Formular."
    : "Eltern lesen den Text im Anmeldeformular.";
}

const LEGAL_DOCUMENT_MAX_BYTES = 10 * 1024 * 1024;

/** Check before sending: the backend takes a PDF of at most 10 MB. */
function legalDocumentFileProblem(file: File): string | null {
  if (file.type !== "application/pdf") {
    return "Bitte wählen Sie eine PDF-Datei.";
  }
  if (file.size > LEGAL_DOCUMENT_MAX_BYTES) {
    return "Die PDF-Datei ist zu groß. Bitte wählen Sie eine Datei bis 10 MB.";
  }
  return null;
}

async function uploadEnrollmentLegalAGBDocument(file: File): Promise<string> {
  const formData = new FormData();
  formData.append("document", file);

  const response = await transportFetch(
    "/api/settings/enrollment/legal-agb-document",
    { method: "POST", body: formData },
  );

  if (!response.ok) {
    throw await apiErrorFromResponse(
      response,
      `AGB document upload failed (${response.status})`,
    );
  }

  const result = (await response.json()) as {
    data?: { document_url?: string };
  };
  const documentURL = result.data?.document_url;
  if (!documentURL) {
    throw new ApiError("AGB document upload returned no URL", 502);
  }
  return documentURL;
}

async function deleteEnrollmentLegalAGBDocument(): Promise<void> {
  const response = await transportFetch(
    "/api/settings/enrollment/legal-agb-document",
    { method: "DELETE" },
  );

  if (!response.ok && response.status !== 204) {
    throw await apiErrorFromResponse(
      response,
      `AGB document delete failed (${response.status})`,
    );
  }
}

type SaveOutcome = { readonly ok: true } | SaveFailure;

/** A failed save, shown on the shared API error path. */
interface SaveFailure {
  readonly ok: false;
  readonly error: unknown;
}

async function persistSetting(
  onSave: (key: string, value: unknown) => Promise<void>,
  key: string,
  value: unknown,
): Promise<SaveOutcome> {
  try {
    await onSave(key, value);
    return { ok: true };
  } catch (error) {
    return { ok: false, error };
  }
}

/** The preset colours a colour setting offers (registry `options.static`). */
/** Presets of a text setting (#6042); they never restrict the value. */
function textPresets(setting: ResolvedSetting): ColorPreset[] {
  return colorPresets(setting);
}

const OWN_TEXT_VALUE = "__own__";

/**
 * Text setting with presets (#6042): the presets and „Eigener Name“ are one
 * value choice; the text field opens for the own name. A stored value
 * outside the presets shows as the own name. `ownFor` keeps the field open
 * for the value typed there, even when it equals a preset.
 */
function PresetTextField({
  ariaLabel,
  value,
  onChange,
  onPick,
  onBlur,
  disabled,
  presets,
  errorId,
}: {
  readonly ariaLabel: string;
  readonly value: string;
  readonly onChange: (value: string) => void;
  readonly onPick: (value: string) => void;
  readonly onBlur: () => Promise<void>;
  readonly disabled: boolean;
  readonly presets: readonly ColorPreset[];
  readonly errorId?: string;
}) {
  const [ownFor, setOwnFor] = useState<string | null>(null);
  const own =
    !presets.some((preset) => preset.value === value) || ownFor === value;
  return (
    <div className="w-full min-w-0 space-y-3">
      <SegmentedControl
        ariaLabel={ariaLabel}
        className="max-sm:w-full max-sm:grid-flow-row"
        value={own ? OWN_TEXT_VALUE : value}
        items={[
          ...presets.map((preset) => ({
            value: preset.value,
            label: preset.label,
            disabled,
          })),
          { value: OWN_TEXT_VALUE, label: "Eigener Name", disabled },
        ]}
        onChange={(next) => {
          if (next === OWN_TEXT_VALUE) {
            setOwnFor(value);
            return;
          }
          setOwnFor(null);
          onPick(next);
        }}
      />
      {own && (
        <TextField
          ariaLabel={`${ariaLabel}: Eigener Name`}
          value={value}
          onChange={(next) => {
            setOwnFor(next);
            onChange(next);
          }}
          onBlur={() => void onBlur()}
          disabled={disabled}
          errorId={errorId}
        />
      )}
    </div>
  );
}

function colorPresets(setting: ResolvedSetting): ColorPreset[] {
  return (setting.options?.static ?? []).flatMap((option) =>
    typeof option.value === "string"
      ? [{ label: option.label, value: option.value }]
      : [],
  );
}

/**
 * A field with a registry example is as tall as the example plus room to
 * type (#5693): a three-line sender needs no ten-line box.
 */
function exampleRows(placeholder: string | undefined): number | undefined {
  if (!placeholder) return undefined;
  return Math.max(4, placeholder.split("\n").length + 2);
}

function renderField(
  setting: ResolvedSetting,
  localValue: unknown,
  onImmediateSave: (value: unknown) => Promise<void>,
  onLocalChange: (value: unknown) => void,
  onBlur: () => Promise<void>,
  revealFn?: (key: string) => Promise<string | null>,
  errorId?: string,
  onPresetPick?: (value: unknown) => Promise<void>,
) {
  const disabled = !setting.writable;

  switch (setting.type) {
    case "boolean":
      return (
        <Switch
          ariaLabel={setting.label}
          checked={Boolean(localValue)}
          onCheckedChange={onImmediateSave}
          disabled={disabled}
        />
      );
    case "number":
      return (
        <NumberField
          ariaLabel={setting.label}
          value={Number(localValue ?? 0)}
          onChange={onLocalChange}
          onBlur={onBlur}
          disabled={disabled}
          min={setting.validation?.min}
          max={setting.validation?.max}
        />
      );
    case "time":
      return (
        <SettingsTimeField
          ariaLabel={setting.label}
          value={toStr(localValue)}
          onChange={onLocalChange}
          onBlur={onBlur}
          disabled={disabled}
          emptyLabel={emptyTimeLabel(setting)}
        />
      );
    case "date":
      return (
        <DateField
          ariaLabel={setting.label}
          value={toStr(localValue)}
          // A picked calendar day is a finished value, so it saves right away
          // like boolean and select do. The debounce-plus-blur pair only exists
          // for fields you type into character by character.
          onChange={(next) => void onImmediateSave(next)}
          disabled={disabled}
          emptyLabel={setting.default === "" ? "Kein Datum" : undefined}
        />
      );
    case "text": {
      const presets = textPresets(setting);
      if (presets.length > 0) {
        return (
          <PresetTextField
            ariaLabel={setting.label}
            value={toStr(localValue)}
            onChange={onLocalChange}
            onPick={(next) => void (onPresetPick ?? onImmediateSave)(next)}
            onBlur={onBlur}
            disabled={disabled}
            presets={presets}
            errorId={errorId}
          />
        );
      }
      return (
        <TextField
          ariaLabel={setting.label}
          value={toStr(localValue)}
          onChange={onLocalChange}
          onBlur={onBlur}
          disabled={disabled}
          errorId={errorId}
        />
      );
    }
    case "color":
      return (
        <ColorField
          ariaLabel={setting.label}
          value={toStr(localValue)}
          fallback={toStr(setting.default)}
          onChange={onLocalChange}
          onBlur={onBlur}
          disabled={disabled}
          presets={colorPresets(setting)}
        />
      );
    case "textarea":
      return (
        <TextareaField
          ariaLabel={setting.label}
          value={toStr(localValue)}
          onChange={onLocalChange}
          onBlur={onBlur}
          disabled={disabled}
          // The payment text shows the sentence letters print without an
          // own text (#5622), not an example; other fields show the registry
          // example (#5693).
          placeholder={
            setting.key === PAYMENT_TEXT_KEY
              ? BUILT_IN_PAYMENT_TEXT
              : setting.placeholder
          }
          rows={exampleRows(setting.placeholder)}
          errorId={errorId}
        />
      );
    case "password":
      return (
        <PasswordField
          hasValue={localValue !== "" && localValue !== null}
          settingKey={setting.key}
          onChange={onImmediateSave}
          disabled={disabled}
          pattern={setting.validation?.pattern}
          revealFn={revealFn}
        />
      );
    case "select":
      return (
        <SelectField
          ariaLabel={setting.label}
          value={localValue}
          onChange={onImmediateSave}
          options={setting.options?.static ?? []}
          disabled={disabled}
        />
      );
    case "multiselect":
      return (
        <SelectField
          ariaLabel={setting.label}
          value={localValue}
          onChange={onImmediateSave}
          options={setting.options?.static ?? []}
          disabled={disabled}
          multiple
        />
      );
    default:
      return (
        <TextField
          ariaLabel={setting.label}
          value={toStr(localValue)}
          onChange={onLocalChange}
          onBlur={onBlur}
          disabled={disabled}
        />
      );
  }
}

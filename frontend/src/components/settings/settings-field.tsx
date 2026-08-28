"use client";

import {
  useState,
  useCallback,
  useEffect,
  useEffectEvent,
  useRef,
} from "react";
import {
  ExternalLink,
  FileText,
  FileUp,
  Pencil,
  RotateCcw,
  Trash2,
} from "lucide-react";
import type { ResolvedSetting } from "~/lib/settings-api";
import { useToast } from "~/contexts/ToastContext";
import { ConfirmationModal } from "~/components/ui/modal";
import { BooleanField } from "./fields/boolean-field";
import { NumberField } from "./fields/number-field";
import { TimeField } from "./fields/time-field";
import { DateField } from "./fields/date-field";
import { TextField } from "./fields/text-field";
import { TextareaField } from "./fields/textarea-field";
import { PasswordField } from "./fields/password-field";
import { SelectField } from "./fields/select-field";

/** Settings keys that require a confirmation dialog before enabling. */
const CONFIRM_ON_ENABLE: Record<string, { title: string; body: string }> = {
  "gdpr.attendance_log_enabled": {
    title: "Anwesenheitshistorie aktivieren",
    body: "Die Aktivierung erfordert die datenschutzrechtliche Einwilligung der Erziehungsberechtigten gem. § 120 Abs. 2 SchulG NRW i.V.m. Art. 6 Abs. 1 lit. a DSGVO.",
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

function toStr(v: unknown): string {
  if (v == null) return "";
  if (typeof v === "string") return v;
  if (typeof v === "number" || typeof v === "boolean") return v.toString();
  return JSON.stringify(v);
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

interface SettingsFieldProps {
  readonly setting: ResolvedSetting;
  readonly categoryItems?: ResolvedSetting[];
  readonly highlighted?: boolean;
  readonly onSave: (key: string, value: unknown) => Promise<string | null>;
  readonly onReset: (key: string) => Promise<string | null>;
  readonly onSchemaRefresh?: () => void;
  readonly onBookingAuthorityEnable?: () => Promise<void>;
  // audience controls the "auch von {other side} änderbar" hint shown
  // on shared settings. Defaults to "admin" (tenant settings page).
  readonly audience?: "admin" | "operator";
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
  const { success: toastSuccess, error: toastError } = useToast();
  const [localValue, setLocalValue] = useState<unknown>(setting.value);
  const [isDirty, setIsDirty] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [showHighlight, setShowHighlight] = useState(false);
  const [legalActivationOpen, setLegalActivationOpen] = useState(false);
  const [legalActivationText, setLegalActivationText] = useState("");
  const [legalActivationDocumentURL, setLegalActivationDocumentURL] =
    useState("");
  const [legalActivationMode, setLegalActivationMode] =
    useState<LegalAGBDisplayMode>(ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_TEXT);
  const [legalActivationSaving, setLegalActivationSaving] = useState(false);
  const [legalActivationError, setLegalActivationError] = useState<
    string | null
  >(null);
  const [legalTextEditOpen, setLegalTextEditOpen] = useState(false);
  const [legalTextEditDraft, setLegalTextEditDraft] = useState("");
  const [legalTextEditSaving, setLegalTextEditSaving] = useState(false);
  const [legalTextEditError, setLegalTextEditError] = useState<string | null>(
    null,
  );
  const [legalDocumentDraftURL, setLegalDocumentDraftURL] = useState("");
  const [legalTextEditMode, setLegalTextEditMode] =
    useState<LegalAGBDisplayMode>(ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_TEXT);
  const [legalDocumentSaving, setLegalDocumentSaving] = useState(false);
  const [legalDocumentError, setLegalDocumentError] = useState<string | null>(
    null,
  );
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
  const isResettingRef = useRef(false);
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

  // Sync local state when setting value changes from server.
  useEffect(() => {
    setLocalValue(setting.value);
    setIsDirty(false);
  }, [setting.value]);

  const cleanupPendingSave = useEffectEvent((pending: PendingSettingSave) => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    // Fire-and-forget save if user switches tab with unsaved changes.
    if (!pending.isDirty) return;
    const localErr = validateLocally(
      pending.setting,
      pending.localValue,
      pending.categoryItems,
    );
    if (!localErr) {
      void onSave(pending.setting.key, pending.localValue);
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

  const doSave = useCallback(
    async (value: unknown): Promise<string | null> => {
      const localError = validateLocally(setting, value, categoryItems);
      if (localError) {
        setError(localError);
        return localError;
      }
      const errorMsg = await onSave(setting.key, value);
      if (errorMsg) {
        setError(errorMsg);
        toastError(errorMsg);
        return errorMsg;
      } else {
        setError(null);
        setIsDirty(false);
        toastSuccess("Einstellung gespeichert");
        return null;
      }
    },
    [setting, categoryItems, onSave, toastSuccess, toastError],
  );

  const handleOpenLegalTextEdit = useCallback(() => {
    setLegalTextEditDraft(toStr(localValue));
    setLegalDocumentDraftURL(legalDocumentURL);
    setLegalTextEditMode(legalAGBDisplayMode);
    setLegalTextEditError(null);
    setLegalDocumentError(null);
    setLegalTextEditOpen(true);
  }, [legalAGBDisplayMode, legalDocumentURL, localValue]);

  const handleLegalTextEditConfirm = useCallback(async () => {
    const trimmedText = legalTextEditDraft.trim();
    if (setting.key !== ENROLLMENT_LEGAL_AGB_TEXT_KEY) {
      if (!hasEnrollmentLegalTextContent(trimmedText)) {
        setLegalTextEditError(REQUIRED_ENROLLMENT_LEGAL_TEXT_ERROR);
        return;
      }
      setLegalTextEditSaving(true);
      try {
        const saveError = await doSave(trimmedText);
        if (saveError) {
          setLegalTextEditError(saveError);
          return;
        }
        setLocalValue(trimmedText);
        setLegalTextEditOpen(false);
        setLegalTextEditError(null);
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
      setLegalTextEditError(selectedAGBSourceError(legalTextEditMode));
      return;
    }
    setLegalTextEditSaving(true);
    try {
      if (legalTextEditMode !== legalAGBDisplayMode) {
        const modeError = await onSave(
          ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_KEY,
          legalTextEditMode,
        );
        if (modeError) {
          setLegalTextEditError(modeError);
          toastError(modeError);
          return;
        }
      }
      if (
        legalTextEditMode === ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_TEXT &&
        toStr(localValue) !== trimmedText
      ) {
        const textError = await onSave(setting.key, trimmedText);
        if (textError) {
          setLegalTextEditError(textError);
          toastError(textError);
          return;
        }
        setLocalValue(trimmedText);
      }
      setIsDirty(false);
      setError(null);
      toastSuccess("Einstellung gespeichert");
      setLegalTextEditOpen(false);
      setLegalTextEditError(null);
    } finally {
      setLegalTextEditSaving(false);
    }
  }, [
    doSave,
    legalAGBDisplayMode,
    legalDocumentDraftURL,
    legalTextEditDraft,
    legalTextEditMode,
    localValue,
    onSave,
    setting.key,
    toastError,
    toastSuccess,
  ]);

  // Immediate save — for booleans and selects.
  // If the setting requires confirmation on enable OR disable (some
  // toggles trigger destructive backend side effects), show the modal
  // first and only persist the new value once the user confirms.
  const enableConfig = CONFIRM_ON_ENABLE[setting.key];
  const disableConfig = CONFIRM_ON_DISABLE[setting.key];
  const pendingValueRef = useRef<unknown>(null);
  const fieldRef = useRef<HTMLDivElement | null>(null);
  const activeConfirmConfig =
    enableConfig && pendingValueRef.current === true
      ? enableConfig
      : disableConfig && pendingValueRef.current === false
        ? disableConfig
        : null;

  const handleImmediateSave = useCallback(
    async (value: unknown) => {
      if (legalActivationTextKey && value === true) {
        setLegalActivationText(toStr(legalActivationTextSetting?.value));
        setLegalActivationDocumentURL(legalDocumentURL);
        setLegalActivationMode(legalAGBDisplayMode);
        setLegalActivationError(null);
        setLegalDocumentError(null);
        setLegalActivationOpen(true);
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
      await doSave(value);
    },
    [
      doSave,
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
    if (
      isAGBActivation
        ? !hasEnrollmentLegalAGBSourceContent(
            legalActivationMode,
            trimmedText,
            legalActivationDocumentURL,
          )
        : !hasEnrollmentLegalTextContent(trimmedText)
    ) {
      setLegalActivationError(
        isAGBActivation
          ? selectedAGBSourceError(legalActivationMode)
          : "Bitte tragen Sie zuerst einen Text ein.",
      );
      return;
    }
    setLegalActivationSaving(true);
    try {
      if (isAGBActivation && legalActivationMode !== legalAGBDisplayMode) {
        const modeError = await onSave(
          ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_KEY,
          legalActivationMode,
        );
        if (modeError) {
          setLegalActivationError(modeError);
          toastError(modeError);
          return;
        }
      }
      if (
        !isAGBActivation ||
        legalActivationMode === ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_TEXT
      ) {
        const textError = await onSave(legalActivationTextKey, trimmedText);
        if (textError) {
          setLegalActivationError(textError);
          toastError(textError);
          return;
        }
      }
      const toggleError = await doSave(true);
      if (toggleError) {
        setLegalActivationError(toggleError);
        return;
      }
      setLegalActivationOpen(false);
      setLegalActivationError(null);
    } finally {
      setLegalActivationSaving(false);
    }
  }, [
    doSave,
    legalAGBDisplayMode,
    legalActivationDocumentURL,
    legalActivationMode,
    legalActivationText,
    legalActivationTextKey,
    onSave,
    toastError,
  ]);

  const handleLegalDocumentUpload = useCallback(
    async (file: File | null) => {
      if (!file) return;
      setLegalDocumentSaving(true);
      setLegalDocumentError(null);
      try {
        const uploadedURL = await uploadEnrollmentLegalAGBDocument(file);
        setLegalDocumentDraftURL(uploadedURL);
        setLegalActivationDocumentURL(uploadedURL);
        toastSuccess("AGB-Datei gespeichert");
        onSchemaRefresh?.();
      } catch (uploadError) {
        const message =
          uploadError instanceof Error
            ? uploadError.message
            : "AGB-Datei konnte nicht hochgeladen werden.";
        setLegalDocumentError(message);
        toastError(message);
      } finally {
        setLegalDocumentSaving(false);
      }
    },
    [onSchemaRefresh, toastError, toastSuccess],
  );

  const handleLegalDocumentDelete = useCallback(async () => {
    setLegalDocumentSaving(true);
    setLegalDocumentError(null);
    try {
      await deleteEnrollmentLegalAGBDocument();
      setLegalDocumentDraftURL("");
      setLegalActivationDocumentURL("");
      toastSuccess("AGB-Datei entfernt");
      onSchemaRefresh?.();
    } catch (deleteError) {
      const message =
        deleteError instanceof Error
          ? deleteError.message
          : "AGB-Datei konnte nicht entfernt werden.";
      setLegalDocumentError(message);
      toastError(message);
    } finally {
      setLegalDocumentSaving(false);
    }
  }, [onSchemaRefresh, toastError, toastSuccess]);

  const handleConfirm = useCallback(async () => {
    setConfirmOpen(false);
    if (pendingValueRef.current != null) {
      await doSave(pendingValueRef.current);
      pendingValueRef.current = null;
    }
  }, [doSave]);

  // Local change — for text/number/time (debounce auto-save)
  const handleLocalChange = useCallback(
    (value: unknown) => {
      setLocalValue(value);
      setIsDirty(true);
      setError(null);

      // Reset debounce timer
      if (debounceRef.current) clearTimeout(debounceRef.current);
      debounceRef.current = setTimeout(() => {
        void doSave(value);
      }, AUTO_SAVE_DELAY_MS);
    },
    [doSave],
  );

  // Save on blur — cancel debounce and save immediately
  const handleBlur = useCallback(async () => {
    if (debounceRef.current) {
      clearTimeout(debounceRef.current);
      debounceRef.current = null;
    }
    if (isDirty) {
      await doSave(localValue);
    }
  }, [isDirty, localValue, doSave]);

  const handleReset = useCallback(async () => {
    if (isResettingRef.current) return;
    isResettingRef.current = true;
    try {
      if (debounceRef.current) {
        clearTimeout(debounceRef.current);
        debounceRef.current = null;
      }
      const errorMsg = await onReset(setting.key);
      if (errorMsg) {
        setError(errorMsg);
        toastError(errorMsg);
      } else {
        setError(null);
        setIsDirty(false);
        toastSuccess("Auf Standard zurückgesetzt");
      }
    } finally {
      isResettingRef.current = false;
    }
  }, [setting.key, onReset, toastSuccess, toastError]);

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

  const fieldAlignment =
    setting.type === "boolean" ? "sm:items-center" : "sm:items-start";

  return (
    <div
      ref={fieldRef}
      id={settingsFieldId(setting.key)}
      className={`flex scroll-mt-24 flex-col gap-3 rounded-xl border px-3 py-4 transition-[background-color,border-color,box-shadow] sm:flex-row sm:justify-between sm:gap-4 ${fieldAlignment} ${
        showHighlight
          ? "border-moto-green/35 bg-moto-green/10 shadow-sm duration-500"
          : "border-transparent"
      }`}
    >
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <h4 className="text-sm font-medium text-gray-900">{setting.label}</h4>
          {setting.is_default && (
            <span className="rounded bg-gray-100 px-1.5 py-0.5 text-xs text-gray-500">
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
        {setting.access_policy === "shared" && audience === "operator" && (
          <p className="mt-0.5 text-xs text-gray-400 italic">
            Kann auch vom Schul-Admin geändert werden.
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
        {error && <p className="text-moto-red mt-1 text-xs">{error}</p>}
        {legalDocumentError && (
          <p className="text-moto-red mt-1 text-xs font-medium">
            {legalDocumentError}
          </p>
        )}
      </div>

      <div className="flex shrink-0 items-center gap-3">
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
            )}

        {!setting.is_default &&
          setting.writable &&
          setting.type !== "boolean" &&
          !isEnrollmentLegalTextKey(setting.key) && (
            <button
              type="button"
              onClick={handleReset}
              className="moto-content-surface inline-flex items-center gap-1.5 rounded-lg border px-2.5 py-1.5 text-xs font-medium text-gray-600 shadow-sm transition-colors hover:bg-gray-50 hover:text-gray-900 active:bg-gray-100"
              title="Auf Standard zurücksetzen"
            >
              <RotateCcw className="h-3.5 w-3.5" aria-hidden="true" />
              <span className="hidden sm:inline">Zurücksetzen</span>
            </button>
          )}
      </div>

      {activeConfirmConfig && (
        <ConfirmationModal
          isOpen={confirmOpen}
          onClose={() => {
            setConfirmOpen(false);
            pendingValueRef.current = null;
          }}
          onConfirm={handleConfirm}
          title={activeConfirmConfig.title}
          confirmText={
            pendingValueRef.current === false
              ? (disableConfig?.confirmText ?? "Deaktivieren")
              : "Aktivieren"
          }
          cancelText="Abbrechen"
          confirmButtonClass={
            pendingValueRef.current === false
              ? "bg-moto-red hover:bg-moto-red-strong"
              : "bg-gray-900 hover:bg-gray-700"
          }
        >
          <p className="text-sm text-gray-600">{activeConfirmConfig.body}</p>
        </ConfirmationModal>
      )}

      <ConfirmationModal
        isOpen={legalActivationOpen}
        onClose={() => {
          if (legalActivationSaving) return;
          setLegalActivationOpen(false);
          setLegalActivationError(null);
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
        <div className="space-y-3">
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
                setLegalActivationError(null);
              },
              textValue: legalActivationText,
              onTextChange: (value) => {
                setLegalActivationText(value);
                setLegalActivationError(null);
              },
              documentURL: legalActivationDocumentURL,
              documentSaving: legalDocumentSaving,
              writable: setting.writable,
              documentManagementDisabledReason:
                legalDocumentManagementDisabledReason,
              documentDeleteDisabledReason: legalDocumentDeleteDisabledReason,
              onDocumentUpload: handleLegalDocumentUpload,
              onDocumentDelete: handleLegalDocumentDelete,
            })
          ) : (
            <label className="block">
              <span className="text-sm font-medium text-gray-800">
                Rechtstext
              </span>
              <textarea
                value={legalActivationText}
                onChange={(event) => {
                  setLegalActivationText(event.target.value);
                  setLegalActivationError(null);
                }}
                rows={8}
                className="mt-1 block w-full resize-y rounded-lg border-0 bg-white px-3 py-2.5 font-mono text-sm leading-6 text-gray-900 shadow-sm ring-1 ring-gray-200 ring-inset placeholder:text-gray-400 focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400"
              />
            </label>
          )}
          {legalActivationError && (
            <p className="text-moto-red text-xs font-medium">
              {legalActivationError}
            </p>
          )}
        </div>
      </ConfirmationModal>

      <ConfirmationModal
        isOpen={legalTextEditOpen}
        onClose={() => {
          if (legalTextEditSaving) return;
          setLegalTextEditOpen(false);
          setLegalTextEditError(null);
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
        <div className="space-y-3">
          {setting.key === ENROLLMENT_LEGAL_AGB_TEXT_KEY ? (
            renderAGBSourceEditor({
              mode: legalTextEditMode,
              onModeChange: (mode) => {
                setLegalTextEditMode(mode);
                setLegalTextEditError(null);
              },
              textValue: legalTextEditDraft,
              onTextChange: (value) => {
                setLegalTextEditDraft(value);
                setLegalTextEditError(null);
              },
              documentURL: legalDocumentDraftURL,
              documentSaving: legalDocumentSaving,
              writable: setting.writable,
              documentManagementDisabledReason:
                legalDocumentManagementDisabledReason,
              documentDeleteDisabledReason: legalDocumentDeleteDisabledReason,
              onDocumentUpload: handleLegalDocumentUpload,
              onDocumentDelete: handleLegalDocumentDelete,
            })
          ) : (
            <label className="block">
              <span className="text-sm font-medium text-gray-800">
                Rechtstext
              </span>
              <textarea
                value={legalTextEditDraft}
                onChange={(event) => {
                  setLegalTextEditDraft(event.target.value);
                  setLegalTextEditError(null);
                }}
                rows={10}
                className="mt-1 block w-full resize-y rounded-lg border-0 bg-white px-3 py-2.5 font-mono text-sm leading-6 text-gray-900 shadow-sm ring-1 ring-gray-200 ring-inset placeholder:text-gray-400 focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400"
              />
            </label>
          )}
          {((setting.key === ENROLLMENT_LEGAL_AGB_TEXT_KEY
            ? !hasEnrollmentLegalAGBSourceContent(
                legalTextEditMode,
                legalTextEditDraft,
                legalDocumentDraftURL,
              )
            : !hasEnrollmentLegalTextContent(legalTextEditDraft)) ||
            legalTextEditError) && (
            <p className="text-moto-red text-xs font-medium">
              {legalTextEditError ??
                (setting.key === ENROLLMENT_LEGAL_AGB_TEXT_KEY
                  ? selectedAGBSourceError(legalTextEditMode)
                  : REQUIRED_ENROLLMENT_LEGAL_TEXT_ERROR)}
            </p>
          )}
        </div>
      </ConfirmationModal>
    </div>
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
}: AGBSourceEditorProps) {
  const hasText = textValue.trim() !== "";
  const hasDocument = documentURL.trim() !== "";
  const canManageDocument =
    writable && documentManagementDisabledReason == null;
  return (
    <div className="space-y-4">
      <div className="grid gap-3 sm:grid-cols-2">
        <button
          type="button"
          onClick={() => onModeChange(ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_TEXT)}
          className={`rounded-xl border p-4 text-left transition-colors ${
            mode === ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_TEXT
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
          {hasText && (
            <span className="mt-2 inline-flex rounded-full bg-white px-2 py-0.5 text-xs font-medium text-gray-600 ring-1 ring-gray-200">
              Text gespeichert
            </span>
          )}
        </button>
        <button
          type="button"
          onClick={() => onModeChange(ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_PDF)}
          className={`rounded-xl border p-4 text-left transition-colors ${
            mode === ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_PDF
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
          {hasDocument && (
            <span className="mt-2 inline-flex rounded-full bg-white px-2 py-0.5 text-xs font-medium text-gray-600 ring-1 ring-gray-200">
              PDF gespeichert
            </span>
          )}
        </button>
      </div>

      {mode === ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_TEXT ? (
        <div className="space-y-2">
          <label className="block">
            <span className="text-sm font-medium text-gray-800">AGB-Text</span>
            <textarea
              value={textValue}
              onChange={(event) => onTextChange(event.target.value)}
              rows={10}
              className="mt-1 block w-full resize-y rounded-lg border-0 bg-white px-3 py-2.5 font-mono text-sm leading-6 text-gray-900 shadow-sm ring-1 ring-gray-200 ring-inset placeholder:text-gray-400 focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400"
            />
          </label>
          {hasDocument && (
            <p className="text-xs text-gray-500">
              Eine PDF ist gespeichert, wird aber aktuell nicht angezeigt.
            </p>
          )}
        </div>
      ) : (
        <div className="border-moto-blue/20 bg-moto-blue/5 rounded-xl border p-3">
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
                  className={`border-moto-blue/25 hover:bg-moto-blue/10 text-moto-blue-hover inline-flex cursor-pointer items-center gap-1.5 rounded-lg border bg-white px-2.5 py-1.5 font-medium shadow-sm transition-colors ${
                    !canManageDocument || documentSaving
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
                  className="inline-flex items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-2.5 py-1.5 font-medium text-gray-600 shadow-sm transition-colors hover:bg-gray-50 hover:text-gray-900"
                >
                  <ExternalLink className="h-3.5 w-3.5" aria-hidden="true" />
                  <span>Öffnen</span>
                </a>
              )}
              {hasDocument &&
                canManageDocument &&
                documentDeleteDisabledReason == null && (
                  <button
                    type="button"
                    onClick={onDocumentDelete}
                    disabled={documentSaving}
                    className="border-moto-red/20 text-moto-red-strong hover:bg-moto-red/5 inline-flex items-center gap-1.5 rounded-lg border bg-white px-2.5 py-1.5 font-medium shadow-sm transition-colors disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    <Trash2 className="h-3.5 w-3.5" aria-hidden="true" />
                    <span>Entfernen</span>
                  </button>
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
        </div>
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
  return (
    <div className="w-full sm:w-96">
      <div
        className={`rounded-xl border p-3 text-sm leading-6 ${
          hasContent
            ? "border-gray-100 bg-gray-50 text-gray-700"
            : "border-moto-amber/30 bg-moto-amber-soft text-moto-amber-strong"
        }`}
      >
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div className="min-w-0">
            <p className="font-medium text-gray-900">
              {hasContent ? "Inhalt hinterlegt" : "Noch kein Inhalt hinterlegt"}
            </p>
            <p className="mt-0.5 text-xs text-gray-500">{status}</p>
            {isAGB && (
              <p className="mt-1 inline-flex rounded-full bg-white px-2 py-0.5 text-xs font-medium text-gray-600 ring-1 ring-gray-200">
                Quelle:{" "}
                {agbDisplayMode === ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_PDF
                  ? "PDF-Datei"
                  : "Text"}
              </p>
            )}
          </div>
          <button
            type="button"
            onClick={onEdit}
            disabled={!writable}
            className="inline-flex shrink-0 items-center justify-center gap-1.5 rounded-lg bg-gray-900 px-3 py-2 text-xs font-medium text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
          >
            <Pencil className="h-3.5 w-3.5" aria-hidden="true" />
            <span>{isAGB ? "AGB überarbeiten" : "Rechtstext bearbeiten"}</span>
          </button>
        </div>
      </div>
    </div>
  );
}

function legalContentStatusText(
  text: string,
  hasDocument: boolean,
  isAGB: boolean,
  agbDisplayMode: LegalAGBDisplayMode,
): string {
  if (isAGB) {
    if (agbDisplayMode === ENROLLMENT_LEGAL_AGB_DISPLAY_MODE_PDF) {
      return hasDocument
        ? "Im Anmeldeformular wird ein Link zur PDF angezeigt."
        : "Lade eine PDF hoch, um diese Quelle zu verwenden.";
    }
    return text !== ""
      ? "Im Anmeldeformular wird der eingegebene Text angezeigt."
      : "Trage einen Text ein, um diese Quelle zu verwenden.";
  }
  return text !== ""
    ? "Der Text wird im Anmeldeformular angezeigt."
    : "Hinterlege den Text, bevor du den Block aktivierst.";
}

async function uploadEnrollmentLegalAGBDocument(file: File): Promise<string> {
  const formData = new FormData();
  formData.append("document", file);

  const response = await fetch("/api/settings/enrollment/legal-agb-document", {
    method: "POST",
    body: formData,
  });

  if (!response.ok) {
    throw new Error(await legalDocumentErrorMessage(response));
  }

  const result = (await response.json()) as {
    data?: { document_url?: string };
  };
  const documentURL = result.data?.document_url;
  if (!documentURL) {
    throw new Error("AGB-Datei konnte nicht hochgeladen werden.");
  }
  return documentURL;
}

async function deleteEnrollmentLegalAGBDocument(): Promise<void> {
  const response = await fetch("/api/settings/enrollment/legal-agb-document", {
    method: "DELETE",
  });

  if (!response.ok && response.status !== 204) {
    throw new Error(await legalDocumentErrorMessage(response));
  }
}

async function legalDocumentErrorMessage(response: Response): Promise<string> {
  if (response.status === 413) {
    return "Die PDF-Datei darf maximal 10 MB groß sein.";
  }
  if (response.status === 415) {
    return "Bitte eine PDF-Datei hochladen.";
  }
  if (response.status === 403) {
    return "Für AGB-Dateien ist die Berechtigung für rechtliche Einstellungen erforderlich.";
  }
  try {
    const body = (await response.json()) as { error?: string };
    if (body.error) return body.error;
  } catch {
    // Fall through to generic copy.
  }
  return "AGB-Datei konnte nicht gespeichert werden.";
}

function renderField(
  setting: ResolvedSetting,
  localValue: unknown,
  onImmediateSave: (value: unknown) => Promise<void>,
  onLocalChange: (value: unknown) => void,
  onBlur: () => Promise<void>,
  revealFn?: (key: string) => Promise<string | null>,
) {
  const disabled = !setting.writable;

  switch (setting.type) {
    case "boolean":
      return (
        <BooleanField
          value={Boolean(localValue)}
          onChange={onImmediateSave}
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
        <TimeField
          ariaLabel={setting.label}
          value={toStr(localValue)}
          onChange={onLocalChange}
          onBlur={onBlur}
          disabled={disabled}
          emptyLabel={setting.default === "" ? "Jederzeit" : undefined}
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
    case "text":
      return (
        <TextField
          ariaLabel={setting.label}
          value={toStr(localValue)}
          onChange={onLocalChange}
          onBlur={onBlur}
          disabled={disabled}
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

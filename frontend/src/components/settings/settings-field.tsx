"use client";

import { useState, useCallback, useEffect, useRef } from "react";
import { ExternalLink, FileUp, Pencil, Trash2 } from "lucide-react";
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
  // audience controls the "auch von {other side} änderbar" hint shown
  // on shared settings. Defaults to "admin" (tenant settings page).
  readonly audience?: "admin" | "operator";
  // revealFn overrides how PasswordField fetches the unmasked value.
  // Defaults to the tenant reveal endpoint; the operator page passes a
  // school-bound function so reveal hits the operator endpoint instead.
  readonly revealFn?: (key: string) => Promise<string | null>;
}

export function SettingsField({
  setting,
  categoryItems = EMPTY_CATEGORY_ITEMS,
  highlighted = false,
  onSave,
  onReset,
  onSchemaRefresh,
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
  const legalDocumentSetting =
    setting.key === ENROLLMENT_LEGAL_AGB_TEXT_KEY
      ? categoryItems.find(
          (item) => item.key === ENROLLMENT_LEGAL_AGB_DOCUMENT_URL_KEY,
        )
      : undefined;
  const legalDocumentURL = toStr(legalDocumentSetting?.value).trim();
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
  const isDirtyRef = useRef(false);
  const localValueRef = useRef<unknown>(setting.value);

  // Keep refs in sync with state
  isDirtyRef.current = isDirty;
  localValueRef.current = localValue;

  // Sync local state when setting value changes from server.
  useEffect(() => {
    setLocalValue(setting.value);
    setIsDirty(false);
  }, [setting.value]);

  // Save dirty values on unmount (e.g., tab change) + cleanup timers
  useEffect(() => {
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
      // Fire-and-forget save if user switches tab with unsaved changes
      if (isDirtyRef.current) {
        const localErr = validateLocally(
          setting,
          localValueRef.current,
          categoryItems,
        );
        if (!localErr) {
          void onSave(setting.key, localValueRef.current);
        }
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
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
    setLegalTextEditError(null);
    setLegalTextEditOpen(true);
  }, [localValue]);

  const handleLegalTextEditConfirm = useCallback(async () => {
    const trimmedText = legalTextEditDraft.trim();
    if (!hasEnrollmentLegalContent(setting.key, trimmedText, categoryItems)) {
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
  }, [categoryItems, doSave, legalTextEditDraft, setting.key]);

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
        setLegalActivationError(null);
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
      await doSave(value);
    },
    [
      doSave,
      enableConfig,
      disableConfig,
      legalActivationTextKey,
      legalActivationTextSetting?.value,
    ],
  );

  const handleLegalActivationConfirm = useCallback(async () => {
    if (!legalActivationTextKey) return;
    const trimmedText = legalActivationText.trim();
    if (
      !hasEnrollmentLegalContent(
        legalActivationTextKey,
        trimmedText,
        categoryItems,
      )
    ) {
      setLegalActivationError(
        "Bitte tragen Sie zuerst einen Text ein oder hinterlegen Sie eine PDF-Datei.",
      );
      return;
    }
    setLegalActivationSaving(true);
    try {
      if (trimmedText !== "") {
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
    categoryItems,
    doSave,
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
        await uploadEnrollmentLegalAGBDocument(file);
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
          ? "border-[#83CD2D]/35 bg-[#83CD2D]/10 shadow-sm duration-500"
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
            <span className="rounded bg-yellow-50 px-1.5 py-0.5 text-xs text-yellow-700">
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
          <p className="mt-1 text-xs font-medium text-amber-700">
            {legalTextWarning}
          </p>
        )}
        {legalTextStoredStatus && (
          <p className="mt-1 text-xs text-gray-500">{legalTextStoredStatus}</p>
        )}
        {error && <p className="mt-1 text-xs text-red-600">{error}</p>}
        {legalDocumentError && (
          <p className="mt-1 text-xs font-medium text-red-600">
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
              legalDocumentSaving,
              handleLegalDocumentUpload,
              handleLegalDocumentDelete,
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
              <svg
                className="h-3.5 w-3.5"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                />
              </svg>
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
              ? "bg-[#FF3130] hover:bg-[#CC2626]"
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
          !hasEnrollmentLegalContent(
            legalActivationTextKey ?? "",
            legalActivationText,
            categoryItems,
          )
        }
      >
        <div className="space-y-3">
          <p className="text-sm text-gray-600">
            Dieser Block erscheint im Anmeldeformular, sobald er aktiviert ist
            und ein Text oder eine PDF-Datei hinterlegt wurde.
          </p>
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
          {legalActivationError && (
            <p className="text-xs font-medium text-red-600">
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
        title={`${setting.label} bearbeiten`}
        confirmText="Speichern"
        cancelText="Abbrechen"
        isConfirmLoading={legalTextEditSaving}
        isConfirmDisabled={
          !hasEnrollmentLegalContent(
            setting.key,
            legalTextEditDraft,
            categoryItems,
          )
        }
      >
        <div className="space-y-3">
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
          {(!hasEnrollmentLegalContent(
            setting.key,
            legalTextEditDraft,
            categoryItems,
          ) ||
            legalTextEditError) && (
            <p className="text-xs font-medium text-red-600">
              {legalTextEditError ?? REQUIRED_ENROLLMENT_LEGAL_TEXT_ERROR}
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

function hasEnrollmentLegalContent(
  textKey: string,
  textValue: unknown,
  categoryItems: ResolvedSetting[],
): boolean {
  if (toStr(textValue).trim() !== "") return true;
  if (textKey !== ENROLLMENT_LEGAL_AGB_TEXT_KEY) return false;
  return legalDocumentURLFromItems(categoryItems) !== "";
}

function publicLegalDocumentURL(storedURL: string): string {
  const prefix = "/uploads/enrollment-legal-documents/";
  if (storedURL.startsWith(prefix)) {
    return `/api/public/enrollment-legal-documents/${storedURL.slice(prefix.length)}`;
  }
  return storedURL;
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
  documentSaving: boolean,
  onDocumentUpload: (file: File | null) => void,
  onDocumentDelete: () => void,
) {
  const preview = toStr(value).trim();
  const isAGB = settingKey === ENROLLMENT_LEGAL_AGB_TEXT_KEY;
  const hasDocument = documentURL !== "";
  const hasContent = preview !== "" || hasDocument;
  return (
    <div className="w-full space-y-2 sm:w-96">
      <div
        className={`relative min-h-24 overflow-hidden rounded-lg border text-sm leading-6 ${
          hasContent
            ? "border-gray-100 bg-gray-50 text-gray-700"
            : "border-amber-200 bg-amber-50 text-amber-800"
        }`}
      >
        <button
          type="button"
          onClick={onEdit}
          disabled={!writable}
          aria-label="Rechtstext bearbeiten"
          title="Bearbeiten"
          className="absolute top-2 right-2 inline-flex h-7 w-7 items-center justify-center rounded-md text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
        >
          <Pencil className="h-3.5 w-3.5" aria-hidden="true" />
        </button>
        {preview ? (
          <p className="max-h-40 overflow-y-auto py-2 pr-12 pl-3 whitespace-pre-wrap">
            {preview}
          </p>
        ) : hasDocument ? (
          <p className="py-2 pr-12 pl-3">
            PDF-Datei hinterlegt. Optional kann zusätzlich ein Text eingetragen
            werden.
          </p>
        ) : (
          <p className="py-2 pr-12 pl-3">Noch kein Text hinterlegt.</p>
        )}
      </div>
      {isAGB && (
        <div className="flex flex-wrap items-center gap-2 text-xs">
          <label
            className={`inline-flex cursor-pointer items-center gap-1.5 rounded-lg border border-[#5080D8]/25 bg-[#5080D8]/5 px-2.5 py-1.5 font-medium text-[#4070C8] transition-colors hover:bg-[#5080D8]/10 ${
              !writable || documentSaving
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
              disabled={!writable || documentSaving}
              onChange={(event) => {
                onDocumentUpload(event.currentTarget.files?.[0] ?? null);
                event.currentTarget.value = "";
              }}
            />
          </label>
          {hasDocument && (
            <a
              href={publicLegalDocumentURL(documentURL)}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-1.5 rounded-lg border border-gray-200 px-2.5 py-1.5 font-medium text-gray-600 transition-colors hover:bg-gray-50 hover:text-gray-900"
            >
              <ExternalLink className="h-3.5 w-3.5" aria-hidden="true" />
              <span>Öffnen</span>
            </a>
          )}
          {hasDocument && writable && (
            <button
              type="button"
              onClick={onDocumentDelete}
              disabled={documentSaving}
              className="inline-flex items-center gap-1.5 rounded-lg border border-[#FF3130]/20 px-2.5 py-1.5 font-medium text-[#CC2626] transition-colors hover:bg-[#FF3130]/5 disabled:cursor-not-allowed disabled:opacity-50"
            >
              <Trash2 className="h-3.5 w-3.5" aria-hidden="true" />
              <span>Entfernen</span>
            </button>
          )}
        </div>
      )}
    </div>
  );
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
          value={toStr(localValue)}
          onChange={onLocalChange}
          onBlur={onBlur}
          disabled={disabled}
          emptyLabel={setting.default === "" ? "Kein Datum" : undefined}
        />
      );
    case "text":
      return (
        <TextField
          value={toStr(localValue)}
          onChange={onLocalChange}
          onBlur={onBlur}
          disabled={disabled}
        />
      );
    case "textarea":
      return (
        <TextareaField
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
          value={localValue}
          onChange={onImmediateSave}
          options={setting.options?.static ?? []}
          disabled={disabled}
        />
      );
    default:
      return (
        <TextField
          value={toStr(localValue)}
          onChange={onLocalChange}
          onBlur={onBlur}
          disabled={disabled}
        />
      );
  }
}

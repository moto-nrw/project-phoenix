"use client";

import { useState, useCallback, useEffect, useRef } from "react";
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

function validateLocally(
  setting: ResolvedSetting,
  value: unknown,
): string | null {
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

interface SettingsFieldProps {
  readonly setting: ResolvedSetting;
  readonly highlighted?: boolean;
  readonly onSave: (key: string, value: unknown) => Promise<string | null>;
  readonly onReset: (key: string) => Promise<string | null>;
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
  highlighted = false,
  onSave,
  onReset,
  audience = "admin",
  revealFn,
}: SettingsFieldProps) {
  const { success: toastSuccess, error: toastError } = useToast();
  const [localValue, setLocalValue] = useState<unknown>(setting.value);
  const [isDirty, setIsDirty] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [showHighlight, setShowHighlight] = useState(false);

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
        const localErr = validateLocally(setting, localValueRef.current);
        if (!localErr) {
          void onSave(setting.key, localValueRef.current);
        }
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [setting.key]);

  const doSave = useCallback(
    async (value: unknown) => {
      const localError = validateLocally(setting, value);
      if (localError) {
        setError(localError);
        toastError(localError);
        return;
      }
      const errorMsg = await onSave(setting.key, value);
      if (errorMsg) {
        setError(errorMsg);
        toastError(errorMsg);
      } else {
        setError(null);
        setIsDirty(false);
        toastSuccess("Einstellung gespeichert");
      }
    },
    [setting, onSave, toastSuccess, toastError],
  );

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
    [doSave, enableConfig, disableConfig],
  );

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
        {error && <p className="mt-1 text-xs text-red-600">{error}</p>}
      </div>

      <div className="flex shrink-0 items-center gap-3">
        {renderField(
          setting,
          localValue,
          handleImmediateSave,
          handleLocalChange,
          handleBlur,
          revealFn,
        )}

        {!setting.is_default &&
          setting.writable &&
          setting.type !== "boolean" && (
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
    </div>
  );
}

function settingsFieldId(key: string) {
  return `setting-${key.replaceAll(".", "-")}`;
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

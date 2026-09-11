"use client";

import { useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ISODatePicker } from "~/components/ui/date-picker";
import { Input } from "~/components/ui/input";
import { Modal } from "~/components/ui/modal";
import { TimeField } from "~/components/ui/time-field";
import { formatDate, parseISODate, toISODate } from "~/lib/date-helpers";
import { useLocalizedDatePicker } from "~/lib/hooks/use-localized-date-picker";
import { createLogger } from "~/lib/logger";
import {
  ParentApiError,
  listParentRequestEvents,
  updateExcusedRequest,
  updateMasterDataRequest,
  updatePickupChangeRequest,
  type ParentRequestShareType,
} from "~/lib/parent-api";

const logger = createLogger({ component: "RequestEditModal" });

/**
 * Eine noch offene Anfrage, so wie die Eltern sie ändern können. Ein Typ pro
 * Anfrageart, damit ein Dialog reicht statt vier fast gleicher Dialoge.
 */
export type EditableRequest =
  | {
      readonly type: "excused";
      readonly id: string;
      readonly dates: string[];
      readonly note: string;
    }
  | {
      readonly type: "pickup_change";
      readonly id: string;
      readonly date: string;
      readonly pickupTime: string;
      readonly reason: string;
    }
  | {
      readonly type: "master_data";
      readonly id: string;
      readonly label: string;
      readonly value: string;
    };

/**
 * Liest die Änderungsgeschichte einer Anfrage. Der neueste Eintrag trägt die
 * aktuelle Fassung (`expected_version`); ohne ihn wird leer gesendet, was das
 * Backend als "keine Prüfung" behandelt.
 */
export function useRequestVersion(
  studentId: string,
  requestType: ParentRequestShareType,
  requestId: string,
  active: boolean,
): { version: string; editedAt: string | null } {
  const [version, setVersion] = useState("");
  const [editedAt, setEditedAt] = useState<string | null>(null);
  useEffect(() => {
    if (!active) return;
    let running = true;
    void (async () => {
      try {
        const events = await listParentRequestEvents(
          studentId,
          requestType,
          requestId,
        );
        if (!running) return;
        setVersion(events.at(-1)?.version ?? "");
        const edited = events
          .filter((event) => event.event_type === "guardian_edited")
          .at(-1);
        setEditedAt(edited?.created_at ?? null);
      } catch (err) {
        // Die Geschichte ist nur eine Zusatzangabe. Fehlt sie, bleibt das
        // Bearbeiten möglich; das Backend prüft dann ohne Fassungsvergleich.
        logger.warn("parent_request_events_failed", {
          error: err instanceof Error ? err.message : String(err),
          request_type: requestType,
        });
      }
    })();
    return () => {
      running = false;
    };
  }, [active, requestId, requestType, studentId]);
  return { version, editedAt };
}

async function saveRequest(
  studentId: string,
  request: EditableRequest,
  draft: Draft,
  version: string,
): Promise<void> {
  switch (request.type) {
    case "excused":
      await updateExcusedRequest(studentId, request.id, {
        dates: enumerateDates(draft.from, draft.to),
        note: draft.text.trim(),
        expectedVersion: version,
      });
      return;
    case "pickup_change":
      await updatePickupChangeRequest(studentId, request.id, {
        date: draft.from,
        pickupTime: draft.time,
        reason: draft.text.trim(),
        expectedVersion: version,
      });
      return;
    case "master_data":
      await updateMasterDataRequest(studentId, request.id, {
        newValue: draft.text.trim(),
        expectedVersion: version,
      });
  }
}

interface Draft {
  from: string;
  to: string;
  time: string;
  text: string;
}

function initialDraft(request: EditableRequest): Draft {
  if (request.type === "excused") {
    const sorted = [...request.dates].sort((a, b) => a.localeCompare(b));
    return {
      from: sorted[0] ?? "",
      to: sorted.at(-1) ?? "",
      time: "",
      text: request.note,
    };
  }
  if (request.type === "pickup_change") {
    return {
      from: request.date,
      to: request.date,
      time: request.pickupTime,
      text: request.reason,
    };
  }
  return { from: "", to: "", time: "", text: request.value };
}

function enumerateDates(fromISO: string, toISO: string): string[] {
  const out: string[] = [];
  const cursor = parseISODate(fromISO);
  const end = parseISODate(toISO);
  if (Number.isNaN(cursor.getTime()) || Number.isNaN(end.getTime())) return [];
  while (cursor.getTime() <= end.getTime() && out.length < 60) {
    out.push(toISODate(cursor));
    cursor.setDate(cursor.getDate() + 1);
  }
  return out;
}

/**
 * „Anfrage bearbeiten": ändert eine noch offene Anfrage, statt sie
 * zurückzuziehen. Die OGS sieht danach die neue Fassung.
 */
export function RequestEditModal({
  studentId,
  request,
  reasonRequired = true,
  onClose,
  onSaved,
}: Readonly<{
  studentId: string;
  request: EditableRequest;
  reasonRequired?: boolean;
  onClose: () => void;
  onSaved: () => void;
}>) {
  const t = useTranslations("parentRequestEdit");
  const locale = useLocale();
  const datePicker = useLocalizedDatePicker();
  const [draft, setDraft] = useState<Draft>(() => initialDraft(request));
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const { version, editedAt } = useRequestVersion(
    studentId,
    request.type,
    request.id,
    true,
  );

  const set = (patch: Partial<Draft>) =>
    setDraft((current) => ({ ...current, ...patch }));

  const submit = async () => {
    if (request.type === "master_data" && draft.text.trim() === "") {
      setError(t("valueRequired"));
      return;
    }
    if (
      request.type !== "master_data" &&
      reasonRequired &&
      draft.text.trim() === ""
    ) {
      setError(t("reasonRequired"));
      return;
    }
    setSaving(true);
    setError(null);
    try {
      await saveRequest(studentId, request, draft, version);
      onSaved();
      onClose();
    } catch (err) {
      logger.warn("parent_request_edit_failed", {
        error: err instanceof Error ? err.message : String(err),
        request_type: request.type,
      });
      setError(
        err instanceof ParentApiError && err.code === "change_request_stale"
          ? t("staleError")
          : t("saveError"),
      );
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal
      isOpen
      onClose={onClose}
      title={t("title")}
      closeLabel={t("close")}
      backdropLabel={t("close")}
      isDismissDisabled={saving}
      mobileSheet
      footer={
        <>
          <Button
            type="button"
            variant="outline"
            size="md"
            className="hidden sm:inline-flex"
            disabled={saving}
            onClick={onClose}
          >
            {t("cancel")}
          </Button>
          <Button
            type="button"
            size="md"
            className="w-full max-sm:min-h-11 sm:w-auto"
            isLoading={saving}
            loadingText={t("saving")}
            onClick={() => void submit()}
          >
            {t("submit")}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <p className="text-sm leading-6 text-gray-700">{t("intro")}</p>
        {editedAt && (
          <p className="text-sm text-gray-600">
            {t("editedAt", { date: formatDate(editedAt, false, locale) })}
          </p>
        )}
        {request.type === "excused" && (
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <label className="block">
              <span className="mb-1 block text-sm font-medium text-gray-700">
                {t("firstDay")}
              </span>
              <ISODatePicker
                {...datePicker}
                ariaLabel={t("firstDay")}
                value={draft.from}
                onChange={(next) =>
                  set({ from: next, to: next > draft.to ? next : draft.to })
                }
                calendarLayout="popover"
                hideClearButton
              />
            </label>
            <label className="block">
              <span className="mb-1 block text-sm font-medium text-gray-700">
                {t("lastDay")}
              </span>
              <ISODatePicker
                {...datePicker}
                ariaLabel={t("lastDay")}
                value={draft.to}
                min={draft.from}
                onChange={(next) => set({ to: next })}
                calendarLayout="popover"
                hideClearButton
              />
            </label>
          </div>
        )}
        {request.type === "pickup_change" && (
          <>
            <label className="block">
              <span className="mb-1 block text-sm font-medium text-gray-700">
                {t("day")}
              </span>
              <ISODatePicker
                {...datePicker}
                ariaLabel={t("day")}
                value={draft.from}
                onChange={(next) => set({ from: next, to: next })}
                calendarLayout="popover"
                hideClearButton
              />
            </label>
            <TimeField
              value={draft.time}
              onChange={(next) => set({ time: next })}
              label={t("pickupTime")}
              hint={t("timeFormatHint")}
              placeholder={t("timeExample")}
              required
            />
          </>
        )}
        {request.type === "master_data" ? (
          <label className="block">
            <span className="mb-1 block text-sm font-medium text-gray-700">
              {request.label}
            </span>
            <Input
              name="request-value"
              aria-label={request.label}
              autoComplete="off"
              value={draft.text}
              disabled={saving}
              onChange={(event) => set({ text: event.target.value })}
            />
          </label>
        ) : (
          <label className="block">
            <span className="mb-1 block text-sm font-medium text-gray-700">
              {t("reasonLabel")}
              {reasonRequired && <span aria-hidden="true"> *</span>}
            </span>
            <textarea
              value={draft.text}
              onChange={(event) => set({ text: event.target.value })}
              rows={3}
              maxLength={2000}
              className="min-h-20 w-full resize-none rounded-lg border border-gray-300 bg-white px-3 py-2 text-base shadow-sm transition-colors hover:border-gray-400 focus-visible:border-gray-400 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
            />
          </label>
        )}
        {error && <Alert type="error" message={error} />}
      </div>
    </Modal>
  );
}

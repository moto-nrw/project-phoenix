"use client";

import { useEffect, useMemo, useState } from "react";
import { useSWRConfig } from "swr";

import { Button } from "~/components/ui/button";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { CustomSelect } from "~/components/ui/custom-select";
import { EditActions } from "~/components/ui/edit-actions";
import { useFormError } from "~/components/ui/form-error";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import {
  CardGridSkeleton,
  SkeletonRegion,
} from "~/components/ui/page-skeletons";
import { Radio } from "~/components/ui/radio";
import { SectionCard } from "~/components/ui/section-card";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { StatusBadge } from "~/components/ui/status-badge";
import { useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  staffMonthSummaryService,
  staffScheduleService,
  workTimeModelService,
} from "~/lib/staff-api";
import type {
  StaffSchedule,
  UpdateScheduleRequest,
  WorkTimeModel,
} from "~/lib/staff-api";
import {
  resolveWeekIndex,
  startOfWeek,
  toDateKey,
} from "~/lib/staff-metrics-helpers";
import { useSWRAuth } from "~/lib/swr";
import { formatDuration } from "~/lib/time-tracking-helpers";

import {
  formatDecimalHours,
  isStaleAfterModelSave,
  parseDecimalHours,
} from "./arbeitszeit-hours";
import { SonderarbeitszeitenSection } from "./sonderarbeitszeiten-section";

export {
  formatDecimalHours,
  isStaleAfterModelSave,
  parseDecimalHours,
} from "./arbeitszeit-hours";

const logger = createLogger({ component: "ArbeitszeitmodellTab" });

const dayLabels = ["Mo", "Di", "Mi", "Do", "Fr"] as const;
const dayNames = [
  "Montag",
  "Dienstag",
  "Mittwoch",
  "Donnerstag",
  "Freitag",
] as const;
const WORK_DAYS: readonly number[] = [0, 1, 2, 3, 4];
const ROTATION_OPTIONS = [
  { value: 1, label: "Standard (1 Woche)" },
  { value: 2, label: "A/B-Wochen (2 Wochen)" },
  { value: 3, label: "3-Wochen-Rotation" },
  { value: 4, label: "4-Wochen-Rotation" },
] as const;
const WEEK_BADGE_LETTERS = ["A", "B", "C", "D"] as const;
const PREVIEW_WEEKS = 4;
type EditableScheduleEntry = {
  weekIndex: number;
  dayOfWeek: number;
  targetMinutes: number;
  startTime?: string;
};

// Arbeitszeitmodell tab. Shows the staff member's contractual Soll-Stunden,
// which can either come from a tenant-level template (mode=template) or
// from per-staff custom entries (mode=custom). Both paths support 1-4 week
// rotations. The read view summarises the active model and previews the
// next four calendar weeks. Bearbeitet wird am Objekt (BAUARTEN-SPEC Bauart 2
// Regeln 3 und 4, #3119): „Bearbeiten“ schaltet den Reiter in den
// Bearbeiten-Zustand, der die Anzeige durch den Editor (Vorlage oder eigenes
// Modell) ersetzt und mit EINEM „Speichern“ unten schließt.
export function ArbeitszeitmodellTab({
  staffId,
  canEdit,
}: {
  readonly staffId: string;
  readonly canEdit: boolean;
}) {
  const [editing, setEditing] = useState(false);

  const {
    data: schedule,
    isLoading,
    mutate: mutateSchedule,
  } = useSWRAuth(`staff-schedule-${staffId}`, () =>
    staffScheduleService.getSchedule(staffId),
  );

  const { mutate } = useSWRConfig();

  if (isLoading || !schedule) {
    return (
      <SkeletonRegion
        label="Arbeitszeitmodell wird geladen"
        className="space-y-5"
      >
        <CardGridSkeleton
          cards={2}
          rowsPerCard={2}
          className="grid grid-cols-1 gap-2 sm:grid-cols-2"
        />
        <CardGridSkeleton
          cards={4}
          rowsPerCard={1}
          className="grid grid-cols-1 gap-2"
        />
      </SkeletonRegion>
    );
  }

  const today = new Date();
  const weeklyTotal = schedule.weeklyTotals.reduce((s, m) => s + m, 0);
  const weeksLabel =
    schedule.rotationLength > 1
      ? `${schedule.rotationLength}-Wochen-Rotation`
      : "1 Woche";
  const averagePerWeek =
    schedule.rotationLength > 0 ? weeklyTotal / schedule.rotationLength : 0;
  const handleScheduleSaved = () => {
    mutateSchedule();
    void mutate(isStaleAfterModelSave);
    setEditing(false);
  };

  if (editing) {
    // Der Editor ist nur im Bearbeiten-Zustand eingehängt und liest seinen
    // Ausgangsstand beim Einhängen aus `schedule`; Abbrechen hängt ihn aus,
    // also braucht es keinen Reset-Effekt wie im früheren Slide-over.
    return (
      <div className="space-y-5">
        <SectionCard title="Arbeitszeitmodell" headingLevel={3}>
          <ArbeitszeitmodellEditor
            staffId={staffId}
            schedule={schedule}
            onCancel={() => setEditing(false)}
            onSaved={handleScheduleSaved}
          />
        </SectionCard>
      </div>
    );
  }

  return (
    <div className="space-y-5">
      <SectionCard
        title={
          schedule.mode === "template" && schedule.model
            ? schedule.model.name
            : "Eigenes Modell"
        }
        headingLevel={3}
        description={`${weeksLabel} · Schnitt ${formatDuration(
          Math.round(averagePerWeek),
        )} / Woche`}
        action={
          canEdit ? (
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={() => setEditing(true)}
            >
              Bearbeiten
            </Button>
          ) : undefined
        }
      >
        <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
          {schedule.weeklyTotals.map((total, idx) => (
            <div
              key={WEEK_BADGE_LETTERS[idx] ?? `week-${idx}`}
              className="moto-content-surface rounded-xl border p-4 shadow-sm"
            >
              <div className="flex items-center justify-between">
                <span className="text-xs font-semibold tracking-wider text-gray-400 uppercase">
                  Woche {WEEK_BADGE_LETTERS[idx] ?? idx + 1}
                </span>
                <span className="text-sm font-bold text-gray-700 tabular-nums">
                  {formatDuration(total)}
                </span>
              </div>
              <div className="mt-3 grid grid-cols-5 gap-1 text-center text-xs">
                {WORK_DAYS.map((d) => {
                  const entry = schedule.entries.find(
                    (e) => e.weekIndex === idx && e.dayOfWeek === d,
                  );
                  const minutes = entry?.targetMinutes ?? 0;
                  return (
                    <div key={d} className="space-y-0.5">
                      <div className="text-xs tracking-wider text-gray-400 uppercase">
                        {dayLabels[d]}
                      </div>
                      <div
                        className={`tabular-nums ${
                          minutes > 0
                            ? "font-medium text-gray-700"
                            : "text-gray-300"
                        }`}
                      >
                        {minutes > 0 ? formatDuration(minutes) : "-"}
                      </div>
                      {minutes > 0 && entry?.startTime && (
                        <div className="text-xs text-gray-400 tabular-nums">
                          ab {entry.startTime}
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
            </div>
          ))}
        </div>
      </SectionCard>

      <FourWeekPreview staffId={staffId} schedule={schedule} today={today} />

      <SonderarbeitszeitenSection staffId={staffId} canEdit={canEdit} />
    </div>
  );
}

// Die Vorschau zeigt dasselbe Tages-Soll wie die Zeiterfassung (#3259): der
// Server rechnet Feiertage, Schließtage und Sonderarbeitszeiten ein. Solange
// es fehlt, steht kein erfundener Wert aus dem aktuellen Modell da (#1842).
function FourWeekPreview({
  staffId,
  schedule,
  today,
}: {
  readonly staffId: string;
  readonly schedule: StaffSchedule;
  readonly today: Date;
}) {
  const firstMonday = startOfWeek(today);
  const lastDay = new Date(firstMonday);
  lastDay.setDate(lastDay.getDate() + PREVIEW_WEEKS * 7 - 1);
  const fromKey = toDateKey(firstMonday);
  const toKey = toDateKey(lastDay);
  const { data: projection, error: projectionError } = useSWRAuth(
    `staff-schedule-targets-preview-${staffId}-${fromKey}-${toKey}`,
    () => staffMonthSummaryService.getDailyProjection(staffId, fromKey, toKey),
  );

  const weeks = useMemo(() => {
    const result: Array<{
      monday: Date;
      label: string;
      weekIndex: number;
      total: number | null;
      hasOverride: boolean;
      days: Array<{ date: Date; target: number | null; isOverride: boolean }>;
      isCurrent: boolean;
    }> = [];
    for (let w = 0; w < PREVIEW_WEEKS; w++) {
      const wMonday = startOfWeek(today);
      wMonday.setDate(wMonday.getDate() + w * 7);
      const days = WORK_DAYS.map((d) => {
        const date = new Date(wMonday);
        date.setDate(date.getDate() + d);
        const day = projection?.get(toDateKey(date));
        return {
          date,
          target: day ? day.targetMinutes : null,
          isOverride: day?.isOverride === true,
        };
      });
      const total = days.every((d) => d.target !== null)
        ? days.reduce((s, d) => s + (d.target ?? 0), 0)
        : null;
      result.push({
        monday: wMonday,
        label: `KW ${getISOWeek(wMonday)}`,
        weekIndex: resolveWeekIndex(schedule, wMonday),
        total,
        hasOverride: days.some((d) => d.isOverride),
        days,
        isCurrent: w === 0,
      });
    }
    return result;
  }, [schedule, projection, today]);
  const pending = projectionError ? "?" : "…";

  return (
    <SectionCard title="Vorschau (nächste 4 Wochen)" headingLevel={3}>
      <div className="space-y-2">
        {weeks.map((week) => {
          const badge = WEEK_BADGE_LETTERS[week.weekIndex] ?? "?";
          return (
            <div
              key={toDateKey(week.monday)}
              className={`flex flex-wrap items-center gap-3 rounded-xl border px-4 py-3 shadow-sm ${
                week.isCurrent
                  ? "border-moto-orange/40 bg-moto-orange-soft"
                  : "moto-content-surface"
              }`}
            >
              <span className="w-16 text-xs font-semibold tracking-wider text-gray-500 uppercase">
                {week.label}
              </span>
              {schedule.rotationLength > 1 && (
                <StatusBadge tone="gray" label={`Woche ${badge}`} />
              )}
              {week.hasOverride && (
                <StatusBadge tone="blue" label="Sonderarbeitszeit" />
              )}
              <div className="flex flex-1 flex-wrap items-center gap-3 text-xs text-gray-600">
                {week.days.map((d) => (
                  <span
                    key={toDateKey(d.date)}
                    className={
                      d.isOverride
                        ? "font-semibold text-gray-900"
                        : d.target !== null && d.target > 0
                          ? ""
                          : "text-gray-300"
                    }
                  >
                    {dayLabels[(d.date.getDay() + 6) % 7]}{" "}
                    <span className="tabular-nums">
                      {d.target === null
                        ? pending
                        : d.target > 0
                          ? formatDuration(d.target)
                          : d.isOverride
                            ? "0h"
                            : "-"}
                    </span>
                  </span>
                ))}
              </div>
              <span className="ml-auto text-sm font-bold text-gray-700 tabular-nums">
                {week.total === null ? pending : formatDuration(week.total)}
              </span>
            </div>
          );
        })}
      </div>
    </SectionCard>
  );
}

/**
 * Der Bearbeiten-Zustand des Reiters: Modus (Vorlage oder eigenes Modell),
 * darunter die Vorlagenwahl bzw. der Wocheneditor, unten `EditActions`.
 * Fehler stehen im Alert oben (Regel 5); Feldfehler am Dezimalstunden-Feld.
 */
function ArbeitszeitmodellEditor({
  staffId,
  schedule,
  onCancel,
  onSaved,
}: {
  readonly staffId: string;
  readonly schedule: StaffSchedule;
  readonly onCancel: () => void;
  readonly onSaved: () => void;
}) {
  const toast = useToast();

  const [saveError, setSaveError] = useFormError();
  const [mode, setMode] = useState<"template" | "custom">(schedule.mode);
  const [selectedModelId, setSelectedModelId] = useState<string>(
    schedule.model?.id ?? "",
  );
  const [rotationLength, setRotationLength] = useState<number>(
    schedule.rotationLength,
  );
  const [customEntries, setCustomEntries] = useState<EditableScheduleEntry[]>(
    () => initialiseCustomEntries(schedule),
  );
  const [decimalHourInputs, setDecimalHourInputs] = useState<
    Record<string, string>
  >(() => initialiseDecimalHourInputs(schedule));
  const [invalidDecimalHourInputs, setInvalidDecimalHourInputs] = useState<
    Set<string>
  >(() => new Set());
  const [activeWeekTab, setActiveWeekTab] = useState(0);
  const [saveAsTemplateName, setSaveAsTemplateName] = useState("");
  const [saving, setSaving] = useState(false);

  const { data: templates } = useSWRAuth("work-time-models", () =>
    workTimeModelService.list(),
  );

  // Reshape custom entries when the rotation_length toggle changes so the
  // edit grid is always exactly rotation × WORK_DAYS slots.
  useEffect(() => {
    setCustomEntries((prev) => {
      const next: typeof prev = [];
      for (let w = 0; w < rotationLength; w++) {
        for (const d of WORK_DAYS) {
          const existing = prev.find(
            (e) => e.weekIndex === w && e.dayOfWeek === d,
          );
          next.push({
            weekIndex: w,
            dayOfWeek: d,
            targetMinutes: existing?.targetMinutes ?? 0,
            startTime: existing?.startTime,
          });
        }
      }
      return next;
    });
    setInvalidDecimalHourInputs(
      (prev) =>
        new Set(
          [...prev].filter((key) => Number(key.split(":")[0]) < rotationLength),
        ),
    );
    setDecimalHourInputs((prev) =>
      Object.fromEntries(
        Object.entries(prev).filter(
          ([key]) => Number(key.split(":")[0]) < rotationLength,
        ),
      ),
    );
    setActiveWeekTab((curr) => Math.min(curr, rotationLength - 1));
  }, [rotationLength]);

  const handleSave = async () => {
    setSaving(true);
    setSaveError(null);
    try {
      if (mode === "template" && !selectedModelId) {
        setSaveError("Bitte eine Vorlage auswählen.");
        setSaving(false);
        return;
      }

      if (mode === "custom") {
        if (invalidDecimalHourInputs.size > 0) {
          setSaveError("Bitte die ungültigen Dezimalstunden korrigieren.");
          setSaving(false);
          return;
        }

        // Reject empty schedules: a rotation with zero target minutes
        // across all weeks would silently save and then look broken in
        // the read view. Catching this client-side beats waiting for a
        // 400 from the backend.
        const totalMinutes = customEntries.reduce(
          (sum, e) => sum + e.targetMinutes,
          0,
        );
        if (totalMinutes === 0) {
          setSaveError(
            "Das Modell hat kein Wochensoll. Bitte mindestens einen Tag eintragen.",
          );
          setSaving(false);
          return;
        }
        // Each week of the rotation should itself be non-empty,
        // otherwise the rotation has a "blank" week that produces
        // surprising Saldo gaps every Nth week.
        for (let w = 0; w < rotationLength; w++) {
          const weekTotal = customEntries
            .filter((e) => e.weekIndex === w)
            .reduce((sum, e) => sum + e.targetMinutes, 0);
          if (weekTotal === 0) {
            setSaveError(
              `Woche ${WEEK_BADGE_LETTERS[w] ?? w + 1} hat kein Wochensoll. Bitte mindestens einen Tag eintragen.`,
            );
            setSaving(false);
            return;
          }
        }
      }

      const payload: UpdateScheduleRequest =
        mode === "template"
          ? { mode: "template", modelId: selectedModelId }
          : {
              mode: "custom",
              rotationLength,
              entries: customEntries
                .filter((e) => e.targetMinutes > 0)
                .map((e) => ({
                  weekIndex: e.weekIndex,
                  dayOfWeek: e.dayOfWeek,
                  targetMinutes: e.targetMinutes,
                  ...(e.startTime ? { startTime: e.startTime } : {}),
                })),
              ...(saveAsTemplateName.trim()
                ? { saveAsTemplate: saveAsTemplateName.trim() }
                : {}),
            };

      await staffScheduleService.updateSchedule(staffId, payload);
      toast.success("Arbeitszeitmodell gespeichert");
      onSaved();
    } catch (error) {
      logger.error("schedule_save_failed", {
        error: error instanceof Error ? error.message : String(error),
        staff_id: staffId,
      });
      setSaveError(
        "Das Arbeitszeitmodell konnte nicht gespeichert werden. Bitte versuchen Sie es noch einmal.",
      );
    } finally {
      setSaving(false);
    }
  };

  const updateEntry = (
    weekIndex: number,
    dayOfWeek: number,
    minutes: number,
  ) => {
    const clamped = Math.max(0, Math.min(720, minutes));
    setCustomEntries((prev) =>
      prev.map((e) =>
        e.weekIndex === weekIndex && e.dayOfWeek === dayOfWeek
          ? { ...e, targetMinutes: clamped }
          : e,
      ),
    );
  };

  const updateDecimalHours = (
    weekIndex: number,
    dayOfWeek: number,
    value: string,
  ) => {
    const key = entryKey(weekIndex, dayOfWeek);
    setDecimalHourInputs((prev) => ({ ...prev, [key]: value }));

    const result = parseDecimalHours(value);
    setInvalidDecimalHourInputs((prev) => {
      const next = new Set(prev);
      if (result.status === "invalid") next.add(key);
      else next.delete(key);
      return next;
    });

    if (result.status === "empty") {
      updateEntry(weekIndex, dayOfWeek, 0);
    } else if (result.status === "valid") {
      updateEntry(weekIndex, dayOfWeek, result.minutes);
    } else {
      updateEntry(weekIndex, dayOfWeek, 0);
    }
  };

  const updateEntryStartTime = (
    weekIndex: number,
    dayOfWeek: number,
    startTime: string,
  ) => {
    setCustomEntries((prev) =>
      prev.map((e) =>
        e.weekIndex === weekIndex && e.dayOfWeek === dayOfWeek
          ? { ...e, startTime: startTime || undefined }
          : e,
      ),
    );
  };

  const totalForWeek = (weekIndex: number) =>
    customEntries
      .filter((e) => e.weekIndex === weekIndex)
      .reduce((sum, e) => sum + e.targetMinutes, 0);

  const rotationTotal = customEntries.reduce(
    (sum, entry) => sum + entry.targetMinutes,
    0,
  );

  return (
    <div className="space-y-5">
      <FormErrorAlert message={saveError} />

      <ModeChoice mode={mode} onChange={setMode} disabled={saving} />

      {mode === "template" ? (
        <TemplateSelector
          templates={templates ?? []}
          selectedId={selectedModelId}
          onSelect={setSelectedModelId}
        />
      ) : (
        <CustomEditor
          rotationLength={rotationLength}
          onRotationChange={setRotationLength}
          activeWeekTab={activeWeekTab}
          onActiveWeekTabChange={setActiveWeekTab}
          entries={customEntries}
          decimalHourInputs={decimalHourInputs}
          invalidDecimalHourInputs={invalidDecimalHourInputs}
          onDecimalHoursChange={updateDecimalHours}
          onStartTimeChange={updateEntryStartTime}
          totalForWeek={totalForWeek}
          rotationTotal={rotationTotal}
          saveAsTemplateName={saveAsTemplateName}
          onSaveAsTemplateNameChange={setSaveAsTemplateName}
        />
      )}

      <EditActions
        onCancel={onCancel}
        onSave={() => void handleSave()}
        saving={saving}
      />
    </div>
  );
}

const MODE_OPTIONS: ReadonlyArray<{
  readonly value: "template" | "custom";
  readonly label: string;
  readonly description: string;
}> = [
  {
    value: "template",
    label: "Vorlage zuweisen",
    description:
      "Eine Vorlage der Schule auswählen. Änderungen an der Vorlage gelten später für alle Personen mit dieser Vorlage.",
  },
  {
    value: "custom",
    label: "Eigenes Modell",
    description:
      "Eigene Soll-Stunden nur für diese Person. Optional als neue Vorlage speichern.",
  },
];

// Zwei Kacheln nebeneinander: der Reiter hat die volle Seitenbreite, anders
// als das frühere Slide-over.
function ModeChoice({
  mode,
  onChange,
  disabled,
}: {
  readonly mode: "template" | "custom";
  readonly onChange: (next: "template" | "custom") => void;
  readonly disabled: boolean;
}) {
  return (
    <div
      role="radiogroup"
      aria-label="Art des Arbeitszeitmodells"
      className="grid gap-3 md:grid-cols-2"
    >
      {MODE_OPTIONS.map((option) => {
        const id = `arbeitszeitmodell-mode-${option.value}`;
        return (
          <ChoiceTile
            key={option.value}
            htmlFor={id}
            selected={mode === option.value}
            disabled={disabled}
            className="items-start p-3"
          >
            <Radio
              id={id}
              name="arbeitszeitmodell-mode"
              value={option.value}
              checked={mode === option.value}
              disabled={disabled}
              onChange={() => onChange(option.value)}
              className="mt-0.5"
            />
            <span className="min-w-0">
              <span className="block text-sm font-medium">{option.label}</span>
              <span className="mt-1 block text-xs font-normal text-gray-500">
                {option.description}
              </span>
            </span>
          </ChoiceTile>
        );
      })}
    </div>
  );
}

function TemplateSelector({
  templates,
  selectedId,
  onSelect,
}: {
  readonly templates: WorkTimeModel[];
  readonly selectedId: string;
  readonly onSelect: (id: string) => void;
}) {
  if (templates.length === 0) {
    return (
      <p className="rounded-xl bg-gray-50 px-3 py-2 text-xs text-gray-500">
        Es gibt noch keine Vorlagen. Wählen Sie „Eigenes Modell“ und speichern
        Sie das Modell dort als Vorlage.
      </p>
    );
  }
  const selected = templates.find((t) => t.id === selectedId);
  return (
    <div className="space-y-3">
      <label
        id="arbeitszeitmodell-template-select-label"
        htmlFor="arbeitszeitmodell-template-select"
        className="block text-xs font-semibold tracking-wider text-gray-500 uppercase"
      >
        Vorlage
      </label>
      <CustomSelect
        id="arbeitszeitmodell-template-select"
        ariaLabelledBy="arbeitszeitmodell-template-select-label"
        value={selectedId}
        onChange={onSelect}
        options={[
          { value: "", label: "Bitte wählen…" },
          ...templates.map((t) => ({
            value: t.id,
            label:
              t.rotationLength > 1
                ? `${t.name} (${t.rotationLength} Wochen)`
                : t.name,
          })),
        ]}
        placeholder="Bitte wählen…"
      />
      {selected && <TemplatePreviewBlock model={selected} />}
    </div>
  );
}

function TemplatePreviewBlock({ model }: { readonly model: WorkTimeModel }) {
  return (
    <div className="rounded-2xl border border-gray-100 bg-gray-50/50 px-4 py-3">
      <p className="text-xs text-gray-500">
        {model.rotationLength > 1
          ? `${model.rotationLength}-Wochen-Rotation`
          : "1-Wochen-Pattern"}
      </p>
      <div className="mt-2 space-y-1.5">
        {Array.from({ length: model.rotationLength }, (_, w) => {
          const weekTotal = model.entries
            .filter((e) => e.weekIndex === w)
            .reduce((s, e) => s + e.targetMinutes, 0);
          return (
            <div
              key={w}
              className="flex flex-wrap items-center justify-between gap-2 text-xs"
            >
              <span className="font-medium text-gray-700">
                Woche {WEEK_BADGE_LETTERS[w] ?? w + 1}
              </span>
              <div className="flex min-w-full flex-wrap gap-3 text-gray-500 sm:min-w-0 sm:flex-1 sm:justify-center">
                {WORK_DAYS.map((d) => {
                  const entry = model.entries.find(
                    (e) => e.weekIndex === w && e.dayOfWeek === d,
                  );
                  return (
                    <span key={d} className="tabular-nums">
                      {dayLabels[d]}{" "}
                      {entry && entry.targetMinutes > 0
                        ? `${entry.startTime ? `ab ${entry.startTime} · ` : ""}${formatDuration(entry.targetMinutes)}`
                        : "-"}
                    </span>
                  );
                })}
              </div>
              <span className="font-semibold text-gray-700 tabular-nums">
                {formatDuration(weekTotal)}
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
}

function CustomEditor({
  rotationLength,
  onRotationChange,
  activeWeekTab,
  onActiveWeekTabChange,
  entries,
  decimalHourInputs,
  invalidDecimalHourInputs,
  onDecimalHoursChange,
  onStartTimeChange,
  totalForWeek,
  rotationTotal,
  saveAsTemplateName,
  onSaveAsTemplateNameChange,
}: {
  readonly rotationLength: number;
  readonly onRotationChange: (next: number) => void;
  readonly activeWeekTab: number;
  readonly onActiveWeekTabChange: (next: number) => void;
  readonly entries: Array<{
    weekIndex: number;
    dayOfWeek: number;
    targetMinutes: number;
    startTime?: string;
  }>;
  readonly decimalHourInputs: Readonly<Record<string, string>>;
  readonly invalidDecimalHourInputs: ReadonlySet<string>;
  readonly onDecimalHoursChange: (
    weekIndex: number,
    dayOfWeek: number,
    value: string,
  ) => void;
  readonly onStartTimeChange: (
    weekIndex: number,
    dayOfWeek: number,
    startTime: string,
  ) => void;
  readonly totalForWeek: (weekIndex: number) => number;
  readonly rotationTotal: number;
  readonly saveAsTemplateName: string;
  readonly onSaveAsTemplateNameChange: (next: string) => void;
}) {
  return (
    <div className="space-y-4">
      <div>
        <label
          id="arbeitszeitmodell-rotation-select-label"
          htmlFor="arbeitszeitmodell-rotation-select"
          className="mb-2 block text-xs font-semibold tracking-wider text-gray-500 uppercase"
        >
          Rotation
        </label>
        <CustomSelect
          id="arbeitszeitmodell-rotation-select"
          ariaLabelledBy="arbeitszeitmodell-rotation-select-label"
          value={String(rotationLength)}
          onChange={(next) => onRotationChange(Number.parseInt(next, 10))}
          options={ROTATION_OPTIONS.map((opt) => ({
            value: String(opt.value),
            label: opt.label,
          }))}
        />
      </div>

      {rotationLength > 1 && (
        <SegmentedControl
          ariaLabel="Woche der Rotation"
          fullWidth
          items={Array.from({ length: rotationLength }, (_, w) => ({
            value: String(w),
            label: `Woche ${WEEK_BADGE_LETTERS[w] ?? w + 1}`,
          }))}
          value={String(activeWeekTab)}
          onChange={(next) => onActiveWeekTabChange(Number(next))}
        />
      )}

      <p className="text-xs text-gray-500">
        Dezimalstunden mit Komma oder Punkt eingeben. Die Dauer wird auf die
        nächste volle Minute gerundet, halbe Minuten werden aufgerundet.
      </p>

      <div className="space-y-2">
        {WORK_DAYS.map((d) => {
          const entry = entries.find(
            (e) => e.weekIndex === activeWeekTab && e.dayOfWeek === d,
          );
          const minutes = entry?.targetMinutes ?? 0;
          const startTime = entry?.startTime ?? "";
          const key = entryKey(activeWeekTab, d);
          const decimalHours = decimalHourInputs[key] ?? "";
          const isInvalid = invalidDecimalHourInputs.has(key);
          const durationId = `arbeitszeitmodell-duration-${activeWeekTab}-${d}`;
          const errorId = `arbeitszeitmodell-error-${activeWeekTab}-${d}`;
          return (
            <div
              key={d}
              className="flex flex-wrap items-center gap-2 py-1 sm:gap-3"
            >
              <span className="w-10 shrink-0 text-sm font-medium text-gray-600">
                {dayLabels[d]}
              </span>
              <div className="w-28">
                <Input
                  aria-label={`Dezimalstunden ${dayNames[d]}`}
                  aria-describedby={
                    isInvalid ? `${durationId} ${errorId}` : durationId
                  }
                  aria-invalid={isInvalid || undefined}
                  type="text"
                  inputMode="decimal"
                  autoComplete="off"
                  controlSize="compact"
                  value={decimalHours}
                  onFocus={(e) => e.target.select()}
                  onChange={(e) =>
                    onDecimalHoursChange(activeWeekTab, d, e.target.value)
                  }
                  placeholder="z. B. 4,45"
                  className="text-center tabular-nums"
                />
                {isInvalid && (
                  <p
                    id={errorId}
                    role="alert"
                    className="text-moto-red-strong mt-1 text-xs"
                  >
                    Bitte 0 bis 12 eingeben.
                  </p>
                )}
              </div>
              <span className="text-xs text-gray-400">Dezimalstd.</span>
              <label className="flex items-center gap-1 text-xs text-gray-500">
                <span>Start</span>
                <input
                  type="time"
                  value={startTime}
                  disabled={minutes <= 0}
                  onChange={(e) =>
                    onStartTimeChange(activeWeekTab, d, e.target.value)
                  }
                  className="w-24 rounded-lg border border-gray-200 px-2 py-1.5 text-sm text-gray-700 tabular-nums focus:border-gray-400 focus:outline-none disabled:cursor-not-allowed disabled:bg-gray-50 disabled:text-gray-400"
                />
              </label>
              <span
                id={durationId}
                role="status"
                aria-live="polite"
                aria-atomic="true"
                className="ml-auto text-xs text-gray-500 tabular-nums"
              >
                {isInvalid
                  ? "Ungültiger Wert"
                  : `${formatDuration(minutes)} (${minutes} min)`}
              </span>
            </div>
          );
        })}
        <div className="flex items-center justify-between border-t border-gray-100 pt-2 text-xs">
          <span className="text-gray-500">
            Wochensoll Woche{" "}
            {WEEK_BADGE_LETTERS[activeWeekTab] ?? activeWeekTab + 1}
          </span>
          <span className="font-bold text-gray-700 tabular-nums">
            {formatDuration(totalForWeek(activeWeekTab))}
          </span>
        </div>
        {rotationLength > 1 && (
          <div className="flex items-center justify-between text-xs">
            <span className="text-gray-500">
              Rotation ({rotationLength} Wochen)
            </span>
            <span className="font-bold text-gray-700 tabular-nums">
              {formatDuration(rotationTotal)} gesamt · Ø{" "}
              {formatDuration(Math.round(rotationTotal / rotationLength))} /
              Woche
            </span>
          </div>
        )}
      </div>

      <div className="border-t border-gray-100 pt-4">
        <Input
          id="arbeitszeitmodell-save-as-input"
          label="Optional als Vorlage speichern"
          type="text"
          controlSize="compact"
          value={saveAsTemplateName}
          onChange={(e) => onSaveAsTemplateNameChange(e.target.value)}
          placeholder="z. B. Teilzeit 30h A/B"
        />
        <p className="mt-1 text-xs text-gray-500">
          Leer lassen, um nur das Modell dieser Person zu ändern.
        </p>
      </div>
    </div>
  );
}

function entryKey(weekIndex: number, dayOfWeek: number): string {
  return `${weekIndex}:${dayOfWeek}`;
}

function initialiseDecimalHourInputs(
  schedule: StaffSchedule,
): Record<string, string> {
  return Object.fromEntries(
    initialiseCustomEntries(schedule).map((entry) => [
      entryKey(entry.weekIndex, entry.dayOfWeek),
      entry.targetMinutes > 0 ? formatDecimalHours(entry.targetMinutes) : "",
    ]),
  );
}

function initialiseCustomEntries(
  schedule: StaffSchedule,
): EditableScheduleEntry[] {
  const length = Math.max(1, schedule.rotationLength);
  const out: EditableScheduleEntry[] = [];
  for (let w = 0; w < length; w++) {
    for (const d of WORK_DAYS) {
      const entry = schedule.entries.find(
        (e) => e.weekIndex === w && e.dayOfWeek === d,
      );
      out.push({
        weekIndex: w,
        dayOfWeek: d,
        targetMinutes: entry?.targetMinutes ?? 0,
        startTime: entry?.startTime,
      });
    }
  }
  return out;
}

function getISOWeek(date: Date): number {
  const target = new Date(date.valueOf());
  const dayNr = (date.getDay() + 6) % 7;
  target.setDate(target.getDate() - dayNr + 3);
  const firstThursday = target.valueOf();
  target.setMonth(0, 1);
  if (target.getDay() !== 4) {
    target.setMonth(0, 1 + ((4 - target.getDay() + 7) % 7));
  }
  return 1 + Math.ceil((firstThursday - target.valueOf()) / 604_800_000);
}

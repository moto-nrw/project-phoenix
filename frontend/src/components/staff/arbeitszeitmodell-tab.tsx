"use client";

import { useEffect, useMemo, useState } from "react";
import { useSWRConfig } from "swr";

import { CustomSelect } from "~/components/ui/custom-select";
import { Loading } from "~/components/ui/loading";
import { Modal } from "~/components/ui/modal";
import { useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { staffScheduleService, workTimeModelService } from "~/lib/staff-api";
import type {
  StaffSchedule,
  UpdateScheduleRequest,
  WorkTimeModel,
} from "~/lib/staff-api";
import {
  resolveTargetForDate,
  resolveWeekIndex,
  startOfWeek,
  toDateKey,
} from "~/lib/staff-metrics-helpers";
import { useSWRAuth } from "~/lib/swr";
import { formatDuration } from "~/lib/time-tracking-helpers";

const logger = createLogger({ component: "ArbeitszeitmodellTab" });

// A work-time model save rewrites the contractual Soll, so every cache that
// prices days against it goes stale. Two portals read the same staff member's
// targets: the admin staff-detail tabs key them by staff id
// (staff-schedule-targets- / staff-month-summary-), while a manager editing
// their OWN model also has the own-service portal's caches open, which key
// WITHOUT an id (time-tracking-schedule-targets- for the daily table and the
// weekly KPI, time-tracking-month-summary- for the Monatskarte). Both sets must
// be invalidated, or the self-service daily table keeps showing the old
// Soll/Saldo while the monthly summary has already updated (#1842). Everything
// is recomputed live on the server, so a plain invalidation suffices. useSWRAuth
// prefixes keys with the tenant slug, so we match with includes, not startsWith
// — the same convention as staff-session-table's handleSaved.
export function isStaleAfterModelSave(key: unknown): boolean {
  return (
    typeof key === "string" &&
    (key.includes("staff-schedule-targets-") ||
      key.includes("staff-month-summary-") ||
      key.includes("time-tracking-schedule-targets-") ||
      key.includes("time-tracking-month-summary-"))
  );
}

const dayLabels = ["Mo", "Di", "Mi", "Do", "Fr"] as const;
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
// next four calendar weeks; the editor lets admins switch between template
// assignment and a custom pattern.
export function ArbeitszeitmodellTab({
  staffId,
  canEdit,
}: {
  readonly staffId: string;
  readonly canEdit: boolean;
}) {
  const [editorOpen, setEditorOpen] = useState(false);

  const {
    data: schedule,
    isLoading,
    mutate: mutateSchedule,
  } = useSWRAuth(`staff-schedule-${staffId}`, () =>
    staffScheduleService.getSchedule(staffId),
  );

  const { mutate } = useSWRConfig();

  if (isLoading || !schedule) {
    return <Loading fullPage={false} />;
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
  };

  return (
    <div className="space-y-5">
      <div className="rounded-3xl border border-gray-100/50 bg-white/90 p-6 shadow-[0_8px_30px_rgb(0,0,0,0.12)]">
        <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
          <div>
            <h3 className="text-sm font-semibold tracking-wide text-gray-400 uppercase">
              Aktuelles Modell
            </h3>
            <p className="mt-1 text-base font-semibold text-gray-800">
              {schedule.mode === "template" && schedule.model
                ? schedule.model.name
                : "Eigenes Modell"}
            </p>
            <p className="text-xs text-gray-500">
              {weeksLabel} · Schnitt{" "}
              {formatDuration(Math.round(averagePerWeek))} / Woche
            </p>
          </div>
          {canEdit && (
            <button
              type="button"
              onClick={() => setEditorOpen(true)}
              className="rounded-full bg-gray-900 px-4 py-2 text-xs font-medium text-white transition-colors hover:bg-gray-800"
            >
              Bearbeiten
            </button>
          )}
        </div>

        <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
          {schedule.weeklyTotals.map((total, idx) => (
            <div
              key={WEEK_BADGE_LETTERS[idx] ?? `week-${idx}`}
              className="rounded-2xl border border-gray-100 bg-white p-4"
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
                      <div className="text-[10px] tracking-wider text-gray-400 uppercase">
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
                        <div className="text-[10px] text-gray-400 tabular-nums">
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
      </div>

      <FourWeekPreview schedule={schedule} today={today} />

      <EditArbeitszeitmodellModal
        isOpen={editorOpen}
        onClose={() => setEditorOpen(false)}
        staffId={staffId}
        schedule={schedule}
        onSaved={handleScheduleSaved}
      />
    </div>
  );
}

function FourWeekPreview({
  schedule,
  today,
}: {
  readonly schedule: StaffSchedule;
  readonly today: Date;
}) {
  const weeks = useMemo(() => {
    const monday = startOfWeek(today);
    const result: Array<{
      monday: Date;
      label: string;
      weekIndex: number;
      total: number;
      days: Array<{ date: Date; target: number }>;
      isCurrent: boolean;
    }> = [];
    for (let w = 0; w < PREVIEW_WEEKS; w++) {
      const wMonday = new Date(monday);
      wMonday.setDate(wMonday.getDate() + w * 7);
      const days = WORK_DAYS.map((d) => {
        const date = new Date(wMonday);
        date.setDate(date.getDate() + d);
        return { date, target: resolveTargetForDate(schedule, date) };
      });
      const total = days.reduce((s, d) => s + d.target, 0);
      result.push({
        monday: wMonday,
        label: `KW ${getISOWeek(wMonday)}`,
        weekIndex: resolveWeekIndex(schedule, wMonday),
        total,
        days,
        isCurrent: w === 0,
      });
    }
    return result;
  }, [schedule, today]);

  return (
    <div className="rounded-3xl border border-gray-100/50 bg-white/90 p-6 shadow-[0_8px_30px_rgb(0,0,0,0.12)]">
      <h3 className="mb-4 text-sm font-semibold tracking-wide text-gray-400 uppercase">
        Vorschau (nächste 4 Wochen)
      </h3>
      <div className="space-y-2">
        {weeks.map((week) => {
          const badge = WEEK_BADGE_LETTERS[week.weekIndex] ?? "?";
          return (
            <div
              key={toDateKey(week.monday)}
              className={`flex flex-wrap items-center gap-3 rounded-2xl border px-4 py-3 ${
                week.isCurrent
                  ? "border-amber-200 bg-amber-50/60"
                  : "border-gray-100 bg-white"
              }`}
            >
              <span className="w-16 text-xs font-semibold tracking-wider text-gray-500 uppercase">
                {week.label}
              </span>
              {schedule.rotationLength > 1 && (
                <span className="rounded-full bg-gray-100 px-2 py-0.5 text-[10px] font-bold tracking-wider text-gray-600 uppercase">
                  Woche {badge}
                </span>
              )}
              <div className="flex flex-1 flex-wrap items-center gap-3 text-xs text-gray-600">
                {week.days.map((d) => (
                  <span
                    key={d.date.toISOString()}
                    className={d.target > 0 ? "" : "text-gray-300"}
                  >
                    {dayLabels[(d.date.getDay() + 6) % 7]}{" "}
                    <span className="tabular-nums">
                      {d.target > 0 ? formatDuration(d.target) : "-"}
                    </span>
                  </span>
                ))}
              </div>
              <span className="ml-auto text-sm font-bold text-gray-700 tabular-nums">
                {formatDuration(week.total)}
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
}

function EditArbeitszeitmodellModal({
  isOpen,
  onClose,
  staffId,
  schedule,
  onSaved,
}: {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly staffId: string;
  readonly schedule: StaffSchedule;
  readonly onSaved: () => void;
}) {
  const toast = useToast();

  const [mode, setMode] = useState<"template" | "custom">(schedule.mode);
  const [selectedModelId, setSelectedModelId] = useState<string>(
    schedule.model?.id ?? "",
  );
  const [rotationLength, setRotationLength] = useState<number>(
    schedule.rotationLength,
  );
  const [customEntries, setCustomEntries] = useState<EditableScheduleEntry[]>(
    initialiseCustomEntries(schedule),
  );
  const [activeWeekTab, setActiveWeekTab] = useState(0);
  const [saveAsTemplateName, setSaveAsTemplateName] = useState("");
  const [saving, setSaving] = useState(false);

  const { data: templates } = useSWRAuth(
    isOpen ? "work-time-models" : null,
    () => workTimeModelService.list(),
  );

  // Reset local state every time the modal opens so it always reflects the
  // server's current truth, not whatever the previous edit session left
  // behind.
  useEffect(() => {
    if (!isOpen) return;
    setMode(schedule.mode);
    setSelectedModelId(schedule.model?.id ?? "");
    setRotationLength(schedule.rotationLength);
    setCustomEntries(initialiseCustomEntries(schedule));
    setActiveWeekTab(0);
    setSaveAsTemplateName("");
  }, [isOpen, schedule]);

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
    setActiveWeekTab((curr) => Math.min(curr, rotationLength - 1));
  }, [rotationLength]);

  const handleSave = async () => {
    setSaving(true);
    try {
      if (mode === "template" && !selectedModelId) {
        toast.error("Bitte eine Vorlage auswählen.");
        setSaving(false);
        return;
      }

      if (mode === "custom") {
        // Reject empty schedules: a rotation with zero target minutes
        // across all weeks would silently save and then look broken in
        // the read view. Catching this client-side beats waiting for a
        // 400 from the backend.
        const totalMinutes = customEntries.reduce(
          (sum, e) => sum + e.targetMinutes,
          0,
        );
        if (totalMinutes === 0) {
          toast.error(
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
            toast.error(
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
      onClose();
    } catch (error) {
      logger.error("schedule_save_failed", {
        error: error instanceof Error ? error.message : String(error),
        staff_id: staffId,
      });
      toast.error("Fehler beim Speichern");
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

  const footer = (
    <div className="flex w-full flex-col-reverse gap-2 sm:flex-row sm:items-center sm:justify-end">
      <button
        type="button"
        onClick={onClose}
        disabled={saving}
        className="rounded-full border border-gray-200 bg-white px-4 py-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-50 disabled:opacity-40"
      >
        Abbrechen
      </button>
      <button
        type="button"
        onClick={() => {
          handleSave();
        }}
        disabled={saving}
        className="rounded-full bg-gray-900 px-5 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-40"
      >
        {saving ? "Speichern..." : "Speichern"}
      </button>
    </div>
  );

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Arbeitszeitmodell bearbeiten"
      footer={footer}
    >
      <div className="space-y-5">
        <ModeRadioGroup mode={mode} onChange={setMode} />

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
            onEntryChange={updateEntry}
            onStartTimeChange={updateEntryStartTime}
            totalForWeek={totalForWeek}
            saveAsTemplateName={saveAsTemplateName}
            onSaveAsTemplateNameChange={setSaveAsTemplateName}
          />
        )}
      </div>
    </Modal>
  );
}

function ModeRadioGroup({
  mode,
  onChange,
}: {
  readonly mode: "template" | "custom";
  readonly onChange: (next: "template" | "custom") => void;
}) {
  return (
    <div className="space-y-2">
      <label
        htmlFor="mode-template"
        className={`flex cursor-pointer items-start gap-3 rounded-xl border px-3 py-2 ${
          mode === "template" ? "border-gray-900 bg-gray-50" : "border-gray-200"
        }`}
      >
        <input
          id="mode-template"
          type="radio"
          checked={mode === "template"}
          onChange={() => onChange("template")}
          aria-label="Vorlage zuweisen"
          className="mt-1 accent-gray-900"
        />
        <div>
          <p className="text-sm font-medium text-gray-800">Vorlage zuweisen</p>
          <p className="mt-1 text-xs text-gray-500">
            Schulweite Vorlage aus der Liste auswählen. Änderungen an der
            Vorlage wirken zukünftig auf alle Mitarbeitenden mit dieser
            Zuordnung.
          </p>
        </div>
      </label>
      <label
        htmlFor="mode-custom"
        className={`flex cursor-pointer items-start gap-3 rounded-xl border px-3 py-2 ${
          mode === "custom" ? "border-gray-900 bg-gray-50" : "border-gray-200"
        }`}
      >
        <input
          id="mode-custom"
          type="radio"
          checked={mode === "custom"}
          onChange={() => onChange("custom")}
          aria-label="Eigenes Modell"
          className="mt-1 accent-gray-900"
        />
        <div>
          <p className="text-sm font-medium text-gray-800">Eigenes Modell</p>
          <p className="mt-1 text-xs text-gray-500">
            Individuelles Arbeitszeitmodell für diesen Mitarbeiter, optional als
            neue Vorlage speichern.
          </p>
        </div>
      </label>
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
        Es sind noch keine Vorlagen angelegt. Wechsle zu &quot;Eigenes
        Modell&quot; und speichere das Pattern als Vorlage.
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
        options={templates.map((t) => ({
          value: t.id,
          label:
            t.rotationLength > 1
              ? `${t.name} (${t.rotationLength} Wochen)`
              : t.name,
        }))}
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
  onEntryChange,
  onStartTimeChange,
  totalForWeek,
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
  readonly onEntryChange: (
    weekIndex: number,
    dayOfWeek: number,
    minutes: number,
  ) => void;
  readonly onStartTimeChange: (
    weekIndex: number,
    dayOfWeek: number,
    startTime: string,
  ) => void;
  readonly totalForWeek: (weekIndex: number) => number;
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
        <div className="flex gap-1 overflow-x-auto rounded-full border border-gray-200 bg-white p-1">
          {Array.from({ length: rotationLength }, (_, w) => (
            <button
              key={w}
              type="button"
              onClick={() => onActiveWeekTabChange(w)}
              className={`flex-1 shrink-0 rounded-full px-3 py-1.5 text-xs font-medium transition-colors ${
                activeWeekTab === w
                  ? "bg-gray-900 text-white"
                  : "text-gray-500 hover:text-gray-700"
              }`}
            >
              Woche {WEEK_BADGE_LETTERS[w] ?? w + 1}
            </button>
          ))}
        </div>
      )}

      <div className="space-y-2">
        {WORK_DAYS.map((d) => {
          const entry = entries.find(
            (e) => e.weekIndex === activeWeekTab && e.dayOfWeek === d,
          );
          const minutes = entry?.targetMinutes ?? 0;
          const startTime = entry?.startTime ?? "";
          const hours = Math.floor(minutes / 60);
          const mins = minutes % 60;
          return (
            <div
              key={d}
              className="flex flex-wrap items-center gap-2 py-1 sm:gap-3"
            >
              <span className="w-10 shrink-0 text-sm font-medium text-gray-600">
                {dayLabels[d]}
              </span>
              <input
                aria-label={`Arbeitsstunden ${dayLabels[d]}`}
                type="number"
                min={0}
                max={12}
                // Display empty instead of "0" so the user can backspace the
                // existing value and type a new one without the field
                // snapping back to "0" after every keystroke. The state still
                // tracks 0, only the rendered value is hidden.
                value={hours === 0 ? "" : hours}
                onFocus={(e) => e.target.select()}
                onChange={(e) => {
                  const raw = e.target.value;
                  // Empty input is parsed as 0 so the parent state is always
                  // a real number; the value={} ternary above keeps the box
                  // visually empty in that case.
                  const h =
                    raw === ""
                      ? 0
                      : Math.max(
                          0,
                          Math.min(12, Number.parseInt(raw, 10) || 0),
                        );
                  // Backend caps target_minutes at 720 (12h). When the user
                  // hits 12h via the hours field, force minutes to 0 so we
                  // never produce 12h 30min, that would 400 on save.
                  const nextMins = h >= 12 ? 0 : mins;
                  onEntryChange(activeWeekTab, d, h * 60 + nextMins);
                }}
                className="w-14 rounded-lg border border-gray-200 px-2 py-1.5 text-center text-sm text-gray-700 tabular-nums focus:border-gray-400 focus:outline-none"
              />
              <span className="text-xs text-gray-400">h</span>
              <CustomSelect
                ariaLabel={`Arbeitsminuten ${dayLabels[d]}`}
                value={String(mins)}
                onChange={(next) => {
                  const m = Number.parseInt(next, 10) || 0;
                  onEntryChange(activeWeekTab, d, hours * 60 + m);
                }}
                options={[
                  { value: "0", label: "00" },
                  { value: "15", label: "15" },
                  { value: "30", label: "30" },
                  { value: "45", label: "45" },
                ]}
                disabled={hours >= 12}
                triggerClassName="h-8 w-16 border-gray-200 bg-white tabular-nums"
              />
              <span className="text-xs text-gray-400">min</span>
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
              <span className="ml-auto text-xs text-gray-500 tabular-nums">
                {formatDuration(minutes)}
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
      </div>

      <div className="border-t border-gray-100 pt-4">
        <label
          htmlFor="arbeitszeitmodell-save-as-input"
          className="mb-2 block text-xs font-semibold tracking-wider text-gray-500 uppercase"
        >
          Optional als Vorlage speichern
        </label>
        <input
          id="arbeitszeitmodell-save-as-input"
          type="text"
          value={saveAsTemplateName}
          onChange={(e) => onSaveAsTemplateNameChange(e.target.value)}
          placeholder="z.B. Teilzeit 30h A/B"
          className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-gray-400 focus:outline-none"
        />
        <p className="mt-1 text-[11px] text-gray-400">
          Leer lassen, um nur das Modell dieses Mitarbeiters zu ändern.
        </p>
      </div>
    </div>
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

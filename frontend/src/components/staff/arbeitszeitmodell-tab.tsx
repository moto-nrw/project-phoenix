"use client";
import { parseISODate, berlinTodayISO } from "~/lib/date-helpers";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";

import { StatCard } from "~/components/ui/stat-card";
import { Alert } from "~/components/ui/alert";
import { EmptyState } from "~/components/ui/empty-state";
import { MISSING_TARGET_OWNER } from "~/components/staff/missing-target-notice";
import { useStaffHasTarget } from "~/lib/hooks/use-staff-has-target";
import { Button } from "~/components/ui/button";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { CustomSelect } from "~/components/ui/custom-select";
import { EditActions } from "~/components/ui/edit-actions";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import {
  CardGridSkeleton,
  SkeletonRegion,
} from "~/components/ui/page-skeletons";
import { Radio } from "~/components/ui/radio";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import {
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
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
import { useInvalidateMatching, useSWRAuth } from "~/lib/swr";
import { workTimeModelKeys } from "~/lib/swr/staff-keys";
import { formatDuration } from "~/lib/time-tracking-helpers";

import { isStaleAfterModelSave } from "./arbeitszeit-hours";
import { SonderarbeitszeitenSection } from "./sonderarbeitszeiten-section";
import {
  draftFromPattern,
  patternEntries,
  validatePatternDraft,
  WEEK_BADGE_LETTERS,
  WORK_DAYS,
} from "./work-time-pattern-draft";
import { dayLabels, WorkTimePatternEditor } from "./work-time-pattern-editor";
import { InsetPanel } from "~/components/ui/inset-panel";
import {
  CatalogManageLink,
  useCatalogRefreshOnFocus,
} from "~/components/database/catalog/catalog-manage-link";
import { SectionLabel } from "~/components/ui/section-label";

export {
  formatDecimalHours,
  isStaleAfterModelSave,
  parseDecimalHours,
} from "./arbeitszeit-hours";

const logger = createLogger({ component: "ArbeitszeitmodellTab" });

const PREVIEW_WEEKS = 4;

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
  // Gilt in diesem Monat ein Soll? Dieselbe Regel, nach der das Stundenkonto
  // rechnet (#6887); null solange unbekannt.
  // Den Reiter sieht nur die Leitung; sie darf Zeitkonten lesen.
  const hasTarget = useStaffHasTarget(staffId, berlinTodayISO(), canEdit);

  const {
    data: schedule,
    isLoading,
    error: scheduleError,
    mutate: mutateSchedule,
  } = useSWRAuth(`staff-schedule-${staffId}`, () =>
    staffScheduleService.getSchedule(staffId),
  );

  const invalidate = useInvalidateMatching();
  const load = useApiLoadError();
  const showLoadError = load.show;
  const clearLoadError = load.clear;
  useEffect(() => {
    if (scheduleError) {
      void showLoadError(scheduleError, {
        object: "das Arbeitszeitmodell",
        retry: () => void mutateSchedule(),
      });
    } else {
      clearLoadError();
    }
  }, [scheduleError, mutateSchedule, showLoadError, clearLoadError]);

  // Ein Ladefehler steht an Stelle des Modells, statt ewig zu laden.
  if (!schedule && load.error) {
    return <LoadErrorAlert error={load.error} />;
  }

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

  const today = parseISODate(berlinTodayISO());
  const weeklyTotal = schedule.weeklyTotals.reduce((s, m) => s + m, 0);
  const weeksLabel =
    schedule.rotationLength > 1
      ? `${schedule.rotationLength}-Wochen-Rotation`
      : "1 Woche";
  const averagePerWeek =
    schedule.rotationLength > 0 ? weeklyTotal / schedule.rotationLength : 0;
  const handleScheduleSaved = () => {
    mutateSchedule();
    void invalidate(isStaleAfterModelSave);
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

  // Ein leerer regulärer Plan schließt Sonderarbeitszeiten nicht aus.
  // Nur der aufgelöste Sollstatus begründet die Warnung (#6887).
  const scheduleEmpty = !schedule.model && schedule.entries.length === 0;
  const missingTargetText = `Ohne Soll zählen Krankheit, Urlaub und Fortbildung 0 Stunden. Gearbeitete Zeit zählt ganz als Plus. ${
    canEdit
      ? "Tragen Sie mit „Bearbeiten“ die Wochenstunden aus dem Arbeitsvertrag ein."
      : MISSING_TARGET_OWNER
  }`;

  return (
    <div className="space-y-5">
      {hasTarget === false && !scheduleEmpty ? (
        <Alert
          type="warning"
          announce="off"
          message={`In diesem Monat gilt kein Soll. ${missingTargetText}`}
        />
      ) : null}
      <SectionCard
        title={
          scheduleEmpty
            ? "Arbeitszeitmodell"
            : schedule.mode === "template" && schedule.model
              ? schedule.model.name
              : "Eigenes Modell"
        }
        headingLevel={3}
        description={
          scheduleEmpty
            ? undefined
            : `${weeksLabel} · Schnitt ${formatDuration(
                Math.round(averagePerWeek),
              )} / Woche`
        }
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
        {scheduleEmpty ? (
          <EmptyState
            variant="compact"
            title={
              hasTarget === false
                ? "Kein Soll hinterlegt"
                : "Kein reguläres Arbeitszeitmodell"
            }
            description={
              hasTarget === false
                ? missingTargetText
                : canEdit
                  ? "Tragen Sie mit „Bearbeiten“ die Wochenstunden aus dem Arbeitsvertrag ein."
                  : "Das Arbeitszeitmodell trägt die Leitung ein."
            }
          />
        ) : (
          <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
            {schedule.weeklyTotals.map((total, idx) => (
              <div key={WEEK_BADGE_LETTERS[idx] ?? `week-${idx}`}>
                {/* Eigene Hülle: `h-full` der Kachel soll sich nicht auf die Höhe
                  samt Tageszeile darunter beziehen. */}
                <div>
                  <StatCard
                    label={`Woche ${WEEK_BADGE_LETTERS[idx] ?? idx + 1}`}
                    value={formatDuration(total)}
                    tone="neutral"
                  />
                </div>
                <div className="mt-3 grid grid-cols-5 gap-1 text-center text-xs">
                  {WORK_DAYS.map((d) => {
                    const entry = schedule.entries.find(
                      (e) => e.weekIndex === idx && e.dayOfWeek === d,
                    );
                    const minutes = entry?.targetMinutes ?? 0;
                    return (
                      <div key={d} className="space-y-0.5">
                        <div className="text-xs text-gray-500">
                          {dayLabels[d]}
                        </div>
                        <div
                          className={`tabular-nums ${
                            minutes > 0
                              ? "font-medium text-gray-700"
                              : "text-gray-300"
                          }`}
                        >
                          {minutes > 0 ? formatDuration(minutes) : "–"}
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
        )}
      </SectionCard>

      <SonderarbeitszeitenSection staffId={staffId} canEdit={canEdit} />

      <FourWeekPreview staffId={staffId} schedule={schedule} today={today} />
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
  const {
    data: projection,
    error: projectionError,
    mutate: mutateProjection,
  } = useSWRAuth(
    `staff-schedule-targets-preview-${staffId}-${fromKey}-${toKey}`,
    () => staffMonthSummaryService.getDailyProjection(staffId, fromKey, toKey),
  );
  const projectionLoad = useApiLoadError();
  const showProjectionError = projectionLoad.show;
  const clearProjectionError = projectionLoad.clear;
  useEffect(() => {
    if (projectionError) {
      void showProjectionError(projectionError, {
        object: "das Soll der nächsten Wochen",
        retry: () => void mutateProjection(),
      });
    } else {
      clearProjectionError();
    }
  }, [
    projectionError,
    mutateProjection,
    showProjectionError,
    clearProjectionError,
  ]);

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
  // Pending: "…"; unavailable: "–" with the reason in the alert above.
  const pending = projectionError ? "–" : "…";

  return (
    <SectionCard title="Vorschau (nächste 4 Wochen)" headingLevel={3}>
      {projectionLoad.error ? (
        <div className="mb-3">
          <LoadErrorAlert error={projectionLoad.error} />
        </div>
      ) : null}
      <div className="space-y-2">
        {weeks.map((week) => {
          const badge = WEEK_BADGE_LETTERS[week.weekIndex] ?? "?";
          return (
            <InsetPanel
              key={toDateKey(week.monday)}
              tone={week.isCurrent ? "warning" : "white"}
              className="flex flex-wrap items-center gap-3"
            >
              <SectionLabel as="p" className="w-16">
                {week.label}
              </SectionLabel>
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
                        : d.target > 0 || d.isOverride
                          ? formatDuration(d.target)
                          : "–"}
                    </span>
                  </span>
                ))}
              </div>
              <span className="ml-auto text-sm font-bold text-gray-700 tabular-nums">
                {week.total === null ? pending : formatDuration(week.total)}
              </span>
            </InsetPanel>
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

  const formErrors = useApiFormError();
  const [mode, setMode] = useState<"template" | "custom">(schedule.mode);
  const [selectedModelId, setSelectedModelId] = useState<string>(
    schedule.model?.id ?? "",
  );
  const [draft, setDraft] = useState(() =>
    draftFromPattern(schedule.rotationLength, schedule.entries),
  );
  const [saveAsTemplateName, setSaveAsTemplateName] = useState("");
  const [saving, setSaving] = useState(false);

  const { data: templates, mutate: reloadTemplates } = useSWRAuth(
    workTimeModelKeys.picker(),
    () => workTimeModelService.list(),
  );
  // Eine nebenan auf der Katalogseite angelegte Vorlage steht ohne Neuladen
  // zur Wahl.
  const refreshTemplates = useCallback(
    () => void reloadTemplates(),
    [reloadTemplates],
  );
  useCatalogRefreshOnFocus(refreshTemplates, mode === "template");

  const handleSave = async () => {
    setSaving(true);
    formErrors.clear();
    try {
      if (mode === "template" && !selectedModelId) {
        formErrors.invalid("Bitte wählen Sie eine Vorlage.");
        setSaving(false);
        return;
      }

      if (mode === "custom") {
        // Ein leeres Modell oder eine leere Rotationswoche sähe nach dem
        // Speichern kaputt aus und erzeugte jede n-te Woche Minusstunden.
        const invalid = validatePatternDraft(draft);
        if (invalid) {
          formErrors.invalid(invalid);
          setSaving(false);
          return;
        }
      }

      const payload: UpdateScheduleRequest =
        mode === "template"
          ? { mode: "template", modelId: selectedModelId }
          : {
              mode: "custom",
              rotationLength: draft.rotationLength,
              entries: patternEntries(draft),
              ...(saveAsTemplateName.trim()
                ? { saveAsTemplate: saveAsTemplateName.trim() }
                : {}),
            };

      await staffScheduleService.updateSchedule(staffId, payload);
      toast.success("Das Arbeitszeitmodell ist gespeichert.");
      onSaved();
    } catch (error) {
      logger.error("schedule_save_failed", {
        error: error instanceof Error ? error.message : String(error),
        staff_id: staffId,
      });
      await formErrors.show(error, {
        object: "das Arbeitszeitmodell",
        retry: () => void latestSave.current(),
      });
    } finally {
      setSaving(false);
    }
  };
  // „Wiederholen“ sendet den aktuellen Entwurf.
  const latestSave = useRef(handleSave);
  useLayoutEffect(() => {
    latestSave.current = handleSave;
  });

  return (
    <div className="space-y-5">
      <FormErrorAlert message={formErrors.error} />

      <ModeChoice mode={mode} onChange={setMode} disabled={saving} />

      {mode === "template" ? (
        <TemplateSelector
          templates={templates ?? []}
          boundId={schedule.model?.id ?? null}
          selectedId={selectedModelId}
          onSelect={setSelectedModelId}
        />
      ) : (
        <WorkTimePatternEditor draft={draft} onChange={setDraft}>
          <div className="border-t border-gray-100 pt-4">
            <Input
              id="arbeitszeitmodell-save-as-input"
              label="Optional als Vorlage speichern"
              type="text"
              controlSize="compact"
              value={saveAsTemplateName}
              onChange={(e) => setSaveAsTemplateName(e.target.value)}
              placeholder="z. B. Teilzeit 30h A/B"
              hint="Leer lassen, um nur das Modell dieser Person zu ändern."
            />
          </div>
        </WorkTimePatternEditor>
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

const MANAGE_TEMPLATES_LINK = {
  href: "/database/work-time-models",
  label: "Arbeitszeitmodelle verwalten",
} as const;

function TemplateSelector({
  templates: allTemplates,
  boundId,
  selectedId,
  onSelect,
}: {
  readonly templates: WorkTimeModel[];
  /** Die Vorlage, die die Person schon nutzt; sie bleibt wählbar (#6891). */
  readonly boundId: string | null;
  readonly selectedId: string;
  readonly onSelect: (id: string) => void;
}) {
  // Eine archivierte Vorlage wird nicht neu zugewiesen. Wer sie schon nutzt,
  // sieht sie weiter, damit das Speichern die Zuordnung nicht verliert.
  const templates = allTemplates.filter(
    (t) => t.archivedAt === null || t.id === boundId,
  );
  if (templates.length === 0) {
    return (
      <div className="space-y-2">
        <InsetPanel as="p" padding="compact" className="text-xs text-gray-500">
          Es gibt noch keine Vorlagen. Legen Sie eine unter „Arbeitszeitmodelle“
          an. Oder speichern Sie ein eigenes Modell als Vorlage.
        </InsetPanel>
        <CatalogManageLink {...MANAGE_TEMPLATES_LINK} />
      </div>
    );
  }
  const selected = templates.find((t) => t.id === selectedId);
  return (
    <div className="space-y-3">
      <CustomSelect
        label="Vorlage"
        id="arbeitszeitmodell-template-select"
        value={selectedId}
        onChange={onSelect}
        options={[
          { value: "", label: "Bitte wählen…" },
          ...templates.map((t) => ({
            value: t.id,
            label: [
              t.name,
              t.rotationLength > 1 ? ` (${t.rotationLength} Wochen)` : "",
              t.archivedAt === null ? "" : " (nicht mehr angeboten)",
            ].join(""),
          })),
        ]}
        placeholder="Bitte wählen…"
      />
      {selected && <TemplatePreviewBlock model={selected} />}
      <CatalogManageLink {...MANAGE_TEMPLATES_LINK} />
    </div>
  );
}

function TemplatePreviewBlock({ model }: { readonly model: WorkTimeModel }) {
  return (
    <InsetPanel framed padding="md">
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
                        : "–"}
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
    </InsetPanel>
  );
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

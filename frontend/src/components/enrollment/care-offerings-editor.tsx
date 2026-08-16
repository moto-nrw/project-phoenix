"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Check, Copy, Pencil, Plus, Trash2 } from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import {
  type CareOffering,
  type CareOfferingInput,
  type CareOfferingAvailabilityCondition,
  type CareOfferingAvailabilityRule,
  type CareSelectionRule,
  type DaysOfWeekMode,
  SELECTION_RULE_LABELS,
  cloneCareOffering,
  createCareOffering,
  deleteCareOffering,
  listCareOfferings,
  updateCareOffering,
} from "~/lib/care-offering-api";
import {
  type CareOfferingBookingGradeCounts,
  careOfferingAvailabilityRuleError,
  countCareOfferingRuleConflicts,
  describeCareOfferingAvailabilityRule,
} from "~/lib/care-offering-availability";
import {
  type CareOfferingBookingStats,
  fetchCareOfferingBookingStats,
} from "~/lib/care-offering-booking-stats";
import {
  previewCareOfferingPickupRollout,
  rolloutCareOfferingPickupTimes,
  type OfferingPickupRolloutPreview,
} from "~/lib/care-offering-api";
import { Modal } from "~/components/ui/modal";
import { Checkbox } from "~/components/ui/checkbox";
import { Button } from "~/components/ui/button";
import { type Phase, listPhases } from "~/lib/enrollment-phase-api";
import { calendarPeriodService } from "~/lib/calendar-period-api";
import type { CalendarPeriod } from "~/lib/calendar-period-helpers";
import { CARE_OFFERING_TEMPLATE_PERIOD_MISMATCH_MESSAGE } from "~/lib/enrollment-error-messages";
import { createLogger } from "~/lib/logger";
import { timetableService } from "~/lib/timetable-api";
import type { TimetableTemplate } from "~/lib/timetable-types";
import { isSupportedGradeLevelMax } from "~/lib/grade-level";
import { useTenant } from "~/lib/tenant-context";
import { useToast } from "~/contexts/ToastContext";
import {
  copyStableObjectKey,
  getStableObjectKey,
} from "~/lib/stable-object-key";
import {
  DataTable,
  DataTableStatusBadge,
  type DataTableColumn,
} from "~/components/ui/data-table";
import { CustomSelect } from "~/components/ui/custom-select";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { FeaturePill } from "~/components/enrollment/feature-pill";
import { formatOfferingPrice } from "~/lib/care-offering-format";

const logger = createLogger({ component: "CareOfferingsEditor" });

const DAY_LABELS: Record<string, string> = {
  mon: "Mo",
  tue: "Di",
  wed: "Mi",
  thu: "Do",
  fri: "Fr",
  sat: "Sa",
  sun: "So",
};

const WEEKDAY_KEYS = ["mon", "tue", "wed", "thu", "fri"];
const CARE_DAY_TO_ISO: Record<string, number> = {
  mon: 1,
  tue: 2,
  wed: 3,
  thu: 4,
  fri: 5,
  sat: 6,
  sun: 7,
};

const ISO_WEEKDAY_LABELS: Record<number, string> = {
  1: "Mo",
  2: "Di",
  3: "Mi",
  4: "Do",
  5: "Fr",
  6: "Sa",
  7: "So",
};

const KIND_LABELS: Record<Phase["kind"], string> = {
  school_year: "Schuljahr",
  holiday: "Ferienbetreuung",
  custom: "Sonstiges",
};

const CARE_OFFERING_TEMPLATE_UNLINKED_MESSAGE =
  "Die Verknüpfung zum Regeltermin wurde entfernt, weil sein Planungszeitraum den Betreuungszeitraum der neu gewählten Anmeldephase nicht vollständig abdeckt.";
const PLANNER_METADATA_UNAVAILABLE_MESSAGE =
  "Regeltermine und Planungsperioden konnten nicht geladen werden. Die Betreuungsangebote bleiben nutzbar; bestehende Verknüpfungen können beibehalten oder entfernt werden. Neue oder geänderte Verknüpfungen sind bis zum erneuten Laden gesperrt.";
const UNVERIFIABLE_TEMPLATE_CHANGE_MESSAGE =
  "Die neue Regeltermin-Verknüpfung kann derzeit nicht geprüft werden. Entferne die Verknüpfung oder lade Regeltermine und Planungsperioden erneut.";
const INACTIVE_TEMPLATE_PERIOD_MESSAGE =
  "Ein aktives Betreuungsangebot braucht einen aktiven Planungszeitraum. Aktiviere den Zeitraum, wähle einen anderen Regeltermin oder deaktiviere das Angebot.";
const CARE_OFFERING_DAYS_REQUIRED_MESSAGE =
  "Bitte wähle mindestens einen Wochentag für das Angebot aus.";

type PlannerMetadataStatus = "loading" | "ready" | "unavailable";
type TemplateCompatibility = "compatible" | "incompatible" | "unknown";

function blankInput(phaseId: number): CareOfferingInput {
  return {
    phase_id: phaseId,
    activity_group_id: null,
    name: "",
    description: "",
    days_of_week_mode: "fixed",
    // Deliberately empty: pre-selecting Mo-Fr caused offerings whose name
    // said "Montags" to silently accept all weekdays (#1885). Picking the
    // days is a required, conscious input.
    available_days: [],
    includes_holiday_care: false,
    includes_lunch: false,
    capacity: null,
    price_cents: null,
    is_active: true,
    is_required: false,
    counts_as_care: true,
    auto_add_grade_levels: [],
    availability_rule: null,
    auto_add_trigger_offering_ids: [],
    sort_order: 0,
    selection_group: "",
    selection_rule: "optional",
    pickup_times: {},
  };
}

function offeringToInput(offering: CareOffering): CareOfferingInput {
  return {
    phase_id: Number(offering.phase_id),
    activity_group_id: offering.activity_group_id
      ? Number(offering.activity_group_id)
      : null,
    name: offering.name,
    description: offering.description ?? "",
    days_of_week_mode: offering.days_of_week_mode,
    available_days: offering.available_days,
    includes_holiday_care: offering.includes_holiday_care,
    includes_lunch: offering.includes_lunch,
    capacity: offering.capacity ?? null,
    price_cents: offering.price_cents ?? null,
    is_active: offering.is_active,
    is_required: offering.is_required,
    counts_as_care: offering.counts_as_care ?? true,
    auto_add_grade_levels: offering.auto_add_grade_levels ?? [],
    availability_rule: offering.availability_rule ?? null,
    auto_add_trigger_offering_ids: [
      ...(offering.auto_add_trigger_offering_ids ?? []),
    ],
    sort_order: offering.sort_order,
    selection_group: offering.selection_group ?? "",
    selection_rule: offering.selection_rule ?? "optional",
    pickup_times: { ...(offering.pickup_times ?? {}) },
  };
}

function formatPrice(cents?: number | null): string {
  return formatOfferingPrice(cents) ?? "Ohne Preis";
}

function formatDays(days: string[]): string {
  const visibleDays = days.filter((day) => WEEKDAY_KEYS.includes(day));
  if (visibleDays.length === 0) return "Keine Wochentage";
  return visibleDays.map((day) => DAY_LABELS[day] ?? day).join(", ");
}

function safeCareOfferingSaveMessage(err: unknown, creating: boolean): string {
  if (
    err instanceof Error &&
    typeof (err as Error & { status?: unknown }).status === "number"
  ) {
    // API errors are localized and sanitized by readEnrollmentError. Plain
    // runtime/network errors are not safe UI copy and stay in structured logs.
    return err.message;
  }
  return creating
    ? "Betreuungsangebot konnte nicht angelegt werden"
    : "Betreuungsangebot konnte nicht gespeichert werden";
}

function templateLabel(template: TimetableTemplate): string {
  const days = template.schedules
    .map((schedule) => schedule.weekday)
    .sort((a, b) => a - b)
    .map((weekday) => ISO_WEEKDAY_LABELS[weekday] ?? `Tag ${weekday}`)
    .join(", ");
  const time = template.schedules[0]
    ? `${template.schedules[0].startTime}-${template.schedules[0].endTime}`
    : "ohne Zeit";
  return `${template.name} (${days || "keine Tage"}, ${time})`;
}

function singleTemplatePeriodID(template: TimetableTemplate): string | null {
  if (template.schedules.length === 0) return null;

  const periodIDs = new Set<string>();
  for (const schedule of template.schedules) {
    const periodID = schedule.calendarPeriodId ?? template.calendarPeriodId;
    if (!periodID) return null;
    periodIDs.add(periodID);
    if (periodIDs.size > 1) return null;
  }
  return periodIDs.values().next().value ?? null;
}

function templatePhaseCompatibility(
  template: TimetableTemplate,
  phase: Phase,
  periods: CalendarPeriod[],
  requireActivePeriod: boolean,
): TemplateCompatibility {
  const periodID = singleTemplatePeriodID(template);
  if (!periodID) return "unknown";
  const period = periods.find((item) => item.id === periodID);
  if (!period) return "unknown";
  if (requireActivePeriod && !period.isActive) return "incompatible";
  const fits =
    period.startDate <= phase.service_start_date &&
    phase.service_end_date <= period.endDate;
  return fits ? "compatible" : "incompatible";
}

function draftTemplateCompatibility(
  draft: CareOfferingInput,
  phases: Phase[],
  templates: TimetableTemplate[],
  periods: CalendarPeriod[],
  metadataStatus: PlannerMetadataStatus,
): TemplateCompatibility {
  if (!draft.activity_group_id) return "compatible";
  if (metadataStatus !== "ready") return "unknown";
  const phase = phases.find((item) => item.id === String(draft.phase_id));
  const template = templates.find(
    (item) => item.id === String(draft.activity_group_id),
  );
  if (!phase || !template) return "unknown";
  return templatePhaseCompatibility(template, phase, periods, draft.is_active);
}

function draftUsesInactiveTemplatePeriod(
  draft: CareOfferingInput,
  templates: TimetableTemplate[],
  periods: CalendarPeriod[],
  metadataStatus: PlannerMetadataStatus,
): boolean {
  if (
    !draft.is_active ||
    !draft.activity_group_id ||
    metadataStatus !== "ready"
  ) {
    return false;
  }
  const template = templates.find(
    (item) => item.id === String(draft.activity_group_id),
  );
  if (!template) return false;
  const periodID = singleTemplatePeriodID(template);
  if (!periodID) return false;
  return periods.find((period) => period.id === periodID)?.isActive === false;
}

function linkedTemplateWeekdayError(
  draft: CareOfferingInput,
  templates: TimetableTemplate[],
  metadataStatus: PlannerMetadataStatus,
): string | null {
  if (!draft.activity_group_id || metadataStatus !== "ready") return null;
  const template = templates.find(
    (item) => item.id === draft.activity_group_id?.toString(),
  );
  if (!template) return null;

  const templateDays = new Set(
    template.schedules.map((schedule) => schedule.weekday),
  );
  const missingDays = draft.available_days.filter((day) => {
    const isoWeekday = CARE_DAY_TO_ISO[day];
    return isoWeekday !== undefined && !templateDays.has(isoWeekday);
  });
  if (missingDays.length === 0) return null;

  const labels = missingDays.map((day) => DAY_LABELS[day] ?? day).join(", ");
  return `Der Regeltermin deckt die ausgewählten Angebotstage ${labels} nicht ab. Entferne die Verknüpfung oder ergänze passende Slots.`;
}

const GERMAN_WEEKDAY_NAME_PATTERNS: ReadonlyArray<[string, RegExp]> = [
  ["mon", /\bmontags?\b/i],
  ["tue", /\bdienstags?\b/i],
  ["wed", /\bmittwochs?\b/i],
  ["thu", /\bdonnerstags?\b/i],
  ["fri", /\bfreitags?\b/i],
  ["sat", /\b(?:samstags?|sonnabends?)\b/i],
  ["sun", /\bsonntags?\b/i],
];

// Soft warning only: the name heuristic has documented false positives
// ("Fußball (startet am Montag, 1.9.)"), so it must never block saving.
// The hard guard for template-linked offerings is linkedTemplateWeekdayError.
function nameWeekdayMismatchWarning(draft: CareOfferingInput): string | null {
  const matches = GERMAN_WEEKDAY_NAME_PATTERNS.filter(([, pattern]) =>
    pattern.test(draft.name),
  );
  // Zero or several weekday words: the name makes no single-day claim.
  if (matches.length !== 1) return null;
  const namedDay = matches[0]?.[0];
  if (!namedDay) return null;
  const extraDays = draft.available_days.filter((day) => day !== namedDay);
  if (extraDays.length === 0) return null;
  const labels = extraDays.map((day) => DAY_LABELS[day] ?? day).join(", ");
  return `Der Name nennt nur einen Wochentag, ausgewählt sind zusätzlich ${labels}. Bitte prüfen, ob das Angebot wirklich an diesen Tagen stattfindet.`;
}

function hasUnverifiableTemplateChange(
  draft: CareOfferingInput,
  originalActivityGroupID: string | null,
  metadataStatus: PlannerMetadataStatus,
): boolean {
  if (metadataStatus === "ready" || draft.activity_group_id == null) {
    return false;
  }
  return String(draft.activity_group_id) !== originalActivityGroupID;
}

function linkedTemplateWarnings(
  draft: CareOfferingInput,
  templates: TimetableTemplate[],
): string[] {
  if (!draft.activity_group_id) return [];
  const template = templates.find(
    (item) => item.id === draft.activity_group_id?.toString(),
  );
  if (!template)
    return ["Der ausgewählte Regeltermin konnte nicht geladen werden."];
  const warnings: string[] = [];
  if (template.schedules.length === 0) {
    warnings.push("Der ausgewählte Regeltermin hat noch keine Slots.");
    return warnings;
  }
  const templateDays = new Set(
    template.schedules.map((schedule) => schedule.weekday),
  );
  const offeringDays = new Set(
    draft.available_days
      .map((day) => CARE_DAY_TO_ISO[day])
      .filter((day): day is number => day !== undefined),
  );
  if ([...templateDays].some((day) => !offeringDays.has(day))) {
    warnings.push(
      "Der Regeltermin enthält Tage, die im Angebot nicht auswählbar sind.",
    );
  }
  if (!singleTemplatePeriodID(template)) {
    warnings.push(
      "Der Regeltermin muss genau eine Planungsperiode für alle Slots verwenden.",
    );
  }
  return warnings;
}

export function CareOfferingsEditor() {
  const { tenant } = useTenant();
  const gradeLevelMax = isSupportedGradeLevelMax(tenant?.gradeLevelMax)
    ? tenant.gradeLevelMax
    : null;
  const [phases, setPhases] = useState<Phase[]>([]);
  const [selectedPhaseId, setSelectedPhaseId] = useState<string>("");
  const selectedPhaseIdRef = useRef("");
  const catalogLoadSeq = useRef(0);
  const metadataLoadSeq = useRef(0);
  const invalidateLoads = useCallback(() => {
    catalogLoadSeq.current++;
    metadataLoadSeq.current++;
  }, []);
  const [offerings, setOfferings] = useState<CareOffering[]>([]);
  const [templates, setTemplates] = useState<TimetableTemplate[]>([]);
  const [periods, setPeriods] = useState<CalendarPeriod[]>([]);
  const [metadataStatus, setMetadataStatus] =
    useState<PlannerMetadataStatus>("loading");
  const [loading, setLoading] = useState(true);
  const [catalogError, setCatalogError] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [draft, setDraft] = useState<CareOfferingInput | null>(null);
  const [saving, setSaving] = useState(false);
  const [pickupRollout, setPickupRollout] = useState<{
    offeringId: string;
    offeringName: string;
    preview: OfferingPickupRolloutPreview;
    skipIds: Set<string>;
  } | null>(null);
  const [rollingOut, setRollingOut] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [cloneSource, setCloneSource] = useState<CareOffering | null>(null);
  // Booking stats are display-only (#2186): the availability-rule editor uses
  // them to say how many existing bookings a rule would contradict. A failure
  // to load them must never block catalog editing, so it is logged and the
  // hint simply stays absent.
  const [bookingStats, setBookingStats] = useState<
    Record<string, CareOfferingBookingStats>
  >({});
  const toast = useToast();

  const hasNoPhases = phases.length === 0;
  const activeOfferingCount = offerings.filter(
    (offering) => offering.is_active,
  ).length;
  const selectableDaysCount = new Set(
    offerings.flatMap((offering) => offering.available_days),
  ).size;
  const totalCapacity = offerings.reduce((sum, offering) => {
    if (offering.capacity == null) return sum;
    return sum + offering.capacity;
  }, 0);

  const loadPlannerMetadata = useCallback(async () => {
    const requestSeq = ++metadataLoadSeq.current;
    setMetadataStatus("loading");
    try {
      const [templateData, periodData] = await Promise.all([
        timetableService.getTemplates(),
        calendarPeriodService.list(),
      ]);
      if (metadataLoadSeq.current !== requestSeq) return;
      setTemplates(templateData.templates);
      setPeriods(periodData);
      setMetadataStatus("ready");
    } catch (err) {
      if (metadataLoadSeq.current !== requestSeq) return;
      const message =
        err instanceof Error ? err.message : "Unbekannter Metadatenfehler";
      logger.warn("care_offerings_planner_metadata_load_failed", {
        error: message,
      });
      setMetadataStatus("unavailable");
    }
  }, []);

  const loadAll = useCallback(
    async (preferredPhaseId?: string) => {
      const requestSeq = ++catalogLoadSeq.current;
      setLoading(true);
      setError(null);
      setCatalogError(null);
      try {
        const phasesData = await listPhases();
        if (catalogLoadSeq.current !== requestSeq) return;

        let phaseId = preferredPhaseId ?? selectedPhaseIdRef.current;
        if (!phasesData.some((phase) => phase.id === phaseId)) {
          const first =
            phasesData.find((phase) => phase.is_active) ?? phasesData[0];
          phaseId = first?.id ?? "";
        }
        // Planner metadata and the phase-specific catalog are independent once
        // the phase is known. Metadata failure must never hide the catalog.
        void loadPlannerMetadata();
        const offeringsData = phaseId ? await listCareOfferings(phaseId) : [];
        if (catalogLoadSeq.current !== requestSeq) return;
        setPhases(phasesData);
        selectedPhaseIdRef.current = phaseId;
        setSelectedPhaseId(phaseId);
        setOfferings(offeringsData);
      } catch (err) {
        if (catalogLoadSeq.current !== requestSeq) return;
        const message =
          err instanceof Error ? err.message : "Unbekannter Fehler";
        logger.error("care_offerings_load_failed", { error: message });
        setOfferings([]);
        setCatalogError(message);
        toast.error(message);
      } finally {
        if (catalogLoadSeq.current === requestSeq) setLoading(false);
      }
    },
    [loadPlannerMetadata, toast],
  );

  useEffect(() => {
    void loadAll();
    return invalidateLoads;
  }, [invalidateLoads, loadAll]);

  useEffect(() => {
    if (!selectedPhaseId) {
      setBookingStats({});
      return;
    }
    let cancelled = false;
    void fetchCareOfferingBookingStats(selectedPhaseId)
      .then((stats) => {
        if (!cancelled) setBookingStats(stats);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setBookingStats({});
        logger.warn("care_offering_booking_stats_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
    return () => {
      cancelled = true;
    };
  }, [selectedPhaseId, offerings]);

  const beginCreate = () => {
    if (!selectedPhaseId) return;
    setDraft(blankInput(Number(selectedPhaseId)));
    setEditingId("new");
    setCloneSource(null);
    setError(null);
  };

  const beginEdit = (offering: CareOffering) => {
    setDraft(offeringToInput(offering));
    setEditingId(offering.id);
    setCloneSource(null);
    setError(null);
  };

  const cancelFocusMode = () => {
    setDraft(null);
    setEditingId(null);
    setCloneSource(null);
  };

  const handleSave = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!draft) return;
    if (draft.available_days.length === 0) {
      setError(CARE_OFFERING_DAYS_REQUIRED_MESSAGE);
      toast.error(CARE_OFFERING_DAYS_REQUIRED_MESSAGE);
      return;
    }
    const originalActivityGroupID =
      editingId && editingId !== "new"
        ? (offerings.find((offering) => offering.id === editingId)
            ?.activity_group_id ?? null)
        : null;
    if (
      hasUnverifiableTemplateChange(
        draft,
        originalActivityGroupID,
        metadataStatus,
      )
    ) {
      setError(UNVERIFIABLE_TEMPLATE_CHANGE_MESSAGE);
      toast.error(UNVERIFIABLE_TEMPLATE_CHANGE_MESSAGE);
      return;
    }
    if (
      draftTemplateCompatibility(
        draft,
        phases,
        templates,
        periods,
        metadataStatus,
      ) === "incompatible"
    ) {
      const message = draftUsesInactiveTemplatePeriod(
        draft,
        templates,
        periods,
        metadataStatus,
      )
        ? INACTIVE_TEMPLATE_PERIOD_MESSAGE
        : CARE_OFFERING_TEMPLATE_PERIOD_MISMATCH_MESSAGE;
      setError(message);
      toast.error(message);
      return;
    }
    const weekdayError = linkedTemplateWeekdayError(
      draft,
      templates,
      metadataStatus,
    );
    if (weekdayError) {
      setError(weekdayError);
      toast.error(weekdayError);
      return;
    }
    setSaving(true);
    setError(null);
    try {
      let savedOffering: CareOffering | null = null;
      if (editingId === "new") {
        savedOffering = await createCareOffering(draft);
        toast.success(`Betreuungsangebot „${savedOffering.name}" erstellt.`);
      } else if (editingId) {
        savedOffering = await updateCareOffering(editingId, draft);
        toast.success(`Betreuungsangebot „${savedOffering.name}" gespeichert.`);
      }
      cancelFocusMode();
      await loadAll();
      if (savedOffering) {
        await maybeOfferPickupRollout(savedOffering);
      }
    } catch (err) {
      const technicalMessage =
        err instanceof Error ? err.message : "Unbekannter Speicherfehler";
      logger.error("care_offering_save_failed", { error: technicalMessage });
      const message = safeCareOfferingSaveMessage(err, editingId === "new");
      setError(message);
      toast.error(message);
    } finally {
      setSaving(false);
    }
  };

  // Nach dem Speichern: Trockenlauf der Gehzeit-Übertragung. Nur wenn sich
  // dadurch etwas ändern würde, erscheint der Bestätigungsdialog (#2290).
  const maybeOfferPickupRollout = async (offering: CareOffering) => {
    try {
      const preview = await previewCareOfferingPickupRollout(offering.id);
      const changes =
        preview.new_rows +
        preview.updated_rows +
        preview.removed_rows +
        preview.conflicts.length;
      if (changes === 0) return;
      setPickupRollout({
        offeringId: offering.id,
        offeringName: offering.name,
        preview,
        skipIds: new Set<string>(),
      });
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Gehzeit-Vorschau fehlgeschlagen";
      logger.error("care_offering_pickup_preview_failed", { error: message });
      toast.error(message);
    }
  };

  const confirmPickupRollout = async () => {
    if (!pickupRollout) return;
    setRollingOut(true);
    try {
      const result = await rolloutCareOfferingPickupTimes(
        pickupRollout.offeringId,
        [...pickupRollout.skipIds],
      );
      const parts = [
        `${result.created_rows + result.updated_rows} Gehzeiten übertragen`,
      ];
      if (result.deleted_rows > 0)
        parts.push(`${result.deleted_rows} entfernt`);
      if (result.skipped_students > 0)
        parts.push(`${result.skipped_students} Kinder ausgenommen`);
      toast.success(parts.join(", ") + ".");
      setPickupRollout(null);
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Gehzeit-Ausrollen fehlgeschlagen";
      logger.error("care_offering_pickup_rollout_failed", { error: message });
      toast.error(message);
    } finally {
      setRollingOut(false);
    }
  };

  const handleDelete = useCallback(
    async (offering: CareOffering) => {
      if (
        !globalThis.window.confirm(
          `Betreuungsangebot „${offering.name}" wirklich löschen? Diese Aktion ist nicht umkehrbar.`,
        )
      ) {
        return;
      }
      setDeletingId(offering.id);
      setError(null);
      try {
        await deleteCareOffering(offering.id);
        toast.success(`Betreuungsangebot „${offering.name}" gelöscht.`);
        await loadAll();
      } catch (err) {
        const message =
          err instanceof Error ? err.message : "Löschen fehlgeschlagen";
        logger.error("care_offering_delete_failed", { error: message });
        const hint =
          "Wenn das Betreuungsangebot bereits in Anmeldungen verwendet wird, deaktiviere es stattdessen über Bearbeiten.";
        setError(`${message}. ${hint}`);
        toast.error(message);
      } finally {
        setDeletingId(null);
      }
    },
    [loadAll, toast],
  );

  const handleClone = async (targetPhaseId: string) => {
    if (!cloneSource) return;
    setSaving(true);
    setError(null);
    try {
      const cloned = await cloneCareOffering(cloneSource.id, {
        target_phase_id: Number(targetPhaseId),
      });
      toast.success(`Betreuungsangebot „${cloned.name}" dupliziert.`);
      setCloneSource(null);
      await loadAll();
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Klonen fehlgeschlagen";
      logger.error("care_offering_clone_failed", { error: message });
      setError(message);
      toast.error(message);
    } finally {
      setSaving(false);
    }
  };

  const columns = useMemo<DataTableColumn<CareOffering>[]>(
    () => [
      {
        key: "name",
        header: "Betreuungsangebot",
        sortValue: (offering) => offering.name,
        render: (offering) => (
          <div className="min-w-0">
            <p className="truncate font-medium text-gray-900">
              {offering.name}
            </p>
            <p className="mt-0.5 truncate text-xs text-gray-500">
              {offering.description || "Keine Beschreibung hinterlegt"}
            </p>
          </div>
        ),
      },
      {
        key: "days",
        header: "Tage",
        sortValue: (offering) => offering.available_days.length,
        render: (offering) => <DaysCell offering={offering} />,
      },
      {
        key: "details",
        header: "Details",
        render: (offering) => (
          <div className="flex flex-wrap gap-1.5">
            {offering.is_required ? <FeaturePill label="Pflicht" /> : null}
            {offering.includes_lunch ? (
              <FeaturePill label="Mittagessen" />
            ) : null}
            {offering.includes_holiday_care ? (
              <FeaturePill label="Ferienbetreuung" />
            ) : null}
            {offering.activity_group_id ? (
              <FeaturePill label="Betreuungsplan" />
            ) : null}
            <FeaturePill
              label={
                offering.counts_as_care === false
                  ? "Zählt nicht als Betreuungstag"
                  : "Zählt als Betreuungstag"
              }
            />
            {(offering.auto_add_trigger_offering_ids?.length ?? 0) > 0 ? (
              <FeaturePill label="Wird mitgebucht" />
            ) : null}
            <AvailabilityRulePill
              rule={offering.availability_rule}
              gradeLevelMax={gradeLevelMax}
            />
          </div>
        ),
      },
      {
        key: "capacity",
        header: "Kapazität",
        sortValue: (offering) => offering.capacity ?? Number.POSITIVE_INFINITY,
        render: (offering) => (
          <span className="text-sm text-gray-700">
            {offering.capacity == null
              ? "Unbegrenzt"
              : `${offering.capacity} Plätze`}
          </span>
        ),
      },
      {
        key: "price",
        header: "Preis",
        sortValue: (offering) => offering.price_cents ?? 0,
        render: (offering) => (
          <span className="text-sm text-gray-700">
            {formatPrice(offering.price_cents)}
          </span>
        ),
      },
      {
        key: "status",
        header: "Status",
        sortValue: (offering) => (offering.is_active ? 1 : 0),
        render: (offering) => (
          <DataTableStatusBadge active={offering.is_active} />
        ),
      },
      {
        key: "actions",
        header: "Aktionen",
        align: "right",
        render: (offering) => (
          <CareOfferingActions
            offering={offering}
            deleting={deletingId === offering.id}
            saving={saving}
            onEdit={() => beginEdit(offering)}
            onClone={() => {
              setCloneSource(offering);
              setDraft(null);
              setEditingId(null);
              setError(null);
            }}
            onDelete={() => void handleDelete(offering)}
          />
        ),
      },
    ],
    [deletingId, gradeLevelMax, handleDelete, saving],
  );

  if (loading) {
    return (
      <div className="moto-content-surface rounded-2xl border px-5 py-10 text-center text-sm text-gray-500 shadow-sm">
        Betreuungsangebote werden geladen...
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {error && (
        <div
          className="border-moto-red/20 bg-moto-red/10 text-moto-red-strong rounded-2xl border p-4 text-sm"
          role="alert"
          aria-live="polite"
        >
          {error}
        </div>
      )}

      {metadataStatus === "unavailable" ? (
        <PlannerMetadataNotice onRetry={() => void loadPlannerMetadata()} />
      ) : null}

      {catalogError ? (
        <CareOfferingCatalogError
          message={catalogError}
          onRetry={() => void loadAll(selectedPhaseIdRef.current)}
        />
      ) : hasNoPhases ? (
        <NoPhaseState />
      ) : (
        <>
          <CareOfferingToolbar
            phases={phases}
            selectedPhaseId={selectedPhaseId}
            offeringCount={offerings.length}
            activeOfferingCount={activeOfferingCount}
            selectableDaysCount={selectableDaysCount}
            totalCapacity={totalCapacity}
            focusMode={Boolean(draft || cloneSource)}
            onPhaseChange={(phaseId) => {
              selectedPhaseIdRef.current = phaseId;
              void loadAll(phaseId);
            }}
            onCreate={beginCreate}
          />

          {cloneSource ? (
            <CloneOfferingForm
              source={cloneSource}
              phases={phases}
              selectedPhaseId={selectedPhaseId}
              saving={saving}
              onCancel={cancelFocusMode}
              onClone={handleClone}
            />
          ) : null}

          {draft ? (
            <CareOfferingForm
              draft={draft}
              editing={editingId !== "new"}
              editingId={editingId}
              phases={phases}
              offerings={offerings}
              templates={templates}
              periods={periods}
              metadataStatus={metadataStatus}
              originalActivityGroupID={
                editingId && editingId !== "new"
                  ? (offerings.find((offering) => offering.id === editingId)
                      ?.activity_group_id ?? null)
                  : null
              }
              gradeLevelMax={gradeLevelMax}
              bookingStats={bookingStats}
              saving={saving}
              onChange={setDraft}
              onSubmit={handleSave}
              onCancel={cancelFocusMode}
              onTemplateUnlinked={() =>
                toast.warning(CARE_OFFERING_TEMPLATE_UNLINKED_MESSAGE)
              }
            />
          ) : null}

          {selectedPhaseId &&
          offerings.length === 0 &&
          !draft &&
          !cloneSource ? (
            <EmptyCareOfferingState onCreate={beginCreate} />
          ) : offerings.length > 0 && !draft && !cloneSource ? (
            <DataTable
              columns={columns}
              rows={offerings}
              getRowKey={(offering) => offering.id}
              defaultSortKey="name"
              defaultSortDirection="asc"
            />
          ) : null}
        </>
      )}
      <OfferingPickupRolloutDialog
        state={pickupRollout}
        busy={rollingOut}
        onToggleSkip={(studentId, overwrite) =>
          setPickupRollout((current) => {
            if (!current) return current;
            const skipIds = new Set(current.skipIds);
            if (overwrite) {
              skipIds.delete(studentId);
            } else {
              skipIds.add(studentId);
            }
            return { ...current, skipIds };
          })
        }
        onConfirm={() => void confirmPickupRollout()}
        onCancel={() => {
          setPickupRollout(null);
          toast.info(
            "Gehzeit gespeichert, aber nicht auf die Kinder übertragen.",
          );
        }}
      />
    </div>
  );
}

// Bestätigungsdialog für das Ausrollen der Angebots-Gehzeit (#2290):
// Kinder mit abweichender manuell gepflegter Gehzeit sind gelistet und
// standardmäßig zum Überschreiben angehakt; einzelne lassen sich ausnehmen.
function OfferingPickupRolloutDialog({
  state,
  busy,
  onToggleSkip,
  onConfirm,
  onCancel,
}: Readonly<{
  state: {
    offeringName: string;
    preview: OfferingPickupRolloutPreview;
    skipIds: Set<string>;
  } | null;
  busy: boolean;
  onToggleSkip: (studentId: string, overwrite: boolean) => void;
  onConfirm: () => void;
  onCancel: () => void;
}>) {
  if (!state) return null;
  const { preview } = state;
  return (
    <Modal
      isOpen
      onClose={onCancel}
      title="Gehzeit auf Kinder übertragen"
      widthClass="mx-4 w-[calc(100%-2rem)] max-w-2xl"
    >
      <div className="space-y-4">
        <p className="text-sm text-gray-700">
          Die Gehzeit von „{state.offeringName}“ wird auf{" "}
          <strong>{preview.affected_students}</strong>{" "}
          {preview.affected_students === 1 ? "Kind" : "Kinder"} übertragen
          {preview.removed_rows > 0
            ? `; ${preview.removed_rows} nicht mehr passende Gehzeiten werden entfernt`
            : ""}
          .
        </p>
        {preview.conflicts.length > 0 ? (
          <div>
            <p className="text-sm font-medium text-gray-900">
              {preview.conflicts.length === 1
                ? "1 Kind hat eine abweichende, von Hand gepflegte Gehzeit:"
                : `${preview.conflicts.length} Kinder haben abweichende, von Hand gepflegte Gehzeiten:`}
            </p>
            <ul className="mt-2 max-h-64 space-y-1 overflow-y-auto">
              {preview.conflicts.map((conflict) => {
                const key = `${conflict.student_id}-${conflict.weekday}`;
                const overwrite = !state.skipIds.has(conflict.student_id);
                return (
                  <li key={key}>
                    <label className="flex cursor-pointer items-center gap-2 rounded-md border border-gray-200 px-3 py-2 text-sm">
                      <Checkbox
                        checked={overwrite}
                        onChange={(event) =>
                          onToggleSkip(
                            conflict.student_id,
                            event.target.checked,
                          )
                        }
                      />
                      <span className="flex-1 text-gray-900">
                        {conflict.student_name}
                      </span>
                      <span className="text-xs text-gray-500">
                        {ISO_WEEKDAY_LABELS[conflict.weekday] ??
                          `Tag ${conflict.weekday}`}{" "}
                        {conflict.current_time} → {conflict.new_time} Uhr
                      </span>
                    </label>
                  </li>
                );
              })}
            </ul>
            <p className="mt-2 text-xs text-gray-500">
              Angehakte Kinder werden überschrieben; abgehakte behalten ihre
              Gehzeit.
            </p>
          </div>
        ) : null}
        <div className="flex justify-end gap-2">
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={onCancel}
            disabled={busy}
          >
            Nicht übertragen
          </Button>
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={onConfirm}
            disabled={busy}
          >
            {busy ? "Überträgt…" : "Übertragen"}
          </Button>
        </div>
      </div>
    </Modal>
  );
}

function CareOfferingToolbar({
  phases,
  selectedPhaseId,
  offeringCount,
  activeOfferingCount,
  selectableDaysCount,
  totalCapacity,
  focusMode,
  onPhaseChange,
  onCreate,
}: Readonly<{
  phases: Phase[];
  selectedPhaseId: string;
  offeringCount: number;
  activeOfferingCount: number;
  selectableDaysCount: number;
  totalCapacity: number;
  focusMode: boolean;
  onPhaseChange: (phaseId: string) => void;
  onCreate: () => void;
}>) {
  return (
    <section className="moto-content-surface rounded-2xl border p-4 shadow-sm backdrop-blur-md">
      <div className="flex flex-col gap-4 lg:flex-row lg:flex-wrap lg:items-center lg:justify-between">
        <div className="grid gap-2 sm:grid-cols-4">
          <CareOfferingMetric
            icon={<MotoConceptIcon concept="carePlan" size={16} />}
            label="Angebote"
            value={offeringCount}
          />
          <CareOfferingMetric
            icon={<Check className="h-4 w-4" />}
            label="Aktiv"
            value={activeOfferingCount}
          />
          <CareOfferingMetric
            icon={<MotoConceptIcon concept="calendar" size={18} />}
            label="Tage"
            value={selectableDaysCount}
          />
          <CareOfferingMetric
            icon={<MotoConceptIcon concept="children" size={18} />}
            label="Kapazität"
            value={totalCapacity || "frei"}
          />
        </div>

        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-end">
          <label
            className="flex items-center gap-2 text-sm"
            htmlFor="care-offerings-phase"
          >
            <span className="font-medium text-gray-700">Anmeldephase</span>
            <CustomSelect
              id="care-offerings-phase"
              ariaLabel="Anmeldephase"
              value={selectedPhaseId}
              onChange={onPhaseChange}
              className="h-9 min-w-60"
              options={phases.map((phase) => ({
                value: phase.id,
                label: `${phase.name} (${KIND_LABELS[phase.kind]})${
                  !phase.is_active ? " (inaktiv)" : ""
                }`,
              }))}
            />
          </label>
          {!focusMode ? (
            <button
              type="button"
              onClick={onCreate}
              disabled={!selectedPhaseId}
              className="inline-flex h-9 shrink-0 items-center justify-center gap-2 rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
            >
              <Plus className="h-4 w-4" aria-hidden="true" />
              Neues Betreuungsangebot
            </button>
          ) : null}
        </div>
      </div>
    </section>
  );
}

function CareOfferingMetric({
  icon,
  label,
  value,
}: Readonly<{
  icon: React.ReactNode;
  label: string;
  value: number | string;
}>) {
  return (
    <div className="flex items-center gap-2 rounded-xl bg-gray-50 px-3 py-2">
      <span className="flex h-7 w-7 items-center justify-center rounded-lg bg-white text-gray-500 shadow-sm">
        {icon}
      </span>
      <span>
        <span className="block text-sm font-semibold text-gray-900">
          {value}
        </span>
        <span className="block text-xs text-gray-500">{label}</span>
      </span>
    </div>
  );
}

function PlannerMetadataNotice({ onRetry }: Readonly<{ onRetry: () => void }>) {
  return (
    <div
      className="border-moto-amber/50 bg-moto-amber/10 text-moto-amber-strong flex flex-col gap-3 rounded-2xl border p-4 text-sm sm:flex-row sm:items-center sm:justify-between"
      role="status"
    >
      <p>{PLANNER_METADATA_UNAVAILABLE_MESSAGE}</p>
      <button
        type="button"
        onClick={onRetry}
        className="border-moto-amber/60 text-moto-amber-strong hover:bg-moto-amber/10 focus-visible:ring-moto-amber inline-flex h-9 shrink-0 items-center justify-center rounded-lg border bg-white px-3 text-sm font-medium shadow-sm transition-colors focus-visible:ring-2 focus-visible:outline-none"
      >
        Erneut laden
      </button>
    </div>
  );
}

function CareOfferingCatalogError({
  message,
  onRetry,
}: Readonly<{ message: string; onRetry: () => void }>) {
  return (
    <section
      className="moto-content-surface border-moto-red/20 rounded-2xl border px-6 py-10 text-center shadow-sm"
      role="alert"
    >
      <h2 className="text-base font-semibold text-gray-900">
        Betreuungsangebote konnten nicht geladen werden
      </h2>
      <p className="text-moto-red-strong mx-auto mt-2 max-w-xl text-sm leading-6">
        {message}
      </p>
      <button
        type="button"
        onClick={onRetry}
        className="mt-5 inline-flex h-9 items-center justify-center rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
      >
        Betreuungsangebote erneut laden
      </button>
    </section>
  );
}

function NoPhaseState() {
  return (
    <section className="moto-content-surface rounded-2xl border px-6 py-12 text-center shadow-sm backdrop-blur-md">
      <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-2xl bg-gray-100">
        <MotoConceptIcon concept="calendarPeriods" size={28} />
      </div>
      <h2 className="mt-4 text-base font-semibold text-gray-900">
        Erst eine Anmeldephase anlegen
      </h2>
      <p className="mx-auto mt-2 max-w-md text-sm leading-6 text-gray-600">
        Betreuungsangebote gehören immer zu einer Anmeldephase. Lege zuerst den
        Zeitraum an, danach kannst du die passenden Angebote ergänzen.
      </p>
      <Link
        href="/enrollment-phases"
        className="mt-5 inline-flex h-9 items-center justify-center gap-2 rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
      >
        <MotoConceptIcon concept="calendarPeriods" size={18} />
        Anmeldephase anlegen
      </Link>
    </section>
  );
}

function EmptyCareOfferingState({
  onCreate,
}: Readonly<{ onCreate: () => void }>) {
  return (
    <section className="moto-content-surface rounded-2xl border px-6 py-12 text-center shadow-sm backdrop-blur-md">
      <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-2xl bg-gray-100">
        <MotoConceptIcon concept="carePlan" size={24} />
      </div>
      <h2 className="mt-4 text-base font-semibold text-gray-900">
        Noch kein Betreuungsangebot angelegt
      </h2>
      <p className="mx-auto mt-2 max-w-md text-sm leading-6 text-gray-600">
        Erstelle das erste Angebot, das Eltern auswählen können. Typisch sind
        Regelbetreuung, Ferienbetreuung oder ein Angebot mit Mittagessen.
      </p>
      <button
        type="button"
        onClick={onCreate}
        className="mt-5 inline-flex h-9 items-center justify-center gap-2 rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
      >
        <Plus className="h-4 w-4" aria-hidden="true" />
        Erstes Betreuungsangebot anlegen
      </button>
    </section>
  );
}

// Without this pill an availability rule is only visible after opening the
// offering and scrolling to "Bedingungen für die Verfügbarkeit" — which is how
// a school ends up filing a support ticket for a restriction it configured
// itself (#2186).
function AvailabilityRulePill({
  rule,
  gradeLevelMax,
}: Readonly<{
  rule: CareOfferingAvailabilityRule | null | undefined;
  // Passed so a rule covering every grade this school HAS reads as
  // unrestricted rather than as a restriction (#2186 review).
  gradeLevelMax: number | null;
}>) {
  const label = describeCareOfferingAvailabilityRule(
    rule,
    gradeLevelMax ?? undefined,
  );
  if (!label) return null;
  return <FeaturePill label={label} />;
}

function DaysCell({ offering }: Readonly<{ offering: CareOffering }>) {
  return (
    <div className="min-w-0">
      <p className="text-sm text-gray-700">
        {formatDays(offering.available_days)}
      </p>
      <p className="mt-0.5 text-xs text-gray-500">
        {offering.days_of_week_mode === "parent_choice"
          ? "Eltern wählen einzelne Tage"
          : "Fester Wochenrhythmus"}
      </p>
    </div>
  );
}

interface CareOfferingActionsProps {
  readonly offering: CareOffering;
  readonly saving: boolean;
  readonly deleting: boolean;
  readonly onEdit: () => void;
  readonly onClone: () => void;
  readonly onDelete: () => void;
}

function CareOfferingActions({
  offering,
  saving,
  deleting,
  onEdit,
  onClone,
  onDelete,
}: CareOfferingActionsProps) {
  return (
    <div className="flex justify-end">
      <OverflowMenu
        ariaLabel={`Aktionen für ${offering.name}`}
        items={[
          {
            label: "Bearbeiten",
            icon: <Pencil className="h-4 w-4" aria-hidden />,
            disabled: saving,
            onClick: onEdit,
          },
          {
            label: "Duplizieren",
            icon: <Copy className="h-4 w-4" aria-hidden />,
            disabled: saving,
            onClick: onClone,
          },
          { kind: "separator" },
          {
            label: deleting ? "Löscht..." : "Löschen",
            icon: <Trash2 className="h-4 w-4" aria-hidden />,
            destructive: true,
            disabled: deleting || saving,
            onClick: onDelete,
          },
        ]}
      />
    </div>
  );
}

interface CareOfferingFormProps {
  readonly draft: CareOfferingInput;
  readonly editing: boolean;
  readonly editingId: string | null;
  readonly phases: Phase[];
  readonly offerings: CareOffering[];
  readonly templates: TimetableTemplate[];
  readonly periods: CalendarPeriod[];
  readonly metadataStatus: PlannerMetadataStatus;
  readonly originalActivityGroupID: string | null;
  readonly gradeLevelMax: number | null;
  /**
   * Current bookings of the offering being edited, keyed by offering id.
   * Only used to warn about bookings a tightened availability rule would
   * contradict; absent for a new offering, which has none.
   */
  readonly bookingStats: Record<string, CareOfferingBookingStats>;
  readonly saving: boolean;
  readonly onChange: (draft: CareOfferingInput) => void;
  readonly onSubmit: (event: React.FormEvent) => void;
  readonly onCancel: () => void;
  readonly onTemplateUnlinked: () => void;
}

function phaseChangePatch({
  draft,
  phaseID,
  editingID,
  phases,
  offerings,
  templates,
  periods,
  metadataStatus,
}: Readonly<{
  draft: CareOfferingInput;
  phaseID: number;
  editingID: string | null;
  phases: Phase[];
  offerings: CareOffering[];
  templates: TimetableTemplate[];
  periods: CalendarPeriod[];
  metadataStatus: PlannerMetadataStatus;
}>): { patch: Partial<CareOfferingInput>; templateUnlinked: boolean } {
  const nextPhase = phases.find((phase) => phase.id === String(phaseID));
  const linkedTemplate = templates.find(
    (template) => template.id === String(draft.activity_group_id),
  );
  const compatibility =
    metadataStatus === "ready" && nextPhase && linkedTemplate
      ? templatePhaseCompatibility(
          linkedTemplate,
          nextPhase,
          periods,
          draft.is_active,
        )
      : "unknown";
  const templateUnlinked =
    draft.activity_group_id != null && compatibility === "incompatible";
  const validTriggerIDs = new Set(
    offerings
      .filter(
        (offering) =>
          offering.phase_id === String(phaseID) && offering.id !== editingID,
      )
      .map((offering) => offering.id),
  );

  return {
    patch: {
      phase_id: phaseID,
      auto_add_trigger_offering_ids: (
        draft.auto_add_trigger_offering_ids ?? []
      ).filter((id) => validTriggerIDs.has(id)),
      activity_group_id: templateUnlinked ? null : draft.activity_group_id,
    },
    templateUnlinked,
  };
}

function toggleSetValue<T>(values: Iterable<T>, value: T): Set<T> {
  const next = new Set(values);
  if (next.has(value)) next.delete(value);
  else next.add(value);
  return next;
}

function isSupportedAutoAddGrade(value: number): boolean {
  return isSupportedGradeLevelMax(value);
}

function careOfferingGradeOptions(
  gradeLevelMax: number | null,
  selectedGrades: readonly number[],
): number[] {
  const options = new Set(selectedGrades.filter(isSupportedAutoAddGrade));
  if (gradeLevelMax !== null) {
    for (let grade = 1; grade <= gradeLevelMax; grade++) options.add(grade);
  }
  return [...options].sort((a, b) => a - b);
}

function unknownCompatibilityMessage(
  metadataStatus: PlannerMetadataStatus,
): string {
  if (metadataStatus === "loading") {
    return "Die Regeltermin-Kompatibilität wird geladen. Die bestehende Verknüpfung bleibt unverändert.";
  }
  if (metadataStatus === "unavailable") {
    return "Die Regeltermin-Kompatibilität ist derzeit unbekannt. Die bestehende Verknüpfung bleibt erhalten und wird beim Speichern serverseitig geprüft.";
  }
  return "Die Planungsperiode des verknüpften Regeltermins ist nicht eindeutig. Die Verknüpfung bleibt erhalten und wird beim Speichern serverseitig geprüft.";
}

function CareOfferingTemplateField({
  draft,
  phases,
  templates,
  periods,
  metadataStatus,
  onChange,
}: Readonly<{
  draft: CareOfferingInput;
  phases: Phase[];
  templates: TimetableTemplate[];
  periods: CalendarPeriod[];
  metadataStatus: PlannerMetadataStatus;
  onChange: (patch: Partial<CareOfferingInput>) => void;
}>) {
  const selectedPhase = phases.find(
    (phase) => phase.id === String(draft.phase_id),
  );
  const compatibleTemplates =
    metadataStatus === "ready" && selectedPhase
      ? templates.filter(
          (template) =>
            templatePhaseCompatibility(
              template,
              selectedPhase,
              periods,
              draft.is_active,
            ) === "compatible",
        )
      : [];
  const linkedTemplateID = draft.activity_group_id?.toString() ?? "";
  const linkedTemplate = templates.find(
    (template) => template.id === linkedTemplateID,
  );
  const compatibility = draftTemplateCompatibility(
    draft,
    phases,
    templates,
    periods,
    metadataStatus,
  );
  const periodMismatch = compatibility === "incompatible";
  const inactivePeriodMismatch = draftUsesInactiveTemplatePeriod(
    draft,
    templates,
    periods,
    metadataStatus,
  );
  const compatibilityUnknown =
    linkedTemplateID !== "" && compatibility === "unknown";
  // When metadata is ready, an unresolved current link is actionable: keep
  // that invalid option disabled, but let the user choose "Keine Zuordnung"
  // or a compatible replacement. Disabling the whole control would leave the
  // user stuck because the server correctly rejects the unresolved link.
  const selectorDisabled =
    metadataStatus !== "ready" && linkedTemplateID === "";
  const templateWarnings =
    metadataStatus === "ready" ? linkedTemplateWarnings(draft, templates) : [];
  const weekdayError = linkedTemplateWeekdayError(
    draft,
    templates,
    metadataStatus,
  );
  const options: Array<{
    value: string;
    label: string;
    disabled?: boolean;
  }> = [
    {
      value: "",
      label: "Keine automatische Regeltermin-Zuordnung",
    },
  ];

  if (periodMismatch || compatibilityUnknown) {
    const name = linkedTemplate
      ? templateLabel(linkedTemplate)
      : `Regeltermin #${linkedTemplateID}`;
    options.push({
      value: linkedTemplateID,
      label: `${name} (${periodMismatch ? "nicht kompatibel" : "Kompatibilität unbekannt"})`,
      disabled: true,
    });
  }
  options.push(
    ...compatibleTemplates.map((template) => ({
      value: String(template.id),
      label: templateLabel(template),
    })),
  );

  return (
    <section className="rounded-xl border border-gray-200 bg-white/70 p-4">
      <label className="block" htmlFor="care-offering-template">
        <span className="text-xs font-medium text-gray-700">Regeltermin</span>
        <CustomSelect
          id="care-offering-template"
          ariaLabel="Regeltermin"
          value={linkedTemplateID}
          onChange={(value) =>
            onChange({ activity_group_id: value ? Number(value) : null })
          }
          disabled={selectorDisabled}
          className="mt-1 border-gray-200 bg-white"
          invalid={periodMismatch || weekdayError !== null}
          options={options}
        />
      </label>
      <p className="mt-2 text-xs text-gray-600">
        Genehmigte Anmeldungen werden in diesen Regeltermin übernommen und an
        den ausgewählten Angebotstagen erwartet.
      </p>
      {periodMismatch ? (
        <p
          className="border-moto-red/30 bg-moto-red/10 text-moto-red-strong mt-2 rounded-lg border px-3 py-2 text-xs"
          role="alert"
        >
          {inactivePeriodMismatch
            ? INACTIVE_TEMPLATE_PERIOD_MESSAGE
            : CARE_OFFERING_TEMPLATE_PERIOD_MISMATCH_MESSAGE}
        </p>
      ) : null}
      {compatibilityUnknown ? (
        <p className="border-moto-amber/50 bg-moto-amber/10 text-moto-amber-strong mt-2 rounded-lg border px-3 py-2 text-xs">
          {unknownCompatibilityMessage(metadataStatus)}
        </p>
      ) : null}
      {weekdayError ? (
        <p
          className="border-moto-red/30 bg-moto-red/10 text-moto-red-strong mt-2 rounded-lg border px-3 py-2 text-xs"
          role="alert"
        >
          {weekdayError}
        </p>
      ) : null}
      {metadataStatus === "ready" &&
      !periodMismatch &&
      !compatibilityUnknown &&
      compatibleTemplates.length === 0 ? (
        <p className="mt-2 text-xs text-gray-500">
          Kein Regeltermin deckt den gesamten Betreuungszeitraum dieser Phase
          ab.
        </p>
      ) : null}
      {templateWarnings.length > 0 ? (
        <ul className="border-moto-amber/50 bg-moto-amber/10 text-moto-amber-strong mt-2 space-y-1 rounded-lg border px-3 py-2 text-xs">
          {templateWarnings.map((warning) => (
            <li key={warning}>{warning}</li>
          ))}
        </ul>
      ) : null}
    </section>
  );
}

function CareOfferingWeekdayFields({
  draft,
  onChange,
}: Readonly<{
  draft: CareOfferingInput;
  onChange: (patch: Partial<CareOfferingInput>) => void;
}>) {
  const toggleDay = (day: string) => {
    const nextDays = toggleSetValue(draft.available_days, day);
    const patch: Partial<CareOfferingInput> = {
      available_days: WEEKDAY_KEYS.filter((dayKey) => nextDays.has(dayKey)),
    };
    // Ein abgewählter Tag nimmt seine Gehzeit mit.
    if (!nextDays.has(day) && draft.pickup_times?.[day]) {
      const nextTimes = { ...draft.pickup_times };
      delete nextTimes[day];
      patch.pickup_times = nextTimes;
    }
    onChange(patch);
  };
  const setPickupTime = (day: string, value: string) => {
    const nextTimes = { ...(draft.pickup_times ?? {}) };
    if (value) {
      nextTimes[day] = value;
    } else {
      delete nextTimes[day];
    }
    onChange({ pickup_times: nextTimes });
  };
  const nameMismatch = nameWeekdayMismatchWarning(draft);

  return (
    <fieldset className="rounded-xl border border-gray-200 p-4">
      <legend className="px-1 text-xs font-medium text-gray-700">
        Wochentage
      </legend>
      <div className="flex flex-wrap gap-2">
        {WEEKDAY_KEYS.map((day) => {
          const active = draft.available_days.includes(day);
          return (
            <button
              key={day}
              type="button"
              aria-pressed={active}
              onClick={() => toggleDay(day)}
              className={`h-9 rounded-lg border px-3 text-xs font-medium transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
                active
                  ? "border-gray-900 bg-gray-900 text-white"
                  : "border-gray-200 bg-white text-gray-700 hover:bg-gray-50"
              }`}
            >
              {DAY_LABELS[day]}
            </button>
          );
        })}
      </div>
      {nameMismatch ? (
        <p className="border-moto-amber/50 bg-moto-amber/10 text-moto-amber-strong mt-3 rounded-lg border px-3 py-2 text-xs">
          {nameMismatch}
        </p>
      ) : null}
      {draft.available_days.length > 0 ? (
        <div className="mt-4">
          <p className="text-xs font-medium text-gray-700">
            Gehzeit je Wochentag (optional)
          </p>
          <p className="mt-1 text-xs text-gray-500">
            Wird beim Speichern auf alle Kinder mit diesem Angebot übertragen
            und erscheint in Klassenliste und Kinderdetails.
          </p>
          <div className="mt-2 flex flex-wrap gap-3">
            {draft.available_days.map((day) => (
              <label key={day} className="flex items-center gap-2 text-xs">
                <span className="w-6 text-gray-700">{DAY_LABELS[day]}</span>
                <input
                  type="time"
                  value={draft.pickup_times?.[day] ?? ""}
                  onChange={(event) => setPickupTime(day, event.target.value)}
                  className="rounded-md border border-gray-200 px-2 py-1.5 text-xs focus:border-gray-400 focus:outline-none"
                />
              </label>
            ))}
          </div>
        </div>
      ) : null}
      <div className="mt-3">
        <CareOfferingCheckbox
          checked={draft.days_of_week_mode === "parent_choice"}
          onChange={(checked) =>
            onChange({
              days_of_week_mode: checked
                ? "parent_choice"
                : ("fixed" as DaysOfWeekMode),
            })
          }
          label="Eltern können einzelne Tage auswählen"
          hint="Sonst gilt das Angebot für den gesamten gewählten Rhythmus."
        />
      </div>
    </fieldset>
  );
}

function CareOfferingAutomationFields({
  draft,
  offerings,
  editingID,
  gradeLevelMax,
  onChange,
}: Readonly<{
  draft: CareOfferingInput;
  offerings: CareOffering[];
  editingID: string | null;
  gradeLevelMax: number | null;
  onChange: (patch: Partial<CareOfferingInput>) => void;
}>) {
  const triggerOptions = offerings.filter(
    (offering) =>
      offering.phase_id === String(draft.phase_id) && offering.id !== editingID,
  );
  const toggleTriggerOffering = (offeringID: string) => {
    const next = toggleSetValue(
      draft.auto_add_trigger_offering_ids ?? [],
      offeringID,
    );
    onChange({
      auto_add_trigger_offering_ids: triggerOptions
        .map((offering) => offering.id)
        .filter((id) => next.has(id)),
    });
  };
  const selectedGrades = (draft.auto_add_grade_levels ?? [])
    .filter(isSupportedAutoAddGrade)
    .sort((a, b) => a - b);
  const gradeOptions = careOfferingGradeOptions(gradeLevelMax, selectedGrades);
  const toggleAutoAddGrade = (grade: number) => {
    const next = toggleSetValue(selectedGrades, grade);
    onChange({
      auto_add_grade_levels: [...next]
        .filter(isSupportedAutoAddGrade)
        .sort((a, b) => a - b),
    });
  };

  return (
    <fieldset className="rounded-xl border border-gray-200 p-4">
      <legend className="px-1 text-xs font-medium text-gray-700">
        Betreuungstage & Mitbuchung
      </legend>
      <div className="space-y-4">
        <CareOfferingCheckbox
          checked={draft.counts_as_care}
          onChange={(checked) => onChange({ counts_as_care: checked })}
          label="Als Betreuungstage zählen"
          hint="Gilt für Filter, Kennzahlen und Exporte."
        />

        <div className="rounded-lg border border-gray-100 bg-gray-50/70 p-3">
          <p className="text-xs font-medium text-gray-700">
            Dieses Angebot mitbuchen, wenn Eltern eines dieser Angebote wählen:
          </p>
          {triggerOptions.length > 0 ? (
            <div className="mt-2 grid gap-2 sm:grid-cols-2">
              {triggerOptions.map((offering) => (
                <CareOfferingCheckbox
                  key={offering.id}
                  checked={(draft.auto_add_trigger_offering_ids ?? []).includes(
                    offering.id,
                  )}
                  onChange={() => toggleTriggerOffering(offering.id)}
                  label={offering.name}
                  hint={formatDays(offering.available_days)}
                />
              ))}
            </div>
          ) : (
            <p className="mt-2 text-xs text-gray-500">
              In dieser Phase gibt es noch kein anderes Angebot als Auslöser.
            </p>
          )}
          {draft.auto_add_trigger_offering_ids.length > 0 &&
          draft.days_of_week_mode !== "parent_choice" ? (
            <p className="border-moto-amber/50 bg-moto-amber/10 text-moto-amber-strong mt-2 rounded-lg border px-3 py-2 text-xs">
              Mitgebuchte Angebote müssen einzelne Tage auswählbar machen.
            </p>
          ) : null}
        </div>

        <div>
          <p className="text-xs font-medium text-gray-700">
            Mitbuchung gilt für Klassenstufen
          </p>
          <div className="mt-2 flex flex-wrap gap-2">
            {gradeOptions.map((grade) => {
              const active = selectedGrades.includes(grade);
              const aboveCurrentCap =
                gradeLevelMax !== null && grade > gradeLevelMax;
              return (
                <button
                  key={grade}
                  type="button"
                  aria-pressed={active}
                  onClick={() => toggleAutoAddGrade(grade)}
                  className={`h-8 rounded-lg border px-3 text-xs font-medium transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
                    active
                      ? "border-gray-900 bg-gray-900 text-white"
                      : "border-gray-200 bg-white text-gray-700 hover:bg-gray-50"
                  }`}
                >
                  Klasse {grade}
                  {aboveCurrentCap ? " (bestehend)" : ""}
                </button>
              );
            })}
          </div>
          <p className="mt-1 text-xs text-gray-500">
            Keine Auswahl bedeutet: Die Mitbuchung gilt für alle Klassenstufen.
          </p>
        </div>
      </div>
    </fieldset>
  );
}

function CareOfferingAvailabilityFields({
  draft,
  gradeLevelMax,
  bookingGradeCounts,
  onChange,
}: Readonly<{
  draft: CareOfferingInput;
  gradeLevelMax: number | null;
  bookingGradeCounts: CareOfferingBookingGradeCounts | null;
  onChange: (patch: Partial<CareOfferingInput>) => void;
}>) {
  const rule = draft.availability_rule;
  const max = gradeLevelMax ?? 0;
  const gradeOptions = Array.from({ length: max }, (_, index) => index + 1);
  const updateCondition = (
    index: number,
    patch: Partial<CareOfferingAvailabilityCondition>,
  ) => {
    if (!rule) return;
    onChange({
      availability_rule: {
        ...rule,
        conditions: rule.conditions.map((condition, conditionIndex) =>
          conditionIndex === index
            ? copyStableObjectKey(condition, { ...condition, ...patch })
            : condition,
        ),
      },
    });
  };
  const moveCondition = (index: number, offset: -1 | 1) => {
    if (!rule) return;
    const target = index + offset;
    if (target < 0 || target >= rule.conditions.length) return;
    const conditions = [...rule.conditions];
    [conditions[index], conditions[target]] = [
      conditions[target]!,
      conditions[index]!,
    ];
    onChange({ availability_rule: { ...rule, conditions } });
  };
  const error = careOfferingAvailabilityRuleError(rule, gradeLevelMax);
  // Bestandsschutz is intentional — an existing booking is never revoked by a
  // rule change. Saying so out loud is what keeps the data from looking
  // inconsistent later (#2186).
  const conflicts = countCareOfferingRuleConflicts(rule, bookingGradeCounts);

  return (
    <fieldset className="rounded-xl border border-gray-200 p-4">
      <legend className="px-1 text-xs font-medium text-gray-700">
        Bedingungen für die Verfügbarkeit
      </legend>
      <CareOfferingCheckbox
        checked={rule !== null}
        onChange={(checked) =>
          onChange({
            availability_rule: checked
              ? {
                  match: "all",
                  conditions: [
                    { source: "grade_level", operator: "in", value: [] },
                  ],
                }
              : null,
          })
        }
        label={
          rule
            ? "Nur unter Bedingungen anbieten"
            : "Für alle Klassenstufen / ohne Bedingungen"
        }
        hint="Ohne Bedingungen ist dieses Angebot für jedes Kind verfügbar."
      />
      {rule ? (
        <div className="mt-4 space-y-3">
          {rule.conditions.length > 1 ? (
            <label className="block" htmlFor="care-offering-availability-match">
              <span className="text-xs font-medium text-gray-700">
                Mehrere Bedingungen
              </span>
              <CustomSelect
                id="care-offering-availability-match"
                ariaLabel="Mehrere Bedingungen"
                value={rule.match}
                onChange={(value) =>
                  onChange({
                    availability_rule: {
                      ...rule,
                      match: value as "all" | "any",
                    },
                  })
                }
                className="mt-1"
                options={[
                  {
                    value: "all",
                    label: "Alle Bedingungen müssen erfüllt sein",
                  },
                  {
                    value: "any",
                    label: "Mindestens eine Bedingung muss erfüllt sein",
                  },
                ]}
              />
            </label>
          ) : null}
          {rule.conditions.map((condition, index) => (
            <div
              key={getStableObjectKey(condition, "availability-condition")}
              className="rounded-lg border border-gray-200 bg-gray-50/70 p-3"
            >
              <div className="flex items-center justify-between gap-2">
                <p className="text-xs font-semibold text-gray-800">
                  Bedingung {index + 1}
                </p>
                <div className="flex gap-1">
                  <button
                    type="button"
                    disabled={index === 0}
                    onClick={() => moveCondition(index, -1)}
                    className="rounded border border-gray-200 bg-white px-2 py-1 text-xs disabled:opacity-40"
                    aria-label={`Bedingung ${index + 1} nach oben`}
                  >
                    ↑
                  </button>
                  <button
                    type="button"
                    disabled={index === rule.conditions.length - 1}
                    onClick={() => moveCondition(index, 1)}
                    className="rounded border border-gray-200 bg-white px-2 py-1 text-xs disabled:opacity-40"
                    aria-label={`Bedingung ${index + 1} nach unten`}
                  >
                    ↓
                  </button>
                  <button
                    type="button"
                    onClick={() =>
                      onChange({
                        availability_rule: {
                          ...rule,
                          conditions: rule.conditions.filter(
                            (_, i) => i !== index,
                          ),
                        },
                      })
                    }
                    className="border-moto-red/30 text-moto-red-strong rounded border bg-white px-2 py-1 text-xs"
                    aria-label={`Bedingung ${index + 1} löschen`}
                  >
                    Löschen
                  </button>
                </div>
              </div>
              <div className="mt-3 grid gap-3 sm:grid-cols-2">
                <label
                  className="block"
                  htmlFor={`availability-source-${index}`}
                >
                  <span className="text-xs font-medium text-gray-700">
                    Bedingungsquelle
                  </span>
                  <CustomSelect
                    id={`availability-source-${index}`}
                    ariaLabel="Bedingungsquelle"
                    value="grade_level"
                    onChange={() => undefined}
                    options={[
                      {
                        value: "grade_level",
                        label: "Klassenstufe des Kindes",
                      },
                    ]}
                    className="mt-1"
                  />
                </label>
                <label
                  className="block"
                  htmlFor={`availability-operator-${index}`}
                >
                  <span className="text-xs font-medium text-gray-700">
                    Operator
                  </span>
                  <CustomSelect
                    id={`availability-operator-${index}`}
                    ariaLabel="Operator"
                    value={condition.operator}
                    onChange={(value) =>
                      updateCondition(index, {
                        operator: value as "in" | "not_in",
                      })
                    }
                    options={[
                      { value: "in", label: "ist eine von" },
                      { value: "not_in", label: "ist keine von" },
                    ]}
                    className="mt-1"
                  />
                </label>
              </div>
              <div className="mt-3">
                <p className="text-xs font-medium text-gray-700">
                  Klassenstufen
                </p>
                <div className="mt-2 flex flex-wrap gap-2">
                  {gradeOptions.map((grade) => {
                    const active = condition.value.includes(grade);
                    return (
                      <button
                        key={grade}
                        type="button"
                        aria-pressed={active}
                        onClick={() =>
                          updateCondition(index, {
                            value: [
                              ...toggleSetValue(condition.value, grade),
                            ].sort((a, b) => a - b),
                          })
                        }
                        className={`h-8 rounded-lg border px-3 text-xs font-medium ${active ? "border-gray-900 bg-gray-900 text-white" : "border-gray-200 bg-white text-gray-700"}`}
                      >
                        Klasse {grade}
                      </button>
                    );
                  })}
                </div>
              </div>
            </div>
          ))}
          <button
            type="button"
            onClick={() =>
              onChange({
                availability_rule: {
                  ...rule,
                  conditions: [
                    ...rule.conditions,
                    { source: "grade_level", operator: "in", value: [] },
                  ],
                },
              })
            }
            className="inline-flex h-9 items-center gap-2 rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700"
          >
            <Plus className="h-4 w-4" />
            Bedingung hinzufügen
          </button>
          {error ? (
            <p
              role="alert"
              className="border-moto-red/30 bg-moto-red/10 text-moto-red-strong rounded-lg border px-3 py-2 text-xs"
            >
              {error}
            </p>
          ) : null}
          {!error && conflicts > 0 ? (
            <p
              role="status"
              className="border-moto-amber/40 bg-moto-amber/10 text-moto-amber-strong rounded-lg border px-3 py-2 text-xs leading-5"
            >
              {conflicts === 1
                ? "1 bestehende Buchung erfüllt diese Bedingung nicht."
                : `${conflicts} bestehende Buchungen erfüllen diese Bedingung nicht.`}{" "}
              Sie bleiben bestehen — die Regel gilt nur für neue Auswahlen.
            </p>
          ) : null}
        </div>
      ) : null}
    </fieldset>
  );
}

function nullableNumber(value: string): number | null {
  return value === "" ? null : Number(value);
}

function CareOfferingCommercialFields({
  draft,
  onChange,
}: Readonly<{
  draft: CareOfferingInput;
  onChange: (patch: Partial<CareOfferingInput>) => void;
}>) {
  return (
    <div className="grid gap-4 sm:grid-cols-3">
      <label className="block">
        <span className="text-xs font-medium text-gray-700">Kapazität</span>
        <input
          type="number"
          name="capacity"
          min={0}
          value={draft.is_required ? "" : (draft.capacity ?? "")}
          disabled={draft.is_required}
          onChange={(event) =>
            onChange({ capacity: nullableNumber(event.target.value) })
          }
          placeholder={
            draft.is_required ? "Unbegrenzt (Pflicht)" : "Unbegrenzt"
          }
          className="mt-1 h-10 w-full rounded-lg border border-gray-200 px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:bg-gray-50 disabled:text-gray-400"
        />
        {draft.is_required ? (
          <span className="mt-1 block text-xs text-gray-500">
            Pflichtangebote haben keine Platzbegrenzung.
          </span>
        ) : null}
      </label>
      <label className="block">
        <span className="text-xs font-medium text-gray-700">Preis in Cent</span>
        <input
          type="number"
          name="price_cents"
          min={0}
          value={draft.price_cents ?? ""}
          onChange={(event) =>
            onChange({ price_cents: nullableNumber(event.target.value) })
          }
          placeholder="Optional"
          className="mt-1 h-10 w-full rounded-lg border border-gray-200 px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
        />
      </label>
      <label className="block">
        <span className="text-xs font-medium text-gray-700">Reihenfolge</span>
        <input
          type="number"
          name="sort_order"
          value={draft.sort_order}
          onChange={(event) =>
            onChange({ sort_order: Number(event.target.value) })
          }
          className="mt-1 h-10 w-full rounded-lg border border-gray-200 px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
        />
        <span className="mt-1 block text-xs text-gray-500">
          Kleinere Zahlen erscheinen im Elternformular weiter oben.
        </span>
      </label>
    </div>
  );
}

function CareOfferingDisplayFields({
  draft,
  onChange,
}: Readonly<{
  draft: CareOfferingInput;
  onChange: (patch: Partial<CareOfferingInput>) => void;
}>) {
  return (
    <fieldset className="rounded-xl border border-gray-200 p-4">
      <legend className="px-1 text-xs font-medium text-gray-700">
        Anzeige im Elternformular
      </legend>
      <div className="grid gap-2 sm:grid-cols-3">
        <CareOfferingCheckbox
          checked={draft.includes_holiday_care}
          onChange={(checked) => onChange({ includes_holiday_care: checked })}
          label="Ferienbetreuung"
          hint="Wird als Ferienangebot gekennzeichnet"
        />
        <CareOfferingCheckbox
          checked={draft.includes_lunch}
          onChange={(checked) => onChange({ includes_lunch: checked })}
          label="Mittagessen"
          hint="Zeigt Eltern, dass Essen enthalten ist"
        />
        <CareOfferingCheckbox
          checked={draft.is_active}
          onChange={(checked) => onChange({ is_active: checked })}
          label="Aktiv"
          hint="Nur aktive Angebote sind auswählbar"
        />
        <CareOfferingCheckbox
          checked={draft.is_required}
          onChange={(checked) =>
            onChange(
              checked
                ? { is_required: true, capacity: null }
                : { is_required: false },
            )
          }
          label="Pflicht"
          hint="Jedes Kind muss dieses Angebot wählen (ohne Platzbegrenzung)"
        />
      </div>
    </fieldset>
  );
}

function submitLabel(saving: boolean, editing: boolean): string {
  if (saving) return "Speichert...";
  return editing ? "Speichern" : "Erstellen";
}

function CareOfferingFormActions({
  saving,
  editing,
  submitDisabled,
  onCancel,
}: Readonly<{
  saving: boolean;
  editing: boolean;
  submitDisabled: boolean;
  onCancel: () => void;
}>) {
  return (
    <div className="flex justify-end gap-2">
      <button
        type="button"
        onClick={onCancel}
        disabled={saving}
        className="inline-flex h-9 items-center justify-center rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50"
      >
        Abbrechen
      </button>
      <button
        type="submit"
        disabled={submitDisabled}
        className="inline-flex h-9 items-center justify-center rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
      >
        {submitLabel(saving, editing)}
      </button>
    </div>
  );
}

function CareOfferingForm({
  draft,
  editing,
  editingId,
  phases,
  offerings,
  templates,
  periods,
  metadataStatus,
  originalActivityGroupID,
  gradeLevelMax,
  bookingStats,
  saving,
  onChange,
  onSubmit,
  onCancel,
  onTemplateUnlinked,
}: CareOfferingFormProps) {
  const update = (patch: Partial<CareOfferingInput>) =>
    onChange({ ...draft, ...patch });
  const linkedTemplatePeriodMismatch =
    draftTemplateCompatibility(
      draft,
      phases,
      templates,
      periods,
      metadataStatus,
    ) === "incompatible";
  const linkedTemplateWeekdayMismatch =
    linkedTemplateWeekdayError(draft, templates, metadataStatus) !== null;
  const unverifiableTemplateChange = hasUnverifiableTemplateChange(
    draft,
    originalActivityGroupID,
    metadataStatus,
  );

  return (
    <form
      onSubmit={onSubmit}
      noValidate
      className="moto-content-surface space-y-5 rounded-2xl border p-6 shadow-sm backdrop-blur-md"
    >
      <header className="border-b border-gray-100 pb-4">
        <p className="text-moto-blue text-xs font-semibold tracking-wide uppercase">
          Betreuungsangebot
        </p>
        <h2 className="mt-1 text-base font-semibold text-gray-900">
          {editing
            ? "Betreuungsangebot bearbeiten"
            : "Neues Betreuungsangebot anlegen"}
        </h2>
        <p className="mt-1 max-w-2xl text-sm text-gray-600">
          Lege fest, was Eltern auswählen können, welche Tage möglich sind und
          ob Kapazität oder Zusatzleistungen angezeigt werden.
        </p>
      </header>

      <div className="grid gap-4 sm:grid-cols-2">
        <label className="block">
          <span className="text-xs font-medium text-gray-700">Name</span>
          <input
            type="text"
            name="name"
            value={draft.name}
            onChange={(event) => update({ name: event.target.value })}
            placeholder="z. B. Regelbetreuung"
            aria-required="true"
            className="mt-1 h-10 w-full rounded-lg border border-gray-200 px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          />
        </label>
        <label className="block" htmlFor="care-offering-form-phase">
          <span className="text-xs font-medium text-gray-700">
            Anmeldephase
          </span>
          <CustomSelect
            id="care-offering-form-phase"
            ariaLabel="Anmeldephase"
            value={draft.phase_id?.toString() ?? ""}
            onChange={(value) => {
              const phaseID = value ? Number(value) : 0;
              const result = phaseChangePatch({
                draft,
                phaseID,
                editingID: editingId,
                phases,
                offerings,
                templates,
                periods,
                metadataStatus,
              });
              update(result.patch);
              if (result.templateUnlinked) onTemplateUnlinked();
            }}
            className="mt-1"
            options={[
              { value: "", label: "Bitte wählen" },
              ...phases.map((phase) => ({
                value: phase.id,
                label: phase.name,
              })),
            ]}
          />
        </label>
      </div>

      <CareOfferingTemplateField
        draft={draft}
        phases={phases}
        templates={templates}
        periods={periods}
        metadataStatus={metadataStatus}
        onChange={update}
      />

      {unverifiableTemplateChange ? (
        <p
          className="border-moto-red/30 bg-moto-red/10 text-moto-red-strong rounded-lg border px-3 py-2 text-xs"
          role="alert"
        >
          {UNVERIFIABLE_TEMPLATE_CHANGE_MESSAGE}
        </p>
      ) : null}

      <label className="block">
        <span className="text-xs font-medium text-gray-700">Beschreibung</span>
        <textarea
          name="description"
          value={draft.description ?? ""}
          onChange={(event) => update({ description: event.target.value })}
          rows={3}
          placeholder="Kurz erklären, was Eltern hier auswählen."
          className="mt-1 w-full rounded-lg border border-gray-200 px-3 py-2 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
        />
      </label>

      <fieldset className="rounded-xl border border-gray-200 p-4">
        <legend className="px-1 text-xs font-medium text-gray-700">
          Pflichtauswahl
        </legend>
        <p className="mb-3 text-xs leading-5 text-gray-500">
          Angebote mit demselben Gruppennamen werden bei der Anmeldung gemeinsam
          geprüft. Verwende für eine Gruppe überall dieselbe Regel.
        </p>
        <div className="grid gap-3 sm:grid-cols-2">
          <label className="block">
            <span className="text-xs font-medium text-gray-700">
              Gruppe (optional)
            </span>
            <input
              type="text"
              name="selection_group"
              value={draft.selection_group ?? ""}
              onChange={(event) => {
                const group = event.target.value;
                // Clearing the group resets the rule — the backend rejects
                // a non-optional rule without a group name.
                update(
                  group.trim() === ""
                    ? { selection_group: group, selection_rule: "optional" }
                    : { selection_group: group },
                );
              }}
              placeholder="z. B. Betreuungsumfang"
              className="mt-1 h-10 w-full rounded-lg border border-gray-200 px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
            />
          </label>
          <label className="block" htmlFor="care-offering-selection-rule">
            <span className="text-xs font-medium text-gray-700">Regel</span>
            <CustomSelect
              id="care-offering-selection-rule"
              ariaLabel="Regel"
              value={draft.selection_rule ?? "optional"}
              onChange={(value) =>
                update({
                  selection_rule: value as CareSelectionRule,
                })
              }
              disabled={!draft.selection_group?.trim()}
              className="mt-1"
              options={(
                Object.keys(SELECTION_RULE_LABELS) as CareSelectionRule[]
              ).map((rule) => ({
                value: rule,
                label: SELECTION_RULE_LABELS[rule],
              }))}
            />
          </label>
        </div>
      </fieldset>

      <CareOfferingWeekdayFields draft={draft} onChange={update} />

      <CareOfferingAvailabilityFields
        draft={draft}
        gradeLevelMax={gradeLevelMax}
        bookingGradeCounts={
          editingId && editingId !== "new"
            ? (bookingStats[editingId] ?? null)
            : null
        }
        onChange={update}
      />

      <CareOfferingAutomationFields
        draft={draft}
        offerings={offerings}
        editingID={editingId}
        gradeLevelMax={gradeLevelMax}
        onChange={update}
      />

      <CareOfferingCommercialFields draft={draft} onChange={update} />

      <CareOfferingDisplayFields draft={draft} onChange={update} />

      <CareOfferingFormActions
        saving={saving}
        editing={editing}
        submitDisabled={
          saving ||
          !draft.phase_id ||
          !draft.name.trim() ||
          linkedTemplatePeriodMismatch ||
          linkedTemplateWeekdayMismatch ||
          unverifiableTemplateChange ||
          careOfferingAvailabilityRuleError(
            draft.availability_rule,
            gradeLevelMax,
          ) !== null
        }
        onCancel={onCancel}
      />
    </form>
  );
}

function CareOfferingCheckbox({
  checked,
  onChange,
  label,
  hint,
}: Readonly<{
  checked: boolean;
  onChange: (checked: boolean) => void;
  label: string;
  hint?: string;
}>) {
  return (
    <label
      className={`flex cursor-pointer items-start gap-3 rounded-lg border p-3 transition-colors ${
        checked
          ? "border-moto-green/40 bg-moto-green/10"
          : "border-gray-200 bg-white hover:bg-gray-50"
      }`}
    >
      <input
        type="checkbox"
        checked={checked}
        onChange={(event) => onChange(event.target.checked)}
        className="sr-only"
      />
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
        {hint ? (
          <span className="mt-0.5 block text-xs text-gray-500">{hint}</span>
        ) : null}
      </span>
    </label>
  );
}

function CloneOfferingForm({
  source,
  phases,
  selectedPhaseId,
  saving,
  onCancel,
  onClone,
}: Readonly<{
  source: CareOffering;
  phases: Phase[];
  selectedPhaseId: string;
  saving: boolean;
  onCancel: () => void;
  onClone: (phaseId: string) => Promise<void>;
}>) {
  const [targetPhaseId, setTargetPhaseId] = useState(selectedPhaseId);

  return (
    <form
      onSubmit={(event) => {
        event.preventDefault();
        void onClone(targetPhaseId);
      }}
      className="moto-content-surface space-y-4 rounded-2xl border p-6 shadow-sm backdrop-blur-md"
    >
      <header className="border-b border-gray-100 pb-4">
        <p className="text-moto-blue text-xs font-semibold tracking-wide uppercase">
          Duplizieren
        </p>
        <h2 className="mt-1 text-base font-semibold text-gray-900">
          „{source.name}" in eine Anmeldephase übernehmen
        </h2>
        <p className="mt-1 text-sm text-gray-600">
          Wähle die Zielphase. Danach entsteht dort ein eigenes
          Betreuungsangebot, das du separat bearbeiten kannst.
        </p>
      </header>

      <label className="block" htmlFor="care-offering-clone-target-phase">
        <span className="text-xs font-medium text-gray-700">Zielphase</span>
        <CustomSelect
          id="care-offering-clone-target-phase"
          ariaLabel="Zielphase"
          value={targetPhaseId}
          onChange={setTargetPhaseId}
          className="mt-1"
          options={phases.map((phase) => ({
            value: phase.id,
            label: `${phase.name} (${KIND_LABELS[phase.kind]})`,
          }))}
        />
      </label>

      <div className="flex justify-end gap-2">
        <button
          type="button"
          onClick={onCancel}
          disabled={saving}
          className="inline-flex h-9 items-center justify-center rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50"
        >
          Abbrechen
        </button>
        <button
          type="submit"
          disabled={saving || !targetPhaseId}
          className="inline-flex h-9 items-center justify-center rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
        >
          {saving ? "Dupliziert..." : "Duplizieren"}
        </button>
      </div>
    </form>
  );
}

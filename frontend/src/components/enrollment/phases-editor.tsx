"use client";

import { useSearchParams } from "next/navigation";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  OverflowMenu,
  type OverflowMenuEntry,
} from "~/components/ui/page-header/OverflowMenu";
import {
  CalendarClock,
  CalendarPlus,
  CalendarRange,
  Check,
  ClipboardList,
  Clock3,
  ExternalLink,
  FileText,
  Link2,
  Pencil,
  Power,
  Trash2,
  UserCheck,
  X,
} from "lucide-react";
import {
  type Phase,
  type PhaseAudience,
  type PhaseDeleteImpact,
  type PhaseInput,
  type PhaseKind,
  type PhaseCareOverflowMode,
  type PhaseCareOfferingSelectionMode,
  type RolloverResult,
  createPhase,
  deletePhase,
  getPhaseDeleteImpact,
  listPhases,
  phaseToInput,
  updatePhase,
} from "~/lib/enrollment-phase-api";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { ISODatePicker } from "~/components/ui/date-picker";
import { DateTimePicker } from "~/components/ui/date-time-picker";
import { calendarPeriodService } from "~/lib/calendar-period-api";
import {
  type CalendarPeriod,
  formatPeriodRange,
} from "~/lib/calendar-period-helpers";
import {
  latestSchemasByName,
  listSchemas,
  type FormSchema,
} from "~/lib/enrollment-form-schema-api";
import { createLogger } from "~/lib/logger";
import { RolloverForm } from "./rollover-form";
import { useTenant } from "~/lib/tenant-context";
import { isSupportedGradeLevelMax } from "~/lib/grade-level";
import { useToast } from "~/contexts/ToastContext";
import { useEnrollmentPublicUrl } from "~/lib/enrollment-public-url";
import { useTenantAwarePath } from "~/lib/tenant-path";
import { PublicLinkCopyButton } from "~/components/enrollment/public-link-copy-button";
import {
  DataTable,
  DataTableStatusBadge,
  type DataTableColumn,
} from "~/components/ui/data-table";
import { CustomSelect } from "~/components/ui/custom-select";
import { Radio } from "~/components/ui/radio";
import {
  LateInviteModal,
  ManualApprovedEnrollmentModal,
} from "~/components/enrollment/phase-enrollment-actions";

const logger = createLogger({ component: "PhasesEditor" });

const KIND_LABELS: Record<PhaseKind, string> = {
  school_year: "Schuljahr",
  holiday: "Ferienbetreuung",
  custom: "Sonstiges",
};

const OVERFLOW_LABELS: Record<PhaseCareOverflowMode, string> = {
  waitlist: "Auf Warteliste setzen",
  reject: "Anmeldung blockieren",
  allow: "Ohne Hinweis akzeptieren",
};

const CARE_SELECTION_LABELS: Record<PhaseCareOfferingSelectionMode, string> = {
  optional: "Optional",
  at_least_one: "Mindestens ein Angebot",
  exactly_one: "Genau ein Angebot",
};

const AUDIENCE_LABELS: Record<PhaseAudience, string> = {
  open: "Offen für alle",
  new_students: "Nur neue Kinder",
  existing_students: "Nur bereits angemeldete Kinder",
  linked_parents: "Nur Eltern mit Konto an der Schule",
};

// Compact labels for the phases table badge (the full labels are too long
// for a table cell). "open" needs no badge, so it is intentionally omitted.
const AUDIENCE_BADGE_LABELS: Record<Exclude<PhaseAudience, "open">, string> = {
  new_students: "Nur neue Kinder",
  existing_students: "Nur bereits angemeldete",
  linked_parents: "Nur Eltern mit Konto",
};

const AUDIENCE_OPTIONS: ReadonlyArray<{
  value: PhaseAudience;
  label: string;
  hint: string;
}> = [
  {
    value: "open",
    label: AUDIENCE_LABELS.open,
    hint: "Alle können sich anmelden, auch ohne Konto. Die Phase erscheint in der öffentlichen Anmeldeliste.",
  },
  {
    value: "new_students",
    label: AUDIENCE_LABELS.new_students,
    hint: "Anmeldung ohne Konto möglich. Kinder, die an der Schule bereits angemeldet sind, werden abgelehnt.",
  },
  {
    value: "existing_students",
    label: AUDIENCE_LABELS.existing_students,
    hint: "Für Wiederanmeldungen: Nur Kinder, die an der Schule bereits angemeldet sind, können sich anmelden. Neue Kinder werden abgelehnt.",
  },
  {
    value: "linked_parents",
    label: AUDIENCE_LABELS.linked_parents,
    hint: "Wird nicht in der öffentlichen Anmeldeliste angezeigt. Nur Eltern mit Konto und Berechtigung können sich im Elternportal anmelden.",
  },
];

// Schema-source mode.
// "base" sets form_schema_id = null. "reuse" picks an existing schema row.
type SchemaSource = "base" | "reuse";

function blankInput(): PhaseInput {
  // Pre-fill service dates with the next school year so admins do not
  // start from "0001-01-01". They can edit before saving.
  const now = new Date();
  const startYear =
    now.getMonth() >= 6 ? now.getFullYear() : now.getFullYear() - 1;
  return {
    name: "",
    kind: "school_year",
    service_start_date: `${startYear}-09-01`,
    service_end_date: `${startYear + 1}-07-31`,
    enrollment_open_at: null,
    enrollment_close_at: null,
    form_schema_id: null,
    calendar_period_id: null,
    show_status_reason_to_parent: false,
    care_overflow_mode: "waitlist",
    care_offering_selection_mode: "optional",
    is_active: true,
    available_school_classes: [],
    require_school_class: false,
    audience: "open",
    eligible_school_classes: [],
    eligible_grade_levels: [],
  };
}

// withNonEmptyClasses trims a class list and drops blank entries, matching the
// backend's own trim-then-ignore-empty reading of both class lists.
function withNonEmptyClasses(classes: readonly string[]): string[] {
  return classes.map((cls) => cls.trim()).filter((cls) => cls.length > 0);
}

// mergeOfferedClasses returns the offered ("Konkrete Klassen") list with every
// eligible class kept in it, preserving order and dropping duplicates.
//
// The backend requires eligible ⊆ available — the form can only present offered
// classes, so a restriction to a class it never offers rejects every submission.
// ADD rather than REPLACE: available may legitimately be a wider superset,
// because a late-invite recipient bypasses the eligibility gate and keeps the
// full offered list, while every self-service load is narrowed to the eligible
// subset server-side (#1663).
function mergeOfferedClasses(
  offered: readonly string[],
  eligible: readonly string[],
): string[] {
  const merged: string[] = [];
  const seen = new Set<string>();
  for (const cls of [
    ...withNonEmptyClasses(offered),
    ...withNonEmptyClasses(eligible),
  ]) {
    if (seen.has(cls)) continue;
    seen.add(cls);
    merged.push(cls);
  }
  return merged;
}

// RFC3339 to datetime-local "YYYY-MM-DDTHH:MM" helpers. The HTML
// datetime-local input always works in the browser's local zone,
// which matches what an admin expects when they type "1. März 09:00".
function toLocalInputValue(rfc?: string | null): string {
  if (!rfc) return "";
  const d = new Date(rfc);
  if (Number.isNaN(d.getTime())) return "";
  const pad = (n: number) => n.toString().padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function fromLocalInputValue(local: string): string | null {
  if (!local) return null;
  const d = new Date(local);
  if (Number.isNaN(d.getTime())) return null;
  return d.toISOString();
}

function formatDate(value: string): string {
  return new Date(`${value}T00:00:00`).toLocaleDateString("de-DE", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}

function formatDateTime(value: string): string {
  return new Date(value).toLocaleString("de-DE", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function PhasesEditor() {
  const searchParams = useSearchParams();
  const [phases, setPhases] = useState<Phase[]>([]);
  const [schemas, setSchemas] = useState<FormSchema[]>([]);
  const [periods, setPeriods] = useState<CalendarPeriod[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [editingId, setEditingId] = useState<string | null>(null);
  const [draft, setDraft] = useState<PhaseInput | null>(null);
  const [schemaSource, setSchemaSource] = useState<SchemaSource>("base");
  const [saving, setSaving] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  // Delete-confirmation modal state. deleteTarget drives visibility;
  // deleteImpact holds the fetched blast-radius counts.
  const [deleteTarget, setDeleteTarget] = useState<Phase | null>(null);
  const [deleteImpact, setDeleteImpact] = useState<PhaseDeleteImpact | null>(
    null,
  );
  const [impactLoading, setImpactLoading] = useState(false);
  // Preview-load failure (hard-blocks confirmation) vs. delete-call failure
  // (retryable) are tracked separately so a failed delete doesn't permanently
  // gate a retry, and a failed preview never lets the delete through.
  const [impactError, setImpactError] = useState("");
  const [deleteError, setDeleteError] = useState("");
  // Monotonic id of the in-flight delete-impact request. Stale responses
  // (admin opened phase A, then phase B before A resolved) are ignored by
  // comparing against the latest id.
  const impactRequestRef = useRef(0);
  const [rolloverSource, setRolloverSource] = useState<Phase | null>(null);
  const [highlightFormSection, setHighlightFormSection] = useState(false);
  const [highlightActions, setHighlightActions] = useState(false);
  const { tenantSlug, tenant } = useTenant();
  const resolvedGradeLevelMax = tenant?.gradeLevelMax;
  const gradeLevelMax = isSupportedGradeLevelMax(resolvedGradeLevelMax)
    ? resolvedGradeLevelMax
    : null;
  const tenantPath = useTenantAwarePath();
  const toast = useToast();
  const assignFormId = searchParams.get("assignForm");
  const latestSchemas = useMemo(() => latestSchemasByName(schemas), [schemas]);
  const assignSchema = useMemo(
    () => latestSchemas.find((schema) => schema.id === assignFormId) ?? null,
    [assignFormId, latestSchemas],
  );
  const schemaNameById = useMemo(() => {
    const names = new Map<string, string>();
    for (const schema of schemas) {
      names.set(schema.id, schema.name || "Formularvorlage");
    }
    return names;
  }, [schemas]);

  const loadAll = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [phasesData, schemasData, periodsData] = await Promise.all([
        listPhases(),
        listSchemas().catch(() => [] as FormSchema[]),
        // Calendar periods are optional context for the phase form —
        // a load failure must not block phase management.
        calendarPeriodService.list().catch(() => [] as CalendarPeriod[]),
      ]);
      setPhases(phasesData);
      setSchemas(schemasData);
      setPeriods(periodsData);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("phases_load_failed", { error: message });
      setError(message);
      toast.error(message);
    } finally {
      setLoading(false);
    }
  }, [toast]);

  useEffect(() => {
    void loadAll();
  }, [loadAll]);

  useEffect(() => {
    if (!highlightFormSection) return;
    const timeout = window.setTimeout(() => {
      setHighlightFormSection(false);
    }, 2600);
    return () => window.clearTimeout(timeout);
  }, [highlightFormSection]);

  useEffect(() => {
    if (!assignSchema || editingId || rolloverSource) return;
    setHighlightActions(true);
    const timeout = window.setTimeout(() => {
      setHighlightActions(false);
    }, 2600);
    return () => window.clearTimeout(timeout);
  }, [assignSchema, editingId, rolloverSource]);

  const startCreate = useCallback(() => {
    setEditingId("new");
    setDraft({
      ...blankInput(),
      form_schema_id: assignSchema?.id ?? null,
    });
    setSchemaSource(assignSchema ? "reuse" : "base");
    setHighlightFormSection(Boolean(assignSchema));
    setError(null);
  }, [assignSchema]);

  const startEdit = useCallback(
    (phase: Phase, forceFormHighlight = false) => {
      setEditingId(phase.id);
      setDraft({
        ...phaseToInput(phase),
        form_schema_id: assignSchema?.id ?? phase.form_schema_id ?? null,
      });
      setSchemaSource(assignSchema || phase.form_schema_id ? "reuse" : "base");
      setHighlightFormSection(Boolean(assignSchema) || forceFormHighlight);
      setError(null);
    },
    [assignSchema],
  );

  const cancelEdit = () => {
    setEditingId(null);
    setDraft(null);
  };

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!draft) return;
    setSaving(true);
    setError(null);
    try {
      // Normalise schema_source to form_schema_id.
      const payload: PhaseInput = {
        ...draft,
        name: draft.name.trim(),
        form_schema_id: schemaSource === "base" ? null : draft.form_schema_id,
      };
      if (!payload.name) {
        throw new Error("Bitte gib einen Namen für die Anmeldephase ein.");
      }
      if (schemaSource === "reuse" && !payload.form_schema_id) {
        throw new Error("Bitte ein Formular auswählen oder Basis wählen.");
      }
      if (!payload.service_start_date || !payload.service_end_date) {
        throw new Error(
          "Bitte Beginn und Ende des Betreuungszeitraums angeben.",
        );
      }
      if (
        payload.service_start_date &&
        payload.service_end_date &&
        payload.service_end_date < payload.service_start_date
      ) {
        throw new Error(
          "Ende des Betreuungszeitraums muss nach dem Beginn liegen.",
        );
      }
      if (
        payload.enrollment_open_at &&
        payload.enrollment_close_at &&
        new Date(payload.enrollment_close_at) <=
          new Date(payload.enrollment_open_at)
      ) {
        throw new Error(
          "Schließung des Anmeldefensters muss nach der Öffnung liegen.",
        );
      }
      // A "Nur für Klassen" restriction can only be met with a class the form
      // actually offers, and the form offers exactly the "Konkrete Klassen"
      // (available_school_classes) list. So every eligible class must also be
      // an offered one — the backend rejects a phase where it isn't — and the
      // class pick must be mandatory, otherwise the form shows "Klasse offen"
      // and every such submission fails server-side with class_not_eligible
      // after the parent completed the whole form (#1663). The tenant-wide
      // Klassen-Abfrage must still be active for the pick to appear —
      // surfaced in the field hint, not enforceable here.
      const eligibleClasses = withNonEmptyClasses(
        payload.eligible_school_classes ?? [],
      );
      // Grade-1 classes ("1a") are supported (#1663): when a phase offers a
      // grade-1 class the form collects it (grade 1 is no longer restricted to
      // grade-level only), so a grade-1 eligibility restriction is satisfiable.
      //
      // The editor already keeps the two lists in sync while typing; this stays
      // as the safety net for a draft loaded from a phase whose stored lists
      // drifted apart (an older editor build, or a direct API write).
      if (eligibleClasses.length > 0) {
        payload.available_school_classes = mergeOfferedClasses(
          payload.available_school_classes ?? [],
          eligibleClasses,
        );
        payload.require_school_class = true;
      }
      // Both restrictions apply together, so a class outside the selected
      // grades can never be declared — the child would have to be in two
      // grades at once. The backend rejects the pair; catch it here with a
      // concrete message naming the offending class (#1663).
      const eligibleGrades = payload.eligible_grade_levels ?? [];
      if (eligibleGrades.length > 0) {
        const conflicting = eligibleClasses.find((cls) => {
          const prefix = /(\d+)/.exec(cls)?.[1];
          return (
            prefix !== undefined && !eligibleGrades.includes(Number(prefix))
          );
        });
        if (conflicting) {
          throw new Error(
            `Die Klasse „${conflicting}“ passt nicht zu den gewählten Klassenstufen. Entferne die Klasse oder ergänze ihre Klassenstufe.`,
          );
        }
      }
      if (editingId === "new") {
        const created = await createPhase(payload);
        toast.success(`Anmeldephase „${created.name}" erstellt.`);
      } else if (editingId) {
        const updated = await updatePhase(editingId, payload);
        toast.success(`Anmeldephase „${updated.name}" gespeichert.`);
      }
      cancelEdit();
      await loadAll();
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("phase_save_failed", { error: message });
      setError(message);
      toast.error(message);
    } finally {
      setSaving(false);
    }
  };

  // Opens the confirmation modal and fetches the delete blast radius so
  // the admin sees exactly what will be removed vs kept before confirming.
  const requestDelete = useCallback((phase: Phase) => {
    setDeleteTarget(phase);
    setDeleteImpact(null);
    setImpactError("");
    setDeleteError("");
    setImpactLoading(true);
    const requestId = ++impactRequestRef.current;
    getPhaseDeleteImpact(phase.id)
      .then((impact) => {
        if (impactRequestRef.current !== requestId) return;
        setDeleteImpact(impact);
      })
      .catch((err) => {
        if (impactRequestRef.current !== requestId) return;
        const message =
          err instanceof Error ? err.message : "Unbekannter Fehler";
        logger.error("phase_delete_impact_failed", { error: message });
        setImpactError(message);
      })
      .finally(() => {
        if (impactRequestRef.current !== requestId) return;
        setImpactLoading(false);
      });
  }, []);

  const closeDelete = useCallback(() => {
    // Invalidate any in-flight preview so a late response can't populate the
    // next modal we open.
    impactRequestRef.current++;
    setDeleteTarget(null);
    setDeleteImpact(null);
    setImpactError("");
    setDeleteError("");
    setImpactLoading(false);
  }, []);

  const confirmDelete = useCallback(async () => {
    if (!deleteTarget) return;
    // Hard guard mirroring the modal's confirmDisabled gate: never delete
    // until the blast-radius preview has loaded successfully.
    if (impactLoading || !deleteImpact || impactError) return;
    const phase = deleteTarget;
    setDeletingId(phase.id);
    setDeleteError("");
    try {
      await deletePhase(phase.id);
      toast.success(`Anmeldephase „${phase.name}" gelöscht.`);
      closeDelete();
      await loadAll();
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("phase_delete_failed", { error: message });
      setDeleteError(message);
      toast.error(message);
    } finally {
      setDeletingId(null);
    }
  }, [
    deleteTarget,
    deleteImpact,
    impactLoading,
    impactError,
    closeDelete,
    loadAll,
    toast,
  ]);

  const startRollover = (phase: Phase) => {
    setRolloverSource(phase);
    setEditingId(null);
    setDraft(null);
    setError(null);
  };

  const handleRolloverSuccess = (result: RolloverResult) => {
    setRolloverSource(null);
    const summaryBits: string[] = [];
    if (result.summary.rolled_count > 0) {
      summaryBits.push(`${result.summary.rolled_count} übernommen`);
    }
    if (result.summary.review_count > 0) {
      summaryBits.push(`${result.summary.review_count} brauchen Prüfung`);
    }
    if (result.summary.enqueued_emails > 0) {
      summaryBits.push(`${result.summary.enqueued_emails} E-Mails verschickt`);
    }
    const detail = summaryBits.length > 0 ? ` (${summaryBits.join(", ")})` : "";
    toast.success(
      `Anschlussphase „${result.phase.name}" wurde erstellt${detail}.`,
    );
    void loadAll();
  };

  const handleToggleActive = useCallback(
    async (phase: Phase) => {
      setSaving(true);
      setError(null);
      try {
        const updated = await updatePhase(phase.id, {
          ...phaseToInput(phase),
          is_active: !phase.is_active,
        });
        toast.success(
          updated.is_active
            ? `Anmeldephase „${updated.name}" ist jetzt aktiv.`
            : `Anmeldephase „${updated.name}" wurde deaktiviert.`,
        );
        await loadAll();
      } catch (err) {
        const message =
          err instanceof Error ? err.message : "Unbekannter Fehler";
        logger.error("phase_toggle_active_failed", { error: message });
        setError(message);
        toast.error(message);
      } finally {
        setSaving(false);
      }
    },
    [loadAll, toast],
  );

  const activePhaseCount = phases.filter((phase) => phase.is_active).length;
  const periodNameById = useMemo(
    () => new Map(periods.map((period) => [period.id, period.name])),
    [periods],
  );
  const columns = useMemo<DataTableColumn<Phase>[]>(
    () => [
      {
        key: "name",
        header: "Anmeldephase",
        sortValue: (phase) => phase.name,
        render: (phase) => (
          <div className="min-w-0">
            <p className="truncate font-medium text-gray-900">{phase.name}</p>
            <p className="mt-0.5 flex flex-wrap items-center gap-1.5 text-xs text-gray-500">
              <span>{KIND_LABELS[phase.kind]}</span>
              {phase.audience && phase.audience !== "open" ? (
                <span className="inline-flex items-center rounded-full bg-[#5080D8]/10 px-1.5 py-0.5 text-[11px] font-medium text-[#4070C8]">
                  {AUDIENCE_BADGE_LABELS[phase.audience]}
                </span>
              ) : null}
              {(phase.eligible_grade_levels ?? []).length > 0 ? (
                <span className="inline-flex items-center rounded-full bg-[#5080D8]/10 px-1.5 py-0.5 text-[11px] font-medium text-[#4070C8]">
                  {`Nur Klassenstufe ${(phase.eligible_grade_levels ?? []).join(", ")}`}
                </span>
              ) : null}
            </p>
          </div>
        ),
      },
      {
        key: "service_period",
        header: "Betreuungszeitraum",
        sortValue: (phase) => phase.service_start_date,
        render: (phase) => {
          const periodName = phase.calendar_period_id
            ? periodNameById.get(phase.calendar_period_id)
            : undefined;
          return (
            <div className="min-w-0">
              <span className="text-sm text-gray-700">
                {formatDate(phase.service_start_date)} bis{" "}
                {formatDate(phase.service_end_date)}
              </span>
              <p
                className={`mt-0.5 truncate text-xs ${periodName ? "text-gray-500" : "text-gray-400"}`}
              >
                {periodName ?? "Kein Kalenderzeitraum"}
              </p>
            </div>
          );
        },
      },
      {
        key: "enrollment_window",
        header: "Anmeldefenster",
        sortValue: (phase) => phase.enrollment_open_at ?? "",
        render: (phase) => (
          <EnrollmentWindowCell
            openAt={phase.enrollment_open_at}
            closeAt={phase.enrollment_close_at}
          />
        ),
      },
      {
        key: "form",
        header: "Formular",
        sortValue: (phase) =>
          phase.form_schema_id
            ? (schemaNameById.get(phase.form_schema_id) ?? "")
            : "Basisformular",
        render: (phase) => (
          <FormSchemaCell
            schemaName={
              phase.form_schema_id
                ? (schemaNameById.get(phase.form_schema_id) ?? "Eigene Vorlage")
                : null
            }
          />
        ),
      },
      {
        key: "status",
        header: "Status",
        sortValue: (phase) => (phase.is_active ? 1 : 0),
        render: (phase) => <DataTableStatusBadge active={phase.is_active} />,
      },
      {
        key: "actions",
        header: "Aktionen",
        align: "right",
        render: (phase) => (
          <PhaseActions
            phase={phase}
            tenantSlug={tenantSlug}
            gradeLevelMax={gradeLevelMax}
            tenantPath={tenantPath}
            saving={saving}
            deleting={deletingId === phase.id}
            rolloverActive={!!rolloverSource}
            highlight={highlightActions}
            onEdit={() => startEdit(phase)}
            onAssignForm={() => startEdit(phase, true)}
            onRollover={() => startRollover(phase)}
            onToggleActive={() => void handleToggleActive(phase)}
            onDelete={() => requestDelete(phase)}
          />
        ),
      },
    ],
    [
      deletingId,
      requestDelete,
      handleToggleActive,
      highlightActions,
      periodNameById,
      rolloverSource,
      saving,
      schemaNameById,
      startEdit,
      gradeLevelMax,
      tenantSlug,
      tenantPath,
    ],
  );

  if (loading) {
    return (
      <div className="moto-content-surface rounded-2xl border px-5 py-10 text-center text-sm text-gray-500 shadow-sm">
        Anmeldephasen werden geladen...
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {error && (
        <div
          className="rounded-2xl border border-[#FF3130]/20 bg-[#FF3130]/10 p-4 text-sm text-[#CC2626]"
          role="alert"
          aria-live="polite"
        >
          {error}
        </div>
      )}

      <div className="moto-content-surface flex flex-col gap-4 rounded-2xl border p-4 shadow-sm backdrop-blur sm:flex-row sm:items-center sm:justify-between">
        <div className="grid gap-2 sm:grid-cols-3">
          <PhaseMetric
            icon={<CalendarRange className="h-4 w-4" />}
            label="Anmeldephasen"
            value={phases.length}
          />
          <PhaseMetric
            icon={<Clock3 className="h-4 w-4" />}
            label="Aktiv"
            value={activePhaseCount}
          />
          <PhaseMetric
            icon={<CalendarClock className="h-4 w-4" />}
            label="In Vorbereitung"
            value={Math.max(phases.length - activePhaseCount, 0)}
          />
        </div>
        {!editingId && !rolloverSource && (
          <button
            type="button"
            onClick={startCreate}
            className="inline-flex h-9 shrink-0 items-center justify-center gap-2 rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          >
            <CalendarPlus className="h-4 w-4" aria-hidden="true" />
            Neue Anmeldephase
          </button>
        )}
      </div>

      {rolloverSource && (
        <RolloverForm
          source={rolloverSource}
          onCancel={() => setRolloverSource(null)}
          onSuccess={handleRolloverSuccess}
        />
      )}

      {editingId && draft && (
        <PhaseForm
          draft={draft}
          setDraft={setDraft}
          periods={periods}
          schemas={latestSchemas}
          schemaSource={schemaSource}
          setSchemaSource={setSchemaSource}
          editing={editingId !== "new"}
          saving={saving}
          highlightFormSection={highlightFormSection}
          gradeLevelMax={gradeLevelMax}
          onSubmit={handleSave}
          onCancel={cancelEdit}
        />
      )}

      {phases.length === 0 && !editingId && !rolloverSource ? (
        <EmptyPhasesState onCreate={startCreate} />
      ) : phases.length > 0 && !editingId && !rolloverSource ? (
        <DataTable
          columns={columns}
          rows={phases}
          getRowKey={(phase) => phase.id}
          defaultSortKey="service_period"
          defaultSortDirection="desc"
        />
      ) : null}

      {deleteTarget && (
        <ConfirmDeleteModal
          isOpen={Boolean(deleteTarget)}
          title="Anmeldephase löschen"
          description={
            <p>
              Möchtest du die Anmeldephase{" "}
              <span className="font-medium">„{deleteTarget.name}"</span>{" "}
              wirklich endgültig löschen? Sie kann jederzeit gelöscht werden –
              während des Betreuungszeitraums, davor und danach.
            </p>
          }
          warningSlot={
            <div className="space-y-2">
              <div className="rounded-lg bg-[#EAB308]/10 px-3 py-2 text-sm text-[#854D0E]">
                <p className="font-medium">
                  Diese Aktion kann nicht rückgängig gemacht werden:
                </p>
                {impactLoading ? (
                  <p className="mt-1 text-xs">Löschvorschau wird geladen…</p>
                ) : impactError ? (
                  <p className="mt-1 text-xs">
                    Die Löschvorschau konnte nicht geladen werden. Das Löschen
                    ist erst möglich, sobald die Vorschau vorliegt. Bitte
                    schließe den Dialog und versuche es erneut.
                  </p>
                ) : deleteImpact ? (
                  <ul className="mt-1 list-inside list-disc space-y-0.5 text-xs">
                    <li>
                      {deleteImpact.requests}{" "}
                      {deleteImpact.requests === 1
                        ? "Anmeldung"
                        : "Anmeldungen"}{" "}
                      werden endgültig gelöscht
                    </li>
                    <li>
                      {deleteImpact.care_offerings}{" "}
                      {deleteImpact.care_offerings === 1
                        ? "Betreuungsangebot"
                        : "Betreuungsangebote"}{" "}
                      werden endgültig gelöscht
                    </li>
                  </ul>
                ) : null}
              </div>
              {!impactLoading && deleteImpact && (
                <div className="rounded-lg bg-[#83CD2D]/10 px-3 py-2 text-sm text-[#4a7a15]">
                  {deleteImpact.students_kept === 1
                    ? "1 bereits angelegter Kind bleibt erhalten."
                    : `${deleteImpact.students_kept} bereits angelegte Kinder bleiben erhalten.`}
                </div>
              )}
            </div>
          }
          gate={{ mode: "twoStep", firstStepLabel: "Löschen" }}
          confirmDisabled={
            impactLoading || !deleteImpact || Boolean(impactError)
          }
          onConfirm={confirmDelete}
          onClose={closeDelete}
          loading={deletingId === deleteTarget.id}
          error={impactError || deleteError}
        />
      )}
    </div>
  );
}

interface PhaseFormProps {
  readonly draft: PhaseInput;
  readonly setDraft: React.Dispatch<React.SetStateAction<PhaseInput | null>>;
  readonly periods: CalendarPeriod[];
  readonly schemas: FormSchema[];
  readonly schemaSource: SchemaSource;
  readonly setSchemaSource: React.Dispatch<React.SetStateAction<SchemaSource>>;
  readonly editing: boolean;
  readonly saving: boolean;
  readonly highlightFormSection: boolean;
  /** Tenant grade cap; null when unresolved — the grade picker then hides. */
  readonly gradeLevelMax: number | null;
  readonly onSubmit: (e: React.FormEvent) => void;
  readonly onCancel: () => void;
}

function PhaseMetric({
  icon,
  label,
  value,
}: Readonly<{
  icon: React.ReactNode;
  label: string;
  value: number;
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

function EmptyPhasesState({ onCreate }: Readonly<{ onCreate: () => void }>) {
  return (
    <section className="moto-content-surface rounded-2xl border px-6 py-12 text-center shadow-sm backdrop-blur-md">
      <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-2xl bg-[#5080D8]/10 text-[#5080D8]">
        <CalendarRange className="h-6 w-6" aria-hidden="true" />
      </div>
      <h2 className="mt-4 text-base font-semibold text-gray-900">
        Noch keine Anmeldephase angelegt
      </h2>
      <p className="mx-auto mt-2 max-w-md text-sm leading-6 text-gray-600">
        Lege zuerst fest, für welchen Zeitraum Eltern anmelden können, zum
        Beispiel für ein Halbjahr, ein Schuljahr oder eine Ferienbetreuung.
      </p>
      <button
        type="button"
        onClick={onCreate}
        className="mt-5 inline-flex h-9 items-center justify-center gap-2 rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
      >
        <CalendarPlus className="h-4 w-4" aria-hidden="true" />
        Erste Anmeldephase anlegen
      </button>
    </section>
  );
}

function EnrollmentWindowCell({
  openAt,
  closeAt,
}: Readonly<{ openAt?: string | null; closeAt?: string | null }>) {
  if (!openAt && !closeAt) {
    return <span className="text-sm text-gray-400">Jederzeit offen</span>;
  }

  return (
    <span className="text-sm text-gray-700">
      {openAt ? formatDateTime(openAt) : "Nicht gesetzt"} bis{" "}
      {closeAt ? formatDateTime(closeAt) : "Nicht gesetzt"}
    </span>
  );
}

function FormSchemaCell({
  schemaName,
}: Readonly<{ schemaName: string | null }>) {
  return (
    <span className="inline-flex items-center gap-2 text-sm text-gray-700">
      <span
        className={`flex h-7 w-7 items-center justify-center rounded-lg ${
          schemaName
            ? "bg-[#5080D8]/10 text-[#4070C8]"
            : "bg-gray-100 text-gray-500"
        }`}
        aria-hidden="true"
      >
        <FileText className="h-4 w-4" />
      </span>
      <span className="min-w-0">
        <span className="block truncate font-medium text-gray-900">
          {schemaName ?? "Basisformular"}
        </span>
        <span className="block text-xs text-gray-500">
          {schemaName ? "Eigene Vorlage" : "Standard"}
        </span>
      </span>
    </span>
  );
}

interface PhaseActionsProps {
  readonly phase: Phase;
  readonly tenantSlug?: string | null;
  readonly gradeLevelMax: number | null;
  readonly tenantPath: (path: string) => string;
  readonly saving: boolean;
  readonly deleting: boolean;
  readonly rolloverActive: boolean;
  readonly highlight: boolean;
  readonly onEdit: () => void;
  readonly onAssignForm: () => void;
  readonly onRollover: () => void;
  readonly onToggleActive: () => void;
  readonly onDelete: () => void;
}

function PhaseActions({
  phase,
  tenantSlug,
  gradeLevelMax,
  tenantPath,
  saving,
  deleting,
  rolloverActive,
  highlight,
  onEdit,
  onAssignForm,
  onRollover,
  onToggleActive,
  onDelete,
}: PhaseActionsProps) {
  const [lateInviteOpen, setLateInviteOpen] = useState(false);
  const [manualOpen, setManualOpen] = useState(false);
  const hasReviewList = tenantSlug && phase.rollover_source_phase_id;
  // Audience-restricted phases are never publicly reachable: the anonymous form
  // gate refuses BOTH linked_parents and existing_students, so the plain
  // /enroll/{id} URL 404s for either unless it carries a valid late-invite
  // token. The untokenized public actions ("Formular ansehen", the copy-link
  // button) would hand out a dead link, so hide them for both audiences while
  // keeping the tokenized "Nachzügler-Link erstellen" action, which appends the
  // accepted token (#1663). Those parents reach the form through the parents
  // portal picker instead.
  const hasNoPublicForm =
    phase.audience === "linked_parents" ||
    phase.audience === "existing_students";
  const phaseUrl = useEnrollmentPublicUrl({ tenantSlug, phaseId: phase.id });
  const enrollmentsHref = tenantPath(
    `/admin/enrollments/phases/${encodeURIComponent(phase.id)}`,
  );

  const menuEntries: OverflowMenuEntry[] = [
    {
      label: "Bearbeiten",
      icon: <Pencil className="h-4 w-4" aria-hidden />,
      disabled: saving,
      onClick: onEdit,
    },
    ...(hasNoPublicForm
      ? []
      : [
          {
            label: "Formular ansehen",
            icon: <ExternalLink className="h-4 w-4" aria-hidden />,
            href: `/enroll/${encodeURIComponent(phase.id)}`,
            external: true,
            onClick: () => undefined,
          },
        ]),
    {
      label: "Nachzügler-Link erstellen",
      icon: <Link2 className="h-4 w-4" aria-hidden />,
      disabled: saving,
      onClick: () => setLateInviteOpen(true),
    },
    {
      label: "Manuelle Anmeldung",
      icon: <UserCheck className="h-4 w-4" aria-hidden />,
      disabled: saving,
      onClick: () => setManualOpen(true),
    },
    {
      label: "Anmeldungen ansehen",
      icon: <ClipboardList className="h-4 w-4" aria-hidden />,
      href: enrollmentsHref,
      onClick: () => undefined,
    },
    {
      label: "Formular zuweisen",
      icon: <FileText className="h-4 w-4" aria-hidden />,
      disabled: saving,
      onClick: onAssignForm,
    },
    {
      label: "Anschlussphase erstellen",
      icon: <CalendarPlus className="h-4 w-4" aria-hidden />,
      disabled: saving || rolloverActive,
      onClick: onRollover,
    },
    ...(hasReviewList
      ? [
          {
            label: "Prüfliste öffnen",
            icon: <Check className="h-4 w-4" aria-hidden />,
            href: `/enrollment-phases/${encodeURIComponent(phase.id)}/review`,
            onClick: () => undefined,
          },
        ]
      : []),
    {
      label: phase.is_active ? "Deaktivieren" : "Aktivieren",
      icon: <Power className="h-4 w-4" aria-hidden />,
      disabled: saving,
      onClick: onToggleActive,
    },
    { kind: "separator" },
    {
      label: deleting ? "Löscht..." : "Löschen",
      icon: <Trash2 className="h-4 w-4" aria-hidden />,
      destructive: true,
      disabled: deleting || saving,
      onClick: onDelete,
    },
  ];

  return (
    <>
      <div className="flex justify-end gap-1.5">
        {phaseUrl && !hasNoPublicForm ? (
          <PublicLinkCopyButton
            url={phaseUrl}
            componentId={`PhaseActions:${phase.id}`}
          />
        ) : null}
        <OverflowMenu
          ariaLabel={`Aktionen für ${phase.name}`}
          items={menuEntries}
          triggerClassName={
            highlight
              ? "bg-[#83CD2D]/10 shadow-[0_0_0_4px_rgba(131,205,45,0.18)]"
              : ""
          }
        />
      </div>
      <LateInviteModal
        isOpen={lateInviteOpen}
        onClose={() => setLateInviteOpen(false)}
        phase={phase}
        phaseUrl={phaseUrl}
      />
      <ManualApprovedEnrollmentModal
        isOpen={manualOpen}
        onClose={() => setManualOpen(false)}
        phase={phase}
        gradeLevelMax={gradeLevelMax}
      />
    </>
  );
}

function PhaseForm(props: PhaseFormProps) {
  const {
    draft,
    setDraft,
    periods,
    schemas,
    schemaSource,
    setSchemaSource,
    editing,
    saving,
    highlightFormSection,
    gradeLevelMax,
    onSubmit,
    onCancel,
  } = props;
  const formSectionRef = useRef<HTMLFieldSetElement>(null);

  const update = (patch: Partial<PhaseInput>) =>
    setDraft((prev) => (prev ? { ...prev, ...patch } : prev));

  const hasSchoolClasses = (draft.available_school_classes ?? []).length > 0;
  // The linked period whose dates no longer match the phase's own service
  // dates. Linking only prefills once, so the two can drift apart; the form
  // surfaces the drift with a one-click "Daten übernehmen" re-sync.
  const linkedPeriod = draft.calendar_period_id
    ? periods.find((period) => period.id === draft.calendar_period_id)
    : undefined;
  const linkedPeriodDrift =
    linkedPeriod &&
    (draft.service_start_date !== linkedPeriod.startDate ||
      draft.service_end_date !== linkedPeriod.endDate)
      ? linkedPeriod
      : undefined;

  useEffect(() => {
    if (!highlightFormSection) return;
    formSectionRef.current?.scrollIntoView({
      behavior: "smooth",
      block: "center",
    });
  }, [highlightFormSection]);

  return (
    <form
      onSubmit={onSubmit}
      noValidate
      className="moto-content-surface space-y-5 rounded-2xl border p-6 shadow-sm backdrop-blur-md"
    >
      <h3 className="text-base font-semibold text-gray-900">
        {editing ? "Anmeldephase bearbeiten" : "Neue Anmeldephase"}
      </h3>

      <div className="grid gap-4 sm:grid-cols-2">
        <label className="block">
          <span className="text-xs font-medium text-gray-700">Name</span>
          <input
            name="name"
            type="text"
            value={draft.name}
            onChange={(e) => update({ name: e.target.value })}
            placeholder="z. B. Schuljahr 2026/27"
            aria-required="true"
            className="mt-1 h-10 w-full rounded-lg border border-gray-200 px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          />
        </label>

        <label className="block" htmlFor="phase-kind" id="phase-kind-label">
          <span className="text-xs font-medium text-gray-700">Typ</span>
          <CustomSelect
            ariaLabelledBy="phase-kind-label"
            id="phase-kind"
            value={draft.kind}
            onChange={(value) => update({ kind: value as PhaseKind })}
            className="mt-1"
            options={[
              { value: "school_year", label: KIND_LABELS.school_year },
              { value: "holiday", label: KIND_LABELS.holiday },
              { value: "custom", label: KIND_LABELS.custom },
            ]}
          />
        </label>
      </div>

      <fieldset className="rounded-xl border border-gray-200 p-4">
        <legend className="px-1 text-xs font-medium text-gray-700">
          Betreuungszeitraum
        </legend>
        {periods.length > 0 && (
          <label
            className="mb-3 block"
            htmlFor="phase-calendar-period"
            id="phase-calendar-period-label"
          >
            <span className="text-xs text-gray-600">
              Kalenderzeitraum (optional)
            </span>
            <CustomSelect
              ariaLabelledBy="phase-calendar-period-label"
              id="phase-calendar-period"
              value={draft.calendar_period_id ?? ""}
              onChange={(value) => {
                if (!value) {
                  update({ calendar_period_id: null });
                  return;
                }
                const period = periods.find((p) => p.id === value);
                update({
                  calendar_period_id: value,
                  // Prefill the service dates from the linked period —
                  // the admin can still adjust them afterwards.
                  ...(period
                    ? {
                        service_start_date: period.startDate,
                        service_end_date: period.endDate,
                      }
                    : {}),
                });
              }}
              className="mt-1"
              options={[
                { value: "", label: "Kein Kalenderzeitraum" },
                ...periods.map((period) => ({
                  value: period.id,
                  label: `${period.name} (${formatPeriodRange(period)})`,
                })),
              ]}
            />
            <span className="mt-1 block text-xs text-gray-500">
              Verknüpft die Anmeldephase mit einem Zeitraum aus der Planung und
              übernimmt dessen Beginn und Ende.
            </span>
            {!draft.calendar_period_id && (
              <span className="mt-1 block text-xs text-[#F78C10]">
                Ohne Verknüpfung können Betreuungsplan und Dienstplan später
                nicht auf diese Phase aufbauen.
              </span>
            )}
            {linkedPeriodDrift && (
              <span className="mt-1 flex flex-wrap items-center gap-2 text-xs text-[#F78C10]">
                Beginn und Ende weichen vom Kalenderzeitraum ab (
                {formatPeriodRange(linkedPeriodDrift)}).
                <button
                  type="button"
                  onClick={() =>
                    update({
                      service_start_date: linkedPeriodDrift.startDate,
                      service_end_date: linkedPeriodDrift.endDate,
                    })
                  }
                  className="font-medium text-gray-700 underline underline-offset-2 hover:text-gray-900"
                >
                  Daten übernehmen
                </button>
              </span>
            )}
          </label>
        )}
        <div className="grid gap-3 sm:grid-cols-2">
          <div className="block">
            <label
              htmlFor="service_start_date"
              className="text-xs text-gray-600"
            >
              Beginn
            </label>
            <ISODatePicker
              id="service_start_date"
              controlSize="md"
              ariaLabel="Beginn"
              value={draft.service_start_date}
              className="mt-1"
              calendarLayout="popover"
              hideClearButton
              required
              onChange={(next) => {
                setDraft((prev) => {
                  if (!prev) return prev;
                  const patch: Partial<PhaseInput> = {
                    service_start_date: next,
                  };
                  if (
                    next &&
                    prev.service_end_date &&
                    prev.service_end_date < next
                  ) {
                    patch.service_end_date = "";
                  }
                  return { ...prev, ...patch };
                });
              }}
            />
          </div>
          <div className="block">
            <label htmlFor="service_end_date" className="text-xs text-gray-600">
              Ende
            </label>
            <ISODatePicker
              id="service_end_date"
              controlSize="md"
              ariaLabel="Ende"
              min={draft.service_start_date || undefined}
              value={draft.service_end_date}
              onChange={(next) => update({ service_end_date: next })}
              className="mt-1"
              calendarLayout="popover"
              hideClearButton
              required
            />
          </div>
        </div>
      </fieldset>

      <fieldset className="rounded-xl border border-gray-200 p-4">
        <legend className="px-1 text-xs font-medium text-gray-700">
          Anmeldefenster (optional)
        </legend>
        <p className="mb-2 text-xs text-gray-500">
          Lass beide Felder leer, wenn das Anmeldeformular jederzeit erreichbar
          sein soll. Sonst kann die Anmeldung nur in diesem Zeitraum eingereicht
          werden.
        </p>
        <div className="grid gap-3 sm:grid-cols-2">
          <div className="block">
            <label
              htmlFor="enrollment_open_at"
              className="text-xs text-gray-600"
            >
              Öffnung
            </label>
            <DateTimePicker
              id="enrollment_open_at"
              controlSize="md"
              dateAriaLabel="Öffnung"
              timeAriaLabel="Öffnung Uhrzeit"
              className="mt-1"
              value={toLocalInputValue(draft.enrollment_open_at)}
              onChange={(nextLocal) => {
                const nextOpen = fromLocalInputValue(nextLocal);
                setDraft((prev) => {
                  if (!prev) return prev;
                  const patch: Partial<PhaseInput> = {
                    enrollment_open_at: nextOpen,
                  };
                  if (
                    nextOpen &&
                    prev.enrollment_close_at &&
                    new Date(prev.enrollment_close_at) <= new Date(nextOpen)
                  ) {
                    patch.enrollment_close_at = null;
                  }
                  return { ...prev, ...patch };
                });
              }}
            />
          </div>
          <div className="block">
            <label
              htmlFor="enrollment_close_at"
              className="text-xs text-gray-600"
            >
              Schließung
            </label>
            <DateTimePicker
              id="enrollment_close_at"
              controlSize="md"
              dateAriaLabel="Schließung"
              timeAriaLabel="Schließung Uhrzeit"
              className="mt-1"
              min={toLocalInputValue(draft.enrollment_open_at) || undefined}
              value={toLocalInputValue(draft.enrollment_close_at)}
              // A closing date picked without a time should end the day, not
              // start it — otherwise the window shuts at midnight.
              defaultTime="23:59"
              onChange={(nextLocal) =>
                update({
                  enrollment_close_at: fromLocalInputValue(nextLocal),
                })
              }
            />
          </div>
        </div>
      </fieldset>

      <fieldset
        ref={formSectionRef}
        className={`rounded-xl border p-4 transition-colors duration-300 ${
          highlightFormSection
            ? "border-[#83CD2D] bg-[#83CD2D]/8 shadow-[0_0_0_4px_rgba(131,205,45,0.12)]"
            : "border-gray-200"
        }`}
      >
        <legend className="px-1 text-xs font-medium text-gray-700">
          Formular
        </legend>
        <div className="space-y-2 text-sm">
          <label
            htmlFor="schema-source-base"
            className="flex items-start gap-2"
          >
            <input
              id="schema-source-base"
              type="radio"
              name="schema-source"
              value="base"
              checked={schemaSource === "base"}
              onChange={() => {
                setSchemaSource("base");
                update({ form_schema_id: null });
              }}
              className="mt-0.5"
            />
            <span>
              <strong>Basisformular</strong>: Elternteil, Kind, Klassenstufe und
              gewünschtes Betreuungsangebot.
            </span>
          </label>
          <label
            htmlFor="schema-source-reuse"
            className={`flex items-start gap-2 ${
              schemas.length === 0 ? "cursor-not-allowed opacity-50" : ""
            }`}
          >
            <input
              id="schema-source-reuse"
              type="radio"
              name="schema-source"
              value="reuse"
              checked={schemaSource === "reuse"}
              onChange={() => setSchemaSource("reuse")}
              className="mt-0.5"
              disabled={schemas.length === 0}
            />
            <span className="flex-1">
              <strong>Eigene Vorlage wählen</strong>: Eltern sehen zusätzlich
              die Fragen aus dieser Formularvorlage.
              {schemaSource === "reuse" && (
                <CustomSelect
                  value={draft.form_schema_id ?? ""}
                  onChange={(value) =>
                    update({ form_schema_id: value || null })
                  }
                  className="mt-1 py-2"
                  disabled={schemas.length === 0}
                  required={schemaSource === "reuse"}
                  ariaLabel="Formular auswählen"
                  options={[
                    { value: "", label: "Wählen" },
                    ...schemas.map((schema) => ({
                      value: schema.id,
                      label: schema.name || "Formularvorlage",
                    })),
                  ]}
                />
              )}
              {schemas.length === 0 && (
                <p className="mt-1 text-xs text-gray-500">
                  Noch keine eigenen Formulare vorhanden. Erstelle zunächst
                  eines auf der Seite „Anmeldeformulare“.
                </p>
              )}
            </span>
          </label>
        </div>
      </fieldset>

      <div className="grid gap-4 sm:grid-cols-2">
        <label
          className="block"
          htmlFor="phase-care-selection-mode"
          id="phase-care-selection-mode-label"
        >
          <span className="text-xs font-medium text-gray-700">
            Betreuungsauswahl
          </span>
          <CustomSelect
            ariaLabelledBy="phase-care-selection-mode-label"
            id="phase-care-selection-mode"
            value={draft.care_offering_selection_mode}
            onChange={(value) =>
              update({
                care_offering_selection_mode:
                  value as PhaseCareOfferingSelectionMode,
              })
            }
            className="mt-1 py-2"
            options={[
              { value: "optional", label: CARE_SELECTION_LABELS.optional },
              {
                value: "at_least_one",
                label: CARE_SELECTION_LABELS.at_least_one,
              },
              {
                value: "exactly_one",
                label: CARE_SELECTION_LABELS.exactly_one,
              },
            ]}
          />
          <span className="mt-1 block text-xs text-gray-500">
            Pflichtangebote bleiben davon getrennt und sind immer vorausgewählt.
          </span>
        </label>

        <label
          className="block"
          htmlFor="phase-care-overflow-mode"
          id="phase-care-overflow-mode-label"
        >
          <span className="text-xs font-medium text-gray-700">
            Verhalten bei voller Betreuung
          </span>
          <CustomSelect
            ariaLabelledBy="phase-care-overflow-mode-label"
            id="phase-care-overflow-mode"
            value={draft.care_overflow_mode}
            onChange={(value) =>
              update({
                care_overflow_mode: value as PhaseCareOverflowMode,
              })
            }
            className="mt-1 py-2"
            options={[
              { value: "waitlist", label: OVERFLOW_LABELS.waitlist },
              { value: "reject", label: OVERFLOW_LABELS.reject },
              { value: "allow", label: OVERFLOW_LABELS.allow },
            ]}
          />
        </label>

        <div className="flex flex-col gap-2 pt-5">
          <PhaseCheckbox
            checked={draft.show_status_reason_to_parent}
            onChange={(checked) =>
              update({ show_status_reason_to_parent: checked })
            }
            label="Begründung für Eltern sichtbar"
            hint="(auf der Status-Seite und in Status-E-Mails)"
          />
          <PhaseCheckbox
            checked={draft.is_active}
            onChange={(checked) => update({ is_active: checked })}
            label="Aktiv"
            hint="(für Eltern sichtbar, sobald das Anmeldefenster offen ist)"
          />
        </div>
      </div>

      <fieldset className="rounded-xl border border-gray-200 p-4">
        <legend className="px-1 text-xs font-medium text-gray-700">
          Zielgruppe
        </legend>
        <div className="space-y-2 text-sm">
          {AUDIENCE_OPTIONS.map((option) => (
            <label
              key={option.value}
              htmlFor={`phase-audience-${option.value}`}
              aria-label={option.label}
              className="flex cursor-pointer items-start gap-2"
            >
              <Radio
                id={`phase-audience-${option.value}`}
                name="phase-audience"
                value={option.value}
                checked={draft.audience === option.value}
                onChange={() => update({ audience: option.value })}
                className="mt-0.5"
              />
              <span className="flex-1">
                <span className="block font-medium text-gray-900">
                  {option.label}
                </span>
                <span className="mt-0.5 block text-xs font-normal text-gray-500">
                  {option.hint}
                </span>
              </span>
            </label>
          ))}
        </div>
        <div className="mt-4">
          <span className="text-xs font-medium text-gray-700">
            Nur für Klassenstufen (optional)
          </span>
          <p className="mt-1 text-xs leading-5 text-gray-500">
            Leer lassen für keine Einschränkung. Kinder müssen eine dieser
            Klassenstufen angeben, zum Beispiel für eine Phase nur für den 3.
            Jahrgang. Dafür genügt die Klassenstufen-Abfrage, konkrete Klassen
            sind nicht nötig.
          </p>
          <GradeLevelListEditor
            value={draft.eligible_grade_levels ?? []}
            gradeLevelMax={gradeLevelMax}
            onChange={(list) => update({ eligible_grade_levels: list })}
          />
        </div>
        <div className="mt-4">
          <span className="text-xs font-medium text-gray-700">
            Nur für Klassen (optional)
          </span>
          <p className="mt-1 text-xs leading-5 text-gray-500">
            Leer lassen für keine Einschränkung. Kinder müssen eine dieser
            Klassen angeben. Diese Klassen werden automatisch als „Konkrete
            Klassen“ zur Auswahl angeboten und sind ab Klasse 2 bei aktiver
            Klassen-Abfrage in den Einstellungen wählbar.
          </p>
          <SchoolClassListEditor
            value={draft.eligible_school_classes ?? []}
            onChange={(list) =>
              // An eligible class the phase does not offer is rejected by the
              // backend (eligible ⊆ available), and the restriction is only
              // enforceable when the class pick is mandatory. Mirror both here
              // so the "Konkrete Klassen" list below shows the truth while
              // editing instead of the save silently adding the classes (#1663).
              update(
                withNonEmptyClasses(list).length > 0
                  ? {
                      eligible_school_classes: list,
                      available_school_classes: mergeOfferedClasses(
                        draft.available_school_classes ?? [],
                        list,
                      ),
                      require_school_class: true,
                    }
                  : { eligible_school_classes: list },
              )
            }
          />
        </div>
      </fieldset>

      <fieldset className="rounded-xl border border-gray-200 p-4">
        <legend className="px-1 text-xs font-medium text-gray-700">
          Konkrete Klassen
        </legend>
        <p className="text-xs leading-5 text-gray-500">
          Ab der 2. Klasse können Eltern die konkrete Klasse (z. B. 2a) aus
          dieser Liste wählen. Für die 1. Klasse wird weiterhin nur die
          Klassenstufe erfasst. Nur wirksam, wenn „Konkrete Klasse abfragen“ in
          den Einstellungen aktiviert ist.
        </p>
        <SchoolClassListEditor
          value={draft.available_school_classes ?? []}
          onChange={(list) => {
            // A class that is still an eligibility target cannot be dropped
            // from the offered list: the backend refuses eligible ⊄ available,
            // and removing it here while the "Nur für Klassen" list keeps it
            // would look like it worked until the save re-added it (#1663).
            const offered = mergeOfferedClasses(
              list,
              draft.eligible_school_classes ?? [],
            );
            // Clearing the list must also clear "verpflichtend": a mandatory
            // phase with no offered classes is impossible - grade >= 2 parents
            // would be required to pick a class from an empty dropdown and the
            // backend would reject every submission (#1833).
            update(
              offered.length === 0
                ? {
                    available_school_classes: offered,
                    require_school_class: false,
                  }
                : { available_school_classes: offered },
            );
          }}
        />
        <div className="mt-3">
          <PhaseCheckbox
            // Only meaningful once at least one class is offered.
            checked={hasSchoolClasses && (draft.require_school_class ?? false)}
            onChange={(checked) => update({ require_school_class: checked })}
            disabled={!hasSchoolClasses}
            label="Konkrete Klasse verpflichtend (ab Klasse 2)"
            hint="(Eltern müssen ab der 2. Klasse eine Klasse wählen)"
          />
        </div>
      </fieldset>

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
          disabled={saving}
          className="inline-flex h-9 items-center justify-center rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50"
        >
          {saving ? "Speichert..." : editing ? "Speichern" : "Erstellen"}
        </button>
      </div>
    </form>
  );
}

// GradeLevelListEditor picks the grade levels a phase is restricted to
// (#1663). Grades are a closed, server-known set (1..grade_level_max), so
// this is a toggle row rather than the free-text chip editor the concrete
// classes need. An empty selection means "no restriction". Renders nothing
// while the tenant cap is unresolved — guessing a range would let an admin
// save a grade the school does not have.
function GradeLevelListEditor({
  value,
  gradeLevelMax,
  onChange,
}: Readonly<{
  value: number[];
  gradeLevelMax: number | null;
  onChange: (next: number[]) => void;
}>) {
  if (gradeLevelMax === null) {
    return (
      <p className="mt-2 text-xs text-gray-400">
        Klassenstufen konnten nicht geladen werden.
      </p>
    );
  }
  const grades = Array.from({ length: gradeLevelMax }, (_, i) => i + 1);
  const toggle = (grade: number) =>
    onChange(
      value.includes(grade)
        ? value.filter((g) => g !== grade)
        : [...value, grade].sort((a, b) => a - b),
    );
  return (
    <div className="mt-2 flex flex-wrap gap-2">
      {grades.map((grade) => {
        const selected = value.includes(grade);
        return (
          <button
            key={grade}
            type="button"
            onClick={() => toggle(grade)}
            aria-pressed={selected}
            className={`inline-flex h-9 min-w-11 items-center justify-center rounded-full border px-3 text-sm font-medium transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
              selected
                ? "border-[#83CD2D] bg-[#83CD2D] text-white"
                : "border-gray-200 bg-white text-gray-700 hover:border-gray-300 hover:bg-gray-50"
            }`}
          >
            {grade}
          </button>
        );
      })}
    </div>
  );
}

// SchoolClassListEditor edits the phase's admin-managed list of concrete
// classes offered to parents from grade 2 (#1833). A text input adds an
// entry (Enter or the button); entries render as removable chips. Trims,
// drops empties, and dedups case-sensitively.
function SchoolClassListEditor({
  value,
  onChange,
}: Readonly<{ value: string[]; onChange: (next: string[]) => void }>) {
  const [entry, setEntry] = useState("");
  const add = () => {
    const trimmed = entry.trim();
    if (!trimmed || value.includes(trimmed)) {
      setEntry("");
      return;
    }
    onChange([...value, trimmed]);
    setEntry("");
  };
  return (
    <div className="mt-2">
      <div className="flex gap-2">
        <input
          type="text"
          value={entry}
          onChange={(e) => setEntry(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              add();
            }
          }}
          placeholder="z. B. 2a"
          aria-label="Klasse hinzufügen"
          className="h-10 w-full rounded-lg border border-gray-200 px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
        />
        <button
          type="button"
          onClick={add}
          className="inline-flex h-10 shrink-0 items-center rounded-lg border border-gray-200 px-3 text-sm font-semibold text-gray-700 shadow-sm transition-colors hover:border-gray-300 hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
        >
          Hinzufügen
        </button>
      </div>
      {value.length > 0 ? (
        <ul className="mt-2 flex flex-wrap gap-2">
          {value.map((cls) => (
            <li
              key={cls}
              className="inline-flex items-center gap-1.5 rounded-full border border-gray-200 bg-gray-50 py-1 pr-1 pl-3 text-sm font-medium text-gray-700"
            >
              {cls}
              <button
                type="button"
                onClick={() => onChange(value.filter((c) => c !== cls))}
                aria-label={`Klasse ${cls} entfernen`}
                className="flex h-5 w-5 items-center justify-center rounded-full text-gray-400 transition-colors hover:bg-gray-200 hover:text-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
              >
                <X className="h-3.5 w-3.5" aria-hidden="true" />
              </button>
            </li>
          ))}
        </ul>
      ) : (
        <p className="mt-2 text-xs text-gray-400">
          Noch keine Klassen hinterlegt.
        </p>
      )}
    </div>
  );
}

function PhaseCheckbox({
  checked,
  onChange,
  label,
  hint,
  disabled = false,
}: Readonly<{
  checked: boolean;
  onChange: (checked: boolean) => void;
  label: string;
  hint: string;
  disabled?: boolean;
}>) {
  return (
    <label
      className={`flex min-h-11 items-center gap-3 rounded-2xl border px-3 py-2.5 text-sm font-medium text-gray-700 transition-colors focus-within:ring-2 focus-within:ring-gray-300 ${
        disabled
          ? "cursor-not-allowed border-gray-100 bg-gray-50 opacity-60"
          : checked
            ? "cursor-pointer border-gray-300 bg-gray-50"
            : "cursor-pointer border-gray-100 bg-white hover:border-gray-200 hover:bg-gray-50"
      }`}
    >
      <input
        type="checkbox"
        checked={checked}
        disabled={disabled}
        onChange={(event) => onChange(event.target.checked)}
        className="sr-only"
      />
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
      <span className="min-w-0 flex-1 leading-snug">
        {label}
        <span className="ml-1 text-xs font-normal text-gray-500">{hint}</span>
      </span>
    </label>
  );
}

"use client";

import Link from "next/link";
import { createPortal } from "react-dom";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  CalendarClock,
  CalendarPlus,
  CalendarRange,
  Check,
  ClipboardList,
  Clock3,
  MoreVertical,
  Pencil,
  Power,
  Trash2,
} from "lucide-react";
import {
  type Phase,
  type PhaseInput,
  type PhaseKind,
  type PhaseCareOverflowMode,
  type RolloverResult,
  createPhase,
  deletePhase,
  listPhases,
  updatePhase,
} from "~/lib/enrollment-phase-api";
import { listSchemas, type FormSchema } from "~/lib/enrollment-form-schema-api";
import { createLogger } from "~/lib/logger";
import { RolloverForm } from "./rollover-form";
import { useTenantSlugSafe } from "~/components/tenant/tenant-provider";
import { useToast } from "~/contexts/ToastContext";
import {
  DataTable,
  DataTableStatusBadge,
  type DataTableColumn,
} from "~/components/ui/data-table";

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
    show_status_reason_to_parent: false,
    care_overflow_mode: "waitlist",
    is_active: true,
  };
}

function phaseToInput(p: Phase): PhaseInput {
  return {
    name: p.name,
    kind: p.kind,
    service_start_date: p.service_start_date,
    service_end_date: p.service_end_date,
    enrollment_open_at: p.enrollment_open_at ?? null,
    enrollment_close_at: p.enrollment_close_at ?? null,
    form_schema_id: p.form_schema_id ?? null,
    show_status_reason_to_parent: p.show_status_reason_to_parent,
    care_overflow_mode: p.care_overflow_mode,
    is_active: p.is_active,
  };
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
  const [phases, setPhases] = useState<Phase[]>([]);
  const [schemas, setSchemas] = useState<FormSchema[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [editingId, setEditingId] = useState<string | null>(null);
  const [draft, setDraft] = useState<PhaseInput | null>(null);
  const [schemaSource, setSchemaSource] = useState<SchemaSource>("base");
  const [saving, setSaving] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [rolloverSource, setRolloverSource] = useState<Phase | null>(null);
  const tenantSlug = useTenantSlugSafe();
  const toast = useToast();

  const loadAll = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [phasesData, schemasData] = await Promise.all([
        listPhases(),
        listSchemas().catch(() => [] as FormSchema[]),
      ]);
      setPhases(phasesData);
      setSchemas(schemasData);
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

  const startCreate = () => {
    setEditingId("new");
    setDraft(blankInput());
    setSchemaSource("base");
    setError(null);
  };

  const startEdit = (phase: Phase) => {
    setEditingId(phase.id);
    setDraft(phaseToInput(phase));
    setSchemaSource(phase.form_schema_id ? "reuse" : "base");
    setError(null);
  };

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
        form_schema_id: schemaSource === "base" ? null : draft.form_schema_id,
      };
      if (schemaSource === "reuse" && !payload.form_schema_id) {
        throw new Error("Bitte ein Formular auswählen oder Basis wählen.");
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

  const handleDelete = useCallback(
    async (phase: Phase) => {
      if (
        !window.confirm(
          `Anmeldephase „${phase.name}" wirklich löschen? Diese Aktion ist nicht umkehrbar.`,
        )
      ) {
        return;
      }
      setDeletingId(phase.id);
      setError(null);
      try {
        await deletePhase(phase.id);
        toast.success(`Anmeldephase „${phase.name}" gelöscht.`);
        await loadAll();
      } catch (err) {
        const message =
          err instanceof Error ? err.message : "Unbekannter Fehler";
        logger.error("phase_delete_failed", { error: message });
        // The backend returns 409 with a German hint when the phase has
        // care offerings or submissions referencing it. The message
        // already says "deactivate instead", surface as-is.
        setError(message);
        toast.error(message);
      } finally {
        setDeletingId(null);
      }
    },
    [loadAll, toast],
  );

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
  const columns = useMemo<DataTableColumn<Phase>[]>(
    () => [
      {
        key: "name",
        header: "Anmeldephase",
        sortValue: (phase) => phase.name,
        render: (phase) => (
          <div className="min-w-0">
            <p className="truncate font-medium text-gray-900">{phase.name}</p>
            <p className="mt-0.5 text-xs text-gray-500">
              {KIND_LABELS[phase.kind]}
            </p>
          </div>
        ),
      },
      {
        key: "service_period",
        header: "Betreuungszeitraum",
        sortValue: (phase) => phase.service_start_date,
        render: (phase) => (
          <span className="text-sm text-gray-700">
            {formatDate(phase.service_start_date)} bis{" "}
            {formatDate(phase.service_end_date)}
          </span>
        ),
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
            saving={saving}
            deleting={deletingId === phase.id}
            rolloverActive={!!rolloverSource}
            onEdit={() => startEdit(phase)}
            onRollover={() => startRollover(phase)}
            onToggleActive={() => void handleToggleActive(phase)}
            onDelete={() => void handleDelete(phase)}
          />
        ),
      },
    ],
    [
      deletingId,
      handleDelete,
      handleToggleActive,
      rolloverSource,
      saving,
      tenantSlug,
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
          schemas={schemas}
          schemaSource={schemaSource}
          setSchemaSource={setSchemaSource}
          editing={editingId !== "new"}
          saving={saving}
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
    </div>
  );
}

interface PhaseFormProps {
  readonly draft: PhaseInput;
  readonly setDraft: React.Dispatch<React.SetStateAction<PhaseInput | null>>;
  readonly schemas: FormSchema[];
  readonly schemaSource: SchemaSource;
  readonly setSchemaSource: React.Dispatch<React.SetStateAction<SchemaSource>>;
  readonly editing: boolean;
  readonly saving: boolean;
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
      {openAt ? formatDateTime(openAt) : "Nicht gesetzt"}
      <span className="text-gray-400"> bis </span>
      {closeAt ? formatDateTime(closeAt) : "Nicht gesetzt"}
    </span>
  );
}

interface PhaseActionsProps {
  readonly phase: Phase;
  readonly tenantSlug?: string | null;
  readonly saving: boolean;
  readonly deleting: boolean;
  readonly rolloverActive: boolean;
  readonly onEdit: () => void;
  readonly onRollover: () => void;
  readonly onToggleActive: () => void;
  readonly onDelete: () => void;
}

interface PhaseActionsMenuPosition {
  top: number;
  left: number;
  alignRight: boolean;
}

function PhaseActions({
  phase,
  tenantSlug,
  saving,
  deleting,
  rolloverActive,
  onEdit,
  onRollover,
  onToggleActive,
  onDelete,
}: PhaseActionsProps) {
  const [open, setOpen] = useState(false);
  const [mounted, setMounted] = useState(false);
  const [position, setPosition] = useState<PhaseActionsMenuPosition>({
    top: 0,
    left: 0,
    alignRight: false,
  });
  const buttonRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const hasReviewList = tenantSlug && phase.rollover_source_phase_id;

  useEffect(() => {
    setMounted(true);
  }, []);

  useEffect(() => {
    if (!open || !buttonRef.current) return;

    const rect = buttonRef.current.getBoundingClientRect();
    const menuWidth = 240;
    const alignRight = rect.left + menuWidth > window.innerWidth - 16;
    setPosition({
      top: rect.bottom + 6,
      left: alignRight ? rect.right : rect.left,
      alignRight,
    });
  }, [open]);

  useEffect(() => {
    if (!open) return;

    function handleClickOutside(event: MouseEvent) {
      if (
        menuRef.current &&
        event.target instanceof Node &&
        !menuRef.current.contains(event.target)
      ) {
        setOpen(false);
      }
    }

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") setOpen(false);
    }

    document.addEventListener("mousedown", handleClickOutside);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [open]);

  useEffect(() => {
    if (!open) return;

    function handleScroll() {
      setOpen(false);
    }

    window.addEventListener("scroll", handleScroll, true);
    return () => window.removeEventListener("scroll", handleScroll, true);
  }, [open]);

  const menu = open && mounted && (
    <div
      ref={menuRef}
      role="menu"
      tabIndex={-1}
      aria-label={`Aktionen für ${phase.name}`}
      className="fixed z-[9999] w-60 overflow-hidden rounded-lg border border-gray-200 bg-white py-1 text-left shadow-lg"
      style={{
        top: position.top,
        left: position.alignRight ? "auto" : position.left,
        right: position.alignRight ? window.innerWidth - position.left : "auto",
      }}
    >
      <button
        type="button"
        role="menuitem"
        onClick={() => {
          setOpen(false);
          onEdit();
        }}
        className="flex w-full items-center gap-2 px-3 py-2 text-sm text-gray-700 transition-colors hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50"
        disabled={saving}
      >
        <Pencil className="h-4 w-4 text-gray-500" aria-hidden />
        Bearbeiten
      </button>

      <button
        type="button"
        role="menuitem"
        onClick={() => {
          setOpen(false);
          onRollover();
        }}
        className="flex w-full items-center gap-2 px-3 py-2 text-sm text-gray-700 transition-colors hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50"
        disabled={saving || rolloverActive}
      >
        <CalendarPlus className="h-4 w-4 text-gray-500" aria-hidden />
        Anschlussphase erstellen
      </button>

      {hasReviewList ? (
        <Link
          href={`/enrollment-phases/${encodeURIComponent(phase.id)}/review`}
          role="menuitem"
          tabIndex={0}
          onClick={() => setOpen(false)}
          className="flex w-full items-center gap-2 px-3 py-2 text-sm text-gray-700 transition-colors hover:bg-gray-50"
        >
          <ClipboardList className="h-4 w-4 text-gray-500" aria-hidden />
          Prüfliste öffnen
        </Link>
      ) : null}

      <button
        type="button"
        role="menuitem"
        onClick={() => {
          setOpen(false);
          onToggleActive();
        }}
        className="flex w-full items-center gap-2 px-3 py-2 text-sm text-gray-700 transition-colors hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50"
        disabled={saving}
      >
        <Power className="h-4 w-4 text-gray-500" aria-hidden />
        {phase.is_active ? "Deaktivieren" : "Aktivieren"}
      </button>

      <div className="my-1 h-px bg-gray-100" />

      <button
        type="button"
        role="menuitem"
        onClick={() => {
          setOpen(false);
          onDelete();
        }}
        className="flex w-full items-center gap-2 px-3 py-2 text-sm font-medium text-[#CC2626] transition-colors hover:bg-[#FF3130]/10 disabled:cursor-not-allowed disabled:opacity-50"
        disabled={deleting || saving}
      >
        <Trash2 className="h-4 w-4" aria-hidden />
        {deleting ? "Löscht..." : "Löschen"}
      </button>
    </div>
  );

  return (
    <div className="flex justify-end">
      <div>
        <button
          ref={buttonRef}
          type="button"
          onClick={() => setOpen((prev) => !prev)}
          aria-label={`Aktionen für ${phase.name}`}
          aria-haspopup="menu"
          aria-expanded={open}
          className="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-gray-200 bg-white text-gray-500 shadow-sm transition-colors hover:bg-gray-50 hover:text-gray-800 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
        >
          <MoreVertical className="h-4 w-4" aria-hidden="true" />
        </button>
        {mounted ? createPortal(menu, document.body) : null}
      </div>
    </div>
  );
}

function PhaseForm(props: PhaseFormProps) {
  const {
    draft,
    setDraft,
    schemas,
    schemaSource,
    setSchemaSource,
    editing,
    saving,
    onSubmit,
    onCancel,
  } = props;

  const update = (patch: Partial<PhaseInput>) =>
    setDraft((prev) => (prev ? { ...prev, ...patch } : prev));

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

        <label className="block">
          <span className="text-xs font-medium text-gray-700">Typ</span>
          <select
            name="kind"
            value={draft.kind}
            onChange={(e) => update({ kind: e.target.value as PhaseKind })}
            className="moto-select moto-content-surface mt-1 h-10 w-full rounded-lg border px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          >
            <option value="school_year">{KIND_LABELS.school_year}</option>
            <option value="holiday">{KIND_LABELS.holiday}</option>
            <option value="custom">{KIND_LABELS.custom}</option>
          </select>
        </label>
      </div>

      <fieldset className="rounded-xl border border-gray-200 p-4">
        <legend className="px-1 text-xs font-medium text-gray-700">
          Betreuungszeitraum
        </legend>
        <div className="grid gap-3 sm:grid-cols-2">
          <label className="block">
            <span className="text-xs text-gray-600">Beginn</span>
            <input
              name="service_start_date"
              type="date"
              value={draft.service_start_date}
              onChange={(e) => {
                const next = e.target.value;
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
              aria-required="true"
              className="mt-1 h-10 w-full rounded-lg border border-gray-200 px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
            />
          </label>
          <label className="block">
            <span className="text-xs text-gray-600">Ende</span>
            <input
              name="service_end_date"
              key={draft.service_end_date === "" ? "empty" : "set"}
              type="date"
              min={draft.service_start_date || undefined}
              value={draft.service_end_date}
              onChange={(e) => update({ service_end_date: e.target.value })}
              aria-required="true"
              className="mt-1 h-10 w-full rounded-lg border border-gray-200 px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
            />
          </label>
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
          <label className="block">
            <span className="text-xs text-gray-600">Öffnung</span>
            <input
              name="enrollment_open_at"
              type="datetime-local"
              value={toLocalInputValue(draft.enrollment_open_at)}
              onChange={(e) => {
                const nextOpen = fromLocalInputValue(e.target.value);
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
              className="mt-1 h-10 w-full rounded-lg border border-gray-200 px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
            />
          </label>
          <label className="block">
            <span className="text-xs text-gray-600">Schließung</span>
            <input
              name="enrollment_close_at"
              key={draft.enrollment_close_at == null ? "empty" : "set"}
              type="datetime-local"
              min={toLocalInputValue(draft.enrollment_open_at) || undefined}
              value={toLocalInputValue(draft.enrollment_close_at)}
              onChange={(e) =>
                update({
                  enrollment_close_at: fromLocalInputValue(e.target.value),
                })
              }
              className="mt-1 h-10 w-full rounded-lg border border-gray-200 px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
            />
          </label>
        </div>
      </fieldset>

      <fieldset className="rounded-xl border border-gray-200 p-4">
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
              <strong>Basis</strong>: nur die Standardfelder (Eltern, Kinder,
              Geburtsdatum, Klasse).
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
              <strong>Bestehendes Formular wählen</strong>: eigene Felder aus
              einer anderen Anmeldephase übernehmen.
              {schemaSource === "reuse" && (
                <select
                  value={draft.form_schema_id ?? ""}
                  onChange={(e) =>
                    update({ form_schema_id: e.target.value || null })
                  }
                  className="moto-select moto-content-surface mt-1 block w-full rounded-lg border px-3 py-2 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                  disabled={schemas.length === 0}
                  aria-required={schemaSource === "reuse"}
                  aria-label="Formular auswählen"
                >
                  <option value="">Wählen</option>
                  {schemas.map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name || "Standardformular"} (v{s.version}),{" "}
                      {s.fields.length} Felder
                    </option>
                  ))}
                </select>
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
        <label className="block">
          <span className="text-xs font-medium text-gray-700">
            Verhalten bei voller Betreuung
          </span>
          <select
            name="care_overflow_mode"
            value={draft.care_overflow_mode}
            onChange={(e) =>
              update({
                care_overflow_mode: e.target.value as PhaseCareOverflowMode,
              })
            }
            className="moto-select moto-content-surface mt-1 w-full rounded-lg border px-3 py-2 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          >
            <option value="waitlist">{OVERFLOW_LABELS.waitlist}</option>
            <option value="reject">{OVERFLOW_LABELS.reject}</option>
            <option value="allow">{OVERFLOW_LABELS.allow}</option>
          </select>
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

function PhaseCheckbox({
  checked,
  onChange,
  label,
  hint,
}: Readonly<{
  checked: boolean;
  onChange: (checked: boolean) => void;
  label: string;
  hint: string;
}>) {
  return (
    <label
      className={`flex min-h-11 cursor-pointer items-center gap-3 rounded-2xl border px-3 py-2.5 text-sm font-medium text-gray-700 transition-colors focus-within:ring-2 focus-within:ring-gray-300 ${
        checked
          ? "border-gray-300 bg-gray-50"
          : "border-gray-100 bg-white hover:border-gray-200 hover:bg-gray-50"
      }`}
    >
      <input
        type="checkbox"
        checked={checked}
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

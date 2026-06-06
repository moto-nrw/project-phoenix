"use client";

import Link from "next/link";
import { createPortal } from "react-dom";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  CalendarDays,
  Check,
  ClipboardList,
  Copy,
  MoreVertical,
  Pencil,
  Plus,
  Trash2,
  Users,
} from "lucide-react";
import {
  type CareOffering,
  type CareOfferingInput,
  type CareSelectionRule,
  type DaysOfWeekMode,
  SELECTION_RULE_LABELS,
  cloneCareOffering,
  createCareOffering,
  deleteCareOffering,
  listCareOfferings,
  updateCareOffering,
} from "~/lib/care-offering-api";
import { type Phase, listPhases } from "~/lib/enrollment-phase-api";
import { createLogger } from "~/lib/logger";
import { useToast } from "~/contexts/ToastContext";
import {
  DataTable,
  DataTableStatusBadge,
  type DataTableColumn,
} from "~/components/ui/data-table";

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

const KIND_LABELS: Record<Phase["kind"], string> = {
  school_year: "Schuljahr",
  holiday: "Ferienbetreuung",
  custom: "Sonstiges",
};

function blankInput(phaseId: number): CareOfferingInput {
  return {
    phase_id: phaseId,
    activity_group_id: null,
    name: "",
    description: "",
    days_of_week_mode: "fixed",
    available_days: WEEKDAY_KEYS,
    includes_holiday_care: false,
    includes_lunch: false,
    capacity: null,
    price_cents: null,
    is_active: true,
    is_required: false,
    sort_order: 0,
    selection_group: "",
    selection_rule: "optional",
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
    sort_order: offering.sort_order,
    selection_group: offering.selection_group ?? "",
    selection_rule: offering.selection_rule ?? "optional",
  };
}

function formatPrice(cents?: number | null): string {
  if (cents == null) return "Ohne Preis";
  return new Intl.NumberFormat("de-DE", {
    style: "currency",
    currency: "EUR",
  }).format(cents / 100);
}

function formatDays(days: string[]): string {
  const visibleDays = days.filter((day) => WEEKDAY_KEYS.includes(day));
  if (visibleDays.length === 0) return "Keine Wochentage";
  return visibleDays.map((day) => DAY_LABELS[day] ?? day).join(", ");
}

export function CareOfferingsEditor() {
  const [phases, setPhases] = useState<Phase[]>([]);
  const [selectedPhaseId, setSelectedPhaseId] = useState<string>("");
  const [offerings, setOfferings] = useState<CareOffering[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [draft, setDraft] = useState<CareOfferingInput | null>(null);
  const [saving, setSaving] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [cloneSource, setCloneSource] = useState<CareOffering | null>(null);
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

  const loadAll = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const phasesData = await listPhases();
      setPhases(phasesData);

      let phaseId = selectedPhaseId;
      if (!phaseId && phasesData.length > 0) {
        const first =
          phasesData.find((phase) => phase.is_active) ?? phasesData[0];
        if (first) {
          phaseId = first.id;
          setSelectedPhaseId(first.id);
        }
      }

      const offeringsData = phaseId ? await listCareOfferings(phaseId) : [];
      setOfferings(offeringsData);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("care_offerings_load_failed", { error: message });
      setError(message);
      toast.error(message);
    } finally {
      setLoading(false);
    }
  }, [selectedPhaseId, toast]);

  useEffect(() => {
    void loadAll();
  }, [loadAll]);

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
    setSaving(true);
    setError(null);
    try {
      if (editingId === "new") {
        const created = await createCareOffering(draft);
        toast.success(`Betreuungsangebot „${created.name}" erstellt.`);
      } else if (editingId) {
        const updated = await updateCareOffering(editingId, draft);
        toast.success(`Betreuungsangebot „${updated.name}" gespeichert.`);
      }
      cancelFocusMode();
      await loadAll();
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Speichern fehlgeschlagen";
      logger.error("care_offering_save_failed", { error: message });
      setError(message);
      toast.error(message);
    } finally {
      setSaving(false);
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
            {!offering.is_required &&
            !offering.includes_lunch &&
            !offering.includes_holiday_care ? (
              <span className="text-sm text-gray-400">Keine Extras</span>
            ) : null}
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
    [deletingId, handleDelete, saving],
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
          className="rounded-2xl border border-[#FF3130]/20 bg-[#FF3130]/10 p-4 text-sm text-[#CC2626]"
          role="alert"
          aria-live="polite"
        >
          {error}
        </div>
      )}

      {hasNoPhases ? (
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
            onPhaseChange={setSelectedPhaseId}
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
              phases={phases}
              saving={saving}
              onChange={setDraft}
              onSubmit={handleSave}
              onCancel={cancelFocusMode}
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
    </div>
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
            icon={<ClipboardList className="h-4 w-4" />}
            label="Angebote"
            value={offeringCount}
          />
          <CareOfferingMetric
            icon={<Check className="h-4 w-4" />}
            label="Aktiv"
            value={activeOfferingCount}
          />
          <CareOfferingMetric
            icon={<CalendarDays className="h-4 w-4" />}
            label="Tage"
            value={selectableDaysCount}
          />
          <CareOfferingMetric
            icon={<Users className="h-4 w-4" />}
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
            <select
              id="care-offerings-phase"
              name="phase"
              value={selectedPhaseId}
              onChange={(event) => onPhaseChange(event.target.value)}
              className="moto-select moto-content-surface h-9 min-w-60 rounded-lg border px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
            >
              {phases.map((phase) => (
                <option key={phase.id} value={phase.id}>
                  {phase.name} ({KIND_LABELS[phase.kind]})
                  {!phase.is_active ? " (inaktiv)" : ""}
                </option>
              ))}
            </select>
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

function NoPhaseState() {
  return (
    <section className="moto-content-surface rounded-2xl border px-6 py-12 text-center shadow-sm backdrop-blur-md">
      <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-2xl bg-[#F78C10]/10 text-[#C56F0D]">
        <CalendarDays className="h-6 w-6" aria-hidden="true" />
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
        <CalendarDays className="h-4 w-4" aria-hidden="true" />
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
      <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-2xl bg-[#5080D8]/10 text-[#5080D8]">
        <ClipboardList className="h-6 w-6" aria-hidden="true" />
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

function FeaturePill({ label }: Readonly<{ label: string }>) {
  return (
    <span className="inline-flex items-center rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600">
      {label}
    </span>
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

interface MenuPosition {
  top: number;
  left: number;
  alignRight: boolean;
}

function CareOfferingActions({
  offering,
  saving,
  deleting,
  onEdit,
  onClone,
  onDelete,
}: CareOfferingActionsProps) {
  const [open, setOpen] = useState(false);
  const [mounted, setMounted] = useState(false);
  const [position, setPosition] = useState<MenuPosition>({
    top: 0,
    left: 0,
    alignRight: false,
  });
  const buttonRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    setMounted(true);
  }, []);

  useEffect(() => {
    if (!open || !buttonRef.current) return;
    const rect = buttonRef.current.getBoundingClientRect();
    const menuWidth = 224;
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
      aria-label={`Aktionen für ${offering.name}`}
      className="fixed z-[9999] w-56 overflow-hidden rounded-lg border border-gray-200 bg-white py-1 text-left shadow-lg"
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
          onClone();
        }}
        className="flex w-full items-center gap-2 px-3 py-2 text-sm text-gray-700 transition-colors hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50"
        disabled={saving}
      >
        <Copy className="h-4 w-4 text-gray-500" aria-hidden />
        Duplizieren
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
      <button
        ref={buttonRef}
        type="button"
        onClick={() => setOpen((prev) => !prev)}
        aria-label={`Aktionen für ${offering.name}`}
        aria-haspopup="menu"
        aria-expanded={open}
        className="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-gray-200 bg-white text-gray-500 shadow-sm transition-colors hover:bg-gray-50 hover:text-gray-800 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
      >
        <MoreVertical className="h-4 w-4" aria-hidden="true" />
      </button>
      {mounted ? createPortal(menu, document.body) : null}
    </div>
  );
}

interface CareOfferingFormProps {
  readonly draft: CareOfferingInput;
  readonly editing: boolean;
  readonly phases: Phase[];
  readonly saving: boolean;
  readonly onChange: (draft: CareOfferingInput) => void;
  readonly onSubmit: (event: React.FormEvent) => void;
  readonly onCancel: () => void;
}

function CareOfferingForm({
  draft,
  editing,
  phases,
  saving,
  onChange,
  onSubmit,
  onCancel,
}: CareOfferingFormProps) {
  const update = (patch: Partial<CareOfferingInput>) =>
    onChange({ ...draft, ...patch });
  const toggleDay = (day: string) => {
    const nextDays = new Set(draft.available_days);
    if (nextDays.has(day)) nextDays.delete(day);
    else nextDays.add(day);
    update({
      available_days: WEEKDAY_KEYS.filter((dayKey) => nextDays.has(dayKey)),
    });
  };

  return (
    <form
      onSubmit={onSubmit}
      noValidate
      className="moto-content-surface space-y-5 rounded-2xl border p-6 shadow-sm backdrop-blur-md"
    >
      <header className="border-b border-gray-100 pb-4">
        <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
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
        <label className="block">
          <span className="text-xs font-medium text-gray-700">
            Anmeldephase
          </span>
          <select
            name="phase"
            value={draft.phase_id?.toString() ?? ""}
            onChange={(event) =>
              update({
                phase_id: event.target.value ? Number(event.target.value) : 0,
              })
            }
            className="moto-select moto-content-surface mt-1 h-10 w-full rounded-lg border px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          >
            <option value="">Bitte wählen</option>
            {phases.map((phase) => (
              <option key={phase.id} value={phase.id}>
                {phase.name}
              </option>
            ))}
          </select>
        </label>
      </div>

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
          <label className="block">
            <span className="text-xs font-medium text-gray-700">Regel</span>
            <select
              name="selection_rule"
              value={draft.selection_rule ?? "optional"}
              onChange={(event) =>
                update({
                  selection_rule: event.target.value as CareSelectionRule,
                })
              }
              disabled={!draft.selection_group?.trim()}
              className="moto-select moto-content-surface mt-1 h-10 w-full rounded-lg border px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:bg-gray-100 disabled:text-gray-500"
            >
              {(Object.keys(SELECTION_RULE_LABELS) as CareSelectionRule[]).map(
                (rule) => (
                  <option key={rule} value={rule}>
                    {SELECTION_RULE_LABELS[rule]}
                  </option>
                ),
              )}
            </select>
          </label>
        </div>
      </fieldset>

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
        <div className="mt-3">
          <CareOfferingCheckbox
            checked={draft.days_of_week_mode === "parent_choice"}
            onChange={(checked) =>
              update({
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
              update({
                capacity:
                  event.target.value === "" ? null : Number(event.target.value),
              })
            }
            placeholder={
              draft.is_required ? "Unbegrenzt (Pflicht)" : "Unbegrenzt"
            }
            className="mt-1 h-10 w-full rounded-lg border border-gray-200 px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:bg-gray-50 disabled:text-gray-400"
          />
          {draft.is_required && (
            <span className="mt-1 block text-xs text-gray-500">
              Pflichtangebote haben keine Platzbegrenzung.
            </span>
          )}
        </label>
        <label className="block">
          <span className="text-xs font-medium text-gray-700">
            Preis in Cent
          </span>
          <input
            type="number"
            name="price_cents"
            min={0}
            value={draft.price_cents ?? ""}
            onChange={(event) =>
              update({
                price_cents:
                  event.target.value === "" ? null : Number(event.target.value),
              })
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
              update({ sort_order: Number(event.target.value) })
            }
            className="mt-1 h-10 w-full rounded-lg border border-gray-200 px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          />
          <span className="mt-1 block text-xs text-gray-500">
            Kleinere Zahlen erscheinen im Elternformular weiter oben.
          </span>
        </label>
      </div>

      <fieldset className="rounded-xl border border-gray-200 p-4">
        <legend className="px-1 text-xs font-medium text-gray-700">
          Anzeige im Elternformular
        </legend>
        <div className="grid gap-2 sm:grid-cols-3">
          <CareOfferingCheckbox
            checked={draft.includes_holiday_care}
            onChange={(checked) => update({ includes_holiday_care: checked })}
            label="Ferienbetreuung"
            hint="Wird als Ferienangebot gekennzeichnet"
          />
          <CareOfferingCheckbox
            checked={draft.includes_lunch}
            onChange={(checked) => update({ includes_lunch: checked })}
            label="Mittagessen"
            hint="Zeigt Eltern, dass Essen enthalten ist"
          />
          <CareOfferingCheckbox
            checked={draft.is_active}
            onChange={(checked) => update({ is_active: checked })}
            label="Aktiv"
            hint="Nur aktive Angebote sind auswählbar"
          />
          <CareOfferingCheckbox
            checked={draft.is_required}
            onChange={(checked) =>
              update(
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
          disabled={saving || !draft.phase_id || !draft.name.trim()}
          className="inline-flex h-9 items-center justify-center rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
        >
          {saving ? "Speichert..." : editing ? "Speichern" : "Erstellen"}
        </button>
      </div>
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
          ? "border-[#83CD2D]/40 bg-[#83CD2D]/10"
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
            ? "border-[#83CD2D] bg-[#83CD2D] text-white"
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
        <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
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

      <label className="block">
        <span className="text-xs font-medium text-gray-700">Zielphase</span>
        <select
          value={targetPhaseId}
          onChange={(event) => setTargetPhaseId(event.target.value)}
          className="moto-select moto-content-surface mt-1 h-10 w-full rounded-lg border px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
        >
          {phases.map((phase) => (
            <option key={phase.id} value={phase.id}>
              {phase.name} ({KIND_LABELS[phase.kind]})
            </option>
          ))}
        </select>
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

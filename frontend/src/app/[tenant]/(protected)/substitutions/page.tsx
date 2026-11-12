"use client";

import { CalendarClock, Clock, Repeat2, UserPlus } from "lucide-react";
import { useSession } from "next-auth/react";
import { Suspense, useLayoutEffect, useMemo, useRef, useState } from "react";

import { AddSupervisorModal } from "~/components/active-supervisions/add-supervisor-modal";
import { RoleGuard } from "~/components/auth/role-guard";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { Button, ButtonLink } from "~/components/ui/button";
import {
  CollectionGrid,
  CollectionGridTile,
} from "~/components/ui/collection-grid";
import { CustomSelect } from "~/components/ui/custom-select";
import { EmptyState } from "~/components/ui/empty-state";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { ForbiddenPage } from "~/components/ui/forbidden-page";
import { SectionCard } from "~/components/ui/section-card";
import { Input } from "~/components/ui/input";
import { ConfirmationModal, Modal } from "~/components/ui/modal";
import { TenantPage } from "~/components/ui/tenant-page";
import { ListSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { hasEffectiveAdminScope, hasPermission } from "~/lib/auth-utils";
import { canGrantExternalViewAccess } from "~/lib/external-caregiver-access";
import { berlinTodayISO, formatDate } from "~/lib/date-helpers";
import {
  useRefreshAfterWrite,
  type RefreshAfterWrite,
} from "~/lib/hooks/use-refresh-after-write";
import { createLogger } from "~/lib/logger";
import { substitutionService } from "~/lib/substitution-api";
import type {
  RunningSupervision,
  ScheduleSubstitutionOverview,
  Substitution,
  SubstitutionOverview,
} from "~/lib/substitution-helpers";
import { useSWRAuth } from "~/lib/swr";
import { useOpenCareGroupMode } from "~/lib/tenant-context";
import { useTenantAwarePath } from "~/lib/tenant-path";
import { useTenantRouter } from "~/lib/tenant-router";

const logger = createLogger({ component: "SubstitutionsPage" });

const EMPTY_OVERVIEW: SubstitutionOverview = {
  groups: [],
  groupHandovers: [],
  targets: [],
  runningSupervisions: [],
};

type ScheduleSectionProps = Readonly<{
  overview: ScheduleSubstitutionOverview | undefined;
  canRead: boolean;
  canManage: boolean;
  hrefFor: (appointmentId?: string) => string;
}>;

type GroupSectionProps = Readonly<{
  overview: SubstitutionOverview;
  openCare: boolean;
  onAssign: () => void;
  onEnd: (handover: Substitution) => void;
}>;

type AssignmentFieldsProps = Readonly<{
  overview: SubstitutionOverview;
  admin: boolean;
  fieldError: (name: string) => string | undefined;
  values: {
    groupId: string;
    staffId: string;
    today: string;
    start: string;
    end: string;
  };
  setters: {
    group: (value: string) => void;
    staff: (value: string) => void;
    start: (value: string) => void;
    end: (value: string) => void;
  };
}>;

type GroupAssignmentModalProps = Readonly<{
  isOpen: boolean;
  overview: SubstitutionOverview;
  admin: boolean;
  action: ReturnType<typeof useAssignGroup>;
  onClose: () => void;
}>;

function RunningSection({
  rows,
  onAssign,
}: Readonly<{
  rows: RunningSupervision[];
  onAssign: (activeGroupId: string) => void;
}>) {
  if (rows.length === 0) {
    return (
      <EmptyState
        variant="compact"
        title="Keine laufenden Betreuungen"
        description="Zurzeit läuft keine Betreuung in Ihrem Sichtbereich."
      />
    );
  }
  return (
    <ul className="divide-y divide-gray-200">
      {rows.map((row) => (
        <RunningRow key={row.id} row={row} onAssign={onAssign} />
      ))}
    </ul>
  );
}

function RunningRow({
  row,
  onAssign,
}: Readonly<{
  row: RunningSupervision;
  onAssign: (activeGroupId: string) => void;
}>) {
  const details = [
    row.roomName,
    row.supervisors.map((staff) => staff.fullName).join(", "),
  ]
    .filter(Boolean)
    .join(" · ");
  return (
    <li className="flex flex-col gap-3 py-3 first:pt-0 last:pb-0 sm:flex-row sm:items-center sm:justify-between">
      <div className="min-w-0">
        <p className="truncate text-sm font-medium text-gray-900">{row.name}</p>
        <p className="mt-1 text-xs text-gray-500">{details}</p>
      </div>
      {row.canAssign ? (
        <Button
          type="button"
          variant="outline"
          size="md"
          onClick={() => onAssign(row.id)}
          tooltip="Öffnet ein Fenster, um eine weitere Person zur Aufsicht einzutragen."
        >
          <UserPlus className="h-4 w-4" aria-hidden="true" />
          Betreuer hinzufügen
        </Button>
      ) : (
        <p className="text-xs text-gray-500">
          Nur zuständige Personen können jemanden hinzufügen.
        </p>
      )}
    </li>
  );
}

function ScheduleSection({
  overview,
  canRead,
  canManage,
  hrefFor,
}: ScheduleSectionProps) {
  if (!canRead) {
    return (
      <EmptyState
        variant="compact"
        title="Keine Vertretungen verfügbar"
        description="Bitten Sie einen Admin um Zugriff auf Vertretungen."
      />
    );
  }
  const appointments = (overview?.appointments ?? []).filter((appointment) =>
    appointment.staff.some((staff) => staff.isAbsent || staff.isSubstitute),
  );
  if (appointments.length === 0) {
    return (
      <EmptyState
        variant="compact"
        title="Heute keine Vertretungen"
        description="Für heute ist keine Vertretung eingetragen."
      />
    );
  }
  return (
    <ul className="divide-y divide-gray-200">
      {appointments.map((appointment) => (
        <ScheduleRow
          key={appointment.id}
          appointment={appointment}
          canManage={canManage}
          href={hrefFor(appointment.id)}
        />
      ))}
    </ul>
  );
}

function ScheduleRow({
  appointment,
  canManage,
  href,
}: Readonly<{
  appointment: ScheduleSubstitutionOverview["appointments"][number];
  canManage: boolean;
  href: string;
}>) {
  const people = appointment.staff
    .filter((staff) => staff.isAbsent || staff.isSubstitute)
    .map((staff) => staff.name)
    .join(", ");
  return (
    <li className="flex flex-col gap-3 py-3 first:pt-0 last:pb-0 sm:flex-row sm:items-center sm:justify-between">
      <div className="min-w-0">
        <p className="truncate text-sm font-medium text-gray-900">
          {appointment.title}
        </p>
        <p className="mt-1 text-xs text-gray-500">
          {appointment.startTime}–{appointment.endTime} Uhr · {people}
        </p>
      </div>
      {canManage ? (
        <ButtonLink
          variant="outline"
          size="md"
          href={href}
          tooltip="Öffnet diesen Termin im Vertretungsplan von heute."
        >
          Vertretung eintragen
        </ButtonLink>
      ) : (
        <p className="text-xs text-gray-500">
          Ein Admin kann die Vertretung eintragen.
        </p>
      )}
    </li>
  );
}

function GroupSection(props: GroupSectionProps) {
  const { overview, openCare, onAssign, onEnd } = props;
  if (openCare) {
    return (
      <EmptyState
        variant="compact"
        title="Keine Gruppenübergabe nötig"
        description="Diese Schule arbeitet ohne feste Gruppen."
      />
    );
  }
  return (
    <div className="space-y-4">
      {overview.groups.length > 0 && overview.targets.length > 0 ? (
        <Button
          type="button"
          variant="outline"
          size="md"
          onClick={onAssign}
          tooltip="Öffnet ein Fenster, um eine Gruppe an eine andere Person zu geben."
        >
          Gruppe übergeben
        </Button>
      ) : overview.groups.length === 0 ? (
        <p className="text-sm text-gray-600">
          Keine Gruppe verfügbar. Ein Admin kann Gruppenleitungen verwalten.
        </p>
      ) : (
        <p className="text-sm text-gray-600">
          Keine andere Betreuungskraft verfügbar. Ein Admin kann Mitarbeitende
          verwalten.
        </p>
      )}
      {overview.groupHandovers.length === 0 ? (
        <EmptyState
          variant="compact"
          title="Keine Gruppenübergaben"
          description="Zurzeit ist keine Gruppe übergeben."
        />
      ) : (
        // Wenige Übergaben füllen als Raster die Breite der Karte, statt
        // als schmale Zeilen oben links zu stehen (#4365).
        <CollectionGrid as="ul">
          {overview.groupHandovers.map((handover) => (
            <HandoverRow key={handover.id} handover={handover} onEnd={onEnd} />
          ))}
        </CollectionGrid>
      )}
    </div>
  );
}

function HandoverRow({
  handover,
  onEnd,
}: Readonly<{
  handover: Substitution;
  onEnd: (handover: Substitution) => void;
}>) {
  return (
    <CollectionGridTile className="flex flex-col items-start gap-3">
      <div className="min-w-0">
        <p className="truncate text-sm font-medium text-gray-900">
          {handover.groupName}
        </p>
        <p className="mt-1 text-xs text-gray-500">
          Übergeben an {handover.substituteStaffName} · bis{" "}
          {formatDate(handover.endDate, true)}
        </p>
      </div>
      {handover.canEnd ? (
        <Button
          type="button"
          variant="outline_danger"
          size="md"
          onClick={() => onEnd(handover)}
          tooltip="moto fragt vorher nach und beendet dann die Übergabe der Gruppe."
        >
          Beenden
        </Button>
      ) : null}
    </CollectionGridTile>
  );
}

function LabeledSelect({
  id,
  name,
  label,
  value,
  error,
  onChange,
  options,
}: Readonly<{
  id: string;
  /** Backend-Feldname, damit ein Feldfehler das Feld markiert. */
  name: string;
  label: string;
  value: string;
  error: string | undefined;
  onChange: (value: string) => void;
  options: Array<{ id: string; name: string }>;
}>) {
  return (
    <CustomSelect
      id={id}
      name={name}
      label={label}
      error={error}
      value={value}
      onChange={onChange}
      placeholder={`${label} auswählen...`}
      options={options.map((option) => ({
        value: option.id,
        label: option.name,
      }))}
    />
  );
}

function LabeledDate({
  id,
  name,
  label,
  value,
  min,
  error,
  onChange,
}: Readonly<{
  id: string;
  /** Backend-Feldname, damit ein Feldfehler das Feld markiert. */
  name: string;
  label: string;
  value: string;
  min: string;
  error: string | undefined;
  onChange: (value: string) => void;
}>) {
  return (
    <Input
      id={id}
      name={name}
      label={label}
      error={error}
      type="date"
      min={min}
      value={value}
      onChange={(event) => onChange(event.target.value)}
    />
  );
}

function AdminPeriodFields({
  today,
  start,
  end,
  fieldError,
  onStart,
  onEnd,
}: Readonly<{
  today: string;
  start: string;
  end: string;
  fieldError: (name: string) => string | undefined;
  onStart: (value: string) => void;
  onEnd: (value: string) => void;
}>) {
  return (
    <div className="grid gap-4 sm:grid-cols-2">
      <LabeledDate
        id="handover-start"
        name="start_date"
        label="Startdatum"
        value={start}
        min={today}
        error={fieldError("start_date")}
        onChange={onStart}
      />
      <LabeledDate
        id="handover-end"
        name="end_date"
        label="Enddatum"
        value={end}
        min={start}
        error={fieldError("end_date")}
        onChange={onEnd}
      />
    </div>
  );
}

function AssignmentFields(props: AssignmentFieldsProps) {
  const { overview, admin, fieldError, values, setters } = props;
  return (
    <>
      <p className="text-sm text-gray-600">
        {admin
          ? "Wählen Sie die Gruppe, die Person und den Zeitraum."
          : "Sie können nur eigene Gruppen für heute übergeben."}
      </p>
      <LabeledSelect
        id="handover-group"
        name="group_id"
        label="Gruppe"
        value={values.groupId}
        error={fieldError("group_id")}
        onChange={setters.group}
        options={overview.groups}
      />
      <LabeledSelect
        id="handover-staff"
        name="target_staff_id"
        label="Betreuungskraft"
        value={values.staffId}
        error={fieldError("target_staff_id")}
        onChange={setters.staff}
        options={overview.targets.map((target) => ({
          id: target.id,
          name: target.fullName,
        }))}
      />
      {admin ? (
        <AdminPeriodFields
          today={values.today}
          start={values.start}
          end={values.end}
          fieldError={fieldError}
          onStart={setters.start}
          onEnd={setters.end}
        />
      ) : null}
    </>
  );
}

function AssignmentFooter({
  saving,
  disabled,
  disabledReason,
  onClose,
  onSubmit,
}: Readonly<{
  saving: boolean;
  disabled: boolean;
  /** Was im Formular noch fehlt; leer, solange nur gespeichert wird. */
  disabledReason?: string;
  onClose: () => void;
  onSubmit: () => void;
}>) {
  return (
    <>
      <Button
        type="button"
        variant="outline"
        size="md"
        onClick={onClose}
        disabled={saving}
        noTooltip="Schließt nur das Fenster, ohne etwas zu übergeben."
      >
        Abbrechen
      </Button>
      <Button
        type="button"
        size="md"
        onClick={onSubmit}
        disabled={disabled}
        disabledReason={disabledReason}
        isLoading={saving}
        loadingText="Wird übergeben…"
        tooltip="Macht die gewählte Person im Zeitraum für die Gruppe zuständig."
      >
        Zuweisen
      </Button>
    </>
  );
}

/** Grund für den gesperrten Knopf „Zuweisen“, der erste fehlende Schritt. */
function assignmentMissing(
  groupId: string,
  staffId: string,
  start: string,
  end: string,
): string | undefined {
  if (!groupId) return "Wählen Sie zuerst eine Gruppe aus.";
  if (!staffId) return "Wählen Sie zuerst eine Betreuungskraft aus.";
  if (end < start) return "Das Enddatum darf nicht vor dem Startdatum liegen.";
  return undefined;
}

function useAssignmentForm(
  onSubmit: (
    groupId: string,
    staffId: string,
    start: string,
    end: string,
  ) => void,
) {
  const today = berlinTodayISO();
  const [groupId, setGroupId] = useState("");
  const [staffId, setStaffId] = useState("");
  const [start, setStart] = useState(today);
  const [end, setEnd] = useState(today);
  const chooseStart = (value: string) => {
    setStart(value);
    if (end < value) setEnd(value);
  };
  return {
    values: { groupId, staffId, today, start, end },
    setters: {
      group: setGroupId,
      staff: setStaffId,
      start: chooseStart,
      end: setEnd,
    },
    submit: () => onSubmit(groupId, staffId, start, end),
    complete: Boolean(groupId && staffId && end >= start),
    missing: assignmentMissing(groupId, staffId, start, end),
  };
}

function GroupAssignmentModal(props: GroupAssignmentModalProps) {
  const { action } = props;
  // „Wiederholen“ übergibt mit dem AKTUELLEN Formularstand.
  const latestSubmitRef = useRef<() => void>(() => undefined);
  const form = useAssignmentForm((groupId, staffId, start, end) => {
    void action.assign(groupId, staffId, start, end, {
      onDone: props.onClose,
      retry: () => latestSubmitRef.current(),
    });
  });
  useLayoutEffect(() => {
    latestSubmitRef.current = form.submit;
  });
  const close = () => {
    if (!action.saving) props.onClose();
  };
  return (
    <Modal
      isOpen={props.isOpen}
      onClose={close}
      title="Gruppe übergeben"
      isDismissDisabled={action.saving}
      footer={
        <AssignmentFooter
          saving={action.saving}
          disabled={!form.complete || action.saving}
          disabledReason={form.missing}
          onClose={close}
          onSubmit={form.submit}
        />
      }
    >
      <div ref={action.formRef} className="space-y-5">
        <FormErrorAlert message={action.error} />
        <AssignmentFields
          overview={props.overview}
          admin={props.admin}
          fieldError={action.fieldError}
          values={form.values}
          setters={form.setters}
        />
      </div>
    </Modal>
  );
}

function prioritizeRunning(rows: RunningSupervision[]) {
  return [...rows].sort(
    (a, b) =>
      Number(b.isCurrentUserSupervising) - Number(a.isCurrentUserSupervising),
  );
}

function useScheduleData(
  session: Parameters<typeof hasPermission>[0],
  today: string,
) {
  const canRead = hasPermission(session, "schedules:read");
  const canManage = hasPermission(session, "schedules:manage");
  const result = useSWRAuth<ScheduleSubstitutionOverview>(
    canRead ? `substitution-schedule-${today}` : null,
    () => substitutionService.fetchScheduleOverview(today, today),
    { keepPreviousData: true },
  );
  return { ...result, canRead, canManage };
}

function useOverviewData(session: Parameters<typeof hasPermission>[0]) {
  const tenantPath = useTenantAwarePath();
  const today = berlinTodayISO();
  const {
    data: overview = EMPTY_OVERVIEW,
    isLoading,
    error,
    mutate,
  } = useSWRAuth<SubstitutionOverview>(
    "substitution-overview",
    () => substitutionService.fetchOverview(),
    { keepPreviousData: true },
  );
  const schedule = useScheduleData(session, today);
  const running = useMemo(
    () => prioritizeRunning(overview.runningSupervisions),
    [overview.runningSupervisions],
  );
  const scheduleHref = (id?: string) =>
    tenantPath(`/vertretung?d=${today}${id ? `&block=${id}` : ""}`);
  return {
    overview,
    isLoading,
    error,
    mutate,
    schedule: schedule.data,
    scheduleError: schedule.error,
    scheduleMutate: schedule.mutate,
    scheduleLoading: schedule.isLoading,
    running,
    scheduleHref,
    canReadSchedules: schedule.canRead,
    canManageSchedules: schedule.canManage,
  };
}

function refreshAfterMutation(refresh: () => Promise<unknown>) {
  // Bewusst still: die Änderung ist gespeichert und gemeldet. SWR fängt
  // Abruffehler selbst ab und zeigt sie über den Ladefehler der Seite; hier
  // landet nur ein unerwarteter Fehler, der den Erfolg nicht zurücknimmt.
  void refresh().catch((cause) => {
    logger.error("group_handover_refresh_failed", { error: String(cause) });
  });
}

/**
 * Fehlerweg eines Dialogs der Seite (#2516): Meldung und Feldfehler bleiben
 * im offenen Dialog, nie als Toast.
 */
function useDialogErrors() {
  const formRef = useRef<HTMLDivElement>(null);
  const formErrors = useApiFormError(formRef);
  return { formRef, formErrors };
}

function useAssignGroup(refresh: () => Promise<unknown>) {
  const toast = useToast();
  const [saving, setSaving] = useState(false);
  const { formRef, formErrors } = useDialogErrors();
  const assign = async (
    groupId: string,
    staffId: string,
    start: string,
    end: string,
    callbacks: { onDone: () => void; retry: () => void },
  ) => {
    setSaving(true);
    formErrors.clear();
    try {
      await substitutionService.createSubstitution(
        groupId,
        staffId,
        start,
        end,
      );
      toast.success("Die Gruppe wurde übergeben.");
      callbacks.onDone();
      refreshAfterMutation(refresh);
    } catch (cause) {
      logger.error("group_handover_assign_failed", { error: String(cause) });
      void formErrors.show(cause, {
        object: "die Gruppenübergabe",
        retry: callbacks.retry,
      });
    } finally {
      setSaving(false);
    }
  };
  return {
    assign,
    saving,
    formRef,
    error: formErrors.error,
    fieldError: formErrors.fieldError,
    clearError: formErrors.clear,
  };
}

function useEndGroup(refresh: () => Promise<unknown>) {
  const toast = useToast();
  const [saving, setSaving] = useState(false);
  const { formRef, formErrors } = useDialogErrors();
  const end = async (handover: Substitution, onDone: () => void) => {
    setSaving(true);
    formErrors.clear();
    try {
      await substitutionService.deleteSubstitution(handover.id);
      toast.success("Die Übergabe wurde beendet.");
      onDone();
      refreshAfterMutation(refresh);
    } catch (cause) {
      logger.error("group_handover_end_failed", { error: String(cause) });
      void formErrors.show(cause, {
        object: "die Übergabe",
        // Der Dialog ist noch offen und nennt dieselbe Übergabe.
        retry: () => void end(handover, onDone),
      });
    } finally {
      setSaving(false);
    }
  };
  return {
    end,
    saving,
    formRef,
    error: formErrors.error,
    clearError: formErrors.clear,
  };
}

function RunningCard({
  rows,
  onAssign,
}: Readonly<{ rows: RunningSupervision[]; onAssign: (id: string) => void }>) {
  return (
    <SectionCard
      title="Laufende Betreuungen"
      icon={Clock}
      className="flex h-full flex-col"
      bodyClassName="compact:mt-3 mt-4 flex min-h-0 flex-1 flex-col space-y-3"
    >
      <p className="text-sm text-gray-600">
        Hier sehen Sie laufende Aufsichten in Ihrem Sichtbereich.
      </p>
      <RunningSection rows={rows} onAssign={onAssign} />
    </SectionCard>
  );
}

function ScheduleCardContent({
  data,
}: Readonly<{ data: ReturnType<typeof useOverviewData> }>) {
  // Ladefehler vor Ort mit Wiederholen, nie als leere Liste (#2516).
  const loadError = useSwrLoadError(
    data.scheduleError,
    "die Liste der Vertretungen",
    () => data.scheduleMutate(),
  );
  if (data.scheduleError && loadError !== null) {
    return <LoadErrorAlert error={loadError} />;
  }
  // Bis der Katalogtext da ist, bleibt das Skelett stehen.
  if (data.scheduleLoading || data.scheduleError) {
    return (
      <SkeletonRegion label="Vertretungen werden geladen">
        <ListSkeleton rows={2} />
      </SkeletonRegion>
    );
  }
  return (
    <ScheduleSection
      overview={data.schedule}
      canRead={data.canReadSchedules}
      canManage={data.canManageSchedules}
      hrefFor={data.scheduleHref}
    />
  );
}

function ScheduleCard({
  data,
}: Readonly<{ data: ReturnType<typeof useOverviewData> }>) {
  return (
    <SectionCard
      title="Vertretungen heute"
      icon={CalendarClock}
      className="flex h-full flex-col"
      bodyClassName="compact:mt-3 mt-4 flex min-h-0 flex-1 flex-col space-y-3"
    >
      <p className="text-sm text-gray-600">
        Hier sehen Sie Vertretungen für heutige Termine.
      </p>
      <ScheduleCardContent data={data} />
      {data.canManageSchedules ? (
        <ButtonLink
          className="mt-auto self-start"
          variant="ghost"
          size="compact"
          href={data.scheduleHref()}
          tooltip="Öffnet den Vertretungsplan für heute."
        >
          Vertretungsplan öffnen
        </ButtonLink>
      ) : null}
    </SectionCard>
  );
}

function GroupCard({
  overview,
  openCare,
  onAssign,
  onEnd,
}: Readonly<{
  overview: SubstitutionOverview;
  openCare: boolean;
  onAssign: () => void;
  onEnd: (handover: Substitution) => void;
}>) {
  return (
    <SectionCard
      title="Gruppen"
      icon={Repeat2}
      className="flex h-full flex-col"
      bodyClassName="compact:mt-3 mt-4 flex min-h-0 flex-1 flex-col space-y-3"
    >
      <p className="text-sm text-gray-600">
        Hier verwalten Sie zeitlich begrenzte Gruppenübergaben.
      </p>
      <GroupSection
        overview={overview}
        openCare={openCare}
        onAssign={onAssign}
        onEnd={onEnd}
      />
    </SectionCard>
  );
}

function EndDialog({
  handover,
  action,
  onClose,
}: Readonly<{
  handover: Substitution | null;
  action: ReturnType<typeof useEndGroup>;
  onClose: () => void;
}>) {
  return (
    <ConfirmationModal
      isOpen={handover !== null}
      onClose={() => {
        if (!action.saving) onClose();
      }}
      onConfirm={() => {
        if (handover) void action.end(handover, onClose);
      }}
      title="Übergabe beenden?"
      confirmText="Beenden"
      cancelText="Abbrechen"
      isConfirmLoading={action.saving}
      loadingText="Wird beendet …"
      isDismissDisabled={action.saving}
    >
      <div ref={action.formRef} className="space-y-3 text-sm text-gray-600">
        <FormErrorAlert message={action.error} />
        <p>Die Person ist danach nicht mehr für diese Gruppe zuständig.</p>
        {handover ? (
          <p className="font-medium text-gray-900">
            {handover.groupName} · {handover.substituteStaffName}
          </p>
        ) : null}
      </div>
    </ConfirmationModal>
  );
}

type PageOverlaysProps = Readonly<{
  data: ReturnType<typeof useOverviewData>;
  /** Reloads after an added supervisor; never rejects (#6652). */
  onSupervisorAdded: RefreshAfterWrite;
  admin: boolean;
  canCreateExternalCaregiver: boolean;
  canGrantExternalView: boolean;
  assignOpen: boolean;
  runningId: string | null;
  ending: Substitution | null;
  assignAction: ReturnType<typeof useAssignGroup>;
  endAction: ReturnType<typeof useEndGroup>;
  closeAssign: () => void;
  closeRunning: () => void;
  closeEnding: () => void;
}>;

function PageOverlays(props: PageOverlaysProps) {
  const { data, assignAction, endAction } = props;
  return (
    <>
      {props.assignOpen ? (
        <GroupAssignmentModal
          isOpen
          overview={data.overview}
          admin={props.admin}
          action={assignAction}
          onClose={props.closeAssign}
        />
      ) : null}
      {props.runningId ? (
        <AddSupervisorModal
          activeGroupId={props.runningId}
          canCreateExternalCaregiver={props.canCreateExternalCaregiver}
          canGrantExternalView={props.canGrantExternalView}
          isOpen
          onClose={props.closeRunning}
          onAdded={props.onSupervisorAdded}
        />
      ) : null}
      <EndDialog
        handover={props.ending}
        action={endAction}
        onClose={props.closeEnding}
      />
    </>
  );
}

type OverviewContentProps = Readonly<{
  data: ReturnType<typeof useOverviewData>;
  openCare: boolean;
  onAssignGroup: () => void;
  onAssignRunning: (id: string) => void;
  onEndGroup: (handover: Substitution) => void;
}>;

function OverviewContent(props: OverviewContentProps) {
  const { error } = props.data;
  // Ablaufentscheidung, kein Text: ohne Leserecht gibt es keine Übersicht.
  if (error instanceof ApiError && error.status === 403) {
    return <ForbiddenPage embedded />;
  }
  if (error) {
    return (
      <OverviewLoadError error={error} retry={() => props.data.mutate()} />
    );
  }
  return (
    <>
      <RunningCard rows={props.data.running} onAssign={props.onAssignRunning} />
      <ScheduleCard data={props.data} />
      <GroupCard
        overview={props.data.overview}
        openCare={props.openCare}
        onAssign={props.onAssignGroup}
        onEnd={props.onEndGroup}
      />
    </>
  );
}

/** Ladefehler der Übersicht vor Ort, mit Wiederholen (#2516). */
function OverviewLoadError({
  error,
  retry,
}: Readonly<{ error: unknown; retry: () => unknown }>) {
  const loadError = useSwrLoadError(
    error,
    "die Übersicht der Übergaben",
    retry,
  );
  if (loadError === null) {
    // Bis der Katalogtext da ist: Skelett statt leerer Fläche.
    return (
      <SkeletonRegion label="Übergaben werden geladen">
        <ListSkeleton rows={6} />
      </SkeletonRegion>
    );
  }
  return <LoadErrorAlert error={loadError} />;
}

type LoadedPageProps = Readonly<{
  session: Parameters<typeof hasEffectiveAdminScope>[0];
  data: ReturnType<typeof useOverviewData>;
  openCare: boolean;
}>;

function usePageDialogs(data: ReturnType<typeof useOverviewData>) {
  const [assignOpen, setAssignOpen] = useState(false);
  const [runningId, setRunningId] = useState<string | null>(null);
  const [ending, setEnding] = useState<Substitution | null>(null);
  const assignAction = useAssignGroup(data.mutate);
  const endAction = useEndGroup(data.mutate);
  const openAssign = () => {
    assignAction.clearError();
    setAssignOpen(true);
  };
  const openEnd = (handover: Substitution) => {
    endAction.clearError();
    setEnding(handover);
  };
  return {
    assignOpen,
    runningId,
    ending,
    assignAction,
    endAction,
    openAssign,
    openEnd,
    setAssignOpen,
    setRunningId,
    setEnding,
  };
}

function LoadedPage({ session, data, openCare }: LoadedPageProps) {
  const dialogs = usePageDialogs(data);
  // Nachladen nach dem Eintragen einer Aufsicht: ein Fehler dabei steht über
  // der Liste, auch wenn der Dialog zu ist (#6652).
  const supervisorAfterWrite = useRefreshAfterWrite(
    "die Anzeige der Übergaben",
    data.mutate,
  );
  return (
    // Die Kopfkarte des Gerüsts ersetzt den früheren PageHeader; Dialoge
    // stehen im overlays-Slot, damit sie in jedem Zustand gemountet bleiben.
    <TenantPage
      title="Übergaben"
      overlays={
        <PageOverlays
          data={data}
          onSupervisorAdded={supervisorAfterWrite.refresh}
          admin={hasEffectiveAdminScope(session)}
          canCreateExternalCaregiver={hasPermission(session, "users:create")}
          canGrantExternalView={canGrantExternalViewAccess(session)}
          assignOpen={dialogs.assignOpen}
          runningId={dialogs.runningId}
          ending={dialogs.ending}
          assignAction={dialogs.assignAction}
          endAction={dialogs.endAction}
          closeAssign={() => dialogs.setAssignOpen(false)}
          closeRunning={() => dialogs.setRunningId(null)}
          closeEnding={() => dialogs.setEnding(null)}
        />
      }
    >
      <LoadErrorAlert error={supervisorAfterWrite.error} />
      <OverviewContent
        data={data}
        openCare={openCare}
        onAssignGroup={dialogs.openAssign}
        onAssignRunning={dialogs.setRunningId}
        onEndGroup={dialogs.openEnd}
      />
    </TenantPage>
  );
}

function SubstitutionPageContent() {
  const router = useTenantRouter();
  const openCare = useOpenCareGroupMode();
  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated: () => router.push("/"),
  });
  const data = useOverviewData(session);
  if (status === "loading" || data.isLoading) return <LoadingFallback />;
  return <LoadedPage session={session} data={data} openCare={openCare} />;
}

function LoadingFallback() {
  return (
    <TenantPage
      title="Übergaben"
      // Das Gerüst trägt die vorlesbare Ankündigung selbst — ein zweites
      // role="status" darunter wäre eine doppelte Meldung.
      loading={<ListSkeleton rows={6} />}
      loadingLabel="Übergaben werden geladen"
    />
  );
}

export default function SubstitutionPage() {
  return (
    <RoleGuard
      variant="staffOrAdmin"
      // Die Übersicht liest das Backend mit substitutions:read; so öffnet sie
      // auch einer Betreuungsrolle, die die Schule selbst anlegt (#3469).
      permission="substitutions:read"
      fallback={<LoadingFallback />}
    >
      <Suspense fallback={<LoadingFallback />}>
        <SubstitutionPageContent />
      </Suspense>
    </RoleGuard>
  );
}

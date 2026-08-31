"use client";

import { CalendarClock, Clock, Repeat2, UserPlus } from "lucide-react";
import { useSession } from "next-auth/react";
import { Suspense, useMemo, useState } from "react";

import { AddSupervisorModal } from "~/components/active-supervisions/add-supervisor-modal";
import { Alert } from "~/components/ui/alert";
import { Button, ButtonLink } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { EmptyState } from "~/components/ui/empty-state";
import { InfoCard } from "~/components/ui/info-card";
import { Input } from "~/components/ui/input";
import { ConfirmationModal, Modal } from "~/components/ui/modal";
import { PageHeader } from "~/components/ui/page-header/PageHeader";
import { ListSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
import { useToast } from "~/contexts/ToastContext";
import { readableApiMessage } from "~/lib/api-error-message";
import { hasEffectiveAdminScope, hasPermission } from "~/lib/auth-utils";
import { berlinTodayISO, formatDate } from "~/lib/date-helpers";
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
  saving: boolean;
  error: string | null;
  onClose: () => void;
  onSubmit: (
    groupId: string,
    staffId: string,
    start: string,
    end: string,
  ) => void;
}>;

function EmptySection({
  title,
  description,
}: Readonly<{ title: string; description: string }>) {
  return (
    <EmptyState variant="compact" title={title} description={description} />
  );
}

function RunningSection({
  rows,
  onAssign,
}: Readonly<{
  rows: RunningSupervision[];
  onAssign: (activeGroupId: string) => void;
}>) {
  if (rows.length === 0) {
    return (
      <EmptySection
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
      <EmptySection
        title="Keine Terminvertretungen verfügbar"
        description="Bitten Sie einen Admin um Zugriff auf Terminvertretungen."
      />
    );
  }
  const appointments = (overview?.appointments ?? []).filter((appointment) =>
    appointment.staff.some((staff) => staff.isAbsent || staff.isSubstitute),
  );
  if (appointments.length === 0) {
    return (
      <EmptySection
        title="Heute keine Terminvertretungen"
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
        <ButtonLink variant="outline" size="md" href={href}>
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
      <EmptySection
        title="Keine Gruppenübergabe nötig"
        description="Diese Schule arbeitet ohne feste Gruppen."
      />
    );
  }
  return (
    <div className="space-y-4">
      {overview.groups.length > 0 && overview.targets.length > 0 ? (
        <Button type="button" variant="outline" size="md" onClick={onAssign}>
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
        <EmptySection
          title="Keine Gruppenübergaben"
          description="Zurzeit ist keine Gruppe übergeben."
        />
      ) : (
        <ul className="divide-y divide-gray-200">
          {overview.groupHandovers.map((handover) => (
            <HandoverRow key={handover.id} handover={handover} onEnd={onEnd} />
          ))}
        </ul>
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
    <li className="flex flex-col gap-3 py-3 first:pt-0 last:pb-0 sm:flex-row sm:items-center sm:justify-between">
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
        >
          Beenden
        </Button>
      ) : null}
    </li>
  );
}

function LabeledSelect({
  id,
  label,
  value,
  onChange,
  options,
}: Readonly<{
  id: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: Array<{ id: string; name: string }>;
}>) {
  return (
    <div>
      <label
        id={`${id}-label`}
        htmlFor={id}
        className="mb-2 block text-sm font-medium text-gray-700"
      >
        {label}
      </label>
      <CustomSelect
        id={id}
        ariaLabelledBy={`${id}-label`}
        value={value}
        onChange={onChange}
        placeholder={`${label} auswählen...`}
        options={options.map((option) => ({
          value: option.id,
          label: option.name,
        }))}
      />
    </div>
  );
}

function LabeledDate({
  id,
  label,
  value,
  min,
  onChange,
}: Readonly<{
  id: string;
  label: string;
  value: string;
  min: string;
  onChange: (value: string) => void;
}>) {
  return (
    <div>
      <label
        htmlFor={id}
        className="mb-2 block text-sm font-medium text-gray-700"
      >
        {label}
      </label>
      <Input
        id={id}
        name={id}
        type="date"
        min={min}
        value={value}
        onChange={(event) => onChange(event.target.value)}
      />
    </div>
  );
}

function AdminPeriodFields({
  today,
  start,
  end,
  onStart,
  onEnd,
}: Readonly<{
  today: string;
  start: string;
  end: string;
  onStart: (value: string) => void;
  onEnd: (value: string) => void;
}>) {
  return (
    <div className="grid gap-4 sm:grid-cols-2">
      <LabeledDate
        id="handover-start"
        label="Startdatum"
        value={start}
        min={today}
        onChange={onStart}
      />
      <LabeledDate
        id="handover-end"
        label="Enddatum"
        value={end}
        min={start}
        onChange={onEnd}
      />
    </div>
  );
}

function AssignmentFields(props: AssignmentFieldsProps) {
  const { overview, admin, values, setters } = props;
  return (
    <>
      <p className="text-sm text-gray-600">
        {admin
          ? "Wählen Sie die Gruppe, die Person und den Zeitraum."
          : "Sie können nur eigene Gruppen für heute übergeben."}
      </p>
      <LabeledSelect
        id="handover-group"
        label="Gruppe"
        value={values.groupId}
        onChange={setters.group}
        options={overview.groups}
      />
      <LabeledSelect
        id="handover-staff"
        label="Betreuungskraft"
        value={values.staffId}
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
  onClose,
  onSubmit,
}: Readonly<{
  saving: boolean;
  disabled: boolean;
  onClose: () => void;
  onSubmit: () => void;
}>) {
  return (
    <>
      <Button
        type="button"
        variant="secondary"
        size="md"
        onClick={onClose}
        disabled={saving}
      >
        Abbrechen
      </Button>
      <Button
        type="button"
        size="md"
        onClick={onSubmit}
        disabled={disabled}
        isLoading={saving}
        loadingText="Wird übergeben…"
      >
        Zuweisen
      </Button>
    </>
  );
}

function useAssignmentForm(onSubmit: GroupAssignmentModalProps["onSubmit"]) {
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
  };
}

function GroupAssignmentModal(props: GroupAssignmentModalProps) {
  const form = useAssignmentForm(props.onSubmit);
  const close = () => {
    if (!props.saving) props.onClose();
  };
  return (
    <Modal
      isOpen={props.isOpen}
      onClose={close}
      title="Gruppe übergeben"
      isDismissDisabled={props.saving}
      footer={
        <AssignmentFooter
          saving={props.saving}
          disabled={!form.complete || props.saving}
          onClose={close}
          onSubmit={form.submit}
        />
      }
    >
      <div className="space-y-5">
        {props.error ? <Alert type="error" message={props.error} /> : null}
        <AssignmentFields
          overview={props.overview}
          admin={props.admin}
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
    scheduleLoading: schedule.isLoading,
    running,
    scheduleHref,
    canReadSchedules: schedule.canRead,
    canManageSchedules: schedule.canManage,
  };
}

function refreshAfterMutation(refresh: () => Promise<unknown>) {
  void refresh().catch((cause) => {
    logger.error("group_handover_refresh_failed", { error: String(cause) });
  });
}

function useAssignGroup(refresh: () => Promise<unknown>) {
  const toast = useToast();
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const assign = async (
    groupId: string,
    staffId: string,
    start: string,
    end: string,
    onDone: () => void,
  ) => {
    setSaving(true);
    setError(null);
    try {
      await substitutionService.createSubstitution(
        groupId,
        staffId,
        start,
        end,
      );
      toast.success("Die Gruppe wurde übergeben.");
      onDone();
      refreshAfterMutation(refresh);
    } catch (cause) {
      logger.error("group_handover_assign_failed", { error: String(cause) });
      setError(
        readableApiMessage(cause) ??
          "Die Gruppe konnte nicht übergeben werden.",
      );
    } finally {
      setSaving(false);
    }
  };
  return { assign, saving, error, clearError: () => setError(null) };
}

function useEndGroup(refresh: () => Promise<unknown>) {
  const toast = useToast();
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const end = async (handover: Substitution, onDone: () => void) => {
    setSaving(true);
    setError(null);
    try {
      await substitutionService.deleteSubstitution(handover.id);
      toast.success("Die Übergabe wurde beendet.");
      onDone();
      refreshAfterMutation(refresh);
    } catch (cause) {
      logger.error("group_handover_end_failed", { error: String(cause) });
      setError(
        readableApiMessage(cause) ??
          "Die Übergabe konnte nicht beendet werden.",
      );
    } finally {
      setSaving(false);
    }
  };
  return { end, saving, error, clearError: () => setError(null) };
}

function RunningCard({
  rows,
  onAssign,
}: Readonly<{ rows: RunningSupervision[]; onAssign: (id: string) => void }>) {
  return (
    <InfoCard
      title="Laufende Betreuungen"
      icon={<Clock className="h-5 w-5 text-gray-600" aria-hidden="true" />}
    >
      <p className="text-sm text-gray-600">
        Hier sehen Sie laufende Aufsichten in Ihrem Sichtbereich.
      </p>
      <RunningSection rows={rows} onAssign={onAssign} />
    </InfoCard>
  );
}

function ScheduleCardContent({
  data,
}: Readonly<{ data: ReturnType<typeof useOverviewData> }>) {
  if (data.scheduleError) {
    return (
      <Alert
        type="error"
        message="Terminvertretungen konnten nicht geladen werden."
      />
    );
  }
  if (data.scheduleLoading) {
    return (
      <SkeletonRegion label="Terminvertretungen werden geladen">
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
    <InfoCard
      title="Termine"
      icon={
        <CalendarClock className="h-5 w-5 text-gray-600" aria-hidden="true" />
      }
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
        >
          Alle Termine öffnen
        </ButtonLink>
      ) : null}
    </InfoCard>
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
    <InfoCard
      title="Gruppen"
      icon={<Repeat2 className="h-5 w-5 text-gray-600" aria-hidden="true" />}
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
    </InfoCard>
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
      isDismissDisabled={action.saving}
    >
      <div className="space-y-3 text-sm text-gray-600">
        {action.error ? <Alert type="error" message={action.error} /> : null}
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
  admin: boolean;
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
          saving={assignAction.saving}
          error={assignAction.error}
          onClose={props.closeAssign}
          onSubmit={(...args) =>
            void assignAction.assign(...args, props.closeAssign)
          }
        />
      ) : null}
      {props.runningId ? (
        <AddSupervisorModal
          activeGroupId={props.runningId}
          isOpen
          onClose={props.closeRunning}
          onAdded={data.mutate}
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
  if (props.data.error) {
    return (
      <Alert
        type="error"
        message="Vertretungen konnten nicht geladen werden. Bitte laden Sie die Seite neu."
      />
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
  return (
    <>
      <PageHeader title="Vertretungen" concept="groupAccess" />
      <div className="space-y-4">
        <OverviewContent
          data={data}
          openCare={openCare}
          onAssignGroup={dialogs.openAssign}
          onAssignRunning={dialogs.setRunningId}
          onEndGroup={dialogs.openEnd}
        />
      </div>
      <PageOverlays
        data={data}
        admin={hasEffectiveAdminScope(session)}
        assignOpen={dialogs.assignOpen}
        runningId={dialogs.runningId}
        ending={dialogs.ending}
        assignAction={dialogs.assignAction}
        endAction={dialogs.endAction}
        closeAssign={() => dialogs.setAssignOpen(false)}
        closeRunning={() => dialogs.setRunningId(null)}
        closeEnding={() => dialogs.setEnding(null)}
      />
    </>
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
    <SkeletonRegion label="Vertretungen werden geladen">
      <ListSkeleton rows={6} />
    </SkeletonRegion>
  );
}

export default function SubstitutionPage() {
  return (
    <Suspense fallback={<LoadingFallback />}>
      <SubstitutionPageContent />
    </Suspense>
  );
}

"use client";

import {
  type ReactNode,
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";
import { useSWRConfig } from "swr";
import {
  CalendarClock,
  CalendarPlus,
  Check,
  Pencil,
  Thermometer,
  Trash2,
} from "lucide-react";

import {
  DenyAbsenceModal,
  QuestionAbsenceModal,
} from "~/components/staff/absence-decision-modals";
import {
  SickReportModal,
  type SickReportStaff,
} from "~/components/staff/sick-report-modal";
import { AbsenceRequestRow } from "~/components/staff/absence-request-row";
import { AbsenceBookingModal } from "~/components/staff/absence-booking-modal";
import { CustomAllowanceEditForm } from "~/components/staff/custom-allowance-editor";
import { CatalogManageLink } from "~/components/database/catalog/catalog-manage-link";
import {
  ABSENCE_TYPE_HEX,
  ABSENCE_TYPE_LABEL,
  absenceRowActionNoun,
  absenceRowLabel,
  absenceStatusMeta,
  dayCountFor as sharedDayCountFor,
  dispatchAbsencesRefresh,
  formatAbsenceRange,
  formatDayCount,
} from "~/lib/absence-helpers";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { Button } from "~/components/ui/button";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { ISODatePicker } from "~/components/ui/date-picker";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { EditActions } from "~/components/ui/edit-actions";
import { EmptyState } from "~/components/ui/empty-state";
import { useFormError } from "~/components/ui/form-error";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { Modal } from "~/components/ui/modal";
import {
  CardGridSkeleton,
  ListSkeleton,
  SkeletonRegion,
} from "~/components/ui/page-skeletons";
import { SectionCard } from "~/components/ui/section-card";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { StatusBadge } from "~/components/ui/status-badge";
import { StatusColorBadge } from "~/components/ui/status-color-badge";
import { Textarea } from "~/components/ui/textarea";
import { useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  formatDate,
  berlinTodayISO,
  parseISODate,
  todayISO,
  toISODate,
} from "~/lib/date-helpers";
import {
  staffAbsenceService,
  type StaffAbsenceRow,
  type StaffVacationOpening,
  type StaffVacationQuotaSummary,
} from "~/lib/staff-api";
import { useTenantMutateMatching } from "~/lib/swr";
import {
  VERTRETUNG_GAPS_KEY_PREFIX,
  VERTRETUNG_WEEK_KEY_PREFIX,
} from "~/lib/timetable-helpers";
import {
  absenceTypeService,
  type AbsenceType,
  type AbsenceTypeAllowanceSummary,
} from "~/lib/absence-type-api";

const logger = createLogger({ component: "AbwesenheitenTab" });

const VACATION_WORKFLOW_STATUSES = new Set([
  "requested",
  "question",
  "approved",
  "declined",
  "canceled",
]);

const ONE_DECIMAL_INPUT_PATTERN = /^[+-]?(?:\d+(?:[,.]\d)?|[,.]\d)$/;

function parseOneDecimalInput(value: string): number | null {
  const normalized = value.trim();
  if (!ONE_DECIMAL_INPUT_PATTERN.test(normalized)) return null;
  const parsed = Number(normalized.replace(",", "."));
  return Number.isFinite(parsed) ? parsed : null;
}

function formatRange(start: string, end: string): string {
  return formatAbsenceRange(start, end);
}

function dayCountFor(row: StaffAbsenceRow): number {
  return sharedDayCountFor({
    workingDays: row.working_days,
    dateStart: row.date_start,
    dateEnd: row.date_end,
    halfDay: row.half_day,
    startHalfDay: row.start_half_day,
    endHalfDay: row.end_half_day,
    hasBoundaryFields:
      row.start_half_day !== undefined || row.end_half_day !== undefined,
  });
}

function statusMeta(status: string) {
  return absenceStatusMeta(status);
}

function pendingAbsences(absences: StaffAbsenceRow[]): StaffAbsenceRow[] {
  return absences.filter(
    (absence) =>
      absence.status === "requested" || absence.status === "question",
  );
}

function upcomingAbsences(absences: StaffAbsenceRow[]): StaffAbsenceRow[] {
  const today = todayISO();
  return absences.filter(
    (absence) =>
      (absence.status === "approved" || absence.status === "reported") &&
      absence.date_end >= today,
  );
}

function historicalAbsences(absences: StaffAbsenceRow[]): StaffAbsenceRow[] {
  const today = todayISO();
  return absences
    .filter(
      (absence) =>
        absence.status === "declined" ||
        absence.status === "canceled" ||
        ((absence.status === "approved" || absence.status === "reported") &&
          absence.date_end < today),
    )
    .sort((left, right) => (left.date_start < right.date_start ? 1 : -1));
}

export function isVacationWorkflowAbsence(row: StaffAbsenceRow): boolean {
  return (
    row.absence_type === "vacation" &&
    VACATION_WORKFLOW_STATUSES.has(row.status)
  );
}

function TabLoadingBoundary({
  loading,
  children,
}: {
  readonly loading: boolean;
  readonly children: ReactNode;
}) {
  if (loading) {
    return (
      <SkeletonRegion
        label="Abwesenheiten werden geladen"
        className="space-y-5"
      >
        <CardGridSkeleton
          cards={4}
          rowsPerCard={1}
          className="grid grid-cols-2 gap-4 sm:grid-cols-4"
        />
        <ListSkeleton rows={5} avatar={false} />
      </SkeletonRegion>
    );
  }
  return children;
}

export function AbwesenheitenTab({
  staffId,
  canEdit,
  canEditQuota,
  canManageSickReports,
  staff,
}: {
  readonly staffId: string;
  readonly canEdit: boolean;
  // Der Anspruch hängt an einer eigenen Berechtigung: PUT
  // /api/staff/{id}/vacation/quota verlangt time_tracking:manage, nicht die
  // Antragsentscheidung (vacation:approve darf nur lesen, #2906).
  readonly canEditQuota: boolean;
  readonly canManageSickReports: boolean;
  // Passed in from the staff detail page so the booking and sick-report
  // dialogs have the person's name. Optional so the tab still renders
  // (without those actions) where the staff object is not available (#1843).
  readonly staff?: SickReportStaff;
}) {
  const toast = useToast();
  const currentYear = Number.parseInt(berlinTodayISO().slice(0, 4), 10);
  // Ein Jahr für alle Kontingente und die Listen darunter (#3256): im
  // Dezember lässt sich so der Anspruch fürs nächste Jahr eintragen.
  const [year, setYear] = useState(currentYear);
  const [quota, setQuota] = useState<StaffVacationQuotaSummary | null>(null);
  const [absenceTypes, setAbsenceTypes] = useState<AbsenceType[]>([]);
  const [customAllowances, setCustomAllowances] = useState<
    { type: AbsenceType; summary: AbsenceTypeAllowanceSummary }[]
  >([]);
  // Welche Kontingent-Karte gerade bearbeitet wird: "vacation" oder die ID
  // der eigenen Art (BAUARTEN-SPEC Bauart 2 Regel 3, #3119).
  const [editing, setEditing] = useState<string | null>(null);
  const [bookingOpen, setBookingOpen] = useState(false);
  const [absences, setAbsences] = useState<StaffAbsenceRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [pendingActionId, setPendingActionId] = useState<number | null>(null);
  const [denyModal, setDenyModal] = useState<StaffAbsenceRow | null>(null);
  const [questionModal, setQuestionModal] = useState<StaffAbsenceRow | null>(
    null,
  );
  const [openingModal, setOpeningModal] = useState(false);
  // Vacation takeover deletion (#2132) runs through the shared
  // ConfirmDeleteModal at tab level instead of nesting a dialog inside the
  // takeover modal.
  const [openingDeleteTarget, setOpeningDeleteTarget] =
    useState<StaffVacationOpening | null>(null);
  // Snapshot of the staff identity taken when the modal opens: the live
  // `staff` prop can flip to undefined while onCreated's cache invalidation
  // refetches the page data, and a conditional `{staff && <Modal/>}` render
  // would then unmount the modal mid-success-state (#1843).
  const [sickStaff, setSickStaff] = useState<SickReportStaff | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<StaffAbsenceRow | null>(
    null,
  );
  const [deleteLoading, setDeleteLoading] = useState(false);
  const [deleteError, setDeleteError] = useState("");
  const { mutate: swrMutate } = useSWRConfig();

  // A sick report / its deletion cascades into the Dienst- and
  // Vertretungsplan, so revalidate those caches alongside the local reload.
  const refreshPlanCaches = useTenantMutateMatching([
    "dienstplan-overview-",
    "dienstplan-shifts-",
    VERTRETUNG_WEEK_KEY_PREFIX,
    VERTRETUNG_GAPS_KEY_PREFIX,
  ]);

  // silent skips the loading flag: the loading boundary unmounts the
  // whole tab subtree including any open modal, which would wipe the
  // SickReportModal's success state right after a report (#1843).
  const reload = useCallback(
    async (opts?: { silent?: boolean }) => {
      if (!opts?.silent) {
        setLoading(true);
      }
      try {
        const [q, abs, types] = await Promise.all([
          staffAbsenceService.getVacationQuota(staffId, year),
          staffAbsenceService.getAbsences(
            staffId,
            `${year}-01-01`,
            `${year}-12-31`,
          ),
          staff ? absenceTypeService.getAbsenceTypes() : Promise.resolve([]),
        ]);
        const allowanceTypes = types.filter((type) => type.allowanceEnabled);
        const summaries = await Promise.all(
          allowanceTypes.map((type) =>
            absenceTypeService.getAllowance(type.id, staffId, year),
          ),
        );
        setQuota(q);
        setAbsences(abs);
        setAbsenceTypes(types);
        setCustomAllowances(
          allowanceTypes.map((type, index) => ({
            type,
            summary: summaries[index]!,
          })),
        );
        // Page-level tab badge SWR (staff-pending-absences-${staffId}) lives
        // outside this component and otherwise stays stale after approvals,
        // denials and stornos. useSWRAuth prefixes every key with the tenant
        // slug ("phoenix:staff-pending-absences-…"), so a plain startsWith
        // never matches, we use includes for the cache hit.
        swrMutate(
          (key) =>
            typeof key === "string" && key.includes("staff-pending-absences-"),
        );
        // Sidebar pending counter (#1419) listens for this event.
        dispatchAbsencesRefresh();
      } catch (err) {
        logger.error("load_failed", {
          staff_id: staffId,
          error: err instanceof Error ? err.message : String(err),
        });
        toast.error("Daten konnten nicht geladen werden.");
      } finally {
        setLoading(false);
      }
    },
    [staff, staffId, year, swrMutate, toast],
  );

  useEffect(() => {
    // Ein Jahreswechsel lädt still nach: der Reiter bleibt stehen.
    void reload({ silent: true });
  }, [reload]);

  const pending = useMemo(() => pendingAbsences(absences), [absences]);
  const upcoming = useMemo(() => upcomingAbsences(absences), [absences]);
  const history = useMemo(() => historicalAbsences(absences), [absences]);

  const handleApprove = async (row: StaffAbsenceRow) => {
    setPendingActionId(row.id);
    try {
      await staffAbsenceService.approve(row.id);
      toast.success("Antrag genehmigt.");
      await reload();
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Genehmigung fehlgeschlagen.",
      );
    } finally {
      setPendingActionId(null);
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleteLoading(true);
    setDeleteError("");
    try {
      await staffAbsenceService.deleteAbsence(staffId, deleteTarget.id);
      toast.success(`${absenceRowActionNoun(deleteTarget)} gelöscht.`);
      setDeleteTarget(null);
      await Promise.all([refreshPlanCaches(), reload()]);
    } catch (err) {
      setDeleteError(
        err instanceof Error ? err.message : "Löschen fehlgeschlagen.",
      );
    } finally {
      setDeleteLoading(false);
    }
  };

  const afterBooking = async () => {
    await Promise.all([refreshPlanCaches(), reload({ silent: true })]);
  };

  const content = (
    <div className="space-y-5">
      <SectionCard
        title="Kontingente"
        headingLevel={3}
        description="Wie viele Tage diese Person je Art hat und noch übrig sind."
        actions={
          canManageSickReports && staff ? (
            <div className="flex flex-wrap items-center gap-2">
              <Button
                type="button"
                variant="primary"
                size="md"
                onClick={() => setBookingOpen(true)}
              >
                <CalendarPlus className="mr-1.5 h-4 w-4" aria-hidden />
                Abwesenheit eintragen
              </Button>
              <Button
                type="button"
                variant="outline"
                size="md"
                onClick={() => setSickStaff(staff)}
              >
                <Thermometer className="mr-1.5 h-4 w-4" aria-hidden />
                Krank melden
              </Button>
            </div>
          ) : undefined
        }
        bodyClassName="mt-4 space-y-4"
      >
        <div className="flex flex-wrap items-center justify-between gap-3">
          <SegmentedControl
            ariaLabel="Kalenderjahr"
            items={[currentYear - 1, currentYear, currentYear + 1].map(
              (value) => ({ value: String(value), label: String(value) }),
            )}
            value={String(year)}
            onChange={(value) => {
              setEditing(null);
              setYear(Number(value));
            }}
          />
          {canEditQuota ? (
            <CatalogManageLink
              href="/database/absence-types"
              label="Abwesenheitsarten verwalten"
            />
          ) : null}
        </div>
        <div className="grid gap-3 lg:grid-cols-2">
          <AllowanceCard
            title="Urlaub"
            year={year}
            entitled={
              (quota?.entitled_days ?? 0) + (quota?.carryover_days ?? 0)
            }
            entitledHint={
              (quota?.carryover_days ?? 0) > 0
                ? `${formatNumber(quota?.entitled_days ?? 0)} + ${formatNumber(quota?.carryover_days ?? 0)} aus dem Vorjahr`
                : undefined
            }
            taken={(quota?.taken_days ?? 0) + (quota?.taken_before_days ?? 0)}
            takenHint={
              (quota?.taken_before_days ?? 0) > 0
                ? `davon ${formatDayCount(quota?.taken_before_days ?? 0)} vor moto`
                : undefined
            }
            reserved={quota?.reserved_days ?? 0}
            remaining={quota?.remaining_days ?? 0}
            onEdit={
              canEditQuota && quota ? () => setEditing("vacation") : undefined
            }
            extraAction={
              canManageSickReports && quota ? (
                <Button
                  type="button"
                  variant="ghost"
                  size="compact"
                  onClick={() => setOpeningModal(true)}
                >
                  <CalendarClock className="mr-1.5 h-3.5 w-3.5" aria-hidden />
                  Urlaubs-Übernahme
                </Button>
              ) : null
            }
            editForm={
              editing === "vacation" && quota ? (
                <QuotaEditForm
                  staffId={staffId}
                  quota={quota}
                  year={year}
                  onCancel={() => setEditing(null)}
                  onSaved={async () => {
                    setEditing(null);
                    await reload({ silent: true });
                  }}
                />
              ) : null
            }
            footer={
              quota?.opening ? (
                <VacationOpeningSummary opening={quota.opening} />
              ) : null
            }
          />
          {customAllowances.map((entry) => (
            <AllowanceCard
              key={entry.type.id}
              title={entry.type.name}
              year={year}
              retired={!entry.type.isActive}
              entitled={entry.summary.entitledDays}
              taken={entry.summary.takenDays}
              reserved={entry.summary.reservedDays}
              remaining={entry.summary.remainingDays}
              footer={<AllowanceExpiryNote summary={entry.summary} />}
              onEdit={
                canEditQuota ? () => setEditing(entry.type.id) : undefined
              }
              editForm={
                editing === entry.type.id ? (
                  <CustomAllowanceEditForm
                    staffId={staffId}
                    year={year}
                    type={entry.type}
                    summary={entry.summary}
                    onCancel={() => setEditing(null)}
                    onSaved={async () => {
                      setEditing(null);
                      await reload({ silent: true });
                    }}
                  />
                ) : null
              }
            />
          ))}
        </div>
        {staff && canEditQuota && customAllowances.length === 0 ? (
          <p className="text-sm text-gray-500">
            Regenerationstage oder Krank-Urlaubstage zählen? Legen Sie dafür
            unter „Abwesenheitsarten verwalten“ eine Art mit Kontingent an.
          </p>
        ) : null}
      </SectionCard>

      <PendingAbsences
        rows={pending}
        canEdit={canEdit}
        pendingActionId={pendingActionId}
        onApprove={handleApprove}
        onDeny={setDenyModal}
        onQuestion={setQuestionModal}
      />

      {/* Upcoming approved absences */}
      <SectionCard title="Kommende Abwesenheiten" headingLevel={3}>
        {upcoming.length === 0 ? (
          <EmptyState title="Keine geplanten Abwesenheiten." />
        ) : (
          <ul className="space-y-2">
            {upcoming.map((row) => (
              <AbsenceRow
                key={row.id}
                row={row}
                onDelete={
                  canManageSickReports && !isVacationWorkflowAbsence(row)
                    ? () => setDeleteTarget(row)
                    : undefined
                }
              />
            ))}
          </ul>
        )}
      </SectionCard>

      {/* Past history */}
      <SectionCard title={`Historie ${year}`} headingLevel={3}>
        {history.length === 0 ? (
          <EmptyState title="Keine vergangenen oder abgelehnten Abwesenheiten." />
        ) : (
          <ul className="space-y-2">
            {history.slice(0, 20).map((row) => (
              <AbsenceRow
                key={row.id}
                row={row}
                onDelete={
                  canManageSickReports && !isVacationWorkflowAbsence(row)
                    ? () => setDeleteTarget(row)
                    : undefined
                }
              />
            ))}
          </ul>
        )}
      </SectionCard>

      {/* Modals */}
      {sickStaff && (
        <SickReportModal
          isOpen
          staff={sickStaff}
          onClose={() => setSickStaff(null)}
          onCreated={() => {
            Promise.all([refreshPlanCaches(), reload({ silent: true })]).catch(
              (err: unknown) => {
                logger.error("post_create_refresh_failed", {
                  staff_id: staffId,
                  error: err instanceof Error ? err.message : String(err),
                });
              },
            );
          }}
        />
      )}
      <ConfirmDeleteModal
        isOpen={deleteTarget !== null}
        title={`${deleteTarget ? absenceRowActionNoun(deleteTarget) : "Abwesenheit"} löschen`}
        description={
          <>
            Die Abwesenheit{" "}
            <strong>
              {deleteTarget
                ? formatRange(deleteTarget.date_start, deleteTarget.date_end)
                : ""}
            </strong>{" "}
            wird gelöscht.
            {deleteTarget?.absence_type === "sick" && (
              <>
                {" "}
                Stornierte Schichten und als abwesend markierte Betreuungsblöcke
                werden wiederhergestellt, sofern noch kein Ersatz eingetragen
                wurde.
              </>
            )}
          </>
        }
        gate={{ mode: "twoStep" }}
        loading={deleteLoading}
        error={deleteError}
        confirmLabel="Löschen"
        loadingLabel="Wird gelöscht…"
        onConfirm={handleDelete}
        onClose={() => setDeleteTarget(null)}
      />
      {denyModal && (
        <DenyAbsenceModal
          absence={denyModal}
          onClose={() => setDenyModal(null)}
          onDenied={async () => {
            setDenyModal(null);
            await reload();
          }}
        />
      )}
      {questionModal && (
        <QuestionAbsenceModal
          absence={questionModal}
          onClose={() => setQuestionModal(null)}
          onQuestioned={async () => {
            setQuestionModal(null);
            await reload();
          }}
        />
      )}
      {openingDeleteTarget && (
        <VacationOpeningDeleteModal
          staffId={staffId}
          opening={openingDeleteTarget}
          onClose={() => setOpeningDeleteTarget(null)}
          onDeleted={async () => {
            setOpeningDeleteTarget(null);
            await reload();
          }}
        />
      )}
      {openingModal && quota && (
        <VacationOpeningModal
          staffId={staffId}
          year={year}
          quota={quota}
          onClose={() => setOpeningModal(false)}
          onDelete={(opening) => {
            setOpeningModal(false);
            setOpeningDeleteTarget(opening);
          }}
          onSaved={async () => {
            setOpeningModal(false);
            await reload();
          }}
        />
      )}
      {bookingOpen && staff ? (
        <AbsenceBookingModal
          staff={staff}
          types={absenceTypes}
          onClose={() => setBookingOpen(false)}
          onSaved={async () => {
            setBookingOpen(false);
            await afterBooking();
          }}
        />
      ) : null}
    </div>
  );

  return <TabLoadingBoundary loading={loading}>{content}</TabLoadingBoundary>;
}

function formatNumber(days: number): string {
  return String(Math.round(days * 10) / 10).replace(".", ",");
}

// Wann der Rest verfällt (#3257): ein verfallener Rest bleibt sichtbar, und
// im neuen Jahr steht dabei, was aus dem Vorjahr noch nutzbar ist.
function AllowanceExpiryNote({
  summary,
}: {
  readonly summary: AbsenceTypeAllowanceSummary;
}) {
  const lines: { text: string; expired: boolean }[] = [];
  const carried = summary.carriedIn;
  if (carried && carried.remainingDays > 0) {
    lines.push({
      text: `Dazu noch ${formatDayCount(carried.remainingDays)} aus ${carried.year}, nutzbar bis ${formatDate(carried.expiresOn)}. Diese gehen zuerst ab.`,
      expired: false,
    });
  }
  if (carried && carried.expiredDays > 0) {
    lines.push({
      text: `${formatDayCount(carried.expiredDays)} aus ${carried.year} am ${formatDate(carried.expiresOn)} verfallen.`,
      expired: true,
    });
  }
  if (summary.expiredDays > 0) {
    lines.push({
      text: `${formatDayCount(summary.expiredDays)} am ${formatDate(summary.expiresOn)} verfallen.`,
      expired: true,
    });
  } else if (summary.remainingDays > 0) {
    lines.push({
      text: `Rest verfällt am ${formatDate(summary.expiresOn)}.`,
      expired: false,
    });
  }
  if (lines.length === 0) return null;
  return (
    <ul className="space-y-1 text-xs">
      {lines.map((line) => (
        <li
          key={line.text}
          className={line.expired ? "text-moto-red-strong" : "text-gray-600"}
        >
          {line.text}
        </li>
      ))}
    </ul>
  );
}

// Eine Kontingent-Karte (#3256): Urlaub und jede eigene Art mit Kontingent
// sehen gleich aus, damit „wie viel ist noch übrig" an einer Stelle steht.
// Im Bearbeiten-Zustand weichen die Zahlen dem Formular.
function AllowanceCard({
  title,
  year,
  retired = false,
  entitled,
  entitledHint,
  taken,
  takenHint,
  reserved,
  remaining,
  onEdit,
  extraAction,
  editForm,
  footer,
}: {
  readonly title: string;
  readonly year: number;
  readonly retired?: boolean;
  readonly entitled: number;
  readonly entitledHint?: string;
  readonly taken: number;
  readonly takenHint?: string;
  readonly reserved: number;
  readonly remaining: number;
  readonly onEdit?: () => void;
  readonly extraAction?: ReactNode;
  readonly editForm?: ReactNode;
  readonly footer?: ReactNode;
}) {
  const isEditing = Boolean(editForm);
  return (
    <section
      aria-label={`${title} ${year}`}
      className={`moto-content-surface space-y-3 rounded-xl border p-4 shadow-sm ${
        isEditing ? "lg:col-span-2" : ""
      }`}
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex min-w-0 items-center gap-2">
          <h4 className="truncate text-sm font-semibold text-gray-900">
            {title}
          </h4>
          {retired ? <StatusBadge tone="gray" label="ausgeschaltet" /> : null}
        </div>
        {isEditing ? null : (
          <div className="flex flex-wrap items-center gap-1">
            {extraAction}
            {onEdit ? (
              <Button
                type="button"
                variant="ghost"
                size="compact"
                onClick={onEdit}
                aria-label={`Anspruch ändern: ${title}`}
              >
                <Pencil className="mr-1.5 h-3.5 w-3.5" aria-hidden />
                Anspruch ändern
              </Button>
            ) : null}
          </div>
        )}
      </div>
      {isEditing ? (
        editForm
      ) : (
        <DataGrid columns={4}>
          <DataField label="Anspruch">
            <span className="tabular-nums">{formatDayCount(entitled)}</span>
            {entitledHint ? (
              <span className="block text-xs text-gray-500">
                {entitledHint}
              </span>
            ) : null}
          </DataField>
          <DataField label="Genommen">
            <span className="tabular-nums">{formatDayCount(taken)}</span>
            {takenHint ? (
              <span className="block text-xs text-gray-500">{takenHint}</span>
            ) : null}
          </DataField>
          <DataField label="Vorgemerkt">
            <span className="tabular-nums">{formatDayCount(reserved)}</span>
            {reserved > 0 ? (
              <span className="block text-xs text-gray-500">
                offene Anträge
              </span>
            ) : null}
          </DataField>
          <DataField label="Übrig">
            <span
              className={`font-semibold tabular-nums ${
                remaining < 0 ? "text-moto-red-strong" : "text-gray-900"
              }`}
            >
              {formatDayCount(remaining)}
            </span>
          </DataField>
        </DataGrid>
      )}
      {isEditing ? null : footer}
    </section>
  );
}

// Deletion of a vacation takeover (#2132). Own component so the tab renders
// it as a plain `{target && <Modal/>}` branch: target, loading and error state
// belong to the dialog, not to the tab.
function VacationOpeningDeleteModal({
  staffId,
  opening,
  onClose,
  onDeleted,
}: {
  readonly staffId: string;
  readonly opening: StaffVacationOpening;
  readonly onClose: () => void;
  readonly onDeleted: () => Promise<void>;
}) {
  const toast = useToast();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const handleConfirm = async () => {
    setLoading(true);
    setError("");
    try {
      await staffAbsenceService.deleteVacationOpening(staffId, opening.year);
      toast.success("Urlaubs-Übernahme gelöscht.");
      await onDeleted();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Löschen fehlgeschlagen.");
    } finally {
      setLoading(false);
    }
  };

  return (
    <ConfirmDeleteModal
      isOpen
      title="Urlaubs-Übernahme löschen"
      description={
        <>
          Die Übernahme zum Stichtag{" "}
          <strong>{formatDate(opening.effective_date)}</strong> wird gelöscht.
          Der Resturlaub steigt dadurch wieder um{" "}
          {formatDayCount(opening.taken_before_days)}.
        </>
      }
      gate={{ mode: "twoStep" }}
      loading={loading}
      error={error}
      confirmLabel="Löschen"
      loadingLabel="Wird gelöscht…"
      onConfirm={handleConfirm}
      onClose={onClose}
    />
  );
}

function PendingAbsences({
  rows,
  canEdit,
  pendingActionId,
  onApprove,
  onDeny,
  onQuestion,
}: {
  readonly rows: StaffAbsenceRow[];
  readonly canEdit: boolean;
  readonly pendingActionId: number | null;
  readonly onApprove: (row: StaffAbsenceRow) => Promise<void>;
  readonly onDeny: (row: StaffAbsenceRow) => void;
  readonly onQuestion: (row: StaffAbsenceRow) => void;
}) {
  if (rows.length === 0) {
    return (
      <div className="moto-content-surface flex items-center gap-3 rounded-2xl border px-5 py-4 shadow-sm">
        <span className="bg-moto-green/15 text-moto-green-strong inline-flex h-7 w-7 items-center justify-center rounded-full">
          <Check className="h-4 w-4" aria-hidden />
        </span>
        <div>
          <p className="text-sm font-medium text-gray-700">
            Keine offenen Anfragen
          </p>
          <p className="text-xs text-gray-500">Alles bearbeitet.</p>
        </div>
      </div>
    );
  }

  return (
    <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-5">
      <div className="mb-3 flex items-center gap-2">
        <h3 className="text-xs font-semibold tracking-wider text-gray-500 uppercase">
          Eingehende Anfragen
        </h3>
        {/* Neutral, nicht rot: StatusBadge tone="red" loest auf dasselbe
            Tripel auf wie StatusColorBadge(LOCATION_COLORS.SICK), und direkt
            darunter stehen die "Krank"-Typpillen der Zeilen. Ein Zaehler
            offener Anfragen ist ausserdem keine Fehlermeldung. */}
        <StatusBadge tone="gray" label={String(rows.length)} />
      </div>
      <ul className="divide-y divide-gray-100">
        {rows.map((row) => (
          <AbsenceRequestRow
            key={row.id}
            row={row}
            isBusy={pendingActionId === row.id}
            showActions={canEdit}
            onApprove={(r) => {
              onApprove(r);
            }}
            onDeny={onDeny}
            onQuestion={onQuestion}
          />
        ))}
      </ul>
    </div>
  );
}

// Generic notes that duplicate the type badge get suppressed, admins seeded
// "Urlaub" or "Krankmeldung" as boilerplate, and rendering them next to the
// already-visible type badge is just noise.
const REDUNDANT_NOTES: Record<string, ReadonlyArray<string>> = {
  sick: ["krank", "krankheit", "krankmeldung", "au", "au-bescheinigung"],
  vacation: ["urlaub", "erholungsurlaub"],
  training: ["fortbildung", "schulung"],
  other: ["abwesend", "sonstige", "sonstiges"],
};

function isRedundantNote(absenceType: string, note: string): boolean {
  const normalized = note.trim().toLowerCase();
  if (!normalized) return true;
  const typeLabel = (ABSENCE_TYPE_LABEL[absenceType] ?? "").toLowerCase();
  if (normalized === typeLabel) return true;
  return (REDUNDANT_NOTES[absenceType] ?? []).includes(normalized);
}

function AbsenceRow({
  row,
  onDelete,
}: {
  readonly row: StaffAbsenceRow;
  // Delete action for admin-entered absences (status "reported"), e.g. a sick
  // report whose plan cascade can be undone. Omitted for read-only rows.
  readonly onDelete?: () => void;
}) {
  const meta = statusMeta(row.status);
  const showNote = row.note && !isRedundantNote(row.absence_type, row.note);
  const canDelete = onDelete && row.status === "reported";
  return (
    <li className="moto-content-surface flex flex-wrap items-center justify-between gap-3 rounded-xl border px-4 py-3 shadow-sm">
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <StatusColorBadge
            label={absenceRowLabel(row)}
            color={
              ABSENCE_TYPE_HEX[row.absence_type] ?? LOCATION_COLORS.UNKNOWN
            }
          />
          <p className="text-sm font-medium text-gray-800">
            {formatRange(row.date_start, row.date_end)}
            <span className="ml-2 text-xs text-gray-500">
              · {formatDayCount(dayCountFor(row))}
            </span>
          </p>
        </div>
        {showNote && (
          <p className="mt-0.5 truncate text-xs text-gray-500">{row.note}</p>
        )}
        {row.decision_note && (
          <p className="mt-0.5 text-xs text-gray-500">
            <span className="font-medium">Anmerkung Leitung:</span>{" "}
            {row.decision_note}
          </p>
        )}
      </div>
      <div className="flex items-center gap-2">
        <StatusColorBadge label={meta.label} color={meta.color} />
        {canDelete && (
          <Button
            type="button"
            variant="ghost"
            size="icon"
            onClick={onDelete}
            aria-label={`${absenceRowActionNoun(row)} ${formatRange(row.date_start, row.date_end)} löschen`}
            title={`${absenceRowActionNoun(row)} löschen`}
          >
            <Trash2 className="h-4 w-4" aria-hidden />
          </Button>
        )}
      </div>
    </li>
  );
}

// Dezente Zeile unter den Quota-Kacheln: macht sichtbar, dass ein Teil des
// Jahresanspruchs bereits vor der moto-Einführung verbraucht war (#2132).
function VacationOpeningSummary({
  opening,
}: {
  readonly opening: StaffVacationOpening;
}) {
  return (
    <div className="flex flex-wrap items-center gap-x-2 gap-y-1 rounded-2xl border border-gray-100 bg-gray-50 px-4 py-2.5 text-xs text-gray-600">
      <span className="font-semibold tracking-wider text-gray-500 uppercase">
        Übernahme
      </span>
      <span className="tabular-nums">
        {formatDayCount(opening.taken_before_days)} vor Einführung genommen
      </span>
      <span className="text-gray-300">·</span>
      <span>Stichtag {formatDate(opening.effective_date)}</span>
    </div>
  );
}

function previousDayISO(dateKey: string): string {
  const date = parseISODate(dateKey);
  date.setDate(date.getDate() - 1);
  return toISODate(date);
}

// Urlaubs-Übernahme zum Go-live-Stichtag (#2132). Der Admin trägt den
// Resturlaub zum Stichtag ein, der Server leitet daraus die vorher genommenen
// Tage ab (Anspruch + Übertrag − Rest). Existiert die Übernahme bereits, zeigt
// das Modal nur noch den gebuchten Stand: Korrekturen laufen über Löschen und
// neu anlegen, damit die Historie erhalten bleibt.
function VacationOpeningModal({
  staffId,
  year,
  quota,
  onClose,
  onDelete,
  onSaved,
}: {
  readonly staffId: string;
  readonly year: number;
  readonly quota: StaffVacationQuotaSummary;
  readonly onClose: () => void;
  readonly onDelete: (opening: StaffVacationOpening) => void;
  readonly onSaved: () => void | Promise<void>;
}) {
  const existing = quota.opening ?? null;
  const [effectiveDate, setEffectiveDate] = useState("");
  const [remainingDays, setRemainingDays] = useState("");
  const [note, setNote] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useFormError();
  const toast = useToast();

  const yearEndKey = `${year}-12-31`;
  const yesterdayKey = previousDayISO(berlinTodayISO());
  const maxEffectiveDateKey =
    yesterdayKey < yearEndKey ? yesterdayKey : yearEndKey;

  const remaining = parseOneDecimalInput(remainingDays);
  const remainingValid =
    remaining !== null && remaining >= -999 && remaining <= 999;
  const entitledTotal = quota.entitled_days + quota.carryover_days;
  const computedTakenBefore =
    remaining === null ? null : entitledTotal - remaining;

  const handleSubmit = async () => {
    setError(null);
    if (!effectiveDate) {
      setError("Stichtag fehlt.");
      return;
    }
    if (remaining === null || remaining < -999 || remaining > 999) {
      setError("Resturlaub ungültig (-999 bis 999).");
      return;
    }
    if (note.trim() === "") {
      setError("Begründung fehlt.");
      return;
    }
    setSubmitting(true);
    try {
      await staffAbsenceService.setVacationOpening(staffId, {
        effectiveDate,
        remainingDays: remaining,
        note: note.trim(),
      });
      toast.success("Urlaubs-Übernahme gespeichert.");
      await onSaved();
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Übernahme fehlgeschlagen.",
      );
    } finally {
      setSubmitting(false);
    }
  };

  if (existing) {
    return (
      <Modal
        isOpen
        onClose={onClose}
        title={`Urlaubs-Übernahme ${year}`}
        footer={
          <div className="flex w-full flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            <Button type="button" variant="outline" size="md" onClick={onClose}>
              Schließen
            </Button>
            <Button
              type="button"
              variant="outline_danger"
              size="md"
              onClick={() => onDelete(existing)}
            >
              <Trash2 className="mr-1.5 h-4 w-4" aria-hidden />
              Übernahme löschen
            </Button>
          </div>
        }
      >
        <div className="space-y-4">
          <p className="text-sm text-gray-500">
            Für dieses Jahr ist bereits eine Übernahme gebucht. Eine Korrektur
            läuft über Löschen und neues Anlegen, damit die Historie
            nachvollziehbar bleibt.
          </p>
          <DataGrid>
            <DataField label="Stichtag">
              {formatDate(existing.effective_date)}
            </DataField>
            <DataField label="Resturlaub zum Stichtag">
              <span className="tabular-nums">
                {formatDayCount(existing.entered_remaining_days)}
              </span>
            </DataField>
            <DataField label="Vor Einführung genommen">
              <span className="tabular-nums">
                {formatDayCount(existing.taken_before_days)}
              </span>
            </DataField>
            {existing.note && (
              <DataField label="Begründung" fullWidth>
                {existing.note}
              </DataField>
            )}
          </DataGrid>
        </div>
      </Modal>
    );
  }

  const canSubmit =
    !submitting && effectiveDate !== "" && remainingValid && note.trim() !== "";

  return (
    <Modal
      isOpen
      onClose={() => !submitting && onClose()}
      title={`Urlaubs-Übernahme ${year}`}
      footer={
        <div className="flex w-full flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={onClose}
            disabled={submitting}
          >
            Abbrechen
          </Button>
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={handleSubmit}
            disabled={!canSubmit}
          >
            {submitting ? "…" : "Übernahme buchen"}
          </Button>
        </div>
      }
    >
      <div className="space-y-4">
        <FormErrorAlert message={error} />
        <p className="text-sm text-gray-500">
          moto errechnet daraus die vor der Einführung bereits genommenen Tage.
          Der Jahresanspruch bleibt unverändert.
        </p>
        <div>
          <label
            htmlFor="vacation-opening-date"
            className="mb-1 block text-xs font-semibold tracking-wider text-gray-500 uppercase"
          >
            Stichtag
          </label>
          <ISODatePicker
            id="vacation-opening-date"
            min={`${year}-01-01`}
            max={maxEffectiveDateKey}
            value={effectiveDate}
            onChange={setEffectiveDate}
            calendarLayout="popover"
            hideClearButton
          />
          <p className="mt-1 text-xs text-gray-400">
            Der Stichtag muss im Jahr {year} und vor dem heutigen Tag liegen.
          </p>
        </div>
        <div>
          <label
            htmlFor="vacation-opening-remaining"
            className="mb-1 block text-xs font-semibold tracking-wider text-gray-500 uppercase"
          >
            Resturlaub zum Stichtag (Tage)
          </label>
          <Input
            id="vacation-opening-remaining"
            type="text"
            inputMode="decimal"
            controlSize="compact"
            value={remainingDays}
            onChange={(e) => setRemainingDays(e.target.value)}
            placeholder="z. B. 12,5"
            error={
              remainingDays.trim() !== "" && !remainingValid
                ? "Bitte eine Zahl zwischen -999 und 999 mit höchstens einer Nachkommastelle eingeben."
                : undefined
            }
          />
        </div>
        <div className="space-y-1 rounded-xl bg-gray-50 px-3 py-2 text-sm text-gray-700">
          <p>
            Jahresanspruch inklusive Übertrag:{" "}
            <span className="font-medium tabular-nums">
              {formatDayCount(entitledTotal)}
            </span>
          </p>
          {remainingValid && computedTakenBefore !== null && (
            <p>
              Errechnet vor Einführung genommen:{" "}
              <span className="font-medium tabular-nums">
                {formatDayCount(computedTakenBefore)}
              </span>
            </p>
          )}
          {/* Brand-Orange als lesbare Textstufe (moto-orange-strong) statt
              einer generischen Tailwind-Warnfarbe. */}
          {computedTakenBefore !== null && computedTakenBefore < 0 && (
            <p className="text-moto-orange-strong text-xs">
              Der eingegebene Resturlaub liegt über dem Jahresanspruch. moto
              schreibt die Differenz zusätzlich gut.
            </p>
          )}
        </div>
        <div>
          <label
            htmlFor="vacation-opening-note"
            className="mb-1 block text-xs font-semibold tracking-wider text-gray-500 uppercase"
          >
            Begründung (Pflicht)
          </label>
          <Textarea
            id="vacation-opening-note"
            value={note}
            onChange={(e) => setNote(e.target.value)}
            rows={2}
            placeholder="z. B. Übernahme aus Urlaubsliste, Stand 31.07."
          />
        </div>
      </div>
    </Modal>
  );
}

const QUOTA_RANGE_ERROR = "Bitte eine Zahl zwischen 0 und 366 eingeben.";

function quotaFieldError(raw: string): string | undefined {
  const value = Number.parseFloat(raw);
  return Number.isNaN(value) || value < 0 || value > 366
    ? QUOTA_RANGE_ERROR
    : undefined;
}

/**
 * Der Bearbeiten-Zustand des Urlaubsanspruchs (BAUARTEN-SPEC Bauart 2 Regeln
 * 3 bis 5, #3119): an Ort und Stelle der Kennzahl-Kacheln, zwei Felder, ein
 * `EditActions` unten. Fehler stehen im Alert oben und am Feld; der
 * Erfolgs-Toast bleibt. Nur eingehängt, solange bearbeitet wird, deshalb
 * liest der Entwurf seinen Ausgangsstand beim Einhängen aus `quota`.
 */
function QuotaEditForm({
  staffId,
  quota,
  year,
  onCancel,
  onSaved,
}: {
  readonly staffId: string;
  readonly quota: StaffVacationQuotaSummary;
  readonly year: number;
  readonly onCancel: () => void;
  readonly onSaved: () => void | Promise<void>;
}) {
  const [entitled, setEntitled] = useState(String(quota.entitled_days));
  const [carryover, setCarryover] = useState(String(quota.carryover_days));
  const [reason, setReason] = useState("");
  const [fieldErrors, setFieldErrors] = useState<{
    entitled?: string;
    carryover?: string;
    reason?: string;
  }>({});
  const [saving, setSaving] = useState(false);
  const [error, setError] = useFormError();
  const toast = useToast();

  const handleSave = async () => {
    setError(null);
    const nextFieldErrors = {
      entitled: quotaFieldError(entitled),
      carryover: quotaFieldError(carryover),
      reason:
        reason.trim() === ""
          ? "Bitte kurz sagen, warum sich der Anspruch ändert."
          : undefined,
    };
    setFieldErrors(nextFieldErrors);
    if (
      nextFieldErrors.entitled ||
      nextFieldErrors.carryover ||
      nextFieldErrors.reason
    ) {
      setError("Bitte prüfen Sie die markierten Felder.");
      return;
    }
    setSaving(true);
    try {
      await staffAbsenceService.setVacationQuota(staffId, {
        year,
        entitled_days: Number.parseFloat(entitled),
        carryover_days: Number.parseFloat(carryover),
        reason: reason.trim(),
      });
      toast.success("Urlaubsanspruch gespeichert.");
      await onSaved();
    } catch (err) {
      logger.error("quota_save_failed", {
        staff_id: staffId,
        error: err instanceof Error ? err.message : String(err),
      });
      setError(
        err instanceof Error && err.message
          ? err.message
          : "Der Urlaubsanspruch konnte nicht gespeichert werden.",
      );
    } finally {
      setSaving(false);
    }
  };

  return (
    <form
      className="space-y-4"
      noValidate
      aria-label={`Urlaubsanspruch ${year}`}
      onSubmit={(event) => {
        event.preventDefault();
        void handleSave();
      }}
    >
      <FormErrorAlert message={error} />
      <h4 className="text-sm font-semibold text-gray-900">
        Urlaubsanspruch {year}
      </h4>
      <div className="grid gap-3 md:grid-cols-2">
        <Input
          id="quota-entitled"
          label="Jahresanspruch (Tage)"
          type="number"
          min="0"
          max="366"
          step="0.5"
          controlSize="compact"
          value={entitled}
          onChange={(e) => setEntitled(e.target.value)}
          error={fieldErrors.entitled}
          disabled={saving}
        />
        <Input
          id="quota-carryover"
          label="Übertrag aus Vorjahr (Tage)"
          type="number"
          min="0"
          max="366"
          step="0.5"
          controlSize="compact"
          value={carryover}
          onChange={(e) => setCarryover(e.target.value)}
          error={fieldErrors.carryover}
          disabled={saving}
        />
      </div>
      <Input
        id="quota-reason"
        label="Begründung"
        controlSize="compact"
        value={reason}
        onChange={(e) => setReason(e.target.value)}
        placeholder="z. B. Stellenumfang geändert"
        error={fieldErrors.reason}
        disabled={saving}
      />
      <EditActions onCancel={onCancel} saving={saving} />
    </form>
  );
}

"use client";

// Ferienanmeldungen eines Zeitraums prüfen (#4663): alle Anmeldungen mit
// Status, Schule, Woche und Zeitmodell, mit Filtern, Hinweis auf mögliche
// Doppelanmeldungen und Export als CSV oder Excel. Anmeldungen derselben
// Familie führt das Büro hier als Geschwister zusammen (#5628); gleiche
// E-Mail oder gleicher Nachname sind nur ein Hinweis.

import { ClipboardList } from "lucide-react";
import { useParams, useRouter } from "next/navigation";
import {
  type MouseEvent as ReactMouseEvent,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import {
  childSchoolLabel,
  formatHolidayWeek,
  HOLIDAY_REGISTRATION_STATUS_LABELS,
  HOLIDAY_REGISTRATION_STATUS_TONES,
  holidayTimeModelNames,
  schoolNames,
} from "~/components/traeger/holiday-registration-format";
import {
  HolidayHeadFacts,
  holidayPeriodTabs,
} from "~/components/traeger/holiday-period-head";
import { useTraegerSchools } from "~/components/traeger/use-traeger-schools";
import { metadataLoadError } from "~/components/traeger/metadata-load-error";
import { Alert } from "~/components/ui/alert";
import { Button, ButtonLink } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { SelectionBar } from "~/components/ui/selection-bar";
import NavigationLink from "~/components/ui/navigation-link";
import {
  ExportMenu,
  type ExportMenuOption,
  exportFormatOptions,
  exportMenuItems,
} from "~/components/ui/export-menu";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import type {
  ActiveFilter,
  FilterConfig,
} from "~/components/ui/page-header/types";
import { StatusBadge } from "~/components/ui/status-badge";
import { useHolidayPeriodBillingHref } from "~/components/traeger/use-holiday-period-billing-href";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { billingConcept } from "~/lib/concepts/billing";
import { TenantPage } from "~/components/ui/tenant-page";
import {
  useApiErrorDisplay,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { formatBerlinDate } from "~/lib/date-helpers";
import { downloadBlob } from "~/lib/file-download";
import {
  SIBLING_HINT_LABELS,
  type SiblingHint,
  siblingGroupOf,
  siblingHints,
} from "~/lib/holiday-sibling-hints";
import { createLogger } from "~/lib/logger";
import { useTraegerHolidayPeriod } from "~/components/traeger/use-traeger-resources";
import {
  downloadHolidayRegistrations,
  EMPTY_HOLIDAY_REGISTRATION_FILTER,
  fetchHolidayConfirmationSettings,
  fetchHolidayRegistrations,
  type HolidayRegistrationFilter,
  type HolidayRegistrationList,
  type HolidayRegistrationStatus,
  type HolidayRegistrationSummary,
} from "~/lib/traeger-holiday-registrations-api";
import {
  fetchHolidaySiblingGroups,
  type HolidaySiblingGroup,
  mergeHolidaySiblings,
} from "~/lib/traeger-sibling-groups-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerHolidayRegistrationsPage" });

type StatusChoice = HolidayRegistrationStatus | "all";

const EXPORT_FORMATS: readonly ExportMenuOption[] = exportFormatOptions([
  "xlsx",
  "csv",
]);

const STATUS_OPTIONS: { value: StatusChoice; label: string }[] = [
  { value: "all", label: "Alle Status" },
  { value: "received", label: "Eingegangen" },
  { value: "confirmed", label: "Bestätigt" },
  { value: "cancelled", label: "Storniert" },
];

/** The debtor number, only while the Träger asks for it (#5621). */
const DEBTOR_NUMBER_COLUMN: DataTableColumn<HolidayRegistrationSummary> = {
  key: "debtorNumber",
  header: "Debitorennummer",
  render: (registration) =>
    registration.debtorNumber ? (
      <span className="font-mono wrap-anywhere">
        {registration.debtorNumber}
      </span>
    ) : (
      "–"
    ),
  sortValue: (registration) => registration.debtorNumber,
  stacked: "meta",
};

/** What the list knows about the siblings of a registration (#5628). */
interface SiblingView {
  readonly groupOf: ReadonlyMap<string, HolidaySiblingGroup>;
  readonly hints: ReadonlyMap<string, SiblingHint>;
}

/**
 * The siblings column: the other registrations of the group, or the hint
 * that there may be some. A hint is text, no badge: it changes nothing.
 */
function siblingsColumn(
  view: SiblingView,
): DataTableColumn<HolidayRegistrationSummary> {
  return {
    key: "siblings",
    header: "Geschwister",
    render: (registration) => {
      const group = view.groupOf.get(registration.id);
      if (group) {
        const others = group.registrationIds.filter(
          (id) => id !== registration.id,
        );
        return (
          <span className="inline-flex flex-wrap items-center gap-x-2 gap-y-1">
            <StatusBadge tone="green" label="Geschwister" />
            <span className="wrap-anywhere">mit Nr. {others.join(", ")}</span>
          </span>
        );
      }
      const hint = view.hints.get(registration.id);
      if (!hint) return null;
      return (
        <span className="block wrap-anywhere text-gray-700">
          <span className="block">{SIBLING_HINT_LABELS[hint.reason]}</span>
          <span className="block text-gray-500">
            Nr. {hint.registrationIds.join(", ")}
          </span>
        </span>
      );
    },
    stacked: "field",
  };
}

function columns(
  periodId: string,
  names: ReadonlyMap<string, string>,
  debtorNumber: boolean,
  siblings: SiblingView,
): DataTableColumn<HolidayRegistrationSummary>[] {
  return [
    {
      key: "number",
      header: "Nr.",
      render: (registration) => registration.id,
      sortValue: (registration) => Number(registration.id),
      stacked: "meta",
    },
    {
      key: "guardian",
      header: "Eltern",
      // The rows carry selection boxes and are no buttons; the name is the
      // link to the registration for the keyboard (#5628). A plain click
      // opens it through the row, a click for a new tab stays the link's.
      render: (registration) => (
        <NavigationLink
          href={traegerPath(
            `/traeger/ferien/${periodId}/anmeldungen/${registration.id}`,
          )}
          className="font-medium text-gray-900 hover:underline"
          onClick={(event: ReactMouseEvent<HTMLAnchorElement>) => {
            if (
              event.metaKey ||
              event.ctrlKey ||
              event.shiftKey ||
              event.altKey ||
              event.button !== 0
            ) {
              event.stopPropagation();
              return;
            }
            event.preventDefault();
          }}
        >
          {registration.guardianFirstName} {registration.guardianLastName}
        </NavigationLink>
      ),
      sortValue: (registration) => registration.guardianLastName,
      stacked: "title",
    },
    {
      key: "children",
      header: "Kinder und Wochen",
      render: (registration) => (
        <ul className="space-y-2">
          {registration.children.map((child) => (
            <li key={child.id}>
              <div className="flex flex-wrap items-center gap-2">
                <span className="font-medium text-gray-900">
                  {child.firstName} {child.lastName}
                </span>
                <span className="text-gray-500">
                  {childSchoolLabel(child, names)}
                </span>
                {child.duplicateRegistrationIds.length > 0 ? (
                  <StatusBadge
                    tone="orange"
                    label="Mögliche Doppelanmeldung"
                    title={`Auch in Anmeldung Nr. ${child.duplicateRegistrationIds.join(", ")}`}
                  />
                ) : null}
              </div>
              <ul className="text-xs text-gray-500">
                {child.bookings.map((booking) => (
                  <li key={booking.weekId}>
                    {formatHolidayWeek(
                      booking.weekStartsOn,
                      booking.weekEndsOn,
                    )}
                    ,{" "}
                    {names.get(booking.hostSchoolId) ??
                      "Schule nicht mehr vorhanden"}
                    : {booking.timeModelName}
                  </li>
                ))}
              </ul>
            </li>
          ))}
        </ul>
      ),
      stacked: "field",
    },
    {
      key: "submittedAt",
      header: "Eingegangen am",
      render: (registration) => formatBerlinDate(registration.submittedAt),
      sortValue: (registration) => registration.submittedAt,
      stacked: "meta",
    },
    {
      key: "status",
      header: "Status",
      render: (registration) => (
        <div className="flex flex-wrap gap-1">
          <StatusBadge
            tone={HOLIDAY_REGISTRATION_STATUS_TONES[registration.status]}
            label={HOLIDAY_REGISTRATION_STATUS_LABELS[registration.status]}
          />
          {registration.frozen ? (
            <StatusBadge tone="gray" label="Eingefroren" />
          ) : null}
        </div>
      ),
      sortValue: (registration) => registration.status,
      stacked: "meta",
    },
    siblingsColumn(siblings),
    ...(debtorNumber ? [DEBTOR_NUMBER_COLUMN] : []),
  ];
}

/** Die Zahlen je Status in denselben Farben wie die Plaketten der Tabelle. */
function StatusLine({ list }: Readonly<{ list: HolidayRegistrationList }>) {
  return (
    <HolidayHeadFacts>
      <span>
        {list.total === 1 ? "1 Anmeldung" : `${list.total} Anmeldungen`}
      </span>
      <StatusBadge
        tone={HOLIDAY_REGISTRATION_STATUS_TONES.received}
        label={`${list.statusCounts.received} eingegangen`}
      />
      <StatusBadge
        tone={HOLIDAY_REGISTRATION_STATUS_TONES.confirmed}
        label={`${list.statusCounts.confirmed} bestätigt`}
      />
      <StatusBadge
        tone={HOLIDAY_REGISTRATION_STATUS_TONES.cancelled}
        label={`${list.statusCounts.cancelled} storniert`}
      />
    </HolidayHeadFacts>
  );
}

/** Registrations with a child that is maybe registered twice. */
function duplicateCount(list: HolidayRegistrationList | null): number {
  return (list?.registrations ?? []).filter((registration) =>
    registration.children.some(
      (child) => child.duplicateRegistrationIds.length > 0,
    ),
  ).length;
}

export default function TraegerHolidayRegistrationsPage() {
  const router = useRouter();
  const params = useParams<{ id: string }>();
  const periodId = params?.id ?? "";
  const {
    data: period,
    preview,
    loadError: periodError,
    error: periodFailure,
  } = useTraegerHolidayPeriod(periodId);
  const billingHref = useHolidayPeriodBillingHref(period);
  const [list, setList] = useState<HolidayRegistrationList | null>(null);
  const [groups, setGroups] = useState<readonly HolidaySiblingGroup[]>([]);
  // The registrations the Träger-Büro picked to merge as siblings (#5628).
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set());
  const [merging, setMerging] = useState(false);
  const { success } = useToast();
  const [status, setStatus] = useState<StatusChoice>("all");
  const [school, setSchool] = useState("");
  const [weekId, setWeekId] = useState("");
  const [timeModel, setTimeModel] = useState("");
  const [loading, setLoading] = useState(true);
  const [exporting, setExporting] = useState<"csv" | "xlsx" | null>(null);
  // Each new filter starts a new list; an answer for an older one is dropped.
  const generation = useRef(0);
  const {
    schools,
    error: schoolsError,
    fetchError: schoolsFailure,
  } = useTraegerSchools();
  const listLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = listLoad;
  const { show: showActionError } = useApiErrorDisplay();
  // Whether the Träger asks for the debtor number (#5621); the column shows
  // only then.
  const [debtorNumberColumn, setDebtorNumberColumn] = useState(false);

  useEffect(() => {
    let current = true;
    fetchHolidayConfirmationSettings()
      .then((settings) => {
        if (current) setDebtorNumberColumn(settings.debtorNumberRequired);
      })
      .catch((err: unknown) => {
        // The list stays complete without the column; the detail of each
        // registration still names its number.
        logger.warn("traeger_holiday_confirmation_settings_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
    return () => {
      current = false;
    };
  }, []);

  const filter = useMemo<HolidayRegistrationFilter>(
    () => ({
      ...EMPTY_HOLIDAY_REGISTRATION_FILTER,
      status: status === "all" ? "" : status,
      school,
      weekId,
      timeModel,
    }),
    [status, school, weekId, timeModel],
  );

  const load = useCallback(async () => {
    const current = ++generation.current;
    setLoading(true);
    try {
      const [loaded, loadedGroups] = await Promise.all([
        fetchHolidayRegistrations(periodId, filter),
        fetchHolidaySiblingGroups(periodId),
      ]);
      if (current !== generation.current) return;
      setList(loaded);
      setGroups(loadedGroups);
      // Only registrations the list shows stay selected: none is merged
      // unseen.
      const shown = new Set(
        loaded.registrations.map((registration) => registration.id),
      );
      setSelected((selection) => {
        const kept = [...selection].filter((id) => shown.has(id));
        return kept.length === selection.size ? selection : new Set(kept);
      });
      clearLoadError();
    } catch (err) {
      if (current !== generation.current) return;
      logger.error("traeger_holiday_registrations_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Liste der Anmeldungen",
        retry: () => void load(),
      });
    } finally {
      if (current === generation.current) setLoading(false);
    }
  }, [periodId, filter, clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  // A new filter shows other rows; a selection the list no longer shows
  // would be merged unseen.
  useEffect(() => {
    setSelected(new Set());
  }, [filter]);

  const exportFile = async (format: "csv" | "xlsx") => {
    setExporting(format);
    try {
      const file = await downloadHolidayRegistrations(periodId, filter, format);
      downloadBlob(file.blob, file.filename ?? `ferienanmeldungen.${format}`);
    } catch (err) {
      logger.error("traeger_holiday_registrations_export_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showActionError(err, { object: "die Exportdatei" });
    } finally {
      setExporting(null);
    }
  };

  const selectExport = (option: ExportMenuOption) => {
    if (option.format === "csv" || option.format === "xlsx") {
      void exportFile(option.format);
    }
  };

  const mergeSiblings = async () => {
    if (merging || loading || selected.size < 2) return;
    setMerging(true);
    try {
      await mergeHolidaySiblings(periodId, [...selected]);
      success("Die Anmeldungen sind als Geschwister zusammengeführt.");
      setSelected(new Set());
      await load();
    } catch (err) {
      logger.error("traeger_holiday_siblings_merge_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showActionError(err, { object: "die Geschwistergruppe" });
    } finally {
      setMerging(false);
    }
  };

  const names = schoolNames(schools);
  const registrationsShown = list?.registrations ?? [];
  const siblings: SiblingView = {
    groupOf: siblingGroupOf(groups),
    hints: siblingHints(registrationsShown, groups),
  };
  const schoolOptions = [
    { value: "", label: "Alle Schulen" },
    ...(schools ?? []).map((entry) => ({ value: entry.id, label: entry.name })),
    { value: "guest", label: "Feriengäste" },
  ];
  const weekOptions = [
    { value: "", label: "Alle Wochen" },
    ...(period?.weeks ?? []).map((week) => ({
      value: week.id ?? "",
      label: `${formatHolidayWeek(week.startsOn, week.endsOn)}, ${names.get(week.hostSchoolId) ?? "Schule nicht mehr vorhanden"}`,
    })),
  ];
  const timeModelOptions = [
    { value: "", label: "Alle Zeitmodelle" },
    ...(period ? holidayTimeModelNames(period) : []).map((name) => ({
      value: name,
      label: name,
    })),
  ];
  const filtered = Boolean(status !== "all" || school || weekId || timeModel);
  const duplicates = duplicateCount(list);
  const loadError =
    listLoad.error ??
    metadataLoadError(
      periodError ?? schoolsError,
      Boolean(list && period && schools),
      periodFailure,
      schoolsFailure,
    );

  const filters: FilterConfig[] = [
    {
      id: "status",
      label: "Status",
      type: "dropdown",
      value: status,
      onChange: (value) => setStatus(value as StatusChoice),
      options: STATUS_OPTIONS,
    },
    {
      id: "school",
      label: "Schule",
      type: "dropdown",
      value: school,
      onChange: (value) => setSchool(value as string),
      options: schoolOptions,
    },
    {
      id: "week",
      label: "Woche",
      type: "dropdown",
      value: weekId,
      onChange: (value) => setWeekId(value as string),
      options: weekOptions,
    },
    {
      id: "timeModel",
      label: "Zeitmodell",
      type: "dropdown",
      value: timeModel,
      onChange: (value) => setTimeModel(value as string),
      options: timeModelOptions,
    },
  ];
  const optionLabel = (
    options: readonly { value: string; label: string }[],
    value: string,
  ) => options.find((option) => option.value === value)?.label ?? value;
  const activeFilters: ActiveFilter[] = [
    ...(status !== "all"
      ? [
          {
            id: "status",
            label: optionLabel(STATUS_OPTIONS, status),
            onRemove: () => setStatus("all"),
          },
        ]
      : []),
    ...(school
      ? [
          {
            id: "school",
            label: optionLabel(schoolOptions, school),
            onRemove: () => setSchool(""),
          },
        ]
      : []),
    ...(weekId
      ? [
          {
            id: "week",
            label: optionLabel(weekOptions, weekId),
            onRemove: () => setWeekId(""),
          },
        ]
      : []),
    ...(timeModel
      ? [
          {
            id: "timeModel",
            label: timeModel,
            onRemove: () => setTimeModel(""),
          },
        ]
      : []),
  ];
  const clearFilters = () => {
    setStatus("all");
    setSchool("");
    setWeekId("");
    setTimeModel("");
  };

  return (
    <TenantPage
      title={period?.name ?? preview?.name ?? "Ferienzeitraum"}
      back
      backHref={traegerPath("/traeger/ferien")}
      backLabel="Zurück zu den Ferien"
      stats={list ? <StatusLine list={list} /> : null}
      statsLoading={!list && !loadError}
      tabs={holidayPeriodTabs(
        periodId,
        "anmeldungen",
        period?.registrationCount,
      )}
      actions={
        billingHref ? (
          <>
            <ButtonLink href={billingHref} variant="outline" size="md">
              <MotoConceptIcon
                concept={billingConcept}
                size={18}
                className="mr-2"
              />
              Zur Abrechnung
            </ButtonLink>
            {/* One visible action; the download formats wait in the menu
                (TENANT-PAGE-SPEC rule 4). */}
            <OverflowMenu
              ariaLabel="Anmeldungen herunterladen"
              disabled={loading || Boolean(loadError)}
              items={exportMenuItems({
                options: EXPORT_FORMATS,
                busyKey: exporting,
                unavailable: loading || Boolean(loadError),
                onSelect: selectExport,
              })}
            />
          </>
        ) : (
          <ExportMenu
            ariaLabel="Anmeldungen herunterladen"
            options={EXPORT_FORMATS}
            busyKey={exporting}
            unavailable={loading || Boolean(loadError)}
            onSelect={selectExport}
          />
        )
      }
      filters={filters}
      activeFilters={activeFilters}
      onClearAllFilters={clearFilters}
      error={loadError}
    >
      {duplicates > 0 ? (
        <Alert
          type="warning"
          message={
            duplicates === 1
              ? "In einer Anmeldung ist ein Kind vielleicht doppelt angemeldet. Prüfen Sie die markierte Anmeldung."
              : `In ${duplicates} Anmeldungen ist ein Kind vielleicht doppelt angemeldet. Prüfen Sie die markierten Anmeldungen.`
          }
        />
      ) : null}
      <DataTable
        columns={columns(periodId, names, debtorNumberColumn, siblings)}
        rows={[...registrationsShown]}
        getRowKey={(registration) => registration.id}
        selection={{
          selectedKeys: selected,
          // While new rows load, the old ones are no choice any more.
          disabled: merging || loading,
          rowLabel: (registration) =>
            `Anmeldung Nr. ${registration.id} von ${registration.guardianFirstName} ${registration.guardianLastName}`,
          onChange: (keys, on) =>
            setSelected((current) => {
              const next = new Set(current);
              for (const key of keys) {
                if (on) next.add(key);
                else next.delete(key);
              }
              return next;
            }),
        }}
        isLoading={loading && list === null}
        loadingRowCount={4}
        stackedOnMobile
        onRowClick={(registration) =>
          router.push(
            traegerPath(
              `/traeger/ferien/${periodId}/anmeldungen/${registration.id}`,
            ),
          )
        }
        emptyState={
          <EmptyState
            icon={<ClipboardList className="h-12 w-12" aria-hidden="true" />}
            title={
              filtered
                ? "Zu diesen Filtern gibt es keine Anmeldung."
                : "Es ist noch keine Anmeldung eingegangen."
            }
            description={
              filtered
                ? "Wählen Sie andere Filter."
                : "Eltern melden ihre Kinder über den Link für Eltern an."
            }
          />
        }
      />
      {selected.size > 0 ? (
        // What the ticked rows are for, and the action, stick at the bottom
        // of the screen while the list scrolls.
        <SelectionBar
          aria-label="Ausgewählte Anmeldungen"
          placement="bottom"
          count={selected.size}
          label={
            selected.size === 1
              ? "1 Anmeldung ausgewählt."
              : `${selected.size} Anmeldungen ausgewählt.`
          }
          description={
            selected.size === 1
              ? "Wählen Sie mindestens eine weitere Anmeldung derselben Familie."
              : "Als Geschwister zählen ihre Kinder beim Ferienbeitrag zusammen."
          }
          onClear={() => setSelected(new Set())}
          busy={merging}
          actions={
            selected.size > 1 ? (
              <Button
                type="button"
                variant="primary"
                size="compact"
                isLoading={merging}
                disabled={loading}
                loadingText="Führt zusammen …"
                onClick={() => void mergeSiblings()}
              >
                Als Geschwister zusammenführen
              </Button>
            ) : null
          }
        />
      ) : null}
    </TenantPage>
  );
}

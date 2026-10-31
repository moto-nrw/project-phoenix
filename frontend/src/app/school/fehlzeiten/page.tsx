"use client";

// Fehlzeiten in moto schule (#4793): je Kind einer Klasse die Fehltage eines
// Zeitraums, davon entschuldigt und unentschuldigt, für Klassenbuch und
// Zeugnis. Die Lehrkraft sieht ihre Klassen, die Schulverwaltung alle. Ohne
// gewählten Zeitraum belegt das Backend das laufende Schulhalbjahr aus dem
// Schulkalender vor. Gezählt wird im Backend; die Seite zeigt nur an.

import { useMemo, useState } from "react";
import { useSession } from "next-auth/react";
import { Download } from "lucide-react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { ISODatePicker } from "~/components/ui/date-picker";
import { EmptyState } from "~/components/ui/empty-state";
import { ExportDialog } from "~/components/ui/export-dialog";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import { Skeleton } from "~/components/ui/skeleton";
import { useExportRun } from "~/hooks/use-export-run";
import { apiErrorFromResponse, transportFetch } from "~/lib/api-error";
import { formatDate, parseISODate } from "~/lib/date-helpers";
import {
  downloadBlob,
  filenameFromDisposition,
  openBlobForPrint,
} from "~/lib/file-download";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import {
  absenceListExportPath,
  fetchAbsenceListSchool,
  fetchClassDayClassesSchool,
  type AbsenceList,
  type AbsenceListChild,
  type AbsenceListExportFormat,
  type AbsenceListQuery,
} from "~/lib/school-class-day-api";
import { schoolClassLabel } from "~/lib/school-class-label";
import { useSWRAuth } from "~/lib/swr";
import { schoolPortalKeys } from "~/lib/swr/portal-keys";

const EXPORT_FORMATS: readonly AbsenceListExportFormat[] = ["pdf", "xlsx"];

/** Längster Zeitraum einer Liste, wie im Backend: ein Schuljahr mit Puffer. */
const MAX_PERIOD_DAYS = 400;

interface Period {
  readonly from: string;
  readonly to: string;
}

function periodDays(period: Period): number {
  const from = parseISODate(period.from).getTime();
  const to = parseISODate(period.to).getTime();
  return Math.round((to - from) / 86_400_000) + 1;
}

/** Hinweis zu einem ungültigen Zeitraum, mit dem Feld, an dem er steht. */
interface PeriodHint {
  readonly field: keyof Period;
  readonly text: string;
}

function periodHint(period: Period): PeriodHint | null {
  if (!period.from)
    return { field: "from", text: "Bitte wählen Sie ein Von-Datum." };
  if (!period.to)
    return { field: "to", text: "Bitte wählen Sie ein Bis-Datum." };
  if (period.to < period.from)
    return { field: "to", text: "Das Bis-Datum liegt vor dem Von-Datum." };
  if (periodDays(period) > MAX_PERIOD_DAYS)
    return { field: "to", text: "Wählen Sie höchstens ein Schuljahr." };
  return null;
}

function childName(child: AbsenceListChild): string {
  return `${child.lastName}, ${child.firstName}`;
}

/** Zeile unter der Auswahl: welcher Zeitraum gezählt ist. */
function countedLine(list: AbsenceList): string {
  if (!list.countedUntil && list.incomplete) {
    return "Für diesen Zeitraum sind keine vollständigen Angaben mehr vorhanden.";
  }
  if (!list.countedUntil) {
    return "Der Zeitraum liegt in der Zukunft. Noch kein Tag ist gezählt.";
  }
  const parts = [
    `Gezählt bis ${formatDate(list.countedUntil)}`,
    `${list.schoolDays} Schultage`,
    `Klasse gesamt: ${list.totals.absentDays} Fehltage, davon ${list.totals.excusedDays} entschuldigt`,
  ];
  if (list.periodName) parts.unshift(list.periodName);
  return parts.join(" · ");
}

const columns: DataTableColumn<AbsenceListChild>[] = [
  {
    key: "name",
    header: "Name",
    render: childName,
    sortValue: childName,
    stacked: "title",
  },
  ...(
    [
      ["absent", "Fehltage", "absentDays"],
      ["excused", "Davon entschuldigt", "excusedDays"],
      ["unexcused", "Davon unentschuldigt", "unexcusedDays"],
      ["notRecorded", "Nicht erfasst", "notRecordedDays"],
    ] as const
  ).map(([key, header, field]): DataTableColumn<AbsenceListChild> => ({
    key,
    header,
    render: (child) => child[field],
    sortValue: (child) => child[field],
    align: "right",
  })),
];

function childKey(child: AbsenceListChild): string {
  return child.studentId;
}

async function downloadAbsenceList(
  query: AbsenceListQuery,
  format: AbsenceListExportFormat,
  printTarget?: Window,
): Promise<void> {
  const response = await transportFetch(absenceListExportPath(query, format));
  if (!response.ok) {
    throw await apiErrorFromResponse(response, "absence list export failed");
  }
  const blob = await response.blob();
  if (printTarget) {
    await openBlobForPrint(blob, printTarget);
    return;
  }
  downloadBlob(
    blob,
    filenameFromDisposition(response) ?? `fehlzeiten.${format}`,
  );
}

export default function SchoolAbsenceListPage() {
  const { data: session } = useSession();

  const classesQuery = useSWRAuth(
    session ? schoolPortalKeys.classDayClasses() : null,
    fetchClassDayClassesSchool,
    { revalidateOnFocus: false },
  );
  const classes = useMemo(
    () => classesQuery.data?.classes ?? [],
    [classesQuery.data],
  );
  const classesError = useSwrLoadError(
    classesQuery.error,
    "die Klassen",
    () => void classesQuery.mutate(),
  );

  const [chosenClass, setChosenClass] = useState("");
  const schoolClass = classes.includes(chosenClass)
    ? chosenClass
    : (classes[0] ?? "");
  // Ohne eigene Wahl zeigt die Seite das Halbjahr, das das Backend vorbelegt.
  const [chosenPeriod, setChosenPeriod] = useState<Period | null>(null);

  const listQuery = useSWRAuth(
    session && schoolClass
      ? schoolPortalKeys.absenceList(
          schoolClass,
          chosenPeriod?.from,
          chosenPeriod?.to,
        )
      : null,
    () =>
      fetchAbsenceListSchool({
        schoolClass,
        period: chosenPeriod ?? undefined,
      }),
    { revalidateOnFocus: false, keepPreviousData: false },
  );
  const [draft, setDraft] = useState<Period | null>(null);
  const loadedList = listQuery.data;
  const list =
    loadedList &&
    !listQuery.error &&
    loadedList.schoolClass === schoolClass &&
    (!chosenPeriod ||
      (loadedList.from === chosenPeriod.from &&
        loadedList.to === chosenPeriod.to)) &&
    (!draft || (loadedList.from === draft.from && loadedList.to === draft.to))
      ? loadedList
      : undefined;
  const listError = useSwrLoadError(
    listQuery.error,
    "die Fehlzeiten",
    () => void listQuery.mutate(),
  );

  // Die Felder zeigen die Eingabe, sonst das vorbelegte Halbjahr. Nur eine
  // gültige Eingabe wird geladen (chosenPeriod).
  const fields = draft ?? {
    from: loadedList?.from ?? "",
    to: loadedList?.to ?? "",
  };
  const fieldHint = draft ? periodHint(draft) : null;
  const rows = useMemo(() => [...(list?.children ?? [])], [list]);

  const changePeriod = (next: Period) => {
    setDraft(next);
    if (periodHint(next) === null) setChosenPeriod(next);
  };

  const [exportOpen, setExportOpen] = useState(false);
  // Ohne eigene Wahl rechnet auch der Export mit dem vorbelegten Halbjahr;
  // so trägt die Datei dessen Namen.
  const exportQuery: AbsenceListQuery = {
    schoolClass,
    period: chosenPeriod ?? undefined,
  };
  const run = useExportRun({
    isOpen: exportOpen,
    onDownload: (format: AbsenceListExportFormat) =>
      downloadAbsenceList(exportQuery, format),
    onPrint: (target) => downloadAbsenceList(exportQuery, "pdf", target),
    errorObject: "die Fehlzeitenliste",
    onDone: () => setExportOpen(false),
    logName: "school_absence_list",
  });

  const classOptions = classes.map((value) => ({
    value,
    label: schoolClassLabel(value),
  }));
  const noClasses = classesQuery.data !== undefined && classes.length === 0;

  return (
    <div className="-mt-1.5 w-full space-y-6">
      <div>
        <h1 className="text-xl font-semibold text-gray-900 sm:text-2xl">
          Fehlzeiten
        </h1>
        <p className="mt-1 text-sm text-gray-500">
          Fehltage je Kind für Klassenbuch und Zeugnis.
        </p>
      </div>

      {classesQuery.error && classesError ? (
        <LoadErrorAlert error={classesError} />
      ) : noClasses ? (
        <SectionCard>
          <EmptyState
            title="Ihnen ist keine Klasse zugeordnet."
            description="Die OGS-Leitung ordnet Ihnen eine Klasse zu."
          />
        </SectionCard>
      ) : (
        <>
          <SectionCard
            title="Klasse und Zeitraum"
            description="Nicht erfasst heißt: An diesem Schultag gibt es keinen Eintrag."
            overflow="visible"
            action={
              <Button
                type="button"
                variant="outline"
                size="md"
                onClick={() => setExportOpen(true)}
                disabled={!list}
              >
                <Download className="mr-2 h-4 w-4" aria-hidden />
                Exportieren
              </Button>
            }
          >
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
              <label htmlFor="absence-list-class" className="block">
                <span className="mb-1 block text-sm font-medium text-gray-700">
                  Klasse
                </span>
                <CustomSelect
                  id="absence-list-class"
                  ariaLabel="Klasse"
                  value={schoolClass}
                  options={classOptions}
                  onChange={setChosenClass}
                  disabled={classes.length < 2}
                />
              </label>
              <ISODatePicker
                label="Von"
                id="absence-list-from"
                value={fields.from}
                max={fields.to || undefined}
                error={fieldHint?.field === "from" ? fieldHint.text : undefined}
                onChange={(from) => changePeriod({ ...fields, from })}
                calendarLayout="popover"
                hideClearButton
              />
              <ISODatePicker
                label="Bis"
                id="absence-list-to"
                value={fields.to}
                min={fields.from || undefined}
                error={fieldHint?.field === "to" ? fieldHint.text : undefined}
                onChange={(to) => changePeriod({ ...fields, to })}
                calendarLayout="popover"
                hideClearButton
              />
            </div>
            {list ? (
              <p className="mt-4 text-sm text-gray-600">{countedLine(list)}</p>
            ) : (
              <Skeleton className="mt-4 h-4 w-64" />
            )}
          </SectionCard>

          {list?.incomplete && (
            <Alert
              type="warning"
              title="Unvollständiger Zeitraum"
              message={`Gezählt wird erst ab ${formatDate(list.availableFrom)}. Ältere Anwesenheit oder Meldungen können bereits gelöscht sein.`}
            />
          )}

          {listQuery.error && listError ? (
            <LoadErrorAlert error={listError} />
          ) : (
            <DataTable
              columns={columns}
              rows={rows}
              getRowKey={childKey}
              isLoading={!list}
              defaultSortKey="name"
              stackedOnMobile
              emptyState={
                <EmptyState
                  title="In dieser Klasse sind keine Kinder."
                  description="Kinder ohne eigenen Eintrag in moto stehen noch nicht in der Liste."
                />
              }
            />
          )}
        </>
      )}

      <ExportDialog
        isOpen={exportOpen && !!list}
        onClose={() => setExportOpen(false)}
        title="Fehlzeiten exportieren"
        description="Fehltage je Kind der Klasse im gewählten Zeitraum."
        appliedFilters={
          list
            ? [
                schoolClassLabel(list.schoolClass),
                `${formatDate(list.from)} bis ${formatDate(list.to)}`,
              ]
            : []
        }
        formats={EXPORT_FORMATS}
        run={run}
      />
    </div>
  );
}

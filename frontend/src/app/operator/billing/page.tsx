"use client";

import { Suspense, useMemo, useState } from "react";
import { useSession } from "next-auth/react";
// eslint-disable-next-line no-restricted-imports -- operator pages are not tenant-scoped
import useSWR from "swr";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { EditActions } from "~/components/ui/edit-actions";
import { EmptyState } from "~/components/ui/empty-state";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { SectionCard } from "~/components/ui/section-card";
import { StatCard } from "~/components/ui/stat-card";
import { useToast } from "~/contexts/ToastContext";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import {
  berlinClockFromISO,
  berlinTodayISO,
  formatBerlinDate,
  formatDate,
  parseISODate,
} from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  BILLING_KEY_DAY_MAX,
  BILLING_KEY_DAY_MIN,
  billingMonth,
  operatorBillingService,
  type BillingKeyDateCount,
  type BillingKeyDay,
} from "~/lib/operator/billing-api";

const logger = createLogger({ component: "OperatorBillingPage" });

const KEY_DAY_SWR_KEY = "operator-billing-key-day";
const COUNTS_SWR_KEY = "operator-billing-key-date-counts";

const KEY_DAY_OPTIONS = Array.from(
  { length: BILLING_KEY_DAY_MAX - BILLING_KEY_DAY_MIN + 1 },
  (_, index) => {
    const day = String(BILLING_KEY_DAY_MIN + index);
    return { value: day, label: `${day}.` };
  },
);

/** "September 2026" for a period (YYYY-MM-01). */
function monthLabel(period: string): string {
  return parseISODate(period).toLocaleDateString("de-DE", {
    month: "long",
    year: "numeric",
  });
}

/** The capture ran after the key date's day (the worker was down). */
function capturedLate(row: BillingKeyDateCount): boolean {
  return berlinTodayISO(new Date(row.recordedAt)) > row.keyDate;
}

function recordedLabel(row: BillingKeyDateCount): string {
  return `${formatBerlinDate(row.recordedAt)}, ${berlinClockFromISO(row.recordedAt)} Uhr`;
}

const COLUMNS: DataTableColumn<BillingKeyDateCount>[] = [
  {
    key: "school",
    header: "Schule",
    render: (row) => row.schoolName,
    sortValue: (row) => row.schoolName.toLowerCase(),
    stacked: "title",
  },
  {
    key: "organization",
    header: "Träger",
    render: (row) => row.organizationName,
    sortValue: (row) => row.organizationName.toLowerCase(),
    stacked: "meta",
  },
  {
    key: "students",
    header: "Aktive Kinder",
    render: (row) => row.activeStudents,
    sortValue: (row) => row.activeStudents,
    align: "right",
    stacked: "field",
  },
  {
    key: "terminals",
    header: "Aktive Terminals",
    render: (row) => row.activeTerminals,
    sortValue: (row) => row.activeTerminals,
    align: "right",
    stacked: "field",
  },
  {
    key: "recorded",
    header: "Erfasst am",
    render: (row) =>
      capturedLate(row) ? (
        <span title="Die Zahlen wurden nach dem Stichtag erfasst, weil die Erfassung am Stichtag nicht lief.">
          {recordedLabel(row)} · nachträglich
        </span>
      ) : (
        recordedLabel(row)
      ),
    sortValue: (row) => row.recordedAt,
    stacked: "field",
  },
];

function KeyDaySection({
  keyDay,
  loading,
  onSaved,
}: Readonly<{
  keyDay: BillingKeyDay | undefined;
  loading: boolean;
  onSaved: (keyDay: BillingKeyDay) => void;
}>) {
  const { success: toastSuccess } = useToast();
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const startEditing = () => {
    setDraft(String(keyDay?.keyDay ?? ""));
    setError("");
    setEditing(true);
  };

  const save = async () => {
    setSaving(true);
    setError("");
    try {
      const updated = await operatorBillingService.updateKeyDay(Number(draft));
      onSaved(updated);
      setEditing(false);
      toastSuccess(`Stichtag ist jetzt der ${updated.keyDay}. des Monats.`);
    } catch (saveError) {
      logger.error("billing_key_day_update_failed", {
        error: saveError instanceof Error ? saveError.message : "unknown",
      });
      setError(
        "Der Stichtag konnte nicht gespeichert werden. Bitte versuchen Sie es noch einmal.",
      );
    } finally {
      setSaving(false);
    }
  };

  return (
    <SectionCard
      title="Stichtag"
      description="Am Stichtag hält moto für jede Schule fest, wie viele Kinder und Terminals aktiv sind. Diese Zahlen ändern sich danach nicht mehr."
      action={
        editing || !keyDay ? undefined : (
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={startEditing}
          >
            Ändern
          </Button>
        )
      }
    >
      {editing ? (
        <div className="space-y-4">
          <div className="max-w-xs space-y-1.5">
            <label
              id="billing-key-day-label"
              htmlFor="billing-key-day"
              className="block text-sm font-medium text-gray-700"
            >
              Tag im Monat
            </label>
            <CustomSelect
              id="billing-key-day"
              labelId="billing-key-day-label"
              value={draft}
              options={KEY_DAY_OPTIONS}
              onChange={setDraft}
            />
            <p className="text-xs text-gray-500">
              Gilt ab dem nächsten Stichtag. Schon erfasste Monate bleiben, wie
              sie sind. Höchstens der 28., damit jeder Monat den Tag hat.
            </p>
          </div>
          <Alert type="error" message={error} />
          <EditActions
            onCancel={() => setEditing(false)}
            onSave={() => void save()}
            saving={saving}
            disabled={draft === "" || draft === String(keyDay?.keyDay)}
          />
        </div>
      ) : (
        <DataGrid>
          <DataField label="Stichtag">
            {loading || !keyDay ? "–" : `Jeden Monat am ${keyDay.keyDay}.`}
          </DataField>
          <DataField label="Nächster Stichtag">
            {loading || !keyDay ? "–" : formatDate(keyDay.nextKeyDate)}
          </DataField>
        </DataGrid>
      )}
    </SectionCard>
  );
}

function CountingRulesSection() {
  return (
    <SectionCard
      title="So wird gezählt"
      description="Maßgeblich ist der Stand am Stichtag ab 6 Uhr."
    >
      <DataGrid>
        <DataField label="Aktive Kinder">
          Kinder mit dem Status „aktiv“. Ein sofort aktiviertes Kind zählt auch
          vor dem geplanten Betreuungsbeginn. Nicht gezählt werden Kinder mit
          dem Status „ausstehend“, mit beendeter Betreuung, nach dem Schulabgang
          und gelöschte Kinder.
        </DataField>
        <DataField label="Aktive Terminals">
          Physische Geräte mit dem Status „aktiv“. Auch offline Geräte zählen.
          Nicht gezählt werden inaktive Geräte, Geräte in Wartung und die
          Erfassung im Browser. Ein umgezogenes Gerät zählt nur bei der neuen
          Schule.
        </DataField>
      </DataGrid>
    </SectionCard>
  );
}

function OperatorBillingPageContent() {
  useSetBreadcrumb({ pageTitle: "Stichtagszahlen" });
  const { status } = useSession();
  const { error: toastError } = useToast();
  const isAuthenticated = status === "authenticated";

  const {
    data: keyDay,
    isLoading: keyDayLoading,
    mutate: mutateKeyDay,
  } = useSWR(isAuthenticated ? KEY_DAY_SWR_KEY : null, () =>
    operatorBillingService.getKeyDay(),
  );
  const {
    data: counts,
    isLoading: countsLoading,
    error: countsError,
  } = useSWR(isAuthenticated ? COUNTS_SWR_KEY : null, () =>
    operatorBillingService.listKeyDateCounts(),
  );

  const periods = useMemo(
    () => [...new Set((counts ?? []).map((row) => row.period))],
    [counts],
  );
  const [chosenPeriod, setChosenPeriod] = useState<string | null>(null);
  const period =
    chosenPeriod && periods.includes(chosenPeriod) ? chosenPeriod : periods[0];

  const rows = useMemo(
    () => (counts ?? []).filter((row) => row.period === period),
    [counts, period],
  );
  const totalStudents = rows.reduce((sum, row) => sum + row.activeStudents, 0);
  const totalTerminals = rows.reduce(
    (sum, row) => sum + row.activeTerminals,
    0,
  );

  const [downloading, setDownloading] = useState<"month" | "all" | null>(null);
  const download = async (scope: "month" | "all") => {
    setDownloading(scope);
    try {
      await operatorBillingService.downloadKeyDateCounts(
        scope === "month" && period ? billingMonth(period) : undefined,
      );
    } catch (downloadError) {
      logger.error("billing_export_failed", {
        error:
          downloadError instanceof Error ? downloadError.message : "unknown",
      });
      toastError(
        "Die Datei konnte nicht erstellt werden. Bitte versuchen Sie es noch einmal.",
      );
    } finally {
      setDownloading(null);
    }
  };

  const hasCounts = periods.length > 0;

  return (
    <div className="-mt-1.5 w-full space-y-6">
      <PageHeaderWithSearch
        title="Stichtagszahlen"
        concept="reports"
        tabs={{
          items: [{ id: "billing", label: "Stichtagszahlen" }],
          activeTab: "billing",
          onTabChange: () => undefined,
        }}
      />

      <KeyDaySection
        keyDay={keyDay}
        loading={keyDayLoading}
        onSaved={(updated) => void mutateKeyDay(updated, false)}
      />

      <SectionCard
        title="Stichtagszahlen"
        description="Je Monat und Schule, so wie moto sie am Stichtag erfasst hat."
        action={
          hasCounts ? (
            <div className="flex flex-wrap items-center gap-2">
              <CustomSelect
                ariaLabel="Monat"
                value={period ?? ""}
                options={periods.map((value) => ({
                  value,
                  label: monthLabel(value),
                }))}
                onChange={setChosenPeriod}
                className="min-w-44"
              />
              <Button
                type="button"
                variant="outline"
                size="md"
                disabled={downloading !== null}
                onClick={() => void download("month")}
              >
                {downloading === "month" ? "Erstellt…" : "Monat als CSV"}
              </Button>
              <Button
                type="button"
                variant="outline"
                size="md"
                disabled={downloading !== null}
                onClick={() => void download("all")}
              >
                {downloading === "all" ? "Erstellt…" : "Alle Monate als CSV"}
              </Button>
            </div>
          ) : undefined
        }
      >
        {countsError ? (
          <Alert
            type="error"
            message="Die Stichtagszahlen konnten nicht geladen werden. Bitte laden Sie die Seite neu."
          />
        ) : !countsLoading && !hasCounts ? (
          <EmptyState
            title="Noch keine Stichtagszahlen"
            description="moto erfasst die Zahlen am Stichtag ab 6 Uhr. Fällt die Erfassung am Stichtag aus, holt moto sie im selben Monat nach."
          />
        ) : (
          <div className="space-y-4">
            {period && (
              <p className="text-sm text-gray-600">
                Stichtag {rows[0] ? formatDate(rows[0].keyDate) : "–"}
              </p>
            )}
            <div className="grid grid-cols-3 gap-3">
              <StatCard variant="tile" label="Schulen" value={rows.length} />
              <StatCard
                variant="tile"
                label="Aktive Kinder"
                value={totalStudents}
              />
              <StatCard
                variant="tile"
                label="Aktive Terminals"
                value={totalTerminals}
              />
            </div>
            <DataTable
              columns={COLUMNS}
              rows={rows}
              getRowKey={(row) => `${row.period}-${row.schoolId}`}
              isLoading={countsLoading}
              defaultSortKey="organization"
              stackedOnMobile
            />
          </div>
        )}
      </SectionCard>

      <CountingRulesSection />
    </div>
  );
}

export default function OperatorBillingPage() {
  return (
    <Suspense fallback={<div className="-mt-1.5 w-full" />}>
      <OperatorBillingPageContent />
    </Suspense>
  );
}

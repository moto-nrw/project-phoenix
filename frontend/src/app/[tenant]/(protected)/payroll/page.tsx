"use client";

import { payrollKeys } from "~/lib/swr/portal-keys";

import {
  type ReactNode,
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { useSession } from "next-auth/react";
import { CircleCheck, CircleDashed, TriangleAlert } from "lucide-react";
import { useSWRAuth } from "~/lib/swr";
import { SwitchRow } from "~/components/ui/switch";
import { Alert } from "~/components/ui/alert";
import { CustomSelect } from "~/components/ui/custom-select";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { FieldLabel } from "~/components/ui/field-label";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import NavigationLink from "~/components/ui/navigation-link";
import { SectionCard } from "~/components/ui/section-card";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { Skeleton } from "~/components/ui/skeleton";
import {
  StatusBadge,
  type StatusBadgeTone,
} from "~/components/ui/status-badge";
import { TenantPage } from "~/components/ui/tenant-page";
import { useApiFormError } from "~/contexts/ToastContext";
import { formatChatDateTime } from "~/lib/date-helpers";
import { errorStatus } from "~/lib/expected-failure";
import { useRequirePermission } from "~/lib/hooks/use-require-permission";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { createLogger } from "~/lib/logger";
import {
  duplicateLohnartNumbers,
  fetchPayrollDispatch,
  fetchPayrollStatus,
  fetchPayrollTargetCoverage,
  PAYROLL_DISPATCH_CHANNEL_LABELS,
  PAYROLL_DISPATCH_EMAIL_PATTERN,
  PAYROLL_DISPATCH_FORMAT_LABELS,
  PAYROLL_DISPATCH_KEYS,
  payrollDispatchReasonText,
  payrollMonthLabel,
  savePayrollSetting,
  type PayrollDispatchChannel,
  type PayrollDispatchFormat,
  type PayrollDispatchSettings,
  type PayrollDispatchState,
  type PayrollDispatchView,
  type PayrollSettingValue,
  type PayrollStatus,
  type PayrollTargetCoverage,
} from "~/lib/payroll-api";
import { hasPermission } from "~/lib/auth-utils";
import { notBookedModuleOf } from "~/lib/module-not-booked";
import { MISSING_TARGET_OWNER } from "~/components/staff/missing-target-notice";
import { ProgressBar } from "~/components/ui/progress-bar";
import {
  ResponsiveActionList,
  ResponsiveActionRow,
} from "~/components/ui/responsive-action-list";
import { fetchSFTPStatus, type SFTPStatus } from "~/lib/sftp-export-api";
import {
  archiveSurchargeRule,
  archiveWageType,
  clockToMinutes,
  fetchPayrollRules,
  formatRate,
  minutesToClock,
  parseRate,
  setAbsenceTypeWageType,
  SURCHARGE_KIND_LABELS,
  updateSurchargeRule,
  updateWageType,
  WAGE_TYPE_UNIT_LABELS,
  wageTypesForAbsences,
  wageTypesForRate,
  type PayrollAbsenceTypeMapping,
  type PayrollRules,
  type PayrollSurchargeRule,
  type PayrollWageType,
  type WageTypeUnit,
} from "~/lib/payroll-rules-api";
import {
  CreateSurchargeRuleModal,
  CreateWageTypeModal,
} from "~/components/payroll/payroll-rule-modals";
import { Button, ButtonLink } from "~/components/ui/button";
import { ConfirmationModal } from "~/components/ui/modal";
import { EmptyState } from "~/components/ui/empty-state";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";

const logger = createLogger({ component: "PayrollPage" });

// Abrechnung (#1417 Tranche 2b): maintenance surface for the DATEV payroll
// foundation — Lohnart mapping and LODAS client identifiers. Values are
// settings-backed (config.setting_values, audited via config.setting_audit);
// this page is the hand-written panel over them, following the
// personalization-tab precedent instead of the auto-generated settings tab.
//
// Bauart 4 (Einstellungen): die einzige Bauart, die automatisch speichert.
// Das Verhalten steht als Satz in den Beschreibungen der beiden
// Eingabekarten, damit es niemand raten muss.
//
// There are deliberately NO defaults: Lohnartnummern are mandant-specific,
// and an invented preset would silently bill wrong. Empty is the honest
// starting state; the later DATEV writers refuse to export while required
// pieces are missing.

type SaveSetting = (key: string, value: PayrollSettingValue) => Promise<void>;

/** Saves one change of the school's own wage types or rules (#4807). */
type ChangeRules = (
  object: string,
  run: () => Promise<void>,
  queueKey?: string,
) => Promise<boolean>;

// The responsive table remounts its cells when the layout changes. Keep
// their unsaved drafts in the owning card, not in the discarded cell tree.
// A changed server value still replaces the draft, as it did in each editor.
type PayrollDrafts = Map<string, { current: string; value: string }>;

function usePayrollDraft(drafts: PayrollDrafts, key: string, current: string) {
  const [value, setValue] = useState(() => {
    const draft = drafts.get(key);
    return draft?.current === current ? draft.value : current;
  });
  useEffect(() => {
    if (drafts.get(key)?.current !== current) {
      drafts.delete(key);
      setValue(current);
    }
  }, [drafts, key, current]);

  const update = (next: string) => {
    drafts.set(key, { current, value: next });
    setValue(next);
  };
  return [value, update] as const;
}

const UNIT_OPTIONS = [
  { value: "", label: "Nicht festgelegt" },
  { value: "stunden", label: "Stunden" },
  { value: "tage", label: "Tage" },
];

export default function PayrollPage() {
  const { isReady, isLoading: permissionLoading } = useRequirePermission([
    "config:manage",
  ]);

  const {
    data: status,
    error,
    mutate,
  } = useSWRAuth(isReady ? payrollKeys.status() : null, fetchPayrollStatus, {
    keepPreviousData: false,
  });

  const loadError = useSwrLoadError(
    error,
    "die Einstellungen zur Abrechnung",
    () => mutate(),
  );

  // Letzter Versand an das Lohnbüro (#4805): ein eigener Ladefehler im
  // Block, die übrigen Karten bleiben bedienbar.
  const {
    data: latestDispatch,
    error: dispatchError,
    mutate: mutateDispatch,
  } = useSWRAuth(
    isReady ? payrollKeys.dispatch() : null,
    fetchPayrollDispatch,
    { keepPreviousData: false },
  );
  const dispatchLoadError = useSwrLoadError(
    dispatchError,
    "die Übersicht zum letzten Versand",
    () => mutateDispatch(),
  );

  // Eigene Lohnarten und Zuschläge (#4807): ein eigener Ladefehler im
  // Block, die übrigen Karten bleiben bedienbar.
  const {
    data: rules,
    error: rulesError,
    mutate: mutateRules,
  } = useSWRAuth(isReady ? payrollKeys.rules() : null, fetchPayrollRules, {
    keepPreviousData: false,
  });
  const rulesLoadError = useSwrLoadError(
    rulesError,
    "die eigenen Lohnarten und Zuschläge",
    () => mutateRules(),
  );

  // Soll-Prüfung (#6890): dieselbe Rechnung wie die Zeitkonten, nur die
  // Zahlen. Ein eigener Ladefehler in der Karte, die übrigen Zeilen bleiben.
  const {
    data: targetCoverage,
    error: targetCoverageError,
    mutate: mutateTargetCoverage,
  } = useSWRAuth(
    isReady ? payrollKeys.targetCoverage() : null,
    fetchPayrollTargetCoverage,
    { keepPreviousData: false, shouldRetryOnError: false },
  );
  const targetCoverageLoadError = useSwrLoadError(
    targetCoverageError,
    "die Prüfung des Solls",
    () => mutateTargetCoverage(),
  );
  const { data: session } = useSession();
  const canOpenTimeAccounts = hasPermission(session, "time_tracking:manage");

  // Ob die SFTP-Gegenstelle bereit ist, braucht time_tracking:manage. Fehlt
  // das Recht oder scheitert die Abfrage, entfällt nur der Hinweis: Der
  // Versand meldet eine fehlende Gegenstelle ohnehin selbst.
  const { data: sftpStatus } = useSWRAuth(
    isReady && status?.dispatch.channel === "sftp"
      ? payrollKeys.sftpStatus()
      : null,
    fetchSFTPStatus,
    {
      keepPreviousData: false,
      shouldRetryOnError: false,
      onError: (err: unknown) => {
        if (errorStatus(err) === 403) return;
        logger.warn("payroll_sftp_status_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      },
    },
  );
  // Bauart 4 speichert jedes Feld für sich: der Fehler steht über den
  // Karten, mit Wiederholen für genau diesen Wert (#2517).
  const formRef = useRef<HTMLDivElement>(null);
  const {
    error: saveError,
    show: showSaveError,
    clear: clearSaveError,
  } = useApiFormError(formRef);
  const saveQueues = useRef(new Map<string, Promise<void>>());
  const saveRef = useRef<(key: string, value: PayrollSettingValue) => void>(
    () => undefined,
  );

  const save = useCallback(
    (key: string, value: PayrollSettingValue) => {
      const previous = saveQueues.current.get(key) ?? Promise.resolve();
      const next = previous
        .then(async () => {
          clearSaveError();
          await savePayrollSetting(key, value);
          await mutate();
        })
        .catch((err: unknown) => {
          logger.error("payroll_setting_save_failed", {
            key,
            error: err instanceof Error ? err.message : String(err),
          });
          void showSaveError(err, {
            object: "die Einstellung",
            retry: () => saveRef.current(key, value),
          });
        });

      saveQueues.current.set(key, next);
      void next.finally(() => {
        if (saveQueues.current.get(key) === next) {
          saveQueues.current.delete(key);
        }
      });
      return next;
    },
    [mutate, clearSaveError, showSaveError],
  );
  // Wiederholen läuft über die Warteschlange, wie ein neues Speichern.
  useLayoutEffect(() => {
    saveRef.current = (key, value) => void save(key, value);
  });

  // Eine Änderung an den eigenen Lohnarten und Regeln: der Fehler steht wie
  // beim Speichern einer Einstellung über den Karten, danach gilt wieder der
  // Stand vom Server.
  const refreshRules = useCallback(
    async (showFailure = true) => {
      try {
        await mutateRules();
      } catch (err: unknown) {
        logger.error("payroll_rules_refresh_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (showFailure) {
          void showSaveError(err, {
            object: "die Übersicht zu den Lohnarten und Zuschlägen",
          });
        }
      }
    },
    [mutateRules, showSaveError],
  );
  const changeRulesRef = useRef<ChangeRules>(() => Promise.resolve(false));
  const ruleSaveQueues = useRef(new Map<string, Promise<boolean>>());
  const changeRules = useCallback<ChangeRules>(
    async (object, run, queueKey) => {
      const save = async () => {
        clearSaveError();
        let saved = false;
        try {
          await run();
          saved = true;
        } catch (err: unknown) {
          logger.error("payroll_rule_save_failed", {
            error: err instanceof Error ? err.message : String(err),
          });
          void showSaveError(err, {
            object,
            retry: () => void changeRulesRef.current(object, run, queueKey),
          });
        } finally {
          await refreshRules(saved);
        }
        return saved;
      };
      if (!queueKey) {
        return save();
      }

      const previous =
        ruleSaveQueues.current.get(queueKey) ?? Promise.resolve();
      const next = previous.catch(() => false).then(save);
      ruleSaveQueues.current.set(queueKey, next);
      void next.finally(() => {
        if (ruleSaveQueues.current.get(queueKey) === next) {
          ruleSaveQueues.current.delete(queueKey);
        }
      });
      return next;
    },
    [refreshRules, clearSaveError, showSaveError],
  );
  useLayoutEffect(() => {
    changeRulesRef.current = changeRules;
  });

  // Permission-loading joins the data-loading condition below instead of an
  // early return before the header, so the page header renders immediately and
  // only the data region skeletonizes.
  // Bis der Katalogtext eines Ladefehlers da ist, bleibt das Skelett.
  const showSkeleton =
    permissionLoading || !isReady || (!status && (!error || !loadError));

  const duplicates = status ? duplicateLohnartNumbers(status) : [];

  // Statuszeile: die Zahlen, die die Seite ohnehin laedt.
  const statusLine = status
    ? `${status.configuredCategories} von ${status.totalCategories} Lohnarten zugeordnet · DATEV-Mandant ${
        status.lodasHeaderComplete ? "vollständig" : "unvollständig"
      } · ${status.staffWithoutPersonnelNumber} von ${status.staffTotal} ohne Personalnummer`
    : null;

  return (
    <TenantPage
      title="Abrechnung"
      stats={statusLine}
      statsLoading={showSkeleton}
      loading={showSkeleton}
      error={loadError}
    >
      {status && (
        <div ref={formRef} className="space-y-6">
          <ReadinessCard
            status={status}
            rules={rules}
            target={{
              coverage: targetCoverage,
              error: targetCoverageError,
              loadError: targetCoverageLoadError,
              canOpenTimeAccounts,
            }}
          />

          <FormErrorAlert message={saveError} />
          {duplicates.length > 0 && (
            <Alert
              type="warning"
              message={`Lohnartnummer ${duplicates.join(", ")} ist mehreren Kategorien zugeordnet. Das ist zulässig, führt aber meist zu doppelt gebuchten Stunden, bitte prüfen.`}
            />
          )}

          <LohnartenCard status={status} onSave={save} />
          <PayrollRuleCards
            rules={rules}
            loadError={rulesLoadError}
            onChange={changeRules}
            onCreated={() => void refreshRules()}
          />
          <DatevCard status={status} onSave={save} />
          <LohnbueroCard
            status={status}
            rules={rules}
            latest={latestDispatch}
            latestError={dispatchLoadError}
            sftpStatus={sftpStatus}
            onSave={save}
          />
        </div>
      )}
    </TenantPage>
  );
}

/** Zeitkonten des Monats mit dem Filter „ohne Soll“ (#6890). */
function staffWithoutTargetHref(year: number, month: number): string {
  return `/staff?view=accounts&filter=ohne-soll&month=${year}-${String(month).padStart(2, "0")}`;
}

type ReadinessState = "done" | "open" | "checking";

/** Symbol, Farbe und vorgelesener Zustand je Zeile. */
const READINESS_STATE = {
  done: {
    icon: CircleCheck,
    className: "text-moto-green-strong",
    sr: "erledigt",
  },
  open: {
    icon: TriangleAlert,
    className: "text-moto-orange-strong",
    sr: "fehlt",
  },
  checking: {
    icon: CircleDashed,
    className: "text-gray-400",
    sr: "wird geprüft",
  },
} as const;

interface ReadinessItem {
  readonly key: string;
  readonly label: string;
  readonly state: ReadinessState;
  readonly detail: string;
  readonly action?: ReactNode;
}

/** Die Soll-Prüfung (#6890), wie die Karte sie braucht. */
interface TargetCheck {
  /** `undefined` solange die Prüfung lädt. */
  readonly coverage: PayrollTargetCoverage | undefined;
  readonly error: unknown;
  readonly loadError: ReturnType<typeof useSwrLoadError>;
  /** Darf die Zeitkonten mit den Namen öffnen (time_tracking:manage). */
  readonly canOpenTimeAccounts: boolean;
}

function doneOrOpen(ok: boolean): ReadinessState {
  return ok ? "done" : "open";
}

function ReadinessCard({
  status,
  rules,
  target,
}: {
  readonly status: PayrollStatus;
  readonly rules: PayrollRules | undefined;
  readonly target: TargetCheck;
}) {
  const unnumbered = (rules?.wageTypes ?? []).filter((w) => w.number === "");
  const items: ReadinessItem[] = [
    {
      key: "lohnarten",
      label: "Feste Lohnarten",
      state: doneOrOpen(status.configuredCategories > 0),
      detail: `${status.configuredCategories} von ${status.totalCategories} Kategorien konfiguriert`,
    },
  ];
  if (rules && rules.wageTypes.length > 0) {
    items.push({
      key: "eigene-lohnarten",
      label: "Eigene Lohnarten",
      state: doneOrOpen(unnumbered.length === 0),
      detail:
        unnumbered.length === 0
          ? `Alle ${rules.wageTypes.length} haben eine Lohnartnummer`
          : `Ohne Lohnartnummer, fehlen im DATEV-Export: ${unnumbered.map((w) => w.name).join(", ")}`,
    });
  }
  items.push(
    {
      key: "datev",
      label: "DATEV-Mandant (nur LODAS)",
      state: doneOrOpen(status.lodasHeaderComplete),
      detail: status.lodasHeaderComplete
        ? "Berater- und Mandantennummer gesetzt"
        : "Berater- oder Mandantennummer fehlt",
    },
    {
      key: "personalnummern",
      label: "Personalnummern",
      state: doneOrOpen(status.staffWithoutPersonnelNumber === 0),
      detail:
        status.staffWithoutPersonnelNumber === 0
          ? `Alle ${status.staffTotal} Beschäftigten haben eine Personalnummer`
          : `${status.staffWithoutPersonnelNumber} von ${status.staffTotal} Beschäftigten ohne Personalnummer (Pflege im Stammdaten-Tab der Person)`,
    },
  );
  // Ohne gebuchte Zeiterfassung gibt es kein Soll zu prüfen: dann entfällt
  // die Zeile, statt einen Fehler zu zeigen.
  const targetCheckBooked = notBookedModuleOf(target.error) === null;
  if (targetCheckBooked) {
    items.push(targetItem(target));
  }

  const done = items.filter((item) => item.state === "done").length;
  const open = items.filter((item) => item.state === "open").length;
  const checking = items.some((item) => item.state === "checking");

  return (
    <SectionCard
      title="Vollständigkeit"
      titleBadge={
        open > 0 ? (
          <StatusBadge tone="warning" label={`${open} offen`} />
        ) : checking ? null : (
          <StatusBadge tone="green" label="Alles erledigt" />
        )
      }
      description="Ohne vollständige Konfiguration erzeugt der spätere DATEV-Export keine Datei. Eine Kategorie ohne Lohnartnummer wird nicht exportiert."
    >
      <ProgressBar
        value={done}
        max={items.length}
        label={`${done} von ${items.length} Punkten erledigt`}
        tone={open > 0 ? "orange" : "green"}
        size="sm"
        className="mb-4"
      />
      <ResponsiveActionList>
        {items.map((item) => (
          <ReadinessRow key={item.key} item={item} />
        ))}
      </ResponsiveActionList>
      {targetCheckBooked && target.loadError ? (
        <div className="mt-3">
          <LoadErrorAlert error={target.loadError} actionLayout="responsive" />
        </div>
      ) : null}
    </SectionCard>
  );
}

/**
 * Ob alle Personen mit Zeitkonto im Abrechnungsmonat ein Soll haben (#6890):
 * der Vormonat, solange er nicht abgeschlossen ist, sonst der laufende.
 * Dieselbe Zahl wie der Hinweis über den Zeitkonten; der Link führt zu genau
 * diesen Personen. Eine gescheiterte Prüfung zählt als offen, nie als
 * erledigt.
 */
function targetItem({
  coverage,
  error,
  canOpenTimeAccounts,
}: TargetCheck): ReadinessItem {
  const base = { key: "soll", label: "Soll (Arbeitszeit)" };
  if (error) {
    return { ...base, state: "open", detail: "Konnte nicht geprüft werden." };
  }
  if (!coverage) {
    return { ...base, state: "checking", detail: "Wird geprüft …" };
  }
  const month = payrollMonthLabel(coverage.year, coverage.month);
  const missing = coverage.staffWithoutTarget;
  if (missing === 0) {
    return {
      ...base,
      state: "done",
      detail:
        coverage.staffTotal === 1
          ? `Die eine Person mit Zeitkonto hat im ${month} ein Soll`
          : `Alle ${coverage.staffTotal} Personen mit Zeitkonto haben im ${month} ein Soll`,
    };
  }
  return {
    ...base,
    state: "open",
    detail: `${missing} von ${coverage.staffTotal} Personen mit Zeitkonto haben im ${month} kein Soll. Krankheit, Urlaub und Fortbildung zählen bei ihnen 0 Stunden.${
      canOpenTimeAccounts ? "" : ` ${MISSING_TARGET_OWNER}`
    }`,
    action: canOpenTimeAccounts ? (
      <ButtonLink
        href={staffWithoutTargetHref(coverage.year, coverage.month)}
        variant="surface"
        size="compact"
        tooltip="Öffnet die Zeitkonten mit allen Personen ohne Soll."
      >
        Personen ohne Soll
      </ButtonLink>
    ) : undefined,
  };
}

/** Eine Zeile der Vollständigkeit: Zustand als Symbol, Text, Aktion. */
function ReadinessRow({ item }: { readonly item: ReadinessItem }) {
  const { icon: Icon, className, sr } = READINESS_STATE[item.state];
  return (
    <ResponsiveActionRow>
      <div className="flex min-w-0 items-start gap-3">
        <Icon
          className={`mt-0.5 size-5 shrink-0 ${className}`}
          aria-hidden="true"
        />
        <div className="min-w-0">
          <p className="text-sm font-semibold text-gray-900">
            {item.label}
            <span className="sr-only">: {sr}</span>
          </p>
          <p className="mt-0.5 text-sm break-words text-gray-600">
            {item.detail}
          </p>
        </div>
      </div>
      {item.action ? (
        <div className="flex w-full flex-col gap-2 pl-8 min-[480px]:w-auto min-[480px]:flex-row sm:justify-end sm:pl-0">
          {item.action}
        </div>
      ) : null}
    </ResponsiveActionRow>
  );
}

function LohnartenCard({
  status,
  onSave,
}: {
  readonly status: PayrollStatus;
  readonly onSave: SaveSetting;
}) {
  const [drafts] = useState<PayrollDrafts>(() => new Map());
  return (
    <SectionCard
      title="Feste Lohnarten"
      description="Mandantenspezifische Lohnartnummern aus dem Lohnsystem des Trägers (1 bis 4 Ziffern). Für Krank, Urlaub und Fortbildung zusätzlich die Einheit, die die Lohnart erwartet. Änderungen werden sofort gespeichert."
    >
      <DataTable
        // Auf dem Handy untereinander: die Auswahl der Einheit lag sonst
        // rechts außerhalb der Karte (#4975). Als Formular: Beschriftung
        // über dem Feld, alle Felder gleich breit (#4807).
        stackedOnMobile
        stackedLayout="form"
        columns={lohnartColumns(onSave, drafts)}
        rows={status.categories}
        getRowKey={(cat) => cat.id}
        rowHasInteractiveControls
      />
    </SectionCard>
  );
}

type LohnartCategory = PayrollStatus["categories"][number];

function lohnartColumns(
  onSave: SaveSetting,
  drafts: PayrollDrafts,
): DataTableColumn<LohnartCategory>[] {
  return [
    {
      key: "label",
      header: "Kategorie",
      stacked: "title",
      className: "font-medium text-gray-800",
      render: (cat) => cat.label,
    },
    {
      key: "number",
      header: "Lohnartnummer",
      render: (cat) => (
        <LohnartNumberCell cat={cat} onSave={onSave} drafts={drafts} />
      ),
    },
    {
      key: "unit",
      header: "Einheit",
      render: (cat) =>
        cat.unitRequired && cat.unitSettingKey ? (
          <CustomSelect
            ariaLabel={`Einheit ${cat.label}`}
            value={cat.unit}
            options={UNIT_OPTIONS}
            onChange={(next) => {
              if (next !== cat.unit && cat.unitSettingKey) {
                void onSave(cat.unitSettingKey, next);
              }
            }}
            controlSize="dense"
            className="md:w-auto md:min-w-[10rem]"
          />
        ) : (
          <span className="text-sm text-gray-600">Stunden (fest)</span>
        ),
    },
  ];
}

function LohnartNumberCell({
  cat,
  onSave,
  drafts,
}: {
  readonly cat: LohnartCategory;
  readonly drafts: PayrollDrafts;
  readonly onSave: SaveSetting;
}) {
  const [number, setNumber] = usePayrollDraft(drafts, cat.id, cat.number);

  const valid = number.trim() === "" || /^\d{1,4}$/.test(number.trim());

  return (
    <div className="w-full md:max-w-[10rem]">
      <Input
        aria-label={`Lohnartnummer ${cat.label}`}
        value={number}
        onChange={(e) => setNumber(e.target.value)}
        onBlur={() => {
          const trimmed = number.trim();
          if (valid && trimmed !== cat.number) {
            void onSave(cat.settingKey, trimmed);
          }
        }}
        placeholder="Nicht konfiguriert"
        inputMode="numeric"
        controlSize="compact"
        className="tabular-nums"
        error={valid ? undefined : "1 bis 4 Ziffern."}
      />
    </div>
  );
}

function DatevCard({
  status,
  onSave,
}: {
  readonly status: PayrollStatus;
  readonly onSave: SaveSetting;
}) {
  return (
    <SectionCard
      title="DATEV-Mandant"
      description="Kennzahlen für den Kopf der LODAS-Datei. Lohn und Gehalt benötigt sie nicht. Änderungen werden sofort gespeichert."
    >
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <DatevNumberField
          label="Beraternummer"
          settingKey="payroll.datev_beraternummer"
          current={status.beraternummer}
          maxDigits={7}
          onSave={onSave}
        />
        <DatevNumberField
          label="Mandantennummer"
          settingKey="payroll.datev_mandantennummer"
          current={status.mandantennummer}
          maxDigits={5}
          onSave={onSave}
        />
      </div>
    </SectionCard>
  );
}

function DatevNumberField({
  label,
  settingKey,
  current,
  maxDigits,
  onSave,
}: {
  readonly label: string;
  readonly settingKey: string;
  readonly current: string;
  readonly maxDigits: number;
  readonly onSave: SaveSetting;
}) {
  const [value, setValue] = useState(current);
  useEffect(() => setValue(current), [current]);

  const valid =
    value.trim() === "" ||
    new RegExp(String.raw`^\d{1,${maxDigits}}$`).test(value.trim());

  return (
    <Input
      id={`datev-${settingKey}`}
      label={label}
      value={value}
      onChange={(e) => setValue(e.target.value)}
      onBlur={() => {
        const trimmed = value.trim();
        if (valid && trimmed !== current) {
          void onSave(settingKey, trimmed);
        }
      }}
      placeholder="Nicht konfiguriert"
      inputMode="numeric"
      controlSize="compact"
      className="tabular-nums"
      error={valid ? undefined : `1 bis ${maxDigits} Ziffern.`}
    />
  );
}

const DISPATCH_CHANNEL_ITEMS: readonly {
  value: PayrollDispatchChannel;
  label: string;
}[] = [
  { value: "sftp", label: PAYROLL_DISPATCH_CHANNEL_LABELS.sftp },
  { value: "email", label: PAYROLL_DISPATCH_CHANNEL_LABELS.email },
];

const DISPATCH_FORMAT_OPTIONS = (
  Object.keys(PAYROLL_DISPATCH_FORMAT_LABELS) as PayrollDispatchFormat[]
).map((value) => ({ value, label: PAYROLL_DISPATCH_FORMAT_LABELS[value] }));

/** Die Gegenstelle liegt in den Einstellungen bei „Zeitkonten übertragen“. */
const SFTP_SETTINGS_HREF = "/settings?tab=system&highlight=sftp.enabled";

/**
 * Fehlt etwas, das die DATEV-Prüfung beim Versand verlangt? Dieselben Zahlen
 * wie die DATEV-Validierung im Backend; LODAS braucht zusätzlich den
 * Mandanten. Eine nummerierte eigene Lohnart kann die einzige Lohnart im
 * Export sein.
 */
function datevIncomplete(
  status: PayrollStatus,
  rules: PayrollRules | undefined,
  format: PayrollDispatchFormat,
): boolean {
  if (format !== "datev_lodas" && format !== "datev_lug") return false;
  return (
    status.staffWithoutPersonnelNumber > 0 ||
    (status.configuredCategories === 0 &&
      !rules?.wageTypes.some((wageType) => wageType.number !== "")) ||
    (format === "datev_lodas" && !status.lodasHeaderComplete)
  );
}

function LohnbueroCard({
  status,
  rules,
  latest,
  latestError,
  sftpStatus,
  onSave,
}: {
  readonly status: PayrollStatus;
  readonly rules: PayrollRules | undefined;
  /** `undefined` while loading, `null` before the first dispatch. */
  readonly latest: PayrollDispatchView | null | undefined;
  readonly latestError: ReturnType<typeof useSwrLoadError>;
  readonly sftpStatus: SFTPStatus | undefined;
  readonly onSave: SaveSetting;
}) {
  // Der gewählte Wert gilt sofort; bis das Speichern zurück ist, zeigt die
  // Karte ihn statt des geladenen. Danach (auch nach einem Fehler) gilt
  // wieder der Stand vom Server.
  const [draft, setDraft] = useState<Partial<PayrollDispatchSettings>>({});
  const current: PayrollDispatchSettings = { ...status.dispatch, ...draft };

  const change = <K extends keyof PayrollDispatchSettings>(
    field: K,
    value: PayrollDispatchSettings[K],
  ) => {
    if (value === current[field]) return;
    setDraft((prev) => ({ ...prev, [field]: value }));
    void onSave(PAYROLL_DISPATCH_KEYS[field], value).finally(() => {
      setDraft((prev) => {
        if (prev[field] !== value) return prev;
        const next = { ...prev };
        delete next[field];
        return next;
      });
    });
  };

  const [email, setEmail] = useState(status.dispatch.email);
  useEffect(() => setEmail(status.dispatch.email), [status.dispatch.email]);
  const trimmedEmail = email.trim();
  const emailValid =
    trimmedEmail === "" || PAYROLL_DISPATCH_EMAIL_PATTERN.test(trimmedEmail);

  const sftpHint =
    current.channel === "sftp" && sftpStatus && !sftpStatus.ready
      ? `${
          sftpStatus.enabled
            ? "Die Übertragung per SFTP ist noch nicht vollständig eingetragen."
            : "Die Übertragung per SFTP ist ausgeschaltet."
        } Sie steht in den Einstellungen unter Kalender und Export bei Zeitkonten übertragen. Bis dahin sendet moto nichts.`
      : "";
  const emailMissing =
    current.auto && current.channel === "email" && status.dispatch.email === "";

  return (
    <SectionCard
      title="Versand an das Lohnbüro"
      description="Nach dem Monatsabschluss schickt moto die Monatsdatei aller Mitarbeitenden an das Lohnbüro. Änderungen werden sofort gespeichert."
    >
      <div className="space-y-5">
        <SwitchRow
          framed={false}
          label="Automatisch nach dem Monatsabschluss senden"
          description="Schließen Sie einen Monat nach dem Wiederöffnen erneut ab, geht die Datei noch einmal raus. Ihr Name enthält dann „korrektur“."
          checked={current.auto}
          onCheckedChange={(next) => change("auto", next)}
        />

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div>
            <FieldLabel>Versandweg</FieldLabel>
            <SegmentedControl
              ariaLabel="Versandweg"
              items={DISPATCH_CHANNEL_ITEMS}
              value={current.channel}
              onChange={(next) => change("channel", next)}
            />
          </div>
          <div>
            <FieldLabel id="payroll-dispatch-format-label">
              Dateiformat
            </FieldLabel>
            <CustomSelect
              ariaLabelledBy="payroll-dispatch-format-label"
              value={current.format}
              options={DISPATCH_FORMAT_OPTIONS}
              onChange={(next) => {
                if (next in PAYROLL_DISPATCH_FORMAT_LABELS) {
                  change("format", next as PayrollDispatchFormat);
                }
              }}
            />
          </div>
        </div>

        {current.channel === "email" ? (
          <div className="space-y-1.5">
            <Input
              id="payroll-dispatch-email"
              type="email"
              label="E-Mail-Adresse des Lohnbüros"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              onBlur={() => {
                if (emailValid && trimmedEmail !== status.dispatch.email) {
                  change("email", trimmedEmail);
                }
              }}
              placeholder="lohn@beispiel.de"
              autoComplete="off"
              controlSize="compact"
              error={
                emailValid
                  ? undefined
                  : "Bitte geben Sie eine vollständige E-Mail-Adresse ein."
              }
            />
            <p className="text-xs text-gray-500">
              Das Lohnbüro bekommt einen Link, keinen Anhang. Der Link gilt 14
              Tage und funktioniert nur einmal.
            </p>
          </div>
        ) : (
          <p className="text-xs text-gray-500">
            moto legt die Datei auf der Gegenstelle ab, die in den Einstellungen
            bei Zeitkonten übertragen steht.
          </p>
        )}

        {emailMissing && (
          <Alert
            type="warning"
            message="Tragen Sie die E-Mail-Adresse des Lohnbüros ein. Ohne sie sendet moto nichts."
          />
        )}
        {sftpHint && (
          <Alert
            type="warning"
            message={sftpHint}
            action={
              <NavigationLink
                href={SFTP_SETTINGS_HREF}
                className="text-sm font-medium underline underline-offset-2"
              >
                Zu den Einstellungen
              </NavigationLink>
            }
          />
        )}
        {datevIncomplete(status, rules, current.format) && (
          <Alert
            type="warning"
            message="Für DATEV fehlen noch Angaben. Was fehlt, steht oben unter Vollständigkeit. Fehlt beim Monatsabschluss etwas, sendet moto keine Datei."
          />
        )}

        <div className="space-y-3 border-t border-gray-100 pt-4">
          <h3 className="text-sm font-semibold text-gray-900">
            Letzter Versand
          </h3>
          {latestError ? (
            <LoadErrorAlert error={latestError} actionLayout="responsive" />
          ) : latest === undefined ? (
            <Skeleton className="h-16 w-full" />
          ) : latest === null ? (
            <p className="text-sm text-gray-600">
              {current.auto
                ? "Noch nichts gesendet. Der erste Versand startet nach dem nächsten Monatsabschluss."
                : "Noch nichts gesendet."}
            </p>
          ) : (
            <LatestDispatch dispatch={latest} />
          )}
        </div>
      </div>
    </SectionCard>
  );
}

function dispatchStateBadge(dispatch: PayrollDispatchView): {
  label: string;
  tone: StatusBadgeTone;
} {
  const byState: Record<
    PayrollDispatchState,
    { label: string; tone: StatusBadgeTone }
  > = {
    pending:
      dispatch.attempts > 0
        ? { label: "Wird erneut versucht", tone: "blue" }
        : { label: "Wird vorbereitet", tone: "blue" },
    mailing: { label: "E-Mail wird zugestellt", tone: "blue" },
    sent:
      dispatch.channel === "email"
        ? { label: "E-Mail zugestellt", tone: "green" }
        : { label: "Übertragen", tone: "green" },
    blocked: { label: "Nicht gesendet", tone: "warning" },
    failed: { label: "Fehlgeschlagen", tone: "red" },
  };
  return byState[dispatch.state] ?? { label: dispatch.state, tone: "gray" };
}

function LatestDispatch({
  dispatch,
}: {
  readonly dispatch: PayrollDispatchView;
}) {
  const badge = dispatchStateBadge(dispatch);
  const reason = payrollDispatchReasonText(dispatch);
  const isEmail = dispatch.channel === "email";

  return (
    <div className="space-y-3">
      <DataGrid>
        <DataField label="Monat">
          {payrollMonthLabel(dispatch.year, dispatch.month)}
        </DataField>
        <DataField label="Korrektur">
          {dispatch.correction ? "Ja" : "Nein"}
        </DataField>
        <DataField label="Versandweg">
          {isEmail && dispatch.recipient
            ? `E-Mail an ${dispatch.recipient}`
            : PAYROLL_DISPATCH_CHANNEL_LABELS[dispatch.channel]}
        </DataField>
        <DataField label="Dateiformat">
          {PAYROLL_DISPATCH_FORMAT_LABELS[dispatch.format]}
        </DataField>
        <DataField label="Stand">
          <StatusBadge label={badge.label} tone={badge.tone} />
        </DataField>
        {dispatch.filename && (
          <DataField label="Datei" mono>
            {dispatch.filename}
          </DataField>
        )}
        {!isEmail && dispatch.state === "sent" && dispatch.finishedAt && (
          <DataField label="Übertragen am">
            {formatChatDateTime(dispatch.finishedAt)}
          </DataField>
        )}
        {isEmail && dispatch.downloadExpiresAt && (
          <DataField label="Link gültig bis">
            {formatChatDateTime(dispatch.downloadExpiresAt)}
          </DataField>
        )}
        {isEmail && dispatch.state === "sent" && (
          <DataField label="Heruntergeladen">
            {dispatch.downloadedAt
              ? formatChatDateTime(dispatch.downloadedAt)
              : "Noch nicht"}
          </DataField>
        )}
      </DataGrid>
      {reason && (
        // Ein Hinweis zum Stand, kein Fehlschlag dieser Seite: darum
        // „warning“; die rote Plakette trägt das Scheitern.
        <Alert type="warning" announce="polite" message={reason} />
      )}
    </div>
  );
}

// ─── Eigene Lohnarten und Zuschläge (#4807) ─────────────────────────────────
//
// Die Zeilen speichern wie die übrigen Karten dieser Seite sofort (Bauart 4);
// angelegt wird über einen Dialog, archiviert über das Zeilenmenü mit
// Rückfrage. Was eine Lohnart braucht, steht in der Karte selbst.

const WAGE_TYPE_UNIT_OPTIONS = (
  Object.keys(WAGE_TYPE_UNIT_LABELS) as WageTypeUnit[]
).map((value) => ({ value, label: WAGE_TYPE_UNIT_LABELS[value] }));

type ArchiveTarget =
  | { kind: "wage_type"; id: string; name: string }
  | { kind: "surcharge_rule"; id: string; name: string };

function PayrollRuleCards({
  rules,
  loadError,
  onChange,
  onCreated,
}: {
  readonly rules: PayrollRules | undefined;
  readonly loadError: ReturnType<typeof useSwrLoadError>;
  readonly onChange: ChangeRules;
  readonly onCreated: () => void;
}) {
  const [creating, setCreating] = useState<"wage_type" | "rule" | null>(null);
  const [archiving, setArchiving] = useState<ArchiveTarget | null>(null);
  const [archiveBusy, setArchiveBusy] = useState(false);

  if (!rules) {
    if (loadError) {
      return <LoadErrorAlert error={loadError} actionLayout="responsive" />;
    }
    return (
      <SectionCard title="Eigene Lohnarten">
        <Skeleton className="h-24 w-full" />
      </SectionCard>
    );
  }

  const confirmArchive = async () => {
    if (!archiving) return;
    setArchiveBusy(true);
    const target = archiving;
    try {
      const archived = await onChange(
        target.kind === "wage_type" ? "die Lohnart" : "die Zuschlagsregel",
        () =>
          target.kind === "wage_type"
            ? archiveWageType(target.id)
            : archiveSurchargeRule(target.id),
      );
      if (archived) setArchiving(null);
    } finally {
      setArchiveBusy(false);
    }
  };

  return (
    <>
      <WageTypesCard
        wageTypes={rules.wageTypes}
        onChange={onChange}
        onCreate={() => setCreating("wage_type")}
        onArchive={(wageType) =>
          setArchiving({
            kind: "wage_type",
            id: wageType.id,
            name: wageType.name,
          })
        }
      />
      <SurchargeRulesCard
        rules={rules}
        onChange={onChange}
        onCreate={() => setCreating("rule")}
        onArchive={(rule) =>
          setArchiving({
            kind: "surcharge_rule",
            id: rule.id,
            name: SURCHARGE_KIND_LABELS[rule.kind],
          })
        }
      />
      <AbsenceTypeWageTypesCard rules={rules} onChange={onChange} />

      <CreateWageTypeModal
        isOpen={creating === "wage_type"}
        onClose={() => setCreating(null)}
        onCreated={onCreated}
      />
      <CreateSurchargeRuleModal
        isOpen={creating === "rule"}
        wageTypes={rules.wageTypes}
        onClose={() => setCreating(null)}
        onCreated={onCreated}
      />
      <ConfirmationModal
        isOpen={archiving !== null}
        onClose={() => setArchiving(null)}
        onConfirm={() => void confirmArchive()}
        title={
          archiving?.kind === "wage_type"
            ? "Lohnart archivieren?"
            : "Zuschlagsregel archivieren?"
        }
        confirmText="Archivieren"
        confirmVariant="warning"
        isConfirmLoading={archiveBusy}
        loadingText="Wird archiviert …"
      >
        <p className="text-sm text-gray-700">
          {archiving?.kind === "wage_type"
            ? `„${archiving.name}“ fehlt ab jetzt im Export.`
            : `Die Regel für ${archiving?.name ?? ""} gilt ab jetzt nicht mehr.`}{" "}
          Abgeschlossene Monate behalten, was beim Abschluss galt.
        </p>
      </ConfirmationModal>
    </>
  );
}

function WageTypesCard({
  wageTypes,
  onChange,
  onCreate,
  onArchive,
}: {
  readonly wageTypes: readonly PayrollWageType[];
  readonly onChange: ChangeRules;
  readonly onCreate: () => void;
  readonly onArchive: (wageType: PayrollWageType) => void;
}) {
  const [drafts] = useState<PayrollDrafts>(() => new Map());
  // The API accepts the full wage-type object. Keep the last confirmed object
  // per row so a queued field save builds on the preceding successful save,
  // rather than on a stale render of another field.
  const savedWageTypes = useRef(new Map<string, PayrollWageType>());
  useEffect(() => {
    savedWageTypes.current = new Map(
      wageTypes.map((wageType) => [wageType.id, wageType]),
    );
  }, [wageTypes]);

  const saveWageType = (
    wageType: PayrollWageType,
    changes: Partial<PayrollWageType>,
  ) => {
    void onChange(
      "die Lohnart",
      async () => {
        const current = savedWageTypes.current.get(wageType.id) ?? wageType;
        const updated = { ...current, ...changes };
        const { id: _, ...input } = updated;
        await updateWageType(wageType.id, input);
        savedWageTypes.current.set(wageType.id, updated);
      },
      `wage-type:${wageType.id}`,
    );
  };

  const columns: DataTableColumn<PayrollWageType>[] = [
    {
      key: "name",
      header: "Bezeichnung",
      stacked: "title",
      render: (wageType) => (
        <WageTypeTextCell
          wageType={wageType}
          field="name"
          onSave={saveWageType}
          drafts={drafts}
        />
      ),
    },
    {
      key: "number",
      header: "Lohnartnummer",
      render: (wageType) => (
        <WageTypeTextCell
          wageType={wageType}
          field="number"
          onSave={saveWageType}
          drafts={drafts}
        />
      ),
    },
    {
      key: "unit",
      header: "Einheit",
      render: (wageType) => (
        <CustomSelect
          ariaLabel={`Einheit ${wageType.name}`}
          value={wageType.unit}
          options={WAGE_TYPE_UNIT_OPTIONS}
          onChange={(next) => {
            if (next !== wageType.unit && next in WAGE_TYPE_UNIT_LABELS) {
              saveWageType(wageType, { unit: next as WageTypeUnit });
            }
          }}
          controlSize="dense"
          className="md:w-auto md:min-w-[10rem]"
        />
      ),
    },
    {
      key: "actions",
      header: "",
      align: "right",
      stacked: "meta",
      render: (wageType) => (
        <OverflowMenu
          ariaLabel={`Aktionen für ${wageType.name}`}
          items={[{ label: "Archivieren", onClick: () => onArchive(wageType) }]}
        />
      ),
    },
  ];

  return (
    <SectionCard
      title="Eigene Lohnarten"
      description="Lohnarten Ihres Lohnbüros für Zuschläge und eigene Abwesenheitsarten, zusätzlich zu den festen Lohnarten oben. Änderungen werden sofort gespeichert."
      action={
        <Button
          type="button"
          variant="outline"
          size="compact"
          onClick={onCreate}
        >
          Lohnart anlegen
        </Button>
      }
    >
      {wageTypes.length === 0 ? (
        <EmptyState
          title="Noch keine eigene Lohnart"
          description="Legen Sie eine an, wenn Ihr Lohnbüro Zuschläge oder eine eigene Abwesenheitsart mit einer eigenen Nummer bucht."
        />
      ) : (
        <DataTable
          stackedOnMobile
          stackedLayout="form"
          stackedTitle={(wageType) => wageType.name}
          columns={columns}
          rows={[...wageTypes]}
          getRowKey={(wageType) => wageType.id}
          rowHasInteractiveControls
        />
      )}
    </SectionCard>
  );
}

function WageTypeTextCell({
  wageType,
  field,
  onSave,
  drafts,
}: {
  readonly wageType: PayrollWageType;
  readonly field: "name" | "number";
  readonly drafts: PayrollDrafts;
  readonly onSave: (
    wageType: PayrollWageType,
    changes: Partial<PayrollWageType>,
  ) => void;
}) {
  const [value, setValue] = usePayrollDraft(
    drafts,
    `${wageType.id}:${field}`,
    wageType[field],
  );

  const trimmed = value.trim();
  const error =
    field === "name"
      ? trimmed === ""
        ? "Bitte eine Bezeichnung eingeben."
        : undefined
      : trimmed === "" || /^\d{1,4}$/.test(trimmed)
        ? undefined
        : "1 bis 4 Ziffern.";

  return (
    <div
      className={
        field === "number"
          ? "w-full md:max-w-[10rem]"
          : "w-full md:max-w-[18rem]"
      }
    >
      <Input
        aria-label={
          field === "name"
            ? `Bezeichnung ${wageType.name}`
            : `Lohnartnummer ${wageType.name}`
        }
        value={value}
        onChange={(event) => setValue(event.target.value)}
        onBlur={() => {
          if (!error && trimmed !== wageType[field]) {
            onSave(
              wageType,
              field === "name" ? { name: trimmed } : { number: trimmed },
            );
          }
        }}
        placeholder={field === "number" ? "Noch keine Nummer" : undefined}
        inputMode={field === "number" ? "numeric" : undefined}
        maxLength={field === "name" ? 100 : undefined}
        controlSize="compact"
        error={error}
      />
    </div>
  );
}

function SurchargeRulesCard({
  rules,
  onChange,
  onCreate,
  onArchive,
}: {
  readonly rules: PayrollRules;
  readonly onChange: ChangeRules;
  readonly onCreate: () => void;
  readonly onArchive: (rule: PayrollSurchargeRule) => void;
}) {
  const [drafts] = useState<PayrollDrafts>(() => new Map());
  // The API replaces the complete rule. Keep each row's last confirmed value
  // so a queued field save builds on the preceding successful save, instead
  // of a stale render of another field.
  const savedSurchargeRules = useRef(new Map<string, PayrollSurchargeRule>());
  useEffect(() => {
    savedSurchargeRules.current = new Map(
      rules.surchargeRules.map((rule) => [rule.id, rule]),
    );
  }, [rules.surchargeRules]);
  const saveSurchargeRule = (
    rule: PayrollSurchargeRule,
    changes: Partial<PayrollSurchargeRule>,
  ) => {
    void onChange(
      "die Zuschlagsregel",
      async () => {
        const current = savedSurchargeRules.current.get(rule.id) ?? rule;
        const updated = { ...current, ...changes };
        await updateSurchargeRule(rule.id, updated);
        savedSurchargeRules.current.set(rule.id, updated);
      },
      `surcharge-rule:${rule.id}`,
    );
  };
  const wageTypeName = (id: string) =>
    rules.wageTypes.find((wageType) => wageType.id === id)?.name ?? "–";
  const columns: DataTableColumn<PayrollSurchargeRule>[] = [
    {
      key: "kind",
      header: "Art",
      stacked: "title",
      className: "font-medium text-gray-800",
      render: (rule) => SURCHARGE_KIND_LABELS[rule.kind],
    },
    {
      key: "window",
      header: "Zeitraum",
      render: (rule) =>
        rule.kind === "nacht" ? (
          <NightWindowCell
            rule={rule}
            onSave={saveSurchargeRule}
            drafts={drafts}
          />
        ) : (
          <span className="text-sm text-gray-600">ganzer Tag</span>
        ),
    },
    {
      key: "rate",
      header: "Höhe",
      render: (rule) => (
        <RateCell rule={rule} onSave={saveSurchargeRule} drafts={drafts} />
      ),
    },
    {
      key: "wageType",
      header: "Lohnart",
      render: (rule) => {
        const choices = wageTypesForRate(rules.wageTypes, rule.rateType);
        return (
          <CustomSelect
            ariaLabel={`Lohnart ${SURCHARGE_KIND_LABELS[rule.kind]}`}
            value={rule.wageTypeId}
            options={choices.map((wageType) => ({
              value: wageType.id,
              label: wageType.name,
            }))}
            placeholder={wageTypeName(rule.wageTypeId)}
            onChange={(next) => {
              if (next !== rule.wageTypeId) {
                saveSurchargeRule(rule, { wageTypeId: next });
              }
            }}
            controlSize="dense"
            className="md:w-auto md:min-w-[10rem]"
          />
        );
      },
    },
    {
      key: "actions",
      header: "",
      align: "right",
      stacked: "meta",
      render: (rule) => (
        <OverflowMenu
          ariaLabel={`Aktionen für ${SURCHARGE_KIND_LABELS[rule.kind]}`}
          items={[{ label: "Archivieren", onClick: () => onArchive(rule) }]}
        />
      ),
    },
  ];
  const noWageType = rules.wageTypes.length === 0;

  return (
    <SectionCard
      title="Zuschläge"
      description="moto zählt die gestempelten Stunden ohne Pausen. Feiertage kommen aus dem Bundesland der Schule. Ist ein Sonntag auch Feiertag, gilt die höhere Regel. Nachtarbeit zählt zusätzlich. Änderungen werden sofort gespeichert."
      action={
        <Button
          type="button"
          variant="outline"
          size="compact"
          onClick={onCreate}
          disabled={noWageType}
        >
          Zuschlagsregel anlegen
        </Button>
      }
    >
      {noWageType ? (
        <p className="text-sm text-gray-700">
          Ein Zuschlag wird auf eine eigene Lohnart gebucht. Legen Sie zuerst
          oben eine Lohnart an.
        </p>
      ) : rules.surchargeRules.length === 0 ? (
        <EmptyState
          title="Noch keine Zuschlagsregel"
          description="Ohne Regel zählt moto keine Zuschläge."
        />
      ) : (
        <DataTable
          stackedOnMobile
          stackedLayout="form"
          columns={columns}
          rows={[...rules.surchargeRules]}
          getRowKey={(rule) => rule.id}
          rowHasInteractiveControls
        />
      )}
    </SectionCard>
  );
}

type SaveSurchargeRule = (
  rule: PayrollSurchargeRule,
  changes: Partial<PayrollSurchargeRule>,
) => void;

function NightWindowCell({
  rule,
  onSave,
  drafts,
}: {
  readonly rule: PayrollSurchargeRule;
  readonly drafts: PayrollDrafts;
  readonly onSave: SaveSurchargeRule;
}) {
  const [from, setFrom] = usePayrollDraft(
    drafts,
    `${rule.id}:from`,
    minutesToClock(rule.windowStartMinute ?? 0),
  );
  const [until, setUntil] = usePayrollDraft(
    drafts,
    `${rule.id}:until`,
    minutesToClock(rule.windowEndMinute ?? 0),
  );

  const fromMinute = clockToMinutes(from);
  const untilMinute = clockToMinutes(until);
  const valid =
    fromMinute !== null && untilMinute !== null && fromMinute !== untilMinute;

  const commit = () => {
    if (
      !valid ||
      (fromMinute === rule.windowStartMinute &&
        untilMinute === rule.windowEndMinute)
    ) {
      return;
    }
    onSave(rule, {
      windowStartMinute: fromMinute,
      windowEndMinute: untilMinute,
    });
  };

  return (
    <div className="grid w-full grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] items-start gap-2 md:flex md:w-auto">
      <Input
        aria-label="Nachtarbeit von"
        type="time"
        value={from}
        onChange={(event) => setFrom(event.target.value)}
        onBlur={commit}
        controlSize="compact"
        className="w-full md:w-28"
      />
      <span className="pt-2.5 text-sm text-gray-600">bis</span>
      <Input
        aria-label="Nachtarbeit bis"
        type="time"
        value={until}
        onChange={(event) => setUntil(event.target.value)}
        onBlur={commit}
        controlSize="compact"
        className="w-full md:w-28"
        error={valid ? undefined : "Zwei verschiedene Uhrzeiten."}
      />
    </div>
  );
}

function RateCell({
  rule,
  onSave,
  drafts,
}: {
  readonly rule: PayrollSurchargeRule;
  readonly drafts: PayrollDrafts;
  readonly onSave: SaveSurchargeRule;
}) {
  const [text, setText] = usePayrollDraft(
    drafts,
    `${rule.id}:rate`,
    formatRate(rule.rateType, rule.rateHundredths),
  );
  const value = parseRate(text);

  return (
    // Die Einheit steht im Feld: So bleibt es so breit wie die anderen.
    <div className="relative w-full md:max-w-[11rem]">
      <div>
        <Input
          aria-label={`Höhe ${SURCHARGE_KIND_LABELS[rule.kind]} in ${rule.rateType === "prozent" ? "Prozent" : "Euro je Stunde"}`}
          value={text}
          onChange={(event) => setText(event.target.value)}
          onBlur={() => {
            if (value !== null && value !== rule.rateHundredths) {
              onSave(rule, { rateHundredths: value });
            }
          }}
          inputMode="decimal"
          controlSize="compact"
          className={`tabular-nums ${rule.rateType === "prozent" ? "pr-8" : "pr-28"}`}
          error={value === null ? "Eine Zahl größer als 0." : undefined}
        />
      </div>
      <span
        aria-hidden="true"
        className="pointer-events-none absolute top-0 right-4 flex h-10 items-center text-sm whitespace-nowrap text-gray-500"
      >
        {rule.rateType === "prozent" ? "%" : "€ je Stunde"}
      </span>
    </div>
  );
}

const NO_WAGE_TYPE = "";

function AbsenceTypeWageTypesCard({
  rules,
  onChange,
}: {
  readonly rules: PayrollRules;
  readonly onChange: ChangeRules;
}) {
  const options = [
    { value: NO_WAGE_TYPE, label: "Keine eigene Lohnart" },
    ...wageTypesForAbsences(rules.wageTypes).map((wageType) => ({
      value: wageType.id,
      label: wageType.name,
    })),
  ];
  const columns: DataTableColumn<PayrollAbsenceTypeMapping>[] = [
    {
      key: "name",
      header: "Abwesenheitsart",
      stacked: "title",
      className: "font-medium text-gray-800",
      render: (absenceType) => (
        <span className="inline-flex items-center gap-2">
          {absenceType.name}
          {absenceType.isActive ? null : (
            <StatusBadge label="ausgeschaltet" tone="gray" />
          )}
        </span>
      ),
    },
    {
      key: "wageType",
      header: "Lohnart",
      render: (absenceType) => (
        <CustomSelect
          ariaLabel={`Lohnart ${absenceType.name}`}
          value={absenceType.wageTypeId ?? NO_WAGE_TYPE}
          options={options}
          onChange={(next) => {
            const wageTypeId = next === NO_WAGE_TYPE ? null : next;
            if (wageTypeId !== absenceType.wageTypeId) {
              void onChange("die Abwesenheitsart", () =>
                setAbsenceTypeWageType(absenceType.id, wageTypeId),
              );
            }
          }}
          controlSize="dense"
          className="md:w-auto md:min-w-[12rem]"
        />
      ),
    },
  ];

  return (
    <SectionCard
      title="Eigene Abwesenheitsarten"
      description="Ohne eigene Lohnart fehlen sie im DATEV-Export. In CSV und Excel stehen sie dann unter Gutschrift Sonstige. Änderungen werden sofort gespeichert."
    >
      {rules.absenceTypes.length === 0 ? (
        <EmptyState
          title="Noch keine eigene Abwesenheitsart"
          description="Eigene Abwesenheitsarten legen Sie in der Datenverwaltung an."
          action={
            <NavigationLink
              href="/database/absence-types"
              className="text-sm font-medium text-gray-800 underline"
            >
              Zu den Abwesenheitsarten
            </NavigationLink>
          }
        />
      ) : (
        <DataTable
          stackedOnMobile
          stackedLayout="form"
          columns={columns}
          rows={[...rules.absenceTypes]}
          getRowKey={(absenceType) => absenceType.id}
          rowHasInteractiveControls
        />
      )}
    </SectionCard>
  );
}

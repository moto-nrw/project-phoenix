"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import useSWR from "swr";
import { BooleanField } from "~/components/settings/fields/boolean-field";
import { Alert } from "~/components/ui/alert";
import { CustomSelect } from "~/components/ui/custom-select";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { FieldLabel } from "~/components/ui/field-label";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
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
} from "~/lib/payroll-api";
import { fetchSFTPStatus, type SFTPStatus } from "~/lib/sftp-export-api";

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
  } = useSWR(isReady ? "payroll-status" : null, fetchPayrollStatus);

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
  } = useSWR(isReady ? "payroll-dispatch" : null, fetchPayrollDispatch);
  const dispatchLoadError = useSwrLoadError(
    dispatchError,
    "die Übersicht zum letzten Versand",
    () => mutateDispatch(),
  );

  // Ob die SFTP-Gegenstelle bereit ist, braucht time_tracking:manage. Fehlt
  // das Recht oder scheitert die Abfrage, entfällt nur der Hinweis: Der
  // Versand meldet eine fehlende Gegenstelle ohnehin selbst.
  const { data: sftpStatus } = useSWR(
    isReady && status?.dispatch.channel === "sftp"
      ? "payroll-sftp-status"
      : null,
    fetchSFTPStatus,
    {
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
          <ReadinessCard status={status} />

          <FormErrorAlert message={saveError} />
          {duplicates.length > 0 && (
            <Alert
              type="warning"
              message={`Lohnartnummer ${duplicates.join(", ")} ist mehreren Kategorien zugeordnet. Das ist zulässig, führt aber meist zu doppelt gebuchten Stunden, bitte prüfen.`}
            />
          )}

          <LohnartenCard status={status} onSave={save} />
          <DatevCard status={status} onSave={save} />
          <LohnbueroCard
            status={status}
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

function ReadinessCard({ status }: { readonly status: PayrollStatus }) {
  const items: { label: string; ok: boolean; detail: string }[] = [
    {
      label: "Lohnarten",
      ok: status.configuredCategories > 0,
      detail: `${status.configuredCategories} von ${status.totalCategories} Kategorien konfiguriert`,
    },
    {
      label: "DATEV-Mandant (nur LODAS)",
      ok: status.lodasHeaderComplete,
      detail: status.lodasHeaderComplete
        ? "Berater- und Mandantennummer gesetzt"
        : "Berater- oder Mandantennummer fehlt",
    },
    {
      label: "Personalnummern",
      ok: status.staffWithoutPersonnelNumber === 0,
      detail:
        status.staffWithoutPersonnelNumber === 0
          ? `Alle ${status.staffTotal} Beschäftigten haben eine Personalnummer`
          : `${status.staffWithoutPersonnelNumber} von ${status.staffTotal} Beschäftigten ohne Personalnummer (Pflege im Stammdaten-Tab der Person)`,
    },
  ];

  return (
    <SectionCard
      title="Vollständigkeit"
      description="Ohne vollständige Konfiguration erzeugt der spätere DATEV-Export keine Datei. Eine Kategorie ohne Lohnartnummer wird nicht exportiert."
    >
      <ul className="space-y-2">
        {items.map((item) => (
          <li key={item.label} className="flex items-start gap-2 text-sm">
            <span
              aria-hidden
              className={`mt-0.5 inline-block h-2.5 w-2.5 shrink-0 rounded-full ${
                item.ok ? "bg-moto-green" : "bg-moto-orange"
              }`}
            />
            <span>
              <span className="font-medium text-gray-800">{item.label}:</span>{" "}
              <span className="text-gray-600">{item.detail}</span>
            </span>
          </li>
        ))}
      </ul>
    </SectionCard>
  );
}

function LohnartenCard({
  status,
  onSave,
}: {
  readonly status: PayrollStatus;
  readonly onSave: SaveSetting;
}) {
  return (
    <SectionCard
      title="Lohnarten"
      description="Mandantenspezifische Lohnartnummern aus dem Lohnsystem des Trägers (1 bis 4 Ziffern). Für Krank, Urlaub und Fortbildung zusätzlich die Einheit, die die Lohnart erwartet. Änderungen werden sofort gespeichert."
    >
      <DataTable
        // Auf dem Handy untereinander: die Auswahl der Einheit lag sonst
        // rechts außerhalb der Karte (#4975).
        stackedOnMobile
        columns={lohnartColumns(onSave)}
        rows={status.categories}
        getRowKey={(cat) => cat.id}
        rowHasInteractiveControls
        caption="Lohnartnummern je Kategorie"
      />
    </SectionCard>
  );
}

type LohnartCategory = PayrollStatus["categories"][number];

function lohnartColumns(
  onSave: SaveSetting,
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
      render: (cat) => <LohnartNumberCell cat={cat} onSave={onSave} />,
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
            triggerClassName="moto-content-surface h-9 w-auto min-w-[10rem] hover:border-gray-300"
          />
        ) : (
          <span className="text-xs text-gray-400">Stunden (fest)</span>
        ),
    },
  ];
}

function LohnartNumberCell({
  cat,
  onSave,
}: {
  readonly cat: LohnartCategory;
  readonly onSave: SaveSetting;
}) {
  const [number, setNumber] = useState(cat.number);
  useEffect(() => setNumber(cat.number), [cat.number]);

  const valid = number.trim() === "" || /^\d{1,4}$/.test(number.trim());

  return (
    <div className="max-w-[10rem]">
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
        className="h-9 py-1 text-sm tabular-nums"
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
      className="h-9 py-1 text-sm tabular-nums"
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
 * wie die Karte „Vollständigkeit“; LODAS braucht zusätzlich den Mandanten.
 */
function datevIncomplete(
  status: PayrollStatus,
  format: PayrollDispatchFormat,
): boolean {
  if (format !== "datev_lodas" && format !== "datev_lug") return false;
  return (
    status.staffWithoutPersonnelNumber > 0 ||
    status.configuredCategories < status.totalCategories ||
    (format === "datev_lodas" && !status.lodasHeaderComplete)
  );
}

function LohnbueroCard({
  status,
  latest,
  latestError,
  sftpStatus,
  onSave,
}: {
  readonly status: PayrollStatus;
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
    current.auto &&
    current.channel === "email" &&
    status.dispatch.email === "";

  return (
    <SectionCard
      title="Versand an das Lohnbüro"
      description="Nach dem Monatsabschluss schickt moto die Monatsdatei aller Mitarbeitenden an das Lohnbüro. Änderungen werden sofort gespeichert."
    >
      <div className="space-y-5">
        <div className="flex items-start justify-between gap-4">
          <div>
            <span className="block text-sm font-medium text-gray-800">
              Automatisch nach dem Monatsabschluss senden
            </span>
            <p className="mt-1 text-xs text-gray-500">
              Schließen Sie einen Monat nach dem Wiederöffnen erneut ab, geht
              die Datei noch einmal raus. Ihr Name enthält dann „korrektur“.
            </p>
          </div>
          <BooleanField
            ariaLabel="Automatisch nach dem Monatsabschluss senden"
            value={current.auto}
            onChange={(next) => change("auto", next)}
          />
        </div>

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
              className="h-9 py-1 text-sm"
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
            moto legt die Datei auf der Gegenstelle ab, die in den
            Einstellungen bei Zeitkonten übertragen steht.
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
        {datevIncomplete(status, current.format) && (
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
    blocked: { label: "Nicht gesendet", tone: "orange" },
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

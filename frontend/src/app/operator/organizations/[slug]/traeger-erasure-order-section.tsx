"use client";

import { operatorProvisioningKeys } from "~/lib/swr/portal-keys";

import { useCallback, useRef, useState } from "react";
import { useSWRAuth } from "~/lib/swr";
import { Button } from "~/components/ui/button";
import { DatePicker } from "~/components/ui/date-picker";
import {
  DataField,
  DataFieldSkeleton,
  DataGrid,
} from "~/components/ui/detail-modal-components";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { Alert } from "~/components/ui/alert";
import { ConsentCheckbox } from "~/components/ui/consent-checkbox";
import { CopyField } from "~/components/ui/copy-field";
import { ConfirmationModal } from "~/components/ui/modal";
import { SectionCard } from "~/components/ui/section-card";
import { SectionLabel } from "~/components/ui/section-label";
import {
  SegmentedControl,
  type SegmentedControlItem,
} from "~/components/ui/segmented-control";
import { FormField } from "~/app/operator/provisioning/provisioning-shared";
import {
  useApiErrorDisplay,
  useApiFormError,
  useToast,
} from "~/contexts/ToastContext";
import {
  formatDate,
  parseISODate,
  toISODate,
  berlinTodayISO,
} from "~/lib/date-helpers";
import {
  STALE_DATA_SUFFIX,
  useRefreshAfterWrite,
  type RefreshAfterWrite,
} from "~/lib/hooks/use-refresh-after-write";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { createLogger } from "~/lib/logger";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type {
  RecordTraegerErasureOrderRecordsRequest,
  RecordTraegerErasureOrderRequest,
  TraegerErasureOrder,
  TraegerRecordsChoice,
} from "~/lib/operator/provisioning-helpers";

const logger = createLogger({ component: "TraegerErasureOrderSection" });

const RECORDS_CHOICES: readonly SegmentedControlItem<TraegerRecordsChoice>[] = [
  { value: "handed_over", label: "Rückgabe übergeben" },
  { value: "erasure_chosen", label: "Kunde wählt Löschung" },
];

const EXPORT_CHECKSUM = /^[0-9a-f]{64}$/;

/** What the customer chose after the hint on the records (#4720). */
function recordsChoiceText(order: TraegerErasureOrder): string {
  if (order.recordsChoice === "erasure_chosen") return "Kunde wählt Löschung";
  if (order.recordsChoice === "handed_over" && order.recordsHandedOverOn) {
    return `Rückgabe übergeben am ${formatDate(order.recordsHandedOverOn)}`;
  }
  return "Nicht erfasst";
}

function tomorrow(): Date {
  const date = parseISODate(berlinTodayISO());
  date.setDate(date.getDate() + 1);
  return date;
}

/**
 * The hint and the customer's choice as a form records them (#4720): its
 * fields, whether they are complete, and the request values they send.
 */
function useRecordsHandover() {
  const [noticeAcknowledged, setNoticeAcknowledged] = useState(false);
  const [choice, setChoice] = useState<TraegerRecordsChoice | null>(null);
  const [handedOverOn, setHandedOverOn] = useState<Date | null>(null);
  const [exportChecksum, setExportChecksum] = useState("");
  const checksum = exportChecksum.trim().toLowerCase();
  // The checksum belongs to the handover only; the choice of erasure sends
  // none, so a value left over from the handover never blocks it.
  const checksumInvalid =
    choice === "handed_over" &&
    checksum !== "" &&
    !EXPORT_CHECKSUM.test(checksum);
  const complete =
    noticeAcknowledged &&
    (choice === "erasure_chosen" ||
      (choice === "handed_over" && handedOverOn !== null)) &&
    !checksumInvalid;
  const request = (): RecordTraegerErasureOrderRecordsRequest | null => {
    if (!complete || !choice) return null;
    const records: RecordTraegerErasureOrderRecordsRequest = {
      records_notice_acknowledged: noticeAcknowledged,
      records_choice: choice,
    };
    if (choice === "handed_over" && handedOverOn) {
      records.records_handed_over_on = toISODate(handedOverOn);
      if (checksum) records.records_export_sha256 = checksum;
    }
    return records;
  };
  return {
    complete,
    request,
    fields: {
      noticeAcknowledged,
      onNoticeAcknowledged: setNoticeAcknowledged,
      choice,
      onChoice: setChoice,
      handedOverOn,
      onHandedOverOn: setHandedOverOn,
      exportChecksum,
      onExportChecksum: setExportChecksum,
      checksumInvalid,
    },
  };
}

/**
 * Erasure order of a Träger (#4657): after the end of the contract the moto
 * team records the written confirmation of a deactivated Träger and the day
 * from which the internal erasure procedure may delete it. Until then the
 * grace period runs and the order can be withdrawn.
 */
export function TraegerErasureOrderSection({
  organizationId,
  organizationActive,
}: {
  readonly organizationId: string;
  readonly organizationActive: boolean;
}) {
  const {
    data: order,
    error,
    isLoading,
    mutate,
  } = useSWRAuth(
    operatorProvisioningKeys.orgErasureOrder(organizationId),
    () => operatorProvisioningService.getTraegerErasureOrder(organizationId),
    { keepPreviousData: false },
  );
  // A failed reload keeps the confirmed order and stands above it; only
  // without data does the error replace the section (#6636).
  const loadError = useSwrLoadError(
    error,
    // Feminine object: the catalog texts use it as subject („… ist gerade
    // nicht erreichbar“) and after „für“; „den Löschauftrag“ read „Den
    // Löschauftrag ist …“ (#6652).
    "die Anzeige des Löschauftrags",
    () => void mutate(),
    order === undefined ? undefined : STALE_DATA_SUFFIX,
  );
  // The reload after recording runs after the write; its failure stands
  // above the section, never as an error of the form (#6652).
  const recordRefresh = useRefreshAfterWrite(
    "die Anzeige des Löschauftrags",
    () => mutate(),
  );

  if (loadError && order === undefined) {
    return <LoadErrorAlert error={loadError} />;
  }
  const staleError = (
    <>
      {loadError ? <LoadErrorAlert error={loadError} /> : null}
      <LoadErrorAlert error={recordRefresh.error} />
    </>
  );
  if (isLoading || order === undefined) {
    return (
      <SectionCard title="Löschauftrag">
        <DataGrid>
          <DataFieldSkeleton />
          <DataFieldSkeleton />
        </DataGrid>
      </SectionCard>
    );
  }
  if (order) {
    return (
      <>
        {staleError}
        <OpenErasureOrder
          order={order}
          organizationId={organizationId}
          onChanged={() => mutate()}
        />
      </>
    );
  }
  if (organizationActive) {
    return (
      <>
        {staleError}
        <SectionCard
          title="Löschauftrag"
          description="Einen Löschauftrag erfassen Sie erst, wenn der Träger deaktiviert ist. Deaktivieren Sie ihn oben über den Status."
        >
          <Alert
            type="warning"
            message="Vor dem Deaktivieren: Weisen Sie den Träger auf die Sicherung seiner aufbewahrungspflichtigen Unterlagen hin (AVV Ziff. 8.4). Nach dem Deaktivieren kann das Träger-Büro nichts mehr herunterladen, etwa die SEPA-Dateien."
          />
        </SectionCard>
      </>
    );
  }
  return (
    <>
      {staleError}
      <RecordErasureOrderForm
        organizationId={organizationId}
        onRecorded={recordRefresh.refresh}
      />
    </>
  );
}

function OpenErasureOrder({
  order,
  organizationId,
  onChanged,
}: {
  readonly order: TraegerErasureOrder;
  readonly organizationId: string;
  readonly onChanged: () => Promise<unknown>;
}) {
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [withdrawing, setWithdrawing] = useState(false);
  const { show: showError } = useApiErrorDisplay();
  const { success: toastSuccess } = useToast();
  // Only a failed withdrawal is an error of the withdrawal; a failed reload
  // afterwards is a load error with a retry that only reloads (#6636).
  const withdrawRefresh = useRefreshAfterWrite(
    "die Anzeige des Löschauftrags",
    onChanged,
  );
  const { refresh: refreshAfterWithdraw } = withdrawRefresh;

  const handleWithdraw = useCallback(async () => {
    setWithdrawing(true);
    try {
      await operatorProvisioningService.withdrawTraegerErasureOrder(
        organizationId,
      );
    } catch (withdrawError) {
      logger.warn("traeger_erasure_order_withdraw_failed", {
        error:
          withdrawError instanceof Error
            ? withdrawError.message
            : String(withdrawError),
      });
      void showError(withdrawError, {
        object: "die Rücknahme des Löschauftrags",
      });
      setWithdrawing(false);
      return;
    }
    setConfirmOpen(false);
    setWithdrawing(false);
    toastSuccess("Der Löschauftrag ist zurückgenommen.");
    await refreshAfterWithdraw();
  }, [organizationId, refreshAfterWithdraw, showError, toastSuccess]);

  return (
    <>
      <SectionCard
        title={order.startedAt ? "Löschung läuft" : "Löschauftrag erfasst"}
        description={
          order.startedAt
            ? "Das interne Löschverfahren hat begonnen. Der Auftrag kann nicht mehr zurückgenommen werden. Das moto-Team schließt die Löschung mit demselben Auftrag ab."
            : `Ab dem ${formatDate(order.eraseNotBefore)} darf das moto-Team den Träger mit dem internen Löschverfahren endgültig löschen: alle Schulen mit ihren Daten, das Träger-Büro und den Träger selbst. Bis dahin können Sie den Auftrag zurücknehmen.`
        }
        action={
          order.startedAt ? undefined : (
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={() => setConfirmOpen(true)}
            >
              Löschauftrag zurücknehmen
            </Button>
          )
        }
      >
        <LoadErrorAlert error={withdrawRefresh.error} className="mb-4" />
        {order.recordsChoice ? null : (
          <div className="mb-4">
            <Alert
              type="warning"
              title="Löschung so nicht möglich"
              message={
                order.startedAt
                  ? "Dieser Auftrag wurde ohne Hinweis auf die Unterlagen erfasst. Die Löschung läuft erst weiter, wenn Hinweis und Wahl eingetragen sind. Tragen Sie Hinweis und Wahl hier nach."
                  : "Dieser Auftrag wurde ohne Hinweis auf die Unterlagen erfasst. Das Löschverfahren führt ihn so nicht aus. Tragen Sie Hinweis und Wahl hier nach."
              }
            />
          </div>
        )}
        <DataGrid>
          <DataField label="Vorgangsnummer" mono>
            {order.orderReference}
          </DataField>
          <DataField label="Schriftliche Bestätigung vom">
            {formatDate(order.confirmedOn)}
          </DataField>
          <DataField label="Löschen frühestens am">
            {formatDate(order.eraseNotBefore)}
          </DataField>
          <DataField label="Erfasst am">
            {formatDate(order.recordedAt)}
          </DataField>
          <DataField label="Aufbewahrungspflichtige Unterlagen">
            {recordsChoiceText(order)}
          </DataField>
        </DataGrid>
        {order.recordsChoice ? null : (
          <RecordRecordsChoiceForm
            organizationId={organizationId}
            onRecorded={refreshAfterWithdraw}
          />
        )}
      </SectionCard>
      <ConfirmationModal
        isOpen={confirmOpen}
        onClose={() => setConfirmOpen(false)}
        onConfirm={() => void handleWithdraw()}
        title="Löschauftrag zurücknehmen?"
        confirmText="Zurücknehmen"
        isConfirmLoading={withdrawing}
        loadingText="Wird zurückgenommen..."
      >
        <p className="text-sm text-gray-700">
          Der Träger bleibt deaktiviert. Danach können Sie ihn wieder aktivieren
          oder einen neuen Löschauftrag erfassen.
        </p>
      </ConfirmationModal>
    </>
  );
}

function RecordErasureOrderForm({
  organizationId,
  onRecorded,
}: {
  readonly organizationId: string;
  /** Reloads the order; never rejects (#6652). */
  readonly onRecorded: RefreshAfterWrite;
}) {
  const [orderReference, setOrderReference] = useState("");
  const [confirmedOn, setConfirmedOn] = useState<Date | null>(null);
  const [eraseNotBefore, setEraseNotBefore] = useState<Date | null>(null);
  const records = useRecordsHandover();
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { show: showError, clear: clearError } = formErrors;
  const { success: toastSuccess } = useToast();
  const complete =
    orderReference.trim() !== "" &&
    confirmedOn !== null &&
    eraseNotBefore !== null &&
    records.complete;

  const handleSubmit = useCallback(
    async (event: React.FormEvent) => {
      event.preventDefault();
      const recordsRequest = records.request();
      if (!confirmedOn || !eraseNotBefore || !recordsRequest || !complete)
        return;
      const request: RecordTraegerErasureOrderRequest = {
        order_reference: orderReference.trim().toUpperCase(),
        confirmed_on: toISODate(confirmedOn),
        erase_not_before: toISODate(eraseNotBefore),
        ...recordsRequest,
      };
      setSaving(true);
      clearError();
      try {
        await operatorProvisioningService.recordTraegerErasureOrder(
          organizationId,
          request,
        );
      } catch (recordError) {
        logger.warn("traeger_erasure_order_record_failed", {
          error:
            recordError instanceof Error
              ? recordError.message
              : String(recordError),
        });
        // „das Erfassen“ fits every catalog text („Für das Erfassen …“,
        // „Das Erfassen … konnte nicht …“); „der Löschauftrag“ did not.
        void showError(recordError, {
          object: "das Erfassen des Löschauftrags",
        });
        setSaving(false);
        return;
      }
      setSaving(false);
      toastSuccess("Der Löschauftrag ist erfasst.");
      await onRecorded();
    },
    [
      clearError,
      complete,
      confirmedOn,
      eraseNotBefore,
      onRecorded,
      orderReference,
      organizationId,
      records,
      showError,
      toastSuccess,
    ],
  );

  return (
    <SectionCard
      title="Löschauftrag"
      description="Nach Vertragsende und schriftlicher Bestätigung erfassen Sie hier den Löschauftrag. Ab dem gewählten Tag darf das interne Löschverfahren den Träger mit allen Schulen endgültig löschen. Bis dahin läuft die Schonfrist."
    >
      <form
        ref={formRef}
        onSubmit={(event) => void handleSubmit(event)}
        className="space-y-4"
      >
        <FormErrorAlert message={formErrors.error} />
        <Input
          id="traeger-erasure-order-reference"
          label="Vorgangsnummer"
          name="order_reference"
          value={orderReference}
          onChange={(event) => setOrderReference(event.target.value)}
          error={formErrors.fieldError("order_reference")}
          maxLength={40}
          autoComplete="off"
          required
        />
        <p className="-mt-2 text-xs text-gray-500">
          3 bis 40 Großbuchstaben, Ziffern oder Bindestriche. Keine Namen.
        </p>
        <div className="grid gap-4 sm:grid-cols-2">
          <FormField
            label="Schriftliche Bestätigung vom"
            htmlFor="traeger-erasure-order-confirmed-on"
            required
          >
            <DatePicker
              id="traeger-erasure-order-confirmed-on"
              value={confirmedOn}
              onChange={setConfirmedOn}
              maxDate={parseISODate(berlinTodayISO())}
              placeholder="Datum auswählen"
              controlSize="large"
              required
            />
          </FormField>
          <FormField
            label="Löschen frühestens am"
            htmlFor="traeger-erasure-order-erase-not-before"
            required
          >
            <DatePicker
              id="traeger-erasure-order-erase-not-before"
              value={eraseNotBefore}
              onChange={setEraseNotBefore}
              minDate={tomorrow()}
              placeholder="Datum auswählen"
              controlSize="large"
              required
            />
          </FormField>
        </div>
        <RecordsHandoverFields
          organizationId={organizationId}
          {...records.fields}
        />
        <div className="flex justify-end">
          <Button
            type="submit"
            variant="primary"
            size="md"
            disabled={saving || !complete}
          >
            {saving ? "Wird erfasst..." : "Löschauftrag erfassen"}
          </Button>
        </div>
      </form>
    </SectionCard>
  );
}

/**
 * The hint and the customer's choice recorded afterwards on an open order
 * recorded without them (#4720). A started order cannot be withdrawn, so
 * this is how its erasure continues.
 */
function RecordRecordsChoiceForm({
  organizationId,
  onRecorded,
}: {
  readonly organizationId: string;
  /** Reloads the order; never rejects (#6652). */
  readonly onRecorded: RefreshAfterWrite;
}) {
  const records = useRecordsHandover();
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { show: showError, clear: clearError } = formErrors;
  const { success: toastSuccess } = useToast();

  const handleSubmit = useCallback(
    async (event: React.FormEvent) => {
      event.preventDefault();
      const request = records.request();
      if (!request) return;
      setSaving(true);
      clearError();
      try {
        await operatorProvisioningService.recordTraegerErasureOrderRecords(
          organizationId,
          request,
        );
      } catch (recordError) {
        logger.warn("traeger_erasure_order_records_failed", {
          error:
            recordError instanceof Error
              ? recordError.message
              : String(recordError),
        });
        void showError(recordError, { object: "die Wahl des Kunden" });
        setSaving(false);
        return;
      }
      setSaving(false);
      toastSuccess("Die Wahl ist eingetragen.");
      await onRecorded();
    },
    [clearError, onRecorded, organizationId, records, showError, toastSuccess],
  );

  return (
    <form
      ref={formRef}
      onSubmit={(event) => void handleSubmit(event)}
      className="mt-6 space-y-4 border-t border-gray-100 pt-6"
    >
      <FormErrorAlert message={formErrors.error} />
      <RecordsHandoverFields
        organizationId={organizationId}
        {...records.fields}
      />
      <div className="flex justify-end">
        <Button
          type="submit"
          variant="primary"
          size="md"
          disabled={saving || !records.complete}
        >
          {saving ? "Wird eingetragen..." : "Wahl eintragen"}
        </Button>
      </div>
    </form>
  );
}

/**
 * Before the erasure order: the hint that the Träger secures its records
 * subject to retention itself (AVV Ziff. 8.4), the Rückgabe export of all its
 * Belege, and the customer's choice (#4720). Lawful erasure stays possible
 * without a handover.
 */
function RecordsHandoverFields({
  organizationId,
  noticeAcknowledged,
  onNoticeAcknowledged,
  choice,
  onChoice,
  handedOverOn,
  onHandedOverOn,
  exportChecksum,
  onExportChecksum,
  checksumInvalid,
}: {
  readonly organizationId: string;
  readonly noticeAcknowledged: boolean;
  readonly onNoticeAcknowledged: (acknowledged: boolean) => void;
  readonly choice: TraegerRecordsChoice | null;
  readonly onChoice: (choice: TraegerRecordsChoice) => void;
  readonly handedOverOn: Date | null;
  readonly onHandedOverOn: (day: Date | null) => void;
  readonly exportChecksum: string;
  readonly onExportChecksum: (checksum: string) => void;
  readonly checksumInvalid: boolean;
}) {
  return (
    <div className="space-y-4">
      <Alert
        type="warning"
        title="Erst Unterlagen sichern lassen"
        message={
          <>
            Der Träger muss seine aufbewahrungspflichtigen Unterlagen selbst
            sichern: Rechnungen, Gutschriften, Bescheide, Verträge und
            SEPA-Mandate (AVV Ziff. 8.4). Bieten Sie ihm vorher die Rückgabe an.
            Wählt der Kunde ausdrücklich die Löschung, gilt seine Wahl.
          </>
        }
      />
      <div className="space-y-2">
        <p className="text-sm text-gray-700">
          Die Rückgabe enthält alle Belege dieses Trägers mit Fassungen,
          Korrekturen und Originaldateien, dazu vollständige Bankdaten und
          SEPA-Dateien. Übergeben Sie sie nur verschlüsselt. Erstellen Sie sie
          im internen Verfahren (Runbook „Interne Auskunft und Datenrückgabe“):
        </p>
        <CopyField
          value={`tenant export --organization ${organizationId} --operator <IHRE_OPERATOR_ID> --uploads <UPLOAD_VERZEICHNIS> --out <NEUES_VERZEICHNIS>`}
          monospace
          copyLabel="Befehl kopieren"
        />
      </div>
      <ConsentCheckbox
        id="traeger-erasure-order-records-notice"
        name="records_notice_acknowledged"
        label="Ich habe den Träger auf die Sicherung seiner Unterlagen hingewiesen."
        checked={noticeAcknowledged}
        onChange={onNoticeAcknowledged}
        required
      />
      <div className="space-y-2">
        <SectionLabel as="p" required>
          Was hat der Kunde gewählt?
        </SectionLabel>
        <SegmentedControl
          ariaLabel="Was hat der Kunde gewählt?"
          items={RECORDS_CHOICES}
          value={choice}
          onChange={onChoice}
        />
      </div>
      {choice === "handed_over" ? (
        <div className="grid gap-4 sm:grid-cols-2">
          <FormField
            label="Rückgabe übergeben am"
            htmlFor="traeger-erasure-order-records-handed-over-on"
            required
          >
            <DatePicker
              id="traeger-erasure-order-records-handed-over-on"
              value={handedOverOn}
              onChange={onHandedOverOn}
              maxDate={parseISODate(berlinTodayISO())}
              placeholder="Datum auswählen"
              controlSize="large"
              required
            />
          </FormField>
          <Input
            id="traeger-erasure-order-records-export-sha256"
            label="Prüfsumme der Rückgabe (Datei COMPLETE)"
            name="records_export_sha256"
            value={exportChecksum}
            onChange={(event) => onExportChecksum(event.target.value)}
            error={
              checksumInvalid
                ? "Bitte die 64 Zeichen aus der Datei COMPLETE eintragen."
                : undefined
            }
            autoComplete="off"
            maxLength={80}
          />
        </div>
      ) : null}
    </div>
  );
}

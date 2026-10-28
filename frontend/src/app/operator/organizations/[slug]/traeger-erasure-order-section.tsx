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
import { ConfirmationModal } from "~/components/ui/modal";
import { SectionCard } from "~/components/ui/section-card";
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
  todayISO,
} from "~/lib/date-helpers";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { createLogger } from "~/lib/logger";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type { TraegerErasureOrder } from "~/lib/operator/provisioning-helpers";

const logger = createLogger({ component: "TraegerErasureOrderSection" });

function tomorrow(): Date {
  const date = parseISODate(todayISO());
  date.setDate(date.getDate() + 1);
  return date;
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
  } = useSWRAuth(operatorProvisioningKeys.orgErasureOrder(organizationId), () =>
    operatorProvisioningService.getTraegerErasureOrder(organizationId),
  );
  const loadError = useSwrLoadError(
    error,
    "den Löschauftrag",
    () => void mutate(),
  );

  if (loadError) {
    return <LoadErrorAlert error={loadError} />;
  }
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
      <OpenErasureOrder
        order={order}
        organizationId={organizationId}
        onChanged={() => mutate()}
      />
    );
  }
  if (organizationActive) {
    return (
      <SectionCard
        title="Löschauftrag"
        description="Einen Löschauftrag erfassen Sie erst, wenn der Träger deaktiviert ist. Deaktivieren Sie ihn oben über den Status."
      />
    );
  }
  return (
    <RecordErasureOrderForm
      organizationId={organizationId}
      onRecorded={() => mutate()}
    />
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

  const handleWithdraw = useCallback(async () => {
    setWithdrawing(true);
    try {
      await operatorProvisioningService.withdrawTraegerErasureOrder(
        organizationId,
      );
      setConfirmOpen(false);
      toastSuccess("Der Löschauftrag ist zurückgenommen.");
      await onChanged();
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
    } finally {
      setWithdrawing(false);
    }
  }, [onChanged, organizationId, showError, toastSuccess]);

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
        </DataGrid>
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
  readonly onRecorded: () => Promise<unknown>;
}) {
  const [orderReference, setOrderReference] = useState("");
  const [confirmedOn, setConfirmedOn] = useState<Date | null>(null);
  const [eraseNotBefore, setEraseNotBefore] = useState<Date | null>(null);
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { show: showError, clear: clearError } = formErrors;
  const { success: toastSuccess } = useToast();
  const complete =
    orderReference.trim() !== "" &&
    confirmedOn !== null &&
    eraseNotBefore !== null;

  const handleSubmit = useCallback(
    async (event: React.FormEvent) => {
      event.preventDefault();
      if (!confirmedOn || !eraseNotBefore || !orderReference.trim()) return;
      setSaving(true);
      clearError();
      try {
        await operatorProvisioningService.recordTraegerErasureOrder(
          organizationId,
          {
            order_reference: orderReference.trim().toUpperCase(),
            confirmed_on: toISODate(confirmedOn),
            erase_not_before: toISODate(eraseNotBefore),
          },
        );
        toastSuccess("Der Löschauftrag ist erfasst.");
        await onRecorded();
      } catch (recordError) {
        logger.warn("traeger_erasure_order_record_failed", {
          error:
            recordError instanceof Error
              ? recordError.message
              : String(recordError),
        });
        void showError(recordError, { object: "der Löschauftrag" });
      } finally {
        setSaving(false);
      }
    },
    [
      clearError,
      confirmedOn,
      eraseNotBefore,
      onRecorded,
      orderReference,
      organizationId,
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
              maxDate={parseISODate(todayISO())}
              placeholder="Datum auswählen"
              controlSize="lg"
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
              controlSize="lg"
              required
            />
          </FormField>
        </div>
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

"use client";

// Freigabe „Personal beim Träger“ (#4705, ADR 0089 Abschnitt 2). Vor der
// Freigabe bestätigt der Träger eine von zwei Erklärungen (Entscheidung E2
// vom 13.11.2026): eine Vereinbarung mit Betriebsrat oder
// Mitarbeitervertretung zu Zeiterfassung und Einsatzplanung über Standorte,
// oder dass es keine Vertretung gibt. Freigabe und Rücknahme stehen im
// Operator-Audit. Zurücknehmen geht erst, wenn der Träger keine Akte mehr
// führt.

import { useCallback, useEffect, useRef, useState } from "react";
import { Button } from "~/components/ui/button";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { ConfirmationModal } from "~/components/ui/modal";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { EditActions } from "~/components/ui/edit-actions";
import { FieldLabel } from "~/components/ui/field-label";
import { FormModal } from "~/components/ui/form-modal";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { Radio } from "~/components/ui/radio";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { useFormModalPending } from "~/components/ui/use-form-modal-pending";
import {
  useApiErrorDisplay,
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { formatBerlinDate } from "~/lib/date-helpers";
import { useRefreshAfterWrite } from "~/lib/hooks/use-refresh-after-write";
import { createLogger } from "~/lib/logger";
import {
  traegerStaffReleaseService,
  type StaffRepresentation,
  type TraegerStaffReleaseState,
} from "~/lib/operator/traeger-staff-release-api";

const logger = createLogger({ component: "TraegerStaffReleaseSection" });

/** Die zwei Erklärungen des Trägers, wörtlich wie im Operator-Audit. */
export const STAFF_REPRESENTATIONS: readonly {
  readonly value: StaffRepresentation;
  readonly label: string;
  readonly hint: string;
}[] = [
  {
    value: "agreement",
    label: "Vereinbarung mit der Vertretung liegt vor",
    hint: "Es gibt eine Betriebsvereinbarung oder Dienstvereinbarung mit Betriebsrat oder Mitarbeitervertretung zu Zeiterfassung und Einsatzplanung über Standorte (§ 87 Abs. 1 Nr. 2 und Nr. 6 BetrVG, MAVO, MVG.EKD).",
  },
  {
    value: "none",
    label: "Keine Vertretung",
    hint: "Beim Träger gibt es weder Betriebsrat noch Mitarbeitervertretung.",
  },
];

const representationLabel = (value: StaffRepresentation) =>
  STAFF_REPRESENTATIONS.find((entry) => entry.value === value)?.label ?? value;

export function TraegerStaffReleaseSection({
  organizationId,
}: {
  readonly organizationId: string;
}) {
  const [state, setState] = useState<TraegerStaffReleaseState | null>(null);
  const [granting, setGranting] = useState(false);
  const [representation, setRepresentation] =
    useState<StaffRepresentation | null>(null);
  const [withdrawing, setWithdrawing] = useState(false);
  const [withdrawRunning, setWithdrawRunning] = useState(false);
  const save = useFormModalPending(granting);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const load = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = load;
  const { show: showError } = useApiErrorDisplay();
  const { success } = useToast();
  const afterWrite = useRefreshAfterWrite("die Freigabe", async () => {
    setState(await traegerStaffReleaseService.getState(organizationId));
  });

  const refresh = useCallback(async () => {
    try {
      setState(await traegerStaffReleaseService.getState(organizationId));
      clearLoadError();
    } catch (err) {
      logger.warn("traeger_staff_release_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Freigabe Personal beim Träger",
        retry: () => void refresh(),
      });
    }
  }, [organizationId, clearLoadError, showLoadError]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const grant = async () => {
    formErrors.clear();
    if (!representation) {
      formErrors.invalid("Bitte wählen Sie die Erklärung des Trägers.", {
        representation: "Bitte wählen Sie eine Erklärung.",
      });
      return;
    }
    let granted = false;
    await save.run(async (op) => {
      try {
        await traegerStaffReleaseService.grant(organizationId, representation);
      } catch (err) {
        logger.warn("traeger_staff_release_grant_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (op.isCurrent()) {
          void formErrors.show(err, { object: "die Freigabe" });
        }
        return;
      }
      granted = true;
      if (!op.isCurrent()) return;
      setGranting(false);
      success("Personal beim Träger ist freigeschaltet.");
    });
    if (granted) await afterWrite.refresh();
  };

  const withdraw = async () => {
    setWithdrawRunning(true);
    try {
      await traegerStaffReleaseService.withdraw(organizationId);
    } catch (err) {
      logger.warn("traeger_staff_release_withdraw_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setWithdrawing(false);
      setWithdrawRunning(false);
      void showError(err, { object: "die Freigabe" });
      return;
    }
    setWithdrawing(false);
    setWithdrawRunning(false);
    success("Die Freigabe ist zurückgenommen.");
    await afterWrite.refresh();
  };

  const release = state?.release ?? null;
  return (
    <SectionCard
      title="Personal beim Träger"
      titleBadge={
        state ? (
          <StatusBadge
            label={release ? "Freigeschaltet" : "Nicht freigeschaltet"}
            tone={release ? "green" : "gray"}
          />
        ) : undefined
      }
      description="Mit der Freigabe führt die Personalverwaltung des Trägers eine Personalakte über alle Schulen. Vorher bestätigt der Träger schriftlich eine der beiden Erklärungen."
      loading={!state && !load.error}
      action={
        state ? (
          release ? (
            <Button
              type="button"
              variant="outline_danger"
              disabled={state.records > 0}
              disabledReason={`Der Träger führt noch ${state.records} Personalakten. Er muss sie zuerst im Träger-Portal auflösen.`}
              tooltip="Nimmt die Freigabe zurück. Danach führt jede Schule ihr Personal wieder selbst."
              onClick={() => setWithdrawing(true)}
            >
              Freigabe zurücknehmen
            </Button>
          ) : (
            <Button
              type="button"
              variant="primary"
              tooltip="Hält die Erklärung des Trägers fest und schaltet die Personalakten frei."
              onClick={() => {
                setRepresentation(null);
                formErrors.clear();
                setGranting(true);
              }}
            >
              Freischalten
            </Button>
          )
        ) : undefined
      }
    >
      <LoadErrorAlert error={load.error} />
      <LoadErrorAlert error={afterWrite.error} />
      {release && state ? (
        <DataGrid columns={4}>
          <DataField label="Erklärung des Trägers" fullWidth>
            {representationLabel(release.representation)}
          </DataField>
          <DataField label="Freigeschaltet am">
            {formatBerlinDate(release.releasedAt)}
          </DataField>
          <DataField label="Personalakten">{state.records}</DataField>
        </DataGrid>
      ) : null}

      <FormModal
        isOpen={granting}
        onClose={() => setGranting(false)}
        pending={save}
        title="Personal beim Träger freischalten"
        titleLevel={2}
        size="md"
        error={formErrors.error}
        footer={
          <EditActions
            onCancel={() => setGranting(false)}
            onSave={() => formRef.current?.requestSubmit()}
            saving={save.pending}
            saveLabel="Freischalten"
          />
        }
      >
        <form
          ref={formRef}
          noValidate
          className="space-y-3"
          onSubmit={(event) => {
            event.preventDefault();
            void grant();
          }}
        >
          <FieldLabel id="staff-release-representation-label" required>
            Was hat der Träger schriftlich bestätigt?
          </FieldLabel>
          <div
            role="radiogroup"
            aria-labelledby="staff-release-representation-label"
            className="grid gap-2"
          >
            {STAFF_REPRESENTATIONS.map((entry) => (
              <ChoiceTile
                key={entry.value}
                selected={representation === entry.value}
                className="items-start gap-3 px-4 py-3"
              >
                <Radio
                  name="staff-release-representation"
                  value={entry.value}
                  checked={representation === entry.value}
                  onChange={() => setRepresentation(entry.value)}
                  className="mt-0.5"
                />
                <span className="flex min-w-0 flex-col gap-0.5">
                  <span className="font-medium text-gray-900">
                    {entry.label}
                  </span>
                  <span className="text-sm font-normal text-gray-600">
                    {entry.hint}
                  </span>
                </span>
              </ChoiceTile>
            ))}
          </div>
          {formErrors.fieldError("representation") ? (
            <p className="text-moto-red-strong text-sm">
              {formErrors.fieldError("representation")}
            </p>
          ) : null}
        </form>
      </FormModal>

      <ConfirmationModal
        isOpen={withdrawing}
        onClose={() => setWithdrawing(false)}
        onConfirm={() => void withdraw()}
        title="Freigabe zurücknehmen?"
        confirmText="Zurücknehmen"
        confirmVariant="warning"
        isConfirmLoading={withdrawRunning}
        loadingText="Wird zurückgenommen …"
      >
        <p className="text-sm text-gray-700">
          Danach führt jede Schule ihr Personal wieder selbst. Der Träger kann
          keine Personalakte mehr anlegen. Die Rücknahme steht im
          Operator-Audit.
        </p>
      </ConfirmationModal>
    </SectionCard>
  );
}

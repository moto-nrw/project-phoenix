"use client";

import { useEffect, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { SpinnerIcon } from "~/components/ui/icons";
import { Modal } from "~/components/ui/modal";
import { useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { substitutionService } from "~/lib/substitution-api";
import type { RunningSupervision } from "~/lib/substitution-helpers";

const logger = createLogger({ component: "AddSupervisorModal" });
const loadFallback =
  "Die Betreuung konnte nicht geladen werden. Bitte versuchen Sie es noch einmal.";
const saveFallback =
  "Der Betreuer konnte nicht hinzugefügt werden. Bitte versuchen Sie es noch einmal.";

interface AddSupervisorModalProps {
  readonly activeGroupId: string | null;
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onAdded: () => Promise<unknown>;
}

function useSupervisionOverview(activeGroupId: string | null, isOpen: boolean) {
  const [overview, setOverview] = useState<RunningSupervision | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  useEffect(() => {
    if (!isOpen || !activeGroupId) return;
    let current = true;
    setOverview(null);
    setError(null);
    setIsLoading(true);
    void substitutionService
      .fetchRunningSupervision(activeGroupId)
      .then((result) => current && setOverview(result))
      .catch((cause: unknown) => {
        if (current)
          setError(cause instanceof Error ? cause.message : loadFallback);
      })
      .finally(() => current && setIsLoading(false));
    return () => {
      current = false;
    };
  }, [activeGroupId, isOpen]);
  return { overview, error, setError, isLoading };
}

function SupervisionDetails({
  overview,
  selectedStaffId,
  setSelectedStaffId,
}: {
  overview: RunningSupervision;
  selectedStaffId: string;
  setSelectedStaffId: (value: string) => void;
}) {
  if (!overview.canAssign)
    return (
      <Alert
        type="info"
        message="Sie beaufsichtigen diese Betreuung nicht mehr. Deshalb können Sie niemanden hinzufügen."
      />
    );
  if (overview.availableTargets.length === 0)
    return (
      <Alert
        type="info"
        message="Alle verfügbaren Betreuungskräfte sind schon eingetragen."
      />
    );
  return (
    <SupervisorSelect
      overview={overview}
      selectedStaffId={selectedStaffId}
      setSelectedStaffId={setSelectedStaffId}
    />
  );
}

function SupervisorSelect({
  overview,
  selectedStaffId,
  setSelectedStaffId,
}: {
  overview: RunningSupervision;
  selectedStaffId: string;
  setSelectedStaffId: (value: string) => void;
}) {
  return (
    <div>
      <label
        id="additional-supervisor-label"
        htmlFor="additional-supervisor"
        className="mb-2 block text-sm font-medium text-gray-700"
      >
        Betreuer auswählen
      </label>
      <CustomSelect
        id="additional-supervisor"
        ariaLabelledBy="additional-supervisor-label"
        value={selectedStaffId}
        onChange={setSelectedStaffId}
        placeholder="Person auswählen..."
        options={overview.availableTargets.map((staff) => ({
          value: staff.id,
          label: staff.fullName,
        }))}
      />
    </div>
  );
}

function useAddSupervisor(
  props: AddSupervisorModalProps,
  selectedStaffId: string,
  setError: (error: string | null) => void,
) {
  const [isSaving, setIsSaving] = useState(false);
  const toast = useToast();
  const add = async () => {
    if (!props.activeGroupId || !selectedStaffId) return;
    setError(null);
    setIsSaving(true);
    try {
      const result = await substitutionService.addSupervisor(
        props.activeGroupId,
        selectedStaffId,
      );
      try {
        await props.onAdded();
      } catch (cause) {
        logger.warn("additional_supervision_refresh_failed", {
          error: String(cause),
        });
      }
      toast.success(`${result.targetName} ist jetzt als Betreuer eingetragen.`);
      props.onClose();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : saveFallback);
    } finally {
      setIsSaving(false);
    }
  };
  return { add, isSaving };
}

function ModalBody(props: {
  overview: RunningSupervision | null;
  error: string | null;
  isLoading: boolean;
  selectedStaffId: string;
  setSelectedStaffId: (value: string) => void;
}) {
  return (
    <div className="space-y-5">
      {props.error ? <Alert type="error" message={props.error} /> : null}
      {props.isLoading ? (
        <div className="flex items-center gap-2 text-sm text-gray-600">
          <SpinnerIcon /> Betreuung wird geladen...
        </div>
      ) : null}
      {props.overview ? (
        <>
          <p className="text-sm text-gray-600">
            Die Person betreut diese laufende Aufsicht ab sofort mit.
          </p>
          <p className="text-sm text-gray-700">
            <span className="font-medium text-gray-900">
              Schon eingetragen:
            </span>{" "}
            {props.overview.supervisors
              .map((staff) => staff.fullName)
              .join(", ")}
          </p>
          <SupervisionDetails {...props} overview={props.overview} />
        </>
      ) : null}
    </div>
  );
}

function ModalFooter(props: {
  onClose: () => void;
  onAdd: () => void;
  disabled: boolean;
  isSaving: boolean;
}) {
  return (
    <div className="flex justify-end gap-3">
      <Button
        type="button"
        size="md"
        variant="secondary"
        onClick={props.onClose}
        disabled={props.isSaving}
      >
        Abbrechen
      </Button>
      <Button
        type="button"
        size="md"
        onClick={props.onAdd}
        disabled={props.disabled}
        isLoading={props.isSaving}
        loadingText="Wird hinzugefügt..."
      >
        Hinzufügen
      </Button>
    </div>
  );
}

export function AddSupervisorModal(props: AddSupervisorModalProps) {
  const state = useSupervisionOverview(props.activeGroupId, props.isOpen);
  const [selectedStaffId, setSelectedStaffId] = useState("");
  const action = useAddSupervisor(props, selectedStaffId, state.setError);
  useEffect(() => setSelectedStaffId(""), [props.activeGroupId, props.isOpen]);
  const disabled =
    !selectedStaffId ||
    state.isLoading ||
    !state.overview?.canAssign ||
    state.overview.availableTargets.length === 0 ||
    action.isSaving;
  return (
    <Modal
      isOpen={props.isOpen}
      onClose={props.onClose}
      title="Betreuer hinzufügen"
      footer={
        <ModalFooter
          onClose={props.onClose}
          onAdd={() => void action.add()}
          disabled={disabled}
          isSaving={action.isSaving}
        />
      }
      isDismissDisabled={action.isSaving}
    >
      <ModalBody
        {...state}
        selectedStaffId={selectedStaffId}
        setSelectedStaffId={setSelectedStaffId}
      />
    </Modal>
  );
}

"use client";

import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import type { FormErrorInput } from "~/components/ui/form-error";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { SpinnerIcon } from "~/components/ui/icons";
import { Modal } from "~/components/ui/modal";
import {
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { substitutionService } from "~/lib/substitution-api";
import type { RunningSupervision } from "~/lib/substitution-helpers";

import {
  ExternalCaregiverEntry,
  type ExternalCaregiverResult,
} from "./external-caregiver-entry";

const logger = createLogger({ component: "AddSupervisorModal" });

interface AddSupervisorModalProps {
  readonly activeGroupId: string | null;
  readonly canCreateExternalCaregiver?: boolean;
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onAdded: () => Promise<unknown>;
}

function useSupervisionOverview(activeGroupId: string | null, isOpen: boolean) {
  const [overview, setOverview] = useState<RunningSupervision | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  // Erhöht von „Wiederholen“: lädt die Betreuung erneut.
  const [attempt, setAttempt] = useState(0);
  const {
    error: loadError,
    show: showLoadError,
    clear: clearLoadError,
  } = useApiLoadError();
  useEffect(() => {
    if (!isOpen || !activeGroupId) return;
    let current = true;
    setOverview(null);
    clearLoadError();
    setIsLoading(true);
    void substitutionService
      .fetchRunningSupervision(activeGroupId)
      .then((result) => current && setOverview(result))
      .catch((cause: unknown) => {
        logger.error("running_supervision_load_failed", {
          error: cause instanceof Error ? cause.message : String(cause),
        });
        if (current)
          void showLoadError(cause, {
            object: "die laufende Betreuung",
            retry: () => setAttempt((value) => value + 1),
          });
      })
      .finally(() => current && setIsLoading(false));
    return () => {
      current = false;
    };
  }, [activeGroupId, isOpen, attempt, showLoadError, clearLoadError]);
  return { overview, loadError, isLoading };
}

type SupervisionTarget = RunningSupervision["availableTargets"][number];

function SupervisionDetails({
  overview,
  targets,
  selectedStaffId,
  setSelectedStaffId,
  canCreateExternalCaregiver,
  onExternalAdded,
}: {
  overview: RunningSupervision;
  targets: readonly SupervisionTarget[];
  selectedStaffId: string;
  setSelectedStaffId: (value: string) => void;
  canCreateExternalCaregiver: boolean;
  onExternalAdded: (result: ExternalCaregiverResult) => void;
}) {
  if (!overview.canAssign)
    return (
      <Alert
        type="info"
        message="Sie beaufsichtigen diese Betreuung nicht mehr. Deshalb können Sie niemanden hinzufügen."
      />
    );
  return (
    <div className="space-y-3">
      {targets.length === 0 ? (
        <Alert
          type="info"
          message="Alle verfügbaren Betreuungskräfte sind schon eingetragen."
        />
      ) : (
        <SupervisorSelect
          targets={targets}
          selectedStaffId={selectedStaffId}
          setSelectedStaffId={setSelectedStaffId}
        />
      )}
      {canCreateExternalCaregiver ? (
        <ExternalCaregiverEntry
          existing={[
            ...targets.map((target) => ({
              id: target.id,
              fullName: target.fullName,
              isExternal: target.isExternal === true,
            })),
            ...overview.supervisors.map((supervisor) => ({
              id: supervisor.id,
              fullName: supervisor.fullName,
              isExternal: supervisor.isExternal === true,
              isAlreadyAssigned: true,
            })),
          ]}
          onAdded={onExternalAdded}
        />
      ) : null}
    </div>
  );
}

function targetLabel(target: SupervisionTarget): string {
  return target.isExternal ? `${target.fullName} (extern)` : target.fullName;
}

function SupervisorSelect({
  targets,
  selectedStaffId,
  setSelectedStaffId,
}: {
  targets: readonly SupervisionTarget[];
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
        options={targets.map((target) => ({
          value: target.id,
          label: targetLabel(target),
        }))}
      />
    </div>
  );
}

function useAddSupervisor(
  props: AddSupervisorModalProps,
  selectedStaffId: string,
) {
  const [isSaving, setIsSaving] = useState(false);
  const toast = useToast();
  const formErrors = useApiFormError();
  const { clear: clearFormErrors } = formErrors;
  // „Wiederholen“ trägt die aktuell gewählte Person ein.
  const latestAddRef = useRef<() => void>(() => undefined);
  useEffect(() => {
    if (props.isOpen) clearFormErrors();
  }, [props.isOpen, props.activeGroupId, clearFormErrors]);
  const add = async () => {
    if (!props.activeGroupId || !selectedStaffId) return;
    formErrors.clear();
    setIsSaving(true);
    try {
      const result = await substitutionService.addSupervisor(
        props.activeGroupId,
        selectedStaffId,
      );
      try {
        await props.onAdded();
      } catch (cause) {
        // Bewusst still: die Person ist eingetragen; die Übersicht lädt beim
        // nächsten Abruf den neuen Stand.
        logger.warn("additional_supervision_refresh_failed", {
          error: String(cause),
        });
      }
      toast.success(`${result.targetName} ist jetzt als Betreuer eingetragen.`);
      props.onClose();
    } catch (cause) {
      logger.error("additional_supervision_failed", {
        error: cause instanceof Error ? cause.message : String(cause),
      });
      void formErrors.show(cause, {
        object: "die zusätzliche Betreuung",
        retry: () => latestAddRef.current(),
      });
    } finally {
      setIsSaving(false);
    }
  };
  useLayoutEffect(() => {
    latestAddRef.current = () => void add();
  });
  return { add, isSaving, error: formErrors.error };
}

function ModalBody(props: {
  overview: RunningSupervision | null;
  targets: readonly SupervisionTarget[];
  loadError: FormErrorInput;
  saveError: FormErrorInput;
  isLoading: boolean;
  selectedStaffId: string;
  setSelectedStaffId: (value: string) => void;
  onExternalAdded: (result: ExternalCaregiverResult) => void;
  canCreateExternalCaregiver: boolean;
}) {
  return (
    <div className="space-y-5">
      <LoadErrorAlert error={props.loadError} />
      <FormErrorAlert message={props.saveError} />
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

/**
 * The targets the overview offers plus the external people recorded in this
 * dialog (#3823), so a new entry is selectable without reloading.
 */
function useSupervisionTargets(
  overview: RunningSupervision | null,
  activeGroupId: string | null,
  isOpen: boolean,
  setSelectedStaffId: (value: string) => void,
) {
  const [added, setAdded] = useState<SupervisionTarget[]>([]);
  useEffect(() => setAdded([]), [activeGroupId, isOpen]);
  const offered = overview?.availableTargets ?? [];
  const targets = [
    ...offered,
    ...added.filter((extra) => !offered.some((item) => item.id === extra.id)),
  ];
  const onExternalAdded = (result: ExternalCaregiverResult) => {
    if (result.kind === "created") {
      const { staff } = result;
      const fullName =
        staff.name || [staff.firstName, staff.lastName].join(" ").trim();
      setAdded((prev) => [
        ...prev,
        { id: staff.id, fullName, isExternal: true },
      ]);
      setSelectedStaffId(staff.id);
      return;
    }
    setSelectedStaffId(result.id);
  };
  return { targets, onExternalAdded };
}

export function AddSupervisorModal(props: AddSupervisorModalProps) {
  const state = useSupervisionOverview(props.activeGroupId, props.isOpen);
  const [selectedStaffId, setSelectedStaffId] = useState("");
  const action = useAddSupervisor(props, selectedStaffId);
  useEffect(() => setSelectedStaffId(""), [props.activeGroupId, props.isOpen]);
  const { targets, onExternalAdded } = useSupervisionTargets(
    state.overview,
    props.activeGroupId,
    props.isOpen,
    setSelectedStaffId,
  );
  const disabled =
    !selectedStaffId ||
    state.isLoading ||
    !state.overview?.canAssign ||
    targets.length === 0 ||
    action.isSaving;
  return (
    <Modal
      isOpen={props.isOpen}
      onClose={props.onClose}
      title="Betreuer hinzufügen"
      mobileSheet
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
        targets={targets}
        onExternalAdded={onExternalAdded}
        canCreateExternalCaregiver={props.canCreateExternalCaregiver ?? false}
        saveError={action.error}
        selectedStaffId={selectedStaffId}
        setSelectedStaffId={setSelectedStaffId}
      />
    </Modal>
  );
}

"use client";

import { ArrowLeftRight } from "lucide-react";
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
  SegmentedControl,
  type SegmentedControlItem,
} from "~/components/ui/segmented-control";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { substitutionService } from "~/lib/substitution-api";
import type { RunningSupervision } from "~/lib/substitution-helpers";
import {
  supervisionTargetLabel,
  useRunningSupervision,
  type SupervisionTarget,
} from "~/components/active-supervisions/running-supervision";

const logger = createLogger({ component: "HandOverSupervisionModal" });

type Mode = "hand-over" | "release";

const MODE_ITEMS: readonly SegmentedControlItem<Mode>[] = [
  { value: "hand-over", label: "An Person übergeben" },
  { value: "release", label: "Aufsicht abgeben" },
];

interface HandOverSupervisionModalProps {
  /** The running session the caller supervises. */
  readonly activeGroupId: string | null;
  readonly currentStaffId: string | null | undefined;
  readonly isOpen: boolean;
  readonly onClose: () => void;
  /** Reloads the page and the sidebar after the caller left the session. */
  readonly onDone: () => Promise<unknown>;
}

function useSupervisionChange(
  props: HandOverSupervisionModalProps,
  mode: Mode,
  selectedStaffId: string,
) {
  const [isSaving, setIsSaving] = useState(false);
  const toast = useToast();
  const formErrors = useApiFormError();
  const { clear: clearFormErrors } = formErrors;
  // „Wiederholen“ wiederholt die zuletzt gewählte Aktion.
  const latestRef = useRef<() => void>(() => undefined);
  useEffect(() => {
    clearFormErrors();
  }, [props.isOpen, props.activeGroupId, mode, clearFormErrors]);
  const submit = async () => {
    const activeGroupId = props.activeGroupId;
    if (!activeGroupId) return;
    if (mode === "hand-over" && !selectedStaffId) return;
    formErrors.clear();
    setIsSaving(true);
    try {
      let message = "Sie sind nicht mehr als Aufsicht eingetragen.";
      if (mode === "hand-over") {
        const result = await substitutionService.handOverSupervision(
          activeGroupId,
          selectedStaffId,
        );
        message = `${result.targetName} hat die Aufsicht übernommen.`;
      } else {
        await substitutionService.releaseSupervision(activeGroupId);
      }
      try {
        await props.onDone();
      } catch (cause) {
        // Bewusst still: die Aufsicht ist geändert; die Seite lädt beim
        // nächsten Abruf den neuen Stand.
        logger.warn("supervision_change_refresh_failed", {
          error: String(cause),
        });
      }
      toast.success(message);
      props.onClose();
    } catch (cause) {
      logger.error("supervision_change_failed", {
        mode,
        error: cause instanceof Error ? cause.message : String(cause),
      });
      void formErrors.show(cause, {
        object:
          mode === "hand-over"
            ? "die Übergabe der Aufsicht"
            : "das Abgeben der Aufsicht",
        retry: () => latestRef.current(),
      });
    } finally {
      setIsSaving(false);
    }
  };
  useLayoutEffect(() => {
    latestRef.current = () => void submit();
  });
  return { submit, isSaving, error: formErrors.error };
}

function HandOverFields({
  targets,
  selectedStaffId,
  setSelectedStaffId,
}: {
  targets: readonly SupervisionTarget[];
  selectedStaffId: string;
  setSelectedStaffId: (value: string) => void;
}) {
  if (targets.length === 0)
    return (
      <Alert
        type="info"
        message="Gerade kann niemand die Aufsicht übernehmen. Alle Betreuungskräfte sind schon eingetragen."
      />
    );
  return (
    <div>
      <label
        id="hand-over-supervisor-label"
        htmlFor="hand-over-supervisor"
        className="mb-2 block text-sm font-medium text-gray-700"
      >
        An wen übergeben Sie?
      </label>
      <CustomSelect
        id="hand-over-supervisor"
        ariaLabelledBy="hand-over-supervisor-label"
        value={selectedStaffId}
        onChange={setSelectedStaffId}
        placeholder="Person auswählen..."
        options={targets.map((target) => ({
          value: target.id,
          label: supervisionTargetLabel(target),
        }))}
      />
    </div>
  );
}

function ReleaseFields({ others }: { others: readonly string[] }) {
  if (others.length === 0)
    return (
      <Alert
        type="info"
        message="Sie sind die einzige Aufsicht. Übergeben Sie die Aufsicht an eine Person oder beenden Sie die Aktivität."
      />
    );
  return (
    <p className="text-sm text-gray-700">
      <span className="font-medium text-gray-900">Weiter in der Aufsicht:</span>{" "}
      {others.join(", ")}
    </p>
  );
}

function ModalBody(props: {
  overview: RunningSupervision | null;
  others: readonly string[];
  mode: Mode;
  setMode: (mode: Mode) => void;
  loadError: FormErrorInput;
  saveError: FormErrorInput;
  isLoading: boolean;
  selectedStaffId: string;
  setSelectedStaffId: (value: string) => void;
}) {
  const { overview } = props;
  return (
    <div className="space-y-5">
      <LoadErrorAlert error={props.loadError} />
      <FormErrorAlert message={props.saveError} />
      {props.isLoading ? (
        <div className="flex items-center gap-2 text-sm text-gray-600">
          <SpinnerIcon /> Aufsicht wird geladen...
        </div>
      ) : null}
      {overview && !overview.isCurrentUserSupervising ? (
        <Alert
          type="info"
          message="Sie beaufsichtigen diese Aktivität nicht mehr. Deshalb können Sie die Aufsicht nicht übergeben."
        />
      ) : null}
      {overview?.isCurrentUserSupervising ? (
        <>
          <p className="text-sm text-gray-600">
            Die Kinder bleiben im Raum und die Aktivität läuft weiter. Die
            Planung ändert sich nicht.
          </p>
          <SegmentedControl
            items={MODE_ITEMS}
            value={props.mode}
            onChange={props.setMode}
            ariaLabel="Wie geben Sie die Aufsicht ab?"
            variant="joined"
            fullWidth
          />
          {props.mode === "hand-over" ? (
            <HandOverFields
              targets={overview.availableTargets}
              selectedStaffId={props.selectedStaffId}
              setSelectedStaffId={props.setSelectedStaffId}
            />
          ) : (
            <ReleaseFields others={props.others} />
          )}
        </>
      ) : null}
    </div>
  );
}

function ModalFooter(props: {
  mode: Mode;
  onClose: () => void;
  onSubmit: () => void;
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
        onClick={props.onSubmit}
        disabled={props.disabled}
        isLoading={props.isSaving}
        loadingText={
          props.mode === "hand-over" ? "Wird übergeben..." : "Wird abgegeben..."
        }
      >
        {props.mode === "hand-over" ? "Übergeben" : "Abgeben"}
      </Button>
    </div>
  );
}

/**
 * The head and section action that opens the dialog below, for a session the
 * caller supervises (#4763).
 */
export function HandOverSupervisionButton({
  activeGroupId,
  onHandOver,
  className,
}: Readonly<{
  activeGroupId: string | null;
  onHandOver: (activeGroupId: string) => void;
  className?: string;
}>) {
  if (!activeGroupId) return null;
  return (
    <Button
      type="button"
      variant="outline"
      size="md"
      className={className}
      onClick={() => onHandOver(activeGroupId)}
    >
      <ArrowLeftRight className="h-4 w-4" aria-hidden="true" />
      Aufsicht übergeben
    </Button>
  );
}

/**
 * „Aufsicht übergeben“ in a running room supervision (#4763): hand it over to
 * someone „Betreuer hinzufügen“ could add, or leave while another supervisor
 * stays. Only the running supervision changes, never the planning.
 */
export function HandOverSupervisionModal(props: HandOverSupervisionModalProps) {
  const state = useRunningSupervision(
    props.activeGroupId,
    props.isOpen,
    "die laufende Aufsicht",
  );
  const [mode, setMode] = useState<Mode>("hand-over");
  const [selectedStaffId, setSelectedStaffId] = useState("");
  useEffect(() => {
    setMode("hand-over");
    setSelectedStaffId("");
  }, [props.activeGroupId, props.isOpen]);
  const action = useSupervisionChange(props, mode, selectedStaffId);
  const others = (state.overview?.supervisors ?? [])
    .filter((staff) => staff.id !== props.currentStaffId)
    .map((staff) => staff.fullName);
  const supervising = state.overview?.isCurrentUserSupervising ?? false;
  const disabled =
    state.isLoading ||
    !supervising ||
    action.isSaving ||
    (mode === "hand-over" ? !selectedStaffId : others.length === 0);
  return (
    <Modal
      isOpen={props.isOpen}
      onClose={props.onClose}
      title="Aufsicht übergeben"
      mobileSheet
      footer={
        <ModalFooter
          mode={mode}
          onClose={props.onClose}
          onSubmit={() => void action.submit()}
          disabled={disabled}
          isSaving={action.isSaving}
        />
      }
      isDismissDisabled={action.isSaving}
    >
      <ModalBody
        overview={state.overview}
        others={others}
        mode={mode}
        setMode={setMode}
        loadError={state.loadError}
        saveError={action.error}
        isLoading={state.isLoading}
        selectedStaffId={selectedStaffId}
        setSelectedStaffId={setSelectedStaffId}
      />
    </Modal>
  );
}

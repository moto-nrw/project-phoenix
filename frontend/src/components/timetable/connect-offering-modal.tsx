"use client";

/**
 * „Mit Angebot verbinden“ (#5103): a Regeltermin whose hand-kept children
 * list matches care offerings takes its children from them from the next
 * occurrence on. The dialog names the offerings and the filter, shows per
 * weekday who joins and who leaves, and switches every period of the series
 * in one step. Nothing else of the Regeltermin changes. Several offerings
 * combine (#5139): a child comes when it books one of them, so a Randstunde
 * connected with Randstunde and Ganztag keeps the Ganztag children.
 */

import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";

import { PhasesOutsidePeriodNotice } from "~/components/timetable/phases-outside-period-notice";
import { EditActions } from "~/components/ui/edit-actions";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { FormModal } from "~/components/ui/form-modal";
import { MultiCheckboxSelect } from "~/components/ui/multi-checkbox-select";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { formatDate } from "~/lib/date-helpers";
import { useOfferingPreview } from "~/lib/hooks/use-offering-preview";
import { createLogger } from "~/lib/logger";
import type { OfferingPhaseOutsidePeriod } from "~/lib/offering-gap";
import {
  describeSelectionFilter,
  laterSegmentSentence,
  offeringNamesFor,
  offeringSetNames,
  type OfferingSelection,
  type RosterDeviation,
} from "~/lib/roster-deviation";
import { timetableService } from "~/lib/timetable-api";
import type { OfferingSourceOption } from "~/lib/timetable-types";

import { OfferingPreviewList } from "./offering-preview-list";

const logger = createLogger({ component: "ConnectOfferingModal" });

const OFFERINGS_FIELD_ID = "connect-offering-offerings";

/**
 * The suggested offerings and candidates first, then every other offering of
 * the period, so the school can add one the rule did not name. A combined
 * source needs one enrollment phase: once an offering is chosen, offerings
 * of another phase are disabled, as in the Regeltermin editor.
 */
function offeringOptions(
  deviation: RosterDeviation,
  periodOfferings: readonly OfferingSourceOption[],
  selected: readonly string[],
) {
  const phases = new Map(
    periodOfferings.map((offering) => [offering.id, offering.phaseId]),
  );
  const offerings = new Map<string, string>();
  for (const offering of [
    ...deviation.offerings,
    ...deviation.candidates,
    ...periodOfferings,
  ]) {
    if (!offerings.has(offering.id)) offerings.set(offering.id, offering.name);
  }
  const lockedPhase = selected
    .map((id) => phases.get(id))
    .find((phase) => phase !== undefined);
  return [...offerings].map(([id, name]) => {
    const phase = phases.get(id);
    return {
      value: id,
      label: name,
      disabled:
        lockedPhase !== undefined &&
        phase !== undefined &&
        phase !== lockedPhase &&
        !selected.includes(id),
    };
  });
}

export function ConnectOfferingModal({
  isOpen,
  templateName,
  periodId,
  deviation,
  canManagePhases = false,
  onClose,
  onConnected,
}: Readonly<{
  isOpen: boolean;
  templateName: string;
  /** The planning period whose offerings the school may add. */
  periodId?: string;
  deviation: RosterDeviation;
  /** May open the Anmeldephasen page to align a phase with the period. */
  canManagePhases?: boolean;
  onClose: () => void;
  /** After a successful save; the caller refreshes plan and lists. */
  onConnected: () => void | Promise<void>;
}>) {
  // Another offering fits as well (#5138): the school picks, none is set.
  const initialKey = deviation.ambiguous
    ? ""
    : deviation.offerings.map((offering) => offering.id).join(",");
  const [offeringIds, setOfferingIds] = useState<string[]>(() =>
    initialKey ? initialKey.split(",") : [],
  );
  const [periodOfferings, setPeriodOfferings] = useState<
    OfferingSourceOption[]
  >([]);
  const [phasesOutsidePeriod, setPhasesOutsidePeriod] = useState<
    OfferingPhaseOutsidePeriod[]
  >([]);
  const [saving, setSaving] = useState(false);
  const formErrors = useApiFormError();
  const { clear: clearFormError } = formErrors;
  const { success: toastSuccess } = useToast();
  const latestSaveRef = useRef<() => void>(() => undefined);

  useEffect(() => {
    if (!isOpen) return;
    setOfferingIds(initialKey ? initialKey.split(",") : []);
    clearFormError();
  }, [isOpen, initialKey, clearFormError]);

  useEffect(() => {
    setPhasesOutsidePeriod([]);
    if (!isOpen || !periodId) return;
    let active = true;
    timetableService
      .getOfferingSourceChoice(periodId)
      .then((choice) => {
        if (!active) return;
        setPeriodOfferings(choice.offerings);
        setPhasesOutsidePeriod(choice.phasesOutsidePeriod);
      })
      .catch((err: unknown) => {
        // Bewusst ohne Anzeige: die vorgeschlagenen Angebote bleiben
        // wählbar und das Verbinden funktioniert; es fehlen nur weitere
        // Angebote zur Auswahl.
        logger.warn("offering_sources_load_failed", {
          period_id: periodId,
          error: err instanceof Error ? err.message : String(err),
        });
        if (active) setPhasesOutsidePeriod([]);
      });
    return () => {
      active = false;
    };
  }, [isOpen, periodId]);

  const selection: OfferingSelection = useMemo(
    () => ({
      offeringIds,
      gradeLevels: deviation.gradeLevels,
      schoolClasses: deviation.schoolClasses,
    }),
    [offeringIds, deviation.gradeLevels, deviation.schoolClasses],
  );
  const {
    preview,
    loading,
    error: previewError,
  } = useOfferingPreview(isOpen ? deviation.templateId : undefined, selection);
  const options = offeringOptions(deviation, periodOfferings, offeringIds);
  const selectedOfferings = options
    .filter((option) => offeringIds.includes(option.value))
    .map((option) => ({ id: option.value, name: option.label }));
  const phaseLocked = options.some((option) => option.disabled);
  const segmentSentence = preview ? laterSegmentSentence(preview) : null;

  const handleSave = async () => {
    if (!preview) return;
    setSaving(true);
    formErrors.clear();
    try {
      await timetableService.connectTemplateToOffering(
        deviation.templateId,
        selection,
        preview.from,
      );
      toastSuccess(
        `Regeltermin ist mit ${offeringSetNames(selectedOfferings)} verbunden.`,
      );
      await onConnected();
      onClose();
    } catch (err) {
      logger.error("offering_connection_failed", {
        template_id: deviation.templateId,
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, {
        object: `die Verbindung von „${templateName}“ mit ${selectedOfferings.length > 1 ? "den Angeboten" : "dem Angebot"}`,
        retry: () => latestSaveRef.current(),
      });
    } finally {
      setSaving(false);
    }
  };

  useLayoutEffect(() => {
    latestSaveRef.current = () => void handleSave();
  });

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title="Mit Angebot verbinden"
      size="md"
      closeDisabled={saving}
      error={formErrors.error}
      footer={
        <EditActions
          onCancel={onClose}
          onSave={() => void handleSave()}
          saving={saving}
          disabled={!preview || loading}
          saveLabel="Verbinden"
          savingLabel="Wird verbunden…"
        />
      }
    >
      <div className="flex flex-col gap-4">
        <p className="text-sm text-gray-700">
          {selectedOfferings.length === 0
            ? `${deviation.ambiguous ? "Mehrere Angebote passen zur Kinderliste. " : ""}Wählen Sie die Angebote, deren angemeldete Kinder kommen sollen.`
            : preview
              ? `Ab ${formatDate(preview.from)} kommen nur noch die für ${offeringNamesFor(selectedOfferings)} angemeldeten Kinder. An- und Abmeldungen werden danach automatisch übernommen.`
              : `„${templateName}“ bekommt seine Kinder dann aus ${selectedOfferings.length > 1 ? "den Angeboten" : "dem Angebot"} ${offeringSetNames(selectedOfferings)}.`}
        </p>
        <div className="flex flex-col gap-1">
          <label
            htmlFor={OFFERINGS_FIELD_ID}
            className="text-sm font-medium text-gray-700"
          >
            Angebote
          </label>
          <MultiCheckboxSelect
            id={OFFERINGS_FIELD_ID}
            ariaLabel="Angebote"
            value={offeringIds}
            options={options}
            onChange={setOfferingIds}
            disabled={saving}
            emptyLabel="Angebote wählen"
            summaryLabel={(selected) =>
              selected.map((option) => option.label).join(", ")
            }
          />
          {phaseLocked ? (
            <p className="text-xs text-gray-500">
              Angebote anderer Anmeldephasen sind ausgegraut, solange ein
              Angebot gewählt ist.
            </p>
          ) : null}
        </div>
        <PhasesOutsidePeriodNotice
          phases={phasesOutsidePeriod}
          context="choice"
          canManagePhases={canManagePhases}
        />
        <div className="flex flex-col gap-1">
          <span className="text-sm font-medium text-gray-700">Kinder</span>
          <span className="text-sm text-gray-900">
            {describeSelectionFilter(selection)}
          </span>
        </div>
        {selectedOfferings.length === 0 ? null : (
          <div className="flex flex-col gap-2">
            <span className="text-sm font-medium text-gray-700">
              Das ändert sich
            </span>
            {loading ? (
              <p className="text-sm text-gray-600">
                Die Vorschau wird geladen.
              </p>
            ) : null}
            <LoadErrorAlert error={previewError} />
            {preview && !loading ? (
              <>
                {segmentSentence ? (
                  <p className="text-sm text-gray-700">{segmentSentence}</p>
                ) : null}
                <OfferingPreviewList preview={preview} />
              </>
            ) : null}
          </div>
        )}
      </div>
    </FormModal>
  );
}

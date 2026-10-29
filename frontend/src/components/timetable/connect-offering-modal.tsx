"use client";

/**
 * „Mit Angebot verbinden“ (#5103): a Regeltermin whose hand-kept children
 * list matches a care offering takes its children from that offering from
 * the next occurrence on. The dialog names the offering and the filter,
 * shows per weekday who joins and who leaves, and switches every period of
 * the series in one step. Nothing else of the Regeltermin changes.
 */

import {
  useEffect,
  useId,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";

import { CustomSelect } from "~/components/ui/custom-select";
import { EditActions } from "~/components/ui/edit-actions";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { FormModal } from "~/components/ui/form-modal";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { formatDate } from "~/lib/date-helpers";
import { useOfferingPreview } from "~/lib/hooks/use-offering-preview";
import { createLogger } from "~/lib/logger";
import {
  describeSelectionFilter,
  laterSegmentSentence,
  type OfferingSelection,
  type RosterDeviation,
} from "~/lib/roster-deviation";
import { timetableService } from "~/lib/timetable-api";

import { OfferingPreviewList } from "./offering-preview-list";

const logger = createLogger({ component: "ConnectOfferingModal" });

export function ConnectOfferingModal({
  isOpen,
  templateName,
  deviation,
  onClose,
  onConnected,
}: Readonly<{
  isOpen: boolean;
  templateName: string;
  deviation: RosterDeviation;
  onClose: () => void;
  /** After a successful save; the caller refreshes plan and lists. */
  onConnected: () => void | Promise<void>;
}>) {
  // Several offerings fit equally (#5138): the school picks, none is set.
  const initialOfferingId = deviation.ambiguous ? "" : deviation.offering.id;
  const [offeringId, setOfferingId] = useState(initialOfferingId);
  const [saving, setSaving] = useState(false);
  const formErrors = useApiFormError();
  const { clear: clearFormError } = formErrors;
  const { success: toastSuccess } = useToast();
  const latestSaveRef = useRef<() => void>(() => undefined);
  const offeringLabelId = useId();

  useEffect(() => {
    if (!isOpen) return;
    setOfferingId(initialOfferingId);
    clearFormError();
  }, [isOpen, initialOfferingId, clearFormError]);

  const selection: OfferingSelection = useMemo(
    () => ({
      offeringIds: offeringId ? [offeringId] : [],
      gradeLevels: deviation.gradeLevels,
      schoolClasses: deviation.schoolClasses,
    }),
    [offeringId, deviation.gradeLevels, deviation.schoolClasses],
  );
  const {
    preview,
    loading,
    error: previewError,
  } = useOfferingPreview(isOpen ? deviation.templateId : undefined, selection);
  const candidates =
    deviation.candidates.length > 0
      ? deviation.candidates
      : [deviation.offering];
  const offeringName = candidates.find(
    (candidate) => candidate.id === offeringId,
  )?.name;
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
      toastSuccess(`Regeltermin ist mit „${offeringName}“ verbunden.`);
      await onConnected();
      onClose();
    } catch (err) {
      logger.error("offering_connection_failed", {
        template_id: deviation.templateId,
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, {
        object: `die Verbindung von „${templateName}“ mit dem Angebot`,
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
          {offeringName === undefined
            ? "Mehrere Angebote passen zur Kinderliste. Wählen Sie das Angebot, dessen angemeldete Kinder kommen sollen."
            : preview
              ? `Ab ${formatDate(preview.from)} kommen nur noch die für „${offeringName}“ angemeldeten Kinder. An- und Abmeldungen werden danach automatisch übernommen.`
              : `„${templateName}“ bekommt seine Kinder dann aus dem Angebot „${offeringName}“.`}
        </p>
        <div className="flex flex-col gap-1">
          <span
            id={offeringLabelId}
            className="text-sm font-medium text-gray-700"
          >
            Angebot
          </span>
          {candidates.length > 1 ? (
            <CustomSelect
              value={offeringId}
              labelId={offeringLabelId}
              ariaLabelledBy={offeringLabelId}
              disabled={saving}
              placeholder="Angebot wählen"
              options={candidates.map((candidate) => ({
                value: candidate.id,
                label: candidate.name,
              }))}
              onChange={setOfferingId}
            />
          ) : (
            <span className="text-sm text-gray-900">{offeringName}</span>
          )}
        </div>
        <div className="flex flex-col gap-1">
          <span className="text-sm font-medium text-gray-700">Kinder</span>
          <span className="text-sm text-gray-900">
            {describeSelectionFilter(selection)}
          </span>
        </div>
        {offeringName === undefined ? null : (
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

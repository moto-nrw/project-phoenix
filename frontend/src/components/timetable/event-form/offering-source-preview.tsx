"use client";

import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { formatDate } from "~/lib/date-helpers";
import {
  useDraftOfferingPreview,
  useOfferingPreview,
} from "~/lib/hooks/use-offering-preview";
import type { DraftRoster } from "~/lib/offering-suggestion";
import {
  untouchedSegmentSentence,
  type OfferingPreview,
  type OfferingSelection,
} from "~/lib/roster-deviation";
import type { FormErrorInput } from "~/components/ui/form-error";

import { OfferingPreviewList } from "../offering-preview-list";
import type { EventFormState } from "./form-model";

/** The source rule as the editor would save it. */
function formOfferingSelection(form: EventFormState): OfferingSelection {
  return {
    offeringIds: form.sourceCareOfferingIds,
    gradeLevels:
      form.sourceFilterMode === "jahrgang" ? form.sourceGradeLevels : [],
    schoolClasses:
      form.sourceFilterMode === "klasse" ? form.sourceSchoolClasses : [],
  };
}

/**
 * Per weekday who joins and who leaves an existing Regeltermin when the
 * editor switches it to an offering source (#5103). Before, the hand-kept
 * list disappeared without a word. Also names the days of other periods of
 * the series that this edit does not change.
 */
export function OfferingSourcePreview({
  templateId,
  form,
  from,
}: Readonly<{
  templateId: string;
  form: EventFormState;
  /** Occurrence date of „Ab jetzt dauerhaft“; otherwise the next occurrence. */
  from?: string;
}>) {
  // The hook reloads on the content of the selection, not its identity.
  const { preview, loading, error } = useOfferingPreview(
    templateId,
    formOfferingSelection(form),
    from,
  );
  const untouched = preview
    ? untouchedSegmentSentence(preview, templateId)
    : null;
  return (
    <PreviewPanel
      preview={preview}
      loading={loading}
      error={error}
      untouched={untouched}
    />
  );
}

/**
 * The same preview while a new Regeltermin is planned (#5104): it compares
 * the offering's children with the list planned before the switch.
 */
export function DraftOfferingSourcePreview({
  periodId,
  roster,
  form,
  from,
}: Readonly<{
  periodId: string;
  /** The children per weekday the editor planned before the switch. */
  roster: DraftRoster;
  form: EventFormState;
  /** First day of the new Regeltermin; omitted means today. */
  from?: string;
}>) {
  const { preview, loading, error } = useDraftOfferingPreview(
    periodId,
    formOfferingSelection(form),
    roster,
    from,
  );
  return (
    <PreviewPanel
      preview={preview}
      loading={loading}
      error={error}
      untouched={null}
    />
  );
}

function PreviewPanel({
  preview,
  loading,
  error,
  untouched,
}: Readonly<{
  preview: OfferingPreview | null;
  loading: boolean;
  error: FormErrorInput;
  untouched: string | null;
}>) {
  return (
    <div className="flex flex-col gap-2 rounded-md border border-gray-200 bg-gray-50 p-3">
      <p className="text-xs font-semibold text-gray-700">
        {preview
          ? `Das ändert sich ab ${formatDate(preview.from)} an der Kinderliste`
          : "Das ändert sich an der Kinderliste"}
      </p>
      {loading ? (
        <p className="text-xs text-gray-600">Die Vorschau wird geladen.</p>
      ) : null}
      <LoadErrorAlert error={error} />
      {preview && !loading ? (
        <>
          <OfferingPreviewList preview={preview} />
          {untouched ? (
            <p className="text-xs text-gray-600">{untouched}</p>
          ) : null}
        </>
      ) : null}
    </div>
  );
}

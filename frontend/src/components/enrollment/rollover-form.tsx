"use client";

import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { useApiFormError, useApiLoadError } from "~/contexts/ToastContext";
import { Check } from "lucide-react";
import {
  createRollover,
  fetchRolloverPreview,
  type Phase,
  type RolloverInput,
  type RolloverMode,
  type RolloverPreview,
  type RolloverResult,
} from "~/lib/enrollment-phase-api";
import {
  fromDateTimeLocalValue,
  parseISODate,
  toDateTimeLocalValue,
  toISODate,
} from "~/lib/date-helpers";
import { CHILD_STATUS_LABELS } from "~/components/enrollment/child-status-badge";
import type { ChildStatus } from "~/lib/enrollment-status";
import { createLogger } from "~/lib/logger";
import { CustomSelect } from "~/components/ui/custom-select";
import { CheckboxCard } from "~/components/ui/checkbox-card";
import { FieldLabel } from "~/components/ui/field-label";
import { ISODatePicker } from "~/components/ui/date-picker";
import { DateTimePicker } from "~/components/ui/date-time-picker";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { DataField } from "~/components/ui/data-field";
import { SectionCard } from "~/components/ui/section-card";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { TenantPage } from "~/components/ui/tenant-page";
import { ConceptIconTile } from "~/components/ui/concept-icon-tile";
import { enrollmentsConcept } from "~/lib/concepts/enrollments";
import { FormFieldset } from "~/components/ui/form-fieldset";
import {
  type FormModalOperation,
  type FormModalPending,
  useFormModalPending,
} from "~/components/ui/use-form-modal-pending";

const logger = createLogger({ component: "RolloverForm" });

// Pre-fill helper: the new phase defaults to one year after the
// source phase's service window. Admin can override every field.
function prefillFromSource(source: Phase): RolloverInput {
  const start = parseISODate(source.service_start_date);
  const end = parseISODate(source.service_end_date);
  const oneYear = (d: Date) => {
    const next = new Date(d);
    next.setFullYear(d.getFullYear() + 1);
    return next;
  };

  const nextStart = oneYear(start);
  const nextEnd = oneYear(end);

  // Deadline default: two weeks before the new service start.
  const deadline = new Date(nextStart);
  deadline.setDate(deadline.getDate() - 14);

  return {
    name: `${source.name} (Folgejahr)`,
    kind: source.kind,
    service_start_date: toISODate(nextStart),
    service_end_date: toISODate(nextEnd),
    enrollment_open_at: null,
    enrollment_close_at: null,
    form_schema_id: source.form_schema_id ?? null,
    rollover_mode: "opt_out",
    rollover_auto_approve: false,
    rollover_deadline: deadline.toISOString(),
    rollover_bumps_grade: true,
  };
}

interface Props {
  readonly source: Phase;
  readonly onCancel: () => void;
  /**
   * The rollover exists. `op.isCurrent()` is false once the panel the form
   * sits in was closed or switched to another phase meanwhile (#6593).
   */
  readonly onSuccess: (result: RolloverResult, op: FormModalOperation) => void;
  /**
   * The pending contract of the `SlideOver` the form is embedded in, so its
   * submit locks the panel (#6593). Without it the form tracks its own.
   */
  readonly pending?: FormModalPending;
  /**
   * "page": das Formular ist die Seite und rendert das Seitengerüst
   * (`TenantPage`) um die Formularkarte (eigene Route /rollover).
   * "embedded" (Standard): das Formular steht unter einem fremden Seitenkopf
   * (Anmeldephasen-Editor) und behält seinen eigenen Abschnittskopf.
   */
  readonly variant?: "page" | "embedded";
}

const ROLLOVER_DESCRIPTION =
  "Alle bestätigten Anmeldungen aus dieser Phase werden in eine neue Phase übernommen. Eltern erhalten eine E-Mail mit den nächsten Schritten.";

export function RolloverForm({
  source,
  onCancel,
  onSuccess,
  pending,
  variant = "embedded",
}: Props) {
  const [draft, setDraft] = useState<RolloverInput>(() =>
    prefillFromSource(source),
  );
  const [deadlineLocal, setDeadlineLocal] = useState(
    toDateTimeLocalValue(draft.rollover_deadline),
  );
  const ownSave = useFormModalPending();
  const save = pending ?? ownSave;
  const submitting = save.pending;
  const formErrors = useApiFormError();
  const [preview, setPreview] = useState<RolloverPreview | null>(null);
  const previewLoad = useApiLoadError();
  const showPreviewError = previewLoad.show;
  const clearPreviewError = previewLoad.clear;
  // Bumped by „Wiederholen“ to load the preview again.
  const [previewAttempt, setPreviewAttempt] = useState(0);
  const [loadingPreview, setLoadingPreview] = useState(true);

  const update = <K extends keyof RolloverInput>(
    key: K,
    value: RolloverInput[K],
  ) => {
    setDraft((d) => ({ ...d, [key]: value }));
  };

  useEffect(() => {
    let cancelled = false;
    setLoadingPreview(true);
    fetchRolloverPreview(source.id, draft.rollover_bumps_grade ?? true)
      .then((result) => {
        if (cancelled) return;
        setPreview(result);
        clearPreviewError();
      })
      .catch(async (err: unknown) => {
        if (cancelled) return;
        logger.warn("rollover_preview_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setPreview(null);
        await showPreviewError(err, {
          object: "die Vorschau",
          retry: () => setPreviewAttempt((current) => current + 1),
        });
      })
      .finally(() => {
        if (!cancelled) setLoadingPreview(false);
      });
    return () => {
      cancelled = true;
    };
  }, [
    source.id,
    draft.rollover_bumps_grade,
    previewAttempt,
    showPreviewError,
    clearPreviewError,
  ]);

  const excludedDetails = preview
    ? Object.entries(preview.excluded_by_status)
        .map(
          ([status, count]) =>
            `${CHILD_STATUS_LABELS[status as ChildStatus] ?? status}: ${count}`,
        )
        .join(" · ")
    : "";
  const reviewDetails = preview
    ? Object.entries(preview.review_by_reason)
        .map(
          ([reason, count]) =>
            `${PREVIEW_REVIEW_REASON_LABELS[reason] ?? reason}: ${count}`,
        )
        .join(" · ")
    : "";

  const submit = async () => {
    formErrors.clear();
    if (!draft.service_start_date || !draft.service_end_date) {
      formErrors.invalid(
        "Bitte geben Sie Beginn und Ende des Betreuungszeitraums an.",
        {
          ...(draft.service_start_date
            ? {}
            : { service_start_date: "Bitte wählen Sie den Beginn." }),
          ...(draft.service_end_date
            ? {}
            : { service_end_date: "Bitte wählen Sie das Ende." }),
        },
      );
      return;
    }
    if (!deadlineLocal) {
      formErrors.invalid("Bitte geben Sie eine Frist für die Eltern an.", {
        rollover_deadline: "Bitte wählen Sie eine Frist.",
      });
      return;
    }
    await save.run(async (op) => {
      try {
        const payload: RolloverInput = {
          ...draft,
          rollover_deadline: fromDateTimeLocalValue(deadlineLocal) ?? "",
        };
        const result = await createRollover(source.id, payload);
        onSuccess(result, op);
      } catch (err) {
        logger.error("rollover_create_failed", {
          error: err instanceof Error ? err.message : String(err),
          code: (err as { code?: unknown } | undefined)?.code,
        });
        if (!op.isCurrent()) return;
        await formErrors.show(err, {
          object: "die Anschlussphase",
          retry: () => void latestSubmit.current(),
        });
      }
    });
  };
  // „Wiederholen“ sendet den aktuellen Entwurf.
  const latestSubmit = useRef(submit);
  useLayoutEffect(() => {
    latestSubmit.current = submit;
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    void submit();
  };

  const title = `Anschlussphase für „${source.name}“ erstellen`;

  // Statuszeile des Seitenkopfs: Quelle und die Vorschau, die das Formular
  // ohnehin lädt.
  const statusLine = [
    `Quelle: ${source.name}`,
    // Ohne Vorschau keine Zahlen: ein Ladefehler ist keine „0“.
    preview ? `${preview.carried_count} werden übernommen` : null,
    preview ? `${preview.review_count} zu prüfen` : null,
  ]
    .filter(Boolean)
    .join(" · ");

  const form = (
    <SectionCard>
      <form onSubmit={handleSubmit} className="space-y-5">
        {variant === "embedded" ? (
          <header className="border-b border-gray-100 pb-4">
            <h2 className="text-base font-semibold text-gray-900">{title}</h2>
            <p className="mt-1 text-sm text-gray-600">{ROLLOVER_DESCRIPTION}</p>
          </header>
        ) : null}

        <FormErrorAlert message={formErrors.error} />

        {loadingPreview ? (
          <SectionCard title="Vorschau" icon={Check} loading />
        ) : previewLoad.error ? (
          <LoadErrorAlert error={previewLoad.error} />
        ) : preview ? (
          <SectionCard
            title="Vorschau"
            icon={Check}
            bodyClassName="compact:mt-3 mt-4 space-y-3"
          >
            <dl className="grid gap-3 sm:grid-cols-3">
              <DataField label="Werden übernommen">
                {preview.carried_count}
              </DataField>
              <DataField label="Manuell zu prüfen">
                {preview.review_count}
                {reviewDetails && (
                  <span className="mt-1 block text-xs font-normal text-gray-500">
                    {reviewDetails}
                  </span>
                )}
              </DataField>
              <DataField label="Nicht übernommen">
                {preview.excluded_count}
                {excludedDetails && (
                  <span className="mt-1 block text-xs font-normal text-gray-500">
                    {excludedDetails}
                  </span>
                )}
              </DataField>
            </dl>
            <p className="text-xs text-gray-500">
              Nur bestätigte Anmeldungen werden übernommen. Zurückgezogene,
              abgelehnte oder noch offene Anmeldungen werden nicht fortgeführt.
            </p>
          </SectionCard>
        ) : null}

        <div className="grid gap-4 md:grid-cols-2">
          <Input
            id="rollover-name"
            name="name"
            label="Name der neuen Phase"
            type="text"
            controlSize="compact"
            required
            value={draft.name}
            onChange={(e) => update("name", e.target.value)}
            error={formErrors.fieldError("name")}
          />
          <CustomSelect
            id="rollover-kind"
            label="Typ"
            value={draft.kind}
            onChange={(value) => update("kind", value as RolloverInput["kind"])}
            options={[
              { value: "school_year", label: "Schuljahr" },
              { value: "holiday", label: "Ferienbetreuung" },
              { value: "custom", label: "Sonstiges" },
            ]}
          />
        </div>

        <FormFieldset legend="Betreuungszeitraum">
          <div className="grid gap-3 sm:grid-cols-2">
            <ISODatePicker
              id="rollover-service-start"
              controlSize="md"
              label="Betreuung von"
              ariaLabel="Betreuung von"
              value={draft.service_start_date}
              onChange={(next) => update("service_start_date", next)}
              error={formErrors.fieldError("service_start_date")}
              calendarLayout="popover"
              // The required picker prevents deselection in the calendar; the
              // submit validation remains the safety net for programmatic edits.
              hideClearButton
              required
            />
            <ISODatePicker
              id="rollover-service-end"
              controlSize="md"
              label="Betreuung bis"
              ariaLabel="Betreuung bis"
              min={draft.service_start_date || undefined}
              value={draft.service_end_date}
              onChange={(next) => update("service_end_date", next)}
              error={formErrors.fieldError("service_end_date")}
              calendarLayout="popover"
              hideClearButton
              required
            />
          </div>
        </FormFieldset>

        <FormFieldset legend="Elternrückmeldung">
          <div className="grid gap-3 sm:grid-cols-2">
            <CustomSelect
              id="rollover-mode"
              label="Modus"
              hint={
                draft.rollover_mode === "opt_out"
                  ? "Anmeldungen werden automatisch übernommen. Ohne aktive Abmeldung bis zur Frist landet die Anmeldung in der Prüfung."
                  : "Eltern müssen aktiv bestätigen. Ohne Bestätigung bis zur Frist wird die Anmeldung zurückgezogen."
              }
              value={draft.rollover_mode}
              onChange={(value) =>
                update("rollover_mode", value as RolloverMode)
              }
              options={[
                {
                  value: "opt_out",
                  label: "Opt-Out: Eltern müssen abmelden",
                },
                {
                  value: "opt_in",
                  label: "Opt-In: Eltern müssen aktiv bestätigen",
                },
              ]}
            />
            <div>
              <FieldLabel htmlFor="rollover-deadline" required>
                Frist für die Eltern-Antwort
              </FieldLabel>
              <DateTimePicker
                id="rollover-deadline"
                dateAriaLabel="Frist für die Eltern-Antwort"
                timeAriaLabel="Frist Uhrzeit"
                value={deadlineLocal}
                onChange={setDeadlineLocal}
                invalid={Boolean(formErrors.fieldError("rollover_deadline"))}
                // A deadline without an explicit time should run to the end of the
                // chosen day, not expire at midnight.
                defaultTime="23:59"
                hideClearButton
                required
              />
            </div>
          </div>
        </FormFieldset>

        <div className="space-y-2">
          <CheckboxCard
            checked={draft.rollover_bumps_grade ?? true}
            onChange={(checked) => update("rollover_bumps_grade", checked)}
            label="Klassenstufe automatisch um 1 erhöhen"
            hint="Für jährliche Anmeldephasen aktivieren. Für Halbjahre oder Zeiträume innerhalb eines Schuljahres deaktivieren."
          />
          <CheckboxCard
            checked={draft.rollover_auto_approve}
            onChange={(checked) => update("rollover_auto_approve", checked)}
            label="Vorgemerkte Anmeldungen automatisch genehmigen"
            hint="Nur für Opt-Out sinnvoll. Nach Ablauf der Frist werden vorgemerkte Anmeldungen direkt bestätigt."
          />
        </div>

        <div className="flex justify-end gap-2 pt-2">
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={onCancel}
            disabled={submitting}
          >
            Abbrechen
          </Button>
          <Button
            type="submit"
            variant="primary"
            size="md"
            disabled={submitting}
          >
            {submitting ? "Wird erstellt…" : "Anschlussphase erstellen"}
          </Button>
        </div>
      </form>
    </SectionCard>
  );

  // "page": das Formular ist die Seite und trägt deshalb das Seitengerüst.
  // "embedded": es steht unter einem fremden Seitenkopf und bleibt Inhalt.
  if (variant === "page") {
    return (
      <TenantPage
        title={title}
        back
        backHref="/enrollment-phases"
        backLabel="Zurück zu den Anmeldephasen"
        stats={statusLine}
        statsLoading={loadingPreview}
        leading={<ConceptIconTile concept={enrollmentsConcept} size="page" />}
      >
        {form}
      </TenantPage>
    );
  }

  return form;
}

const PREVIEW_REVIEW_REASON_LABELS: Record<string, string> = {
  grade_above_max: "Klassenstufe über der Höchstgrenze",
  no_grade_level: "Keine Klassenstufe hinterlegt",
};

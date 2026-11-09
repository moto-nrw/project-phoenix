"use client";

// Die Dokumentart einer Leistung (#4688): Rechnung, Gebührenbescheid oder
// (nur Ferien) Bestätigung mit Rechnung. Die Wahl gilt ab der nächsten
// Freigabe; ausgestellte Belege behalten ihre Art.

import { useEffect, useId, useRef, useState } from "react";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { EditActions } from "~/components/ui/edit-actions";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Radio } from "~/components/ui/radio";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import type { MotoConcept } from "~/lib/moto-concepts";
import { createLogger } from "~/lib/logger";
import {
  BILLING_SERVICE_LABEL,
  DOCUMENT_KIND_LABEL,
  type DocumentKind,
  type DocumentKindOverview,
  type ServiceDocumentKind,
  saveDocumentKind,
} from "~/lib/traeger-document-kinds-api";

const logger = createLogger({ component: "DocumentKindCard" });

/** Was jede Art für die Familie bedeutet, in einem Satz. */
const KIND_HINT: Readonly<Record<DocumentKind, string>> = {
  confirmation:
    "Bestätigt die Anmeldung und nennt den Betrag. Mit Hinweisen und Notfallnummern.",
  invoice: "Nennt Leistung, Betrag und Zahlungsfrist.",
  fee_notice:
    "Setzt den Beitrag fest. Mit der Rechtsbehelfsbelehrung aus Ihrer Vorlage.",
};

const SERVICE_DESCRIPTION = {
  holiday: "Gilt für die Schreiben der Ferienläufe.",
  care: "Gilt für die Schreiben der Monatsläufe.",
} as const;

export function DocumentKindCard({
  service,
  concept,
  onSaved,
}: Readonly<{
  service: ServiceDocumentKind;
  concept: MotoConcept;
  onSaved: (overview: DocumentKindOverview) => void;
}>) {
  const [draft, setDraft] = useState<DocumentKind>(service.documentKind);
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLDivElement>(null);
  const formErrors = useApiFormError(formRef);
  const { success } = useToast();
  const name = useId();

  useEffect(() => setDraft(service.documentKind), [service.documentKind]);

  const changed = draft !== service.documentKind;
  const standard = service.choices[0];

  const save = async () => {
    formErrors.clear();
    setSaving(true);
    try {
      const overview = await saveDocumentKind(service.service, draft);
      success(
        `${BILLING_SERVICE_LABEL[service.service]}: ${DOCUMENT_KIND_LABEL[draft]} ist gespeichert.`,
      );
      onSaved(overview);
    } catch (err) {
      logger.error("traeger_document_kind_save_failed", {
        service: service.service,
        error: err instanceof Error ? err.message : String(err),
      });
      await formErrors.show(err, { object: "die Dokumentart" });
    } finally {
      setSaving(false);
    }
  };

  return (
    <SectionCard
      title={BILLING_SERVICE_LABEL[service.service]}
      description={SERVICE_DESCRIPTION[service.service]}
      concept={concept}
      footer={
        changed ? (
          <EditActions
            onCancel={() => setDraft(service.documentKind)}
            onSave={() => void save()}
            saving={saving}
            stretchOnPhone
          />
        ) : undefined
      }
    >
      <div ref={formRef} className="flex flex-col gap-3">
        <FormErrorAlert message={formErrors.error} />
        <div
          role="radiogroup"
          aria-label={`Dokumentart für ${BILLING_SERVICE_LABEL[service.service]}`}
          className="grid gap-2 sm:grid-cols-2 xl:grid-cols-3"
        >
          {service.choices.map((kind) => (
            <ChoiceTile
              key={kind}
              selected={draft === kind}
              disabled={saving}
              className="items-start gap-3 p-4"
            >
              <Radio
                name={name}
                value={kind}
                checked={draft === kind}
                disabled={saving}
                onChange={() => setDraft(kind)}
                className="mt-0.5"
              />
              <span className="flex min-w-0 flex-col gap-1">
                <span className="flex flex-wrap items-center gap-2">
                  <span className="font-semibold text-gray-900">
                    {DOCUMENT_KIND_LABEL[kind]}
                  </span>
                  {kind === standard ? (
                    <StatusBadge tone="gray" size="xs" label="Standard" />
                  ) : null}
                </span>
                <span className="text-sm text-gray-600">{KIND_HINT[kind]}</span>
              </span>
            </ChoiceTile>
          ))}
        </div>
      </div>
    </SectionCard>
  );
}

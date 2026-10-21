"use client";

import { useState } from "react";
import useSWR from "swr";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { useApiFormError } from "~/contexts/ToastContext";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { SectionCard } from "~/components/ui/section-card";
import {
  previewStaffPersonMerges,
  executeStaffPersonMerges,
} from "~/lib/operator/staff-person-merge-api";
import type { MergeSide } from "~/lib/operator/staff-person-merge-api";
import { formatDate } from "~/lib/date-helpers";

function PersonDetails({
  side,
  label,
}: Readonly<{ side: MergeSide; label: string }>) {
  return (
    <div>
      <p className="font-medium">
        {label}: {side.name}
      </p>
      <p>
        Person {side.personId} · erstellt am {formatDate(side.createdAt)}
      </p>
      <p>{side.accountId ? `Konto ${side.accountId}` : "Ohne Konto"}</p>
      <p>
        Beschäftigung {side.staffId} · angelegt am{" "}
        {formatDate(side.staffCreatedAt)}
      </p>
      <p>
        {side.retiredAt
          ? `Ausgetreten am ${formatDate(side.retiredAt)}`
          : "Aktuell beschäftigt"}
      </p>
      {side.birthday && <p>Geburtsdatum: {formatDate(side.birthday)}</p>}
    </div>
  );
}

export function StaffPersonMerge({
  schoolId,
  schoolName,
  onMerged,
}: Readonly<{
  schoolId: string;
  schoolName: string;
  onMerged: () => Promise<unknown>;
}>) {
  const { data, error, isLoading, mutate } = useSWR(
    `operator-staff-person-merges-${schoolId}`,
    () => previewStaffPersonMerges(schoolId),
    { revalidateOnFocus: false },
  );
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [confirmed, setConfirmed] = useState(false);
  const [saving, setSaving] = useState(false);
  const {
    error: failure,
    show: showFailure,
    clear: clearFailure,
  } = useApiFormError();
  const loadError = useSwrLoadError(
    error,
    "die Zusammenführungsvorschläge",
    () => mutate(),
  );
  const [success, setSuccess] = useState(false);
  async function merge() {
    if (!data || !confirmed || saving) return;
    const cases = data.filter((candidate) => selected.has(candidate.revision));
    if (cases.length === 0) return;
    setSaving(true);
    clearFailure();
    setSuccess(false);
    try {
      await executeStaffPersonMerges(schoolId, cases, confirmed);
      setSelected(new Set());
      setConfirmed(false);
      setSuccess(true);
      await Promise.all([mutate(), onMerged()]);
    } catch (error) {
      await showFailure(error, {
        object: "die Personen",
        messageSuffix:
          "Laden Sie die Vorschläge neu und prüfen Sie die Personenliste.",
      });
      setSelected(new Set());
      setConfirmed(false);
    } finally {
      setSaving(false);
    }
  }
  return (
    <SectionCard
      title="Doppelte Personaleinträge prüfen"
      description="Prüfen Sie mit der Schule, ob beide Einträge denselben Menschen meinen."
    >
      <p className="mb-3 text-sm">
        Der Eintrag mit Konto bleibt erhalten. Beide Beschäftigungen bleiben
        bestehen. Zeitkonten, Urlaub und Historie werden nicht zusammengelegt.
      </p>
      {isLoading && <p role="status">Vorschläge werden geladen.</p>}
      <LoadErrorAlert error={loadError} />
      {success && (
        <p role="status">Die ausgewählten Personen wurden zusammengeführt.</p>
      )}
      <FormErrorAlert message={failure} />
      <Button
        variant="outline"
        size="sm"
        disabled={saving || isLoading}
        onClick={() => {
          setSelected(new Set());
          setConfirmed(false);
          clearFailure();
          void mutate();
        }}
      >
        Vorschläge neu laden
      </Button>
      {data?.length === 0 && (
        <p className="mt-3">Keine passenden Doppelsätze gefunden.</p>
      )}
      {data && data.length > 0 && (
        <>
          <p className="my-3 text-sm">
            Namensgleichheit ist kein Nachweis. Wählen Sie jeden Fall erst nach
            Sichtprüfung aus. Unklare Fälle bleiben unverändert.
          </p>
          <ul className="space-y-4">
            {data.map((candidate) => (
              <li key={candidate.revision}>
                <div className="grid gap-3 text-sm sm:grid-cols-2">
                  <PersonDetails
                    side={candidate.keeper}
                    label="Bleibt erhalten"
                  />
                  <PersonDetails
                    side={candidate.donor}
                    label="Wird zusammengeführt"
                  />
                </div>
                <label className="mt-2 flex items-center gap-2 text-sm">
                  <Checkbox
                    checked={selected.has(candidate.revision)}
                    disabled={saving}
                    onChange={(event) =>
                      setSelected((current) => {
                        const next = new Set(current);
                        if (event.target.checked) next.add(candidate.revision);
                        else next.delete(candidate.revision);
                        return next;
                      })
                    }
                  />
                  Ich habe geprüft: Beide Einträge von {candidate.keeper.name}{" "}
                  gehören derselben Person.
                </label>
              </li>
            ))}
          </ul>
          <label className="my-4 flex items-center gap-2 text-sm">
            <Checkbox
              checked={confirmed}
              disabled={saving}
              onChange={(event) => setConfirmed(event.target.checked)}
            />
            Die Zusammenführung der ausgewählten Fälle ist mit {schoolName}{" "}
            abgestimmt.
          </label>
          <Button
            variant="danger"
            disabled={!confirmed || selected.size === 0 || saving || !!error}
            isLoading={saving}
            onClick={() => void merge()}
          >
            {selected.size} geprüfte Fälle zusammenführen
          </Button>
        </>
      )}
    </SectionCard>
  );
}

"use client";

import { useState } from "react";
import Link from "next/link";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Loading } from "~/components/ui/loading";
import { Modal } from "~/components/ui/modal";
import { useSWRAuth } from "~/lib/swr";
import { staffPayrollNumberService } from "~/lib/staff-api";

// Stammdaten tab (#1417 Tranche 2b): starts with the single Abrechnung
// section (Personalnummer). #1423 fills in the remaining Stammdaten sections
// around it — deliberately activated early so the payroll identifier has ONE
// home instead of a second maintenance surface.
export function StammdatenTab({
  staffId,
  canManagePayroll,
}: {
  readonly staffId: string;
  readonly canManagePayroll: boolean;
}) {
  const [editorOpen, setEditorOpen] = useState(false);

  const {
    data: personnelNumber,
    error,
    isLoading,
    isValidating,
    mutate,
  } = useSWRAuth(`staff-payroll-number-${staffId}`, () =>
    staffPayrollNumberService.get(staffId),
  );

  if (isLoading) {
    return <Loading fullPage={false} />;
  }

  if (error) {
    return (
      <div className="space-y-2">
        <Alert
          type="error"
          message="Die Personalnummer konnte nicht geladen werden."
        />
        <Button
          type="button"
          size="compact"
          variant="outline"
          isLoading={isValidating}
          loadingText="Wird geladen..."
          onClick={() => void mutate()}
        >
          Erneut laden
        </Button>
      </div>
    );
  }

  return (
    <div className="space-y-5">
      <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
        <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
          <div>
            <h3 className="text-sm font-semibold tracking-wide text-gray-400 uppercase">
              Abrechnung
            </h3>
            <p className="mt-1 text-xs text-gray-500">
              Personalnummer aus dem Lohnsystem des Trägers. Ohne sie kann der
              spätere DATEV-Export diese Person keiner Abrechnung zuordnen.
            </p>
          </div>
          {canManagePayroll && (
            <Button
              type="button"
              variant="ghost"
              size="compact"
              onClick={() => setEditorOpen(true)}
            >
              Bearbeiten
            </Button>
          )}
        </div>

        <dl className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div>
            <dt className="text-xs font-medium text-gray-500">
              Personalnummer
            </dt>
            <dd className="mt-1 text-sm font-semibold text-gray-800 tabular-nums">
              {personnelNumber ?? (
                <span className="font-normal text-gray-400">Nicht gesetzt</span>
              )}
            </dd>
          </div>
        </dl>

        <p className="mt-4 text-xs text-gray-500">
          Lohnarten und DATEV-Mandantendaten werden zentral unter{" "}
          <Link href="/payroll" className="text-[#5080D8] hover:underline">
            Abrechnung
          </Link>{" "}
          gepflegt.
        </p>
      </div>

      {editorOpen && (
        <PersonnelNumberModal
          staffId={staffId}
          current={personnelNumber ?? null}
          onClose={() => setEditorOpen(false)}
          onSaved={() => {
            setEditorOpen(false);
            void mutate();
          }}
        />
      )}
    </div>
  );
}

function PersonnelNumberModal({
  staffId,
  current,
  onClose,
  onSaved,
}: {
  readonly staffId: string;
  readonly current: string | null;
  readonly onClose: () => void;
  readonly onSaved: () => void;
}) {
  const [value, setValue] = useState(current ?? "");
  const [note, setNote] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const trimmed = value.trim();
  const valid = trimmed === "" || /^\d{1,9}$/.test(trimmed);

  const handleSave = async () => {
    setSaving(true);
    setError(null);
    try {
      await staffPayrollNumberService.update(
        staffId,
        trimmed === "" ? null : trimmed,
        note.trim(),
      );
      onSaved();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Speichern fehlgeschlagen");
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal isOpen onClose={onClose} title="Personalnummer bearbeiten">
      <div className="space-y-4">
        <div>
          <label
            htmlFor="personnel-number"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            Personalnummer
          </label>
          <Input
            id="personnel-number"
            value={value}
            onChange={(e) => setValue(e.target.value)}
            placeholder="z. B. 1023"
            inputMode="numeric"
          />
          <p className="mt-1 text-xs text-gray-500">
            Nur Ziffern. Leer lassen, um die Nummer zu entfernen. Die Nummer
            muss der Personalnummer im Lohnsystem entsprechen und ist pro Schule
            eindeutig.
          </p>
          {!valid && (
            <p className="mt-1 text-xs text-[#FF3130]">
              Nur Ziffern, höchstens 9 Stellen.
            </p>
          )}
        </div>

        <div>
          <label
            htmlFor="personnel-number-note"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            Begründung (optional)
          </label>
          <Input
            id="personnel-number-note"
            value={note}
            onChange={(e) => setNote(e.target.value)}
            placeholder="Erscheint im Änderungsprotokoll"
          />
        </div>

        {error && <p className="text-sm text-[#FF3130]">{error}</p>}

        <div className="flex justify-end gap-2">
          <Button type="button" variant="outline" size="md" onClick={onClose}>
            Abbrechen
          </Button>
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={handleSave}
            disabled={saving || !valid || trimmed === (current ?? "").trim()}
          >
            {saving ? "Speichert…" : "Speichern"}
          </Button>
        </div>
      </div>
    </Modal>
  );
}

"use client";

// Ordner anlegen / bearbeiten (#2596). One name, one visibility. The share
// list only appears for "Ausgewählte Rollen und Personen" — the other two
// modes need nothing else, so nothing else is shown.

import { useEffect, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { MultiCheckboxSelect } from "~/components/ui/multi-checkbox-select";
import { SegmentedControl } from "~/components/ui/segmented-control";
import {
  filesService,
  FOLDER_VISIBILITY_LABELS,
  type AudienceOptions,
  type FileFolder,
  type FolderVisibility,
} from "~/lib/files-api";
import { createLogger } from "~/lib/logger";
import { useSWRAuth } from "~/lib/swr";

const logger = createLogger({ component: "FolderModal" });

const VISIBILITY_ITEMS = [
  { value: "all_staff", label: "Alle Mitarbeitenden" },
  { value: "admins", label: "Nur Leitung" },
  { value: "selected", label: "Ausgewählt" },
] as const satisfies readonly { value: FolderVisibility; label: string }[];

const VISIBILITY_HINTS: Record<FolderVisibility, string> = {
  all_staff:
    "Jede Person mit Zugang zu moto an dieser Schule sieht den Ordner.",
  admins: "Nur Konten mit Leitungsrechten sehen den Ordner.",
  selected:
    "Nur die gewählten Rollen und Personen sehen den Ordner. Die Leitung sieht ihn immer.",
};

export function FolderModal({
  isOpen,
  onClose,
  onSaved,
  initial,
}: {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onSaved: () => void;
  readonly initial: FileFolder | null;
}) {
  const [name, setName] = useState("");
  const [visibility, setVisibility] = useState<FolderVisibility>("all_staff");
  const [roleIds, setRoleIds] = useState<string[]>([]);
  const [accountIds, setAccountIds] = useState<string[]>([]);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const { data: audience } = useSWRAuth<AudienceOptions>(
    isOpen ? "files-audience" : null,
    () => filesService.listAudience(),
  );

  useEffect(() => {
    if (!isOpen) return;
    setName(initial?.name ?? "");
    setVisibility(initial?.visibility ?? "all_staff");
    setRoleIds(initial?.roleIds ?? []);
    setAccountIds(initial?.accountIds ?? []);
    setError(null);
  }, [isOpen, initial]);

  const validate = (): string | null => {
    if (!name.trim()) return "Bitte einen Namen für den Ordner eingeben.";
    if (
      visibility === "selected" &&
      roleIds.length === 0 &&
      accountIds.length === 0
    ) {
      return "Bitte mindestens eine Rolle oder Person auswählen.";
    }
    return null;
  };

  const handleSave = async () => {
    const validationError = validate();
    if (validationError) {
      setError(validationError);
      return;
    }
    setSaving(true);
    setError(null);
    const input = {
      name: name.trim(),
      visibility,
      roleIds: visibility === "selected" ? roleIds : [],
      accountIds: visibility === "selected" ? accountIds : [],
    };
    try {
      if (initial) {
        await filesService.updateFolder(initial.id, input);
      } else {
        await filesService.createFolder(input);
      }
      onSaved();
      onClose();
    } catch (err) {
      logger.error("folder_save_failed", {
        folder_id: initial?.id,
        error: err instanceof Error ? err.message : String(err),
      });
      setError(
        err instanceof Error
          ? err.message
          : "Ordner konnte nicht gespeichert werden.",
      );
    } finally {
      setSaving(false);
    }
  };

  const roleOptions = (audience?.roles ?? []).map((role) => ({
    value: role.id,
    label: role.name,
  }));
  const accountOptions = (audience?.accounts ?? []).map((account) => ({
    value: account.accountId,
    label: `${account.lastName}, ${account.firstName}`,
  }));

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title={initial ? "Ordner bearbeiten" : "Neuer Ordner"}
      size="md"
      closeDisabled={saving}
      footer={
        <div className="flex justify-end gap-2">
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={onClose}
            disabled={saving}
          >
            Abbrechen
          </Button>
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={() => void handleSave()}
            disabled={saving}
          >
            {saving ? "Wird gespeichert…" : "Speichern"}
          </Button>
        </div>
      }
    >
      <div className="space-y-5">
        {error && <Alert type="error" message={error} />}

        <div className="space-y-1.5">
          <label
            htmlFor="ordner-name"
            className="text-xs font-medium text-gray-600"
          >
            Name
          </label>
          <Input
            id="ordner-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="z. B. Konzeption, Formulare, Notfallpläne"
            maxLength={120}
          />
        </div>

        <div className="space-y-1.5">
          <p className="text-xs font-medium text-gray-600">
            Wer sieht den Ordner?
          </p>
          <SegmentedControl
            variant="joined"
            fullWidth
            items={VISIBILITY_ITEMS}
            value={visibility}
            onChange={setVisibility}
            ariaLabel="Sichtbarkeit des Ordners"
          />
          <p className="text-xs leading-5 text-gray-500">
            {VISIBILITY_HINTS[visibility]}
          </p>
        </div>

        {visibility === "selected" && (
          <div className="space-y-4 rounded-xl bg-gray-50 p-4">
            <div className="space-y-1.5">
              <label
                htmlFor="ordner-rollen"
                className="text-xs font-medium text-gray-600"
              >
                Rollen
              </label>
              <MultiCheckboxSelect
                id="ordner-rollen"
                value={roleIds}
                options={roleOptions}
                onChange={setRoleIds}
                emptyLabel="Keine Rolle gewählt"
                unavailableLabel="Keine Rollen vorhanden"
                multipleLabel={(count) => `${count} Rollen`}
                searchable
                searchPlaceholder="Rolle suchen"
                className="w-full"
              />
            </div>
            <div className="space-y-1.5">
              <label
                htmlFor="ordner-personen"
                className="text-xs font-medium text-gray-600"
              >
                Personen
              </label>
              <MultiCheckboxSelect
                id="ordner-personen"
                value={accountIds}
                options={accountOptions}
                onChange={setAccountIds}
                emptyLabel="Keine Person gewählt"
                unavailableLabel="Keine Personen mit Zugang"
                multipleLabel={(count) => `${count} Personen`}
                searchable
                searchPlaceholder="Person suchen"
                className="w-full"
              />
            </div>
            <p className="text-xs leading-5 text-gray-500">
              Eine Person sieht den Ordner, wenn sie selbst gewählt ist oder
              eine der gewählten Rollen hat.
            </p>
          </div>
        )}

        <p className="text-xs leading-5 text-gray-500">
          Sichtbarkeit: {FOLDER_VISIBILITY_LABELS[visibility]}. Alle Dateien im
          Ordner übernehmen diese Einstellung.
        </p>
      </div>
    </FormModal>
  );
}

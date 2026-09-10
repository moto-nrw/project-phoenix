"use client";

import { Button } from "~/components/ui/button";
import { cn } from "~/lib/utils";

/**
 * Der Fuß eines Bearbeiten-Zustands (Bauart 2, Regel 4): genau ein
 * „Speichern“ und ein „Abbrechen“, unten, in dieser Reihenfolge. Jede Fläche,
 * die am Objekt bearbeitet (Stammdaten, Klassen-Zuweisung, Zahlungskonto),
 * schließt mit diesem Bauteil ab, damit das Speichern überall gleich aussieht
 * und gleich heißt. Ohne Bearbeiten-Zustand gibt es außerhalb der
 * Einstellungen kein Speichern, auch kein automatisches (#3112).
 */
export function EditActions({
  onCancel,
  onSave,
  saving = false,
  disabled = false,
  saveLabel = "Speichern",
  savingLabel = "Speichert…",
  cancelLabel = "Abbrechen",
  saveType,
  className,
}: Readonly<{
  onCancel: () => void;
  /** Ohne `onSave` ist der Speichern-Knopf ein Submit des umgebenden `<form>`. */
  onSave?: () => void;
  saving?: boolean;
  /** Der Entwurf ist unvollständig oder ungültig; Abbrechen bleibt möglich. */
  disabled?: boolean;
  saveLabel?: string;
  savingLabel?: string;
  cancelLabel?: string;
  saveType?: "button" | "submit";
  className?: string;
}>) {
  return (
    <div className={cn("flex flex-wrap justify-end gap-2", className)}>
      <Button
        type="button"
        variant="outline"
        size="md"
        onClick={onCancel}
        disabled={saving}
      >
        {cancelLabel}
      </Button>
      <Button
        type={saveType ?? (onSave ? "button" : "submit")}
        variant="primary"
        size="md"
        onClick={onSave}
        disabled={saving || disabled}
      >
        {saving ? savingLabel : saveLabel}
      </Button>
    </div>
  );
}

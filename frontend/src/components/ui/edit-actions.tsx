"use client";

import { Button } from "~/components/ui/button";
import { cn } from "~/lib/utils";
import { useModalDismissDisabled } from "./modal-frame";

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
  cancelDisabled = false,
  saveLabel = "Speichern",
  savingLabel = "Speichert…",
  cancelLabel = "Abbrechen",
  saveType,
  stretchOnPhone = false,
  className,
}: Readonly<{
  onCancel: () => void;
  /** Ohne `onSave` ist der Speichern-Knopf ein Submit des umgebenden `<form>`. */
  onSave?: () => void;
  saving?: boolean;
  /** Der Entwurf ist unvollständig oder ungültig; Abbrechen bleibt möglich. */
  disabled?: boolean;
  /** Eine laufende Operation muss vor dem Verwerfen abgeschlossen sein. */
  cancelDisabled?: boolean;
  saveLabel?: string;
  savingLabel?: string;
  cancelLabel?: string;
  saveType?: "button" | "submit";
  /**
   * Unter `sm` teilen sich beide Knöpfe die volle Breite, damit ihre Kanten
   * mit einem vollbreiten Knopf darüber oder darunter fluchten (#5758).
   */
  stretchOnPhone?: boolean;
  className?: string;
}>) {
  const isDismissDisabled = useModalDismissDisabled();
  const buttonWidth = stretchOnPhone ? "flex-1 sm:flex-none" : undefined;
  return (
    <div
      className={cn(
        "flex flex-wrap justify-end gap-2",
        stretchOnPhone && "w-full flex-nowrap sm:w-auto sm:flex-wrap",
        className,
      )}
    >
      <Button
        type="button"
        variant="outline"
        size="md"
        onClick={onCancel}
        disabled={saving || cancelDisabled || isDismissDisabled}
        className={buttonWidth}
      >
        {cancelLabel}
      </Button>
      <Button
        type={saveType ?? (onSave ? "button" : "submit")}
        variant="primary"
        size="md"
        onClick={onSave}
        disabled={saving || disabled}
        className={buttonWidth}
      >
        {saving ? savingLabel : saveLabel}
      </Button>
    </div>
  );
}

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
  cancelDisabled = false,
  saveLabel = "Speichern",
  saveTooltip,
  disabledReason,
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
  /**
   * Warum Speichern gerade gesperrt ist, solange `disabled` gilt: was fehlt
   * oder zuerst geschehen muss („Ändern Sie zuerst ein Feld.“). Ohne Standard:
   * Gesperrt ist es mal wegen fehlender Pflichtfelder, mal ohne Änderung.
   */
  disabledReason?: string;
  /** Eine laufende Operation muss vor dem Verwerfen abgeschlossen sein. */
  cancelDisabled?: boolean;
  saveLabel?: string;
  /**
   * Was Speichern außer Speichern tut (#6744), z. B. „Speichert die Zeiten
   * und benachrichtigt die Eltern.“ Ohne Angabe keine Blase: ein reines
   * „Speichern“ sagt schon alles.
   */
  saveTooltip?: string;
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
        disabled={saving || cancelDisabled}
        className={buttonWidth}
        noTooltip="Abbrechen verwirft nur den Entwurf, gespeichert bleibt der alte Stand."
      >
        {cancelLabel}
      </Button>
      <Button
        type={saveType ?? (onSave ? "button" : "submit")}
        variant="primary"
        size="md"
        onClick={onSave}
        disabled={saving || disabled}
        // Während des Speicherns bleibt die native Sperre (#5608).
        disabledReason={saving ? undefined : disabledReason}
        // Ohne `saveTooltip` reicht das Wort: ein reines Speichern.
        tooltip={saveTooltip}
        className={buttonWidth}
      >
        {saving ? savingLabel : saveLabel}
      </Button>
    </div>
  );
}

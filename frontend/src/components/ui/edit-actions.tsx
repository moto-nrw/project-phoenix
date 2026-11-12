"use client";

import { Button } from "~/components/ui/button";
import { cn } from "~/lib/utils";

/**
 * Der Fuß eines Bearbeiten-Zustands (Bauart 2, Regel 4): genau ein
 * „Speichern“ und ein „Abbrechen“, unten, in dieser Reihenfolge. Eine Fläche
 * darf daneben eine zweite Art zu speichern anbieten (`alsoSave`), etwa
 * „Speichern und Team informieren“ (#6854); „Speichern“ bleibt der
 * Hauptknopf und tut weiterhin nur das. Jede Fläche,
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
  alsoSave,
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
  /**
   * Eine zweite Art zu speichern, zwischen Abbrechen und Speichern. Sie sagt
   * im `tooltip`, was sie zusätzlich tut, und im `disabledReason`, warum sie
   * gerade nicht geht.
   */
  alsoSave?: {
    label: string;
    onClick: () => void;
    tooltip: string;
    disabledReason?: string;
  };
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
      >
        {cancelLabel}
      </Button>
      {alsoSave ? (
        <Button
          type="button"
          variant="outline"
          size="md"
          onClick={alsoSave.onClick}
          disabled={saving || disabled || alsoSave.disabledReason !== undefined}
          disabledReason={
            saving || disabled ? undefined : alsoSave.disabledReason
          }
          tooltip={alsoSave.tooltip}
          className={buttonWidth}
        >
          {alsoSave.label}
        </Button>
      ) : null}
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

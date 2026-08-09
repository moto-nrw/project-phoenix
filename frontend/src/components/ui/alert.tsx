"use client";

import type { ReactNode } from "react";

type AlertType = "error" | "success" | "warning" | "info";

interface AlertProps {
  readonly type: AlertType;
  readonly message: string;
  readonly announce?: "assertive" | "polite" | "off";
  /**
   * Optionale Aktion am rechten Rand (Link oder Button), für Hinweise, die
   * einen konkreten nächsten Schritt anbieten. Bleibt ein Geschwister-Element
   * der Meldung, damit die Meldung selbst direktes Kind der getönten Fläche
   * bleibt.
   */
  readonly action?: ReactNode;
}

export function Alert({
  type,
  message,
  announce,
  action,
}: Readonly<AlertProps>) {
  if (!message) return null;
  const isAssertive = type === "error" || type === "warning";
  const announcement = announce ?? (isAssertive ? "assertive" : "polite");

  // Brand hexes from LOCATION_COLORS (red DANGER, orange SCHOOLYARD, blue
  // OTHER_ROOM), with darker foregrounds for WCAG AA on the tinted surfaces.
  const styles = {
    error: "bg-moto-red-soft text-moto-red-strong border-moto-red/20",
    success: "bg-moto-green-soft text-moto-green-strong border-moto-green/20",
    warning:
      "bg-moto-orange-soft text-moto-orange-strong border-moto-orange/20",
    info: "bg-moto-blue-soft text-moto-blue-strong border-moto-blue/20",
  };

  // Improved styling with icon indicators
  const icons = {
    error: (
      <svg
        className="mr-2 h-5 w-5 flex-shrink-0"
        fill="currentColor"
        viewBox="0 0 20 20"
        xmlns="http://www.w3.org/2000/svg"
        aria-hidden="true"
      >
        <path
          fillRule="evenodd"
          d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z"
          clipRule="evenodd"
        />
      </svg>
    ),
    success: (
      <svg
        className="mr-2 h-5 w-5 flex-shrink-0"
        fill="currentColor"
        viewBox="0 0 20 20"
        xmlns="http://www.w3.org/2000/svg"
        aria-hidden="true"
      >
        <path
          fillRule="evenodd"
          d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
          clipRule="evenodd"
        />
      </svg>
    ),
    warning: (
      <svg
        className="mr-2 h-5 w-5 flex-shrink-0"
        fill="currentColor"
        viewBox="0 0 20 20"
        xmlns="http://www.w3.org/2000/svg"
        aria-hidden="true"
      >
        <path
          fillRule="evenodd"
          d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z"
          clipRule="evenodd"
        />
      </svg>
    ),
    info: (
      <svg
        className="mr-2 h-5 w-5 flex-shrink-0"
        fill="currentColor"
        viewBox="0 0 20 20"
        xmlns="http://www.w3.org/2000/svg"
        aria-hidden="true"
      >
        <path
          fillRule="evenodd"
          d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z"
          clipRule="evenodd"
        />
      </svg>
    ),
  };

  return (
    <div
      role={
        announcement === "off"
          ? undefined
          : announcement === "assertive"
            ? "alert"
            : "status"
      }
      className={`flex items-center rounded-lg border p-4 text-sm shadow-sm transition-[box-shadow,opacity] duration-200 hover:opacity-95 hover:shadow-md ${action ? "flex-wrap gap-y-2" : ""} ${styles[type]}`}
    >
      {icons[type]}
      {/* Mit Aktion darf die Meldung schrumpfen (min-w-0 flex-1), sonst würde
          sie beim Umbruch komplett unter das Icon rutschen. */}
      <span className={action ? "min-w-0 flex-1" : undefined}>{message}</span>
      {/* basis-full: die Aktion rutscht auf schmalen Bildschirmen unter die
          Meldung, statt den Text in eine schmale Spalte zu quetschen. Ab sm
          steht sie wieder rechts in derselben Zeile. */}
      {action ? (
        <span className="shrink-0 basis-full sm:ml-auto sm:basis-auto sm:pl-4">
          {action}
        </span>
      ) : null}
    </div>
  );
}

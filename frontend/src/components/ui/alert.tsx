"use client";

import type { CSSProperties, HTMLAttributes, ReactNode } from "react";

import { Button } from "~/components/ui/button";
import { CloseIcon } from "~/components/ui/close-icon";
import { TINT_CLASSES, getPillTone } from "~/components/ui/kit-palette";

/** Tinted surface of each type: the palette tint, with its strong text for
 *  WCAG AA on the soft surface. */
function tintClasses(tint: (typeof TINT_CLASSES)[keyof typeof TINT_CLASSES]) {
  return `${tint.surface} ${tint.text} ${tint.border}`;
}

const ALERT_TYPE_CLASSES = {
  error: tintClasses(TINT_CLASSES.red),
  success: tintClasses(TINT_CLASSES.green),
  warning: tintClasses(TINT_CLASSES.orange),
  info: tintClasses(TINT_CLASSES.blue),
};

export type AlertType = "error" | "success" | "warning" | "info";

interface AlertProps extends Omit<HTMLAttributes<HTMLDivElement>, "color"> {
  readonly type: AlertType;
  readonly title?: string;
  /** Text der Meldung; darf Hervorhebungen oder eine Aufzählung enthalten. */
  readonly message: ReactNode;
  readonly announce?: "assertive" | "polite" | "off";
  /**
   * Optionale Aktion am rechten Rand (Link oder Button), für Hinweise, die
   * einen konkreten nächsten Schritt anbieten. Bleibt ein Geschwister-Element
   * der Meldung, damit die Meldung selbst direktes Kind der getönten Fläche
   * bleibt.
   */
  readonly action?: ReactNode;
  /**
   * `inline` keeps compact controls on the same row, for example inside a
   * toast. `stacked` puts the actions on their own row under the message,
   * aligned with its text: for several actions or a long one such as a
   * request ID, which would otherwise wrap ragged at the right edge.
   */
  readonly actionLayout?: "responsive" | "inline" | "stacked";
  /**
   * Weiterer Inhalt unter Titel und Meldung, etwa eine kurze Aufzählung.
   * Wird nur mit `title` gezeigt; das Icon steht dann oben statt mittig.
   */
  readonly details?: ReactNode;
  /**
   * Eigenes Symbol statt des Symbols zum Typ, etwa das Konzept-Symbol
   * (Eltern, Schulhof), wenn der Hinweis von einem Fachbegriff handelt.
   */
  readonly icon?: ReactNode;
  /**
   * `compact` für Hinweise in Feldern und dichten Formularen: kleinere
   * Schrift, weniger Innenabstand, kleineres Symbol.
   */
  readonly size?: "default" | "compact";
  /**
   * Hex aus `LOCATION_COLORS`, wenn der Hinweis einen Standort oder Status
   * trägt (Kommt heute nicht, Klassenfahrt). Tönt Fläche und Text wie
   * `getPillTone`; `type` bestimmt weiter Symbol und Ansage.
   */
  readonly color?: string;
  /** Zeigt oben rechts ein Schließen-X; der Aufrufer blendet den Hinweis aus. */
  readonly onDismiss?: () => void;
  /** Name des Schließen-X für Screenreader. */
  readonly dismissLabel?: string;
  /** Zusätzliche Klassen für den Rahmen, z. B. Außenabstände. */
  readonly className?: string;
}

export function Alert({
  type,
  title,
  message,
  announce,
  action,
  actionLayout = "responsive",
  details,
  icon,
  size = "default",
  color,
  onDismiss,
  dismissLabel = "Hinweis schließen",
  className = "",
  style,
  ...props
}: Readonly<AlertProps>) {
  if (message === null || message === undefined || message === "") return null;
  const compact = size === "compact";
  const isAssertive = type === "error" || type === "warning";
  const announcement = announce ?? (isAssertive ? "assertive" : "polite");

  // Standort- oder Statusfarbe: dieselbe Tönung wie die Pille dieser Farbe,
  // der Rand ist der Grundton mit 20 % Deckkraft wie bei den Typen oben.
  const tone = color ? getPillTone(color) : null;
  const toneStyle: CSSProperties | undefined = tone
    ? {
        backgroundColor: tone.backgroundColor,
        color: tone.textColor,
        borderColor: `color-mix(in srgb, ${color} 20%, transparent)`,
        ...style,
      }
    : style;
  const iconClass = compact
    ? "mr-2 h-4 w-4 flex-shrink-0"
    : "mr-2 h-5 w-5 flex-shrink-0";

  // Improved styling with icon indicators
  const icons = {
    error: (
      <svg
        className={iconClass}
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
        className={iconClass}
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
        className={iconClass}
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
        className={iconClass}
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

  const leading = icon ? (
    <span
      className={`${compact ? "mr-2" : "mr-2.5"} flex shrink-0 items-center`}
      aria-hidden="true"
    >
      {icon}
    </span>
  ) : (
    icons[type]
  );
  const padding = compact ? "px-3 py-2 text-xs" : "p-4 text-sm";

  return (
    <div
      {...props}
      style={toneStyle}
      role={
        announcement === "off"
          ? undefined
          : announcement === "assertive"
            ? "alert"
            : "status"
      }
      aria-live={announcement === "off" ? undefined : announcement}
      aria-atomic={announcement === "off" ? undefined : "true"}
      className={`flex ${(title && details) || onDismiss ? "items-start" : "items-center"} rounded-xl border shadow-sm ${padding} ${action ? "flex-wrap gap-y-2" : ""} ${onDismiss ? "relative pr-12" : ""} ${tone ? "" : ALERT_TYPE_CLASSES[type]} ${className}`}
    >
      {leading}
      {/* Mit Aktion darf die Meldung schrumpfen (min-w-0 flex-1), sonst würde
          sie beim Umbruch komplett unter das Icon rutschen. */}
      {title ? (
        <div className={action ? "min-w-0 flex-1" : undefined}>
          <p className="font-semibold">{title}</p>
          <div className="mt-0.5">{message}</div>
          {details}
        </div>
      ) : (
        <div className={action ? "min-w-0 flex-1" : undefined}>{message}</div>
      )}
      {/* basis-full: die Aktion rutscht auf schmalen Bildschirmen unter die
          Meldung, statt den Text in eine schmale Spalte zu quetschen. Ab sm
          steht sie wieder rechts in derselben Zeile. max-w-full hält eine
          lange Aktion (Vorgangskennung) in der Fläche, statt sie über den
          Rand laufen zu lassen. */}
      {action ? (
        <div
          className={
            actionLayout === "inline"
              ? "ml-auto shrink-0 pl-2"
              : actionLayout === "stacked"
                ? // pl-7 = icon (h-5 w-5) plus its mr-2: flush with the text.
                  `max-w-full min-w-0 basis-full ${compact ? "pl-6" : "pl-7"}`
                : "max-w-full min-w-0 shrink-0 basis-full sm:ml-auto sm:basis-auto sm:pl-4"
          }
        >
          {action}
        </div>
      ) : null}
      {/* Wie im Toast: das X sitzt oben rechts in der Fläche und nimmt die
          Textfarbe des Hinweises an (#5591). */}
      {onDismiss ? (
        <Button
          type="button"
          variant="ghost_tone"
          size="icon"
          aria-label={dismissLabel}
          onClick={onDismiss}
          className="absolute top-2.5 right-2.5"
        >
          <CloseIcon />
        </Button>
      ) : null}
    </div>
  );
}

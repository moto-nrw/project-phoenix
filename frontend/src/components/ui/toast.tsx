"use client";

import { X } from "lucide-react";
import type { MouseEventHandler } from "react";
import { Alert, type AlertType } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { RequestIdButton } from "~/components/ui/request-id-button";
import { cn } from "~/lib/utils";

interface ToastAction {
  readonly label: string;
  readonly accessibleLabel: string;
  readonly onClick: () => void;
}

interface ToastProps {
  readonly type: AlertType;
  readonly message: string;
  readonly accessibleLabel: string;
  readonly closeLabel: string;
  readonly onClose: () => void;
  readonly action?: ToastAction;
  readonly requestId?: string;
  readonly requestIdLabel?: string;
  readonly copyRequestIdLabel?: string;
  readonly copySucceededLabel: string;
  readonly copyFailedLabel: string;
  readonly visible: boolean;
  readonly reducedMotion: boolean;
  readonly touchFriendly: boolean;
  readonly onMouseEnter?: MouseEventHandler<HTMLDivElement>;
  readonly onMouseLeave?: MouseEventHandler<HTMLDivElement>;
}

/**
 * Shared transient-feedback surface. Placement, stacking, and lifetime belong
 * to the provider; this component owns the app's visual and accessible toast.
 */
export function Toast({
  type,
  message,
  accessibleLabel,
  closeLabel,
  onClose,
  action,
  requestId,
  requestIdLabel,
  copyRequestIdLabel,
  copySucceededLabel,
  copyFailedLabel,
  visible,
  reducedMotion,
  touchFriendly,
  onMouseEnter,
  onMouseLeave,
}: Readonly<ToastProps>) {
  // Wiederholen und Vorgangskennung stehen unter der Meldung, bündig mit
  // ihrem Text (wie im Fehlerkasten); -ml-2.5 nimmt das Polster des ersten
  // Knopfs zurück. Das Schließen-X gehört nicht in diese Reihe: in der
  // schmalen Toast-Breite brach es sonst allein in eine eigene Zeile um.
  const actions =
    action || (requestId && copyRequestIdLabel) ? (
      <span className="-ml-2.5 flex flex-wrap items-center gap-x-1">
        {action ? (
          <Button
            type="button"
            variant="ghost"
            size="compact"
            aria-label={action.accessibleLabel}
            onClick={action.onClick}
            className={cn(
              "shrink-0 self-center text-current underline underline-offset-2 hover:bg-black/5 hover:text-current",
              touchFriendly ? "h-11 px-3 text-sm" : "",
            )}
          >
            {action.label}
          </Button>
        ) : null}
        {requestId && copyRequestIdLabel ? (
          <RequestIdButton
            requestId={requestId}
            label={requestIdLabel}
            copyLabel={copyRequestIdLabel}
            copiedLabel={copySucceededLabel}
            copyFailedLabel={copyFailedLabel}
          />
        ) : null}
      </span>
    ) : undefined;

  // Das X sitzt in der Toast-Fläche (es gehört zur Meldung), aber absolut
  // oben rechts auf Höhe der ersten Textzeile: p-4 plus halbe Zeilenhöhe
  // minus halbe Knopfhöhe. Die Alert-Fläche ist dafür `relative`.
  const close = (
    <Button
      type="button"
      variant="ghost"
      size="icon"
      aria-label={closeLabel}
      onClick={onClose}
      className={cn(
        "absolute shrink-0 text-current hover:bg-black/5 hover:text-current",
        touchFriendly ? "top-1 right-1 h-11 w-11" : "top-2.5 right-2.5",
      )}
    >
      <X className="h-4 w-4" aria-hidden="true" />
    </Button>
  );

  return (
    <div
      onMouseEnter={onMouseEnter}
      onMouseLeave={onMouseLeave}
      className={cn(
        "pointer-events-auto w-full transition-[opacity,transform]",
        reducedMotion ? "" : "duration-300 ease-out",
        visible ? "translate-y-0 opacity-100" : "translate-y-2 opacity-0",
      )}
    >
      <Alert
        type={type}
        message={message}
        announce={type === "error" ? "assertive" : "polite"}
        aria-label={accessibleLabel}
        action={
          <>
            {actions}
            {close}
          </>
        }
        // Ohne Aktionen bleibt keine leere Reihe unter der Meldung stehen.
        actionLayout={actions ? "stacked" : "inline"}
        // Rechts Platz für das Schließen-X oben in der Ecke.
        className={cn(
          "relative rounded-xl shadow-lg",
          touchFriendly ? "pr-14" : "pr-12",
        )}
      />
    </div>
  );
}

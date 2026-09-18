"use client";

import { X } from "lucide-react";
import type { MouseEventHandler } from "react";
import { Alert, type AlertType } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
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
  visible,
  reducedMotion,
  touchFriendly,
  onMouseEnter,
  onMouseLeave,
}: Readonly<ToastProps>) {
  const controls = (
    <span className="flex items-center gap-1">
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
      <Button
        type="button"
        variant="ghost"
        size="icon"
        aria-label={closeLabel}
        onClick={onClose}
        className={cn(
          "shrink-0 text-current hover:bg-black/5 hover:text-current",
          touchFriendly ? "h-11" : "",
        )}
      >
        <X className="h-4 w-4" aria-hidden="true" />
      </Button>
    </span>
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
        announce="polite"
        aria-label={accessibleLabel}
        action={controls}
        actionLayout="inline"
        className="rounded-xl shadow-lg"
      />
    </div>
  );
}

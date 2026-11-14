"use client";

import { useId, type ReactNode, type RefObject } from "react";
import { Button } from "~/components/ui/button";
import type { LoginScope } from "~/lib/mfa-api";
import { OTPInputGrid, type OTPInputGridHandle } from "./otp-input-grid";

export const MFA_CODE_LENGTH = 6;

/**
 * Where each portal lists the remembered devices (#6955): the page the
 * person opens from the menu at the top right.
 */
const DEVICE_LIST_PLACE: Record<LoginScope, string> = {
  tenant: "Profil",
  school: "Einstellungen",
  caterer: "Konto",
  traeger: "Konto",
  operator: "Profil",
};

/** „90 Tage“, „1 Tag“. */
function trustedDeviceDuration(days: number): string {
  return days === 1 ? "1 Tag" : `${days} Tage`;
}

/** The two choices of the code step; only the second remembers the device. */
export type CodeConfirmChoice = "once" | "remember";

interface MFACodeConfirmProps {
  readonly scope: LoginScope;
  readonly otpRef: RefObject<OTPInputGridHandle | null>;
  /** The code the person typed so far; the parent clears it on a reset. */
  readonly code: string;
  readonly onCodeChange: (code: string) => void;
  /** The choice being checked, or null while nothing is sent. */
  readonly pending: CodeConfirmChoice | null;
  /**
   * Whether this portal may remember the device here. False hides the
   * second button: the portal or the school does not remember devices, or
   * its setting could not be read.
   */
  readonly trustedDeviceEnabled: boolean;
  readonly trustedDeviceDays: number;
  /** Called with a complete code and the person's choice. */
  readonly onConfirm: (code: string, choice: CodeConfirmChoice) => void;
  /** Called when a button is pressed before all digits are typed. */
  readonly onIncomplete: () => void;
  /** Shown under the buttons, e.g. „Neuen Code anfordern“. */
  readonly children?: ReactNode;
}

/**
 * The code step every portal with a second factor shows (#6955): the code,
 * then an explicit choice. „Nur dieses Mal bestätigen“ is the form's submit
 * button, so Enter in the code field never remembers the device; remembering
 * takes the second button and is never preselected. Shared tablets in a room
 * must not turn into remembered devices by accident.
 */
export function MFACodeConfirm({
  scope,
  otpRef,
  code,
  onCodeChange,
  pending,
  trustedDeviceEnabled,
  trustedDeviceDays,
  onConfirm,
  onIncomplete,
  children,
}: MFACodeConfirmProps) {
  const hintId = useId();
  const offerRemember = trustedDeviceEnabled && trustedDeviceDays > 0;
  const busy = pending !== null;

  const submit = (choice: CodeConfirmChoice) => {
    if (busy) return;
    if (code.length !== MFA_CODE_LENGTH) {
      onIncomplete();
      otpRef.current?.focus();
      return;
    }
    onConfirm(code, choice);
  };

  return (
    <form
      noValidate
      className="space-y-5"
      onSubmit={(event) => {
        event.preventDefault();
        submit("once");
      }}
    >
      <OTPInputGrid
        ref={otpRef}
        length={MFA_CODE_LENGTH}
        disabled={busy}
        onChange={onCodeChange}
        ariaLabel="6-stelliger Bestätigungscode"
      />
      <div className="space-y-3">
        <Button
          type="submit"
          variant="primary"
          size="field"
          className="w-full"
          disabled={busy}
          isLoading={pending === "once"}
          loadingText="Code wird geprüft…"
          tooltip={
            offerRemember
              ? "Meldet Sie an. Beim nächsten Mal fragt moto wieder nach einem Code."
              : "Meldet Sie mit dem Code an."
          }
        >
          {offerRemember ? "Nur dieses Mal bestätigen" : "Bestätigen"}
        </Button>
        {offerRemember && (
          <>
            <Button
              type="button"
              variant="outline"
              size="field"
              className="w-full"
              disabled={busy}
              isLoading={pending === "remember"}
              loadingText="Code wird geprüft…"
              aria-describedby={hintId}
              tooltip={`Meldet Sie an. Auf diesem Gerät fragt moto ${trustedDeviceDuration(trustedDeviceDays)} lang nicht nach einem Code.`}
              onClick={() => submit("remember")}
            >
              Bestätigen und dieses Gerät{" "}
              {trustedDeviceDuration(trustedDeviceDays)} merken
            </Button>
            <p id={hintId} className="text-xs leading-relaxed text-gray-600">
              Nur auf Ihrem eigenen Gerät merken, nicht auf einem geteilten
              Tablet. Unter „{DEVICE_LIST_PLACE[scope]}“ können Sie gemerkte
              Geräte wieder entfernen.
            </p>
          </>
        )}
      </div>
      {children}
    </form>
  );
}

"use client";

import { useCallback, useRef } from "react";
import {
  germanMFAChallengeCopy,
  MFAChallengeForm,
} from "~/components/auth/mfa-challenge-form";
import { Modal } from "~/components/ui/modal";
import type { MFAResult } from "~/lib/mfa-api";
import { type TraegerCodeStep, verifyTraegerCodeStep } from "~/lib/traeger-api";

const TITLE = "Schule mit Code öffnen";

const COPY = { ...germanMFAChallengeCopy("traeger"), title: TITLE };

interface TraegerCodeStepModalProps {
  /** The mailed code; null keeps the dialog closed. */
  readonly step: TraegerCodeStep | null;
  readonly onClose: () => void;
  /**
   * The code was right. `renewed` is false when the renewed session could
   * not be installed: the person then signs in again.
   */
  readonly onVerified: (renewed: boolean) => void | Promise<void>;
}

/**
 * The code step in the running Träger session (#7068): a school that
 * accepts no remembered device opens after the person enters the mailed
 * code, without signing out. The code is checked as soon as it is complete;
 * nothing is remembered, since the school asked for the code itself.
 */
export function TraegerCodeStepModal({
  step,
  onClose,
  onVerified,
}: TraegerCodeStepModalProps) {
  // The verify answers the form; whether the session was renewed travels to
  // onSuccess through this ref.
  const renewed = useRef(false);
  const verify = useCallback(
    async ({
      challengeToken,
      code,
    }: {
      challengeToken: string;
      code: string;
    }): Promise<MFAResult> => {
      renewed.current = await verifyTraegerCodeStep(challengeToken, code);
      return { status: "authenticated" };
    },
    [],
  );
  return (
    <Modal isOpen={Boolean(step)} onClose={onClose} title="" ariaLabel={TITLE}>
      {step && (
        <MFAChallengeForm
          // A new code starts a fresh form.
          key={step.challengeToken}
          scope="traeger"
          challengeToken={step.challengeToken}
          maskedEmail={step.maskedEmail}
          trustedDeviceEnabled={false}
          copy={COPY}
          verify={verify}
          onSuccess={() => onVerified(renewed.current)}
        />
      )}
    </Modal>
  );
}

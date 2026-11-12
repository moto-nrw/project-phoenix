"use client";

import { useRef, useState } from "react";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { DataTable } from "~/components/ui/data-table";
import {
  useApiErrorDisplay,
  useApiFormError,
  useToast,
} from "~/contexts/ToastContext";
import type { RefreshAfterWrite } from "~/lib/hooks/use-refresh-after-write";
import { createLogger } from "~/lib/logger";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type {
  TraegerInvitation,
  TraegerMember,
} from "~/lib/operator/provisioning-helpers";
import {
  traegerBueroColumns,
  traegerBueroRowKey,
  traegerBueroRows,
} from "./traeger-member-columns";

const logger = createLogger({ component: "OperatorTraegerBueroTable" });

/**
 * Die Personen des Träger-Büros und die offenen Einladungen in einer Liste
 * (#4649, #5749). Eine Einladung lässt sich erneut senden oder zurücknehmen;
 * danach lädt die Seite beide Listen neu.
 */
export function TraegerBueroTable({
  organizationId,
  members,
  invitations,
  isLoading,
  onResetMfa,
  onChanged,
}: Readonly<{
  organizationId: string;
  members: readonly TraegerMember[];
  invitations: readonly TraegerInvitation[];
  isLoading: boolean;
  onResetMfa: (member: TraegerMember) => void;
  /** Reloads both lists; never rejects (#6652). */
  onChanged: RefreshAfterWrite;
}>) {
  const [revoking, setRevoking] = useState<TraegerInvitation | null>(null);
  const [confirming, setConfirming] = useState(false);
  const confirmErrors = useApiFormError();
  const { show: showError } = useApiErrorDisplay();
  const { success } = useToast();

  // A second click while the first resend runs would hit the replaced
  // invitation and show an error beside the success.
  const resending = useRef<string | null>(null);

  const resend = async (invitation: TraegerInvitation) => {
    if (resending.current !== null) return;
    resending.current = invitation.id;
    try {
      await operatorProvisioningService.resendTraegerInvitation(
        organizationId,
        invitation.id,
      );
    } catch (err) {
      logger.error("traeger_invitation_resend_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void showError(err, { object: "die Einladung" });
      resending.current = null;
      return;
    }
    success(
      `Die Einladung an ${invitation.email} ist neu verschickt. Der alte Link funktioniert nicht mehr.`,
    );
    await onChanged();
    resending.current = null;
  };

  const closeConfirm = () => {
    setRevoking(null);
    confirmErrors.clear();
  };

  const revoke = async () => {
    if (!revoking) return;
    setConfirming(true);
    confirmErrors.clear();
    try {
      await operatorProvisioningService.revokeTraegerInvitation(
        organizationId,
        revoking.id,
      );
    } catch (err) {
      logger.error("traeger_invitation_revoke_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void confirmErrors.show(err, { object: "das Zurücknehmen" });
      setConfirming(false);
      return;
    }
    success("Die Einladung ist zurückgenommen.");
    setRevoking(null);
    setConfirming(false);
    await onChanged();
  };

  return (
    <>
      <DataTable
        columns={traegerBueroColumns({
          onResetMfa,
          onResend: (invitation) => void resend(invitation),
          onRevoke: setRevoking,
        })}
        rows={traegerBueroRows(members, invitations)}
        getRowKey={traegerBueroRowKey}
        defaultSortKey="email"
        isLoading={isLoading}
        rowHasInteractiveControls
        stackedOnMobile
      />
      <ConfirmDeleteModal
        isOpen={revoking !== null}
        onClose={closeConfirm}
        onConfirm={() => void revoke()}
        title="Einladung zurücknehmen?"
        description={
          <>
            Der Link an{" "}
            <span className="font-medium wrap-anywhere">{revoking?.email}</span>{" "}
            funktioniert danach nicht mehr.
          </>
        }
        confirmLabel="Zurücknehmen"
        loadingLabel="Wird zurückgenommen …"
        gate={{ mode: "twoStep", firstStepLabel: "Ja, zurücknehmen" }}
        loading={confirming}
        error={confirmErrors.error}
      />
    </>
  );
}

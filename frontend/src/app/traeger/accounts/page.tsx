"use client";

// Personen des Träger-Büros (#4650): wer im Träger-Portal arbeitet, mit
// welcher Rolle, und wer eingeladen ist. Nur mit dem Recht „Konten
// verwalten“ (Rolle Verwaltung).

import { useCallback, useEffect, useState } from "react";
import { signOut, useSession } from "next-auth/react";
import { UserPlus, Users } from "lucide-react";
import {
  ChangeTraegerRolesModal,
  InviteTraegerAccountModal,
} from "~/components/traeger/traeger-account-modals";
import { Button } from "~/components/ui/button";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import {
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { formatDate, formatTime } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  fetchTraegerBuero,
  removeTraegerMember,
  revokeTraegerInvitation,
  type TraegerBuero,
  type TraegerInvitation,
  type TraegerMember,
} from "~/lib/traeger-api";
import { TRAEGER_ROLE_LABELS, type TraegerRole } from "~/lib/traeger-roles";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerAccountsPage" });

function roleNames(roles: readonly TraegerRole[]): string {
  return roles.map((role) => TRAEGER_ROLE_LABELS[role]).join(", ") || "–";
}

type Pending =
  | { kind: "remove"; member: TraegerMember }
  | { kind: "revoke"; invitation: TraegerInvitation };

export default function TraegerAccountsPage() {
  const { data: session } = useSession();
  const ownEmail = session?.user?.email ?? "";
  const [buero, setBuero] = useState<TraegerBuero>({
    members: [],
    invitations: [],
  });
  const [loading, setLoading] = useState(true);
  const [inviting, setInviting] = useState(false);
  const [editing, setEditing] = useState<TraegerMember | null>(null);
  const [pending, setPending] = useState<Pending | null>(null);
  const [confirming, setConfirming] = useState(false);
  const bueroLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = bueroLoad;
  const confirmErrors = useApiFormError();
  const { success } = useToast();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setBuero(await fetchTraegerBuero());
      clearLoadError();
    } catch (err) {
      logger.error("traeger_accounts_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Personen des Träger-Büros",
        retry: () => void load(),
      });
    } finally {
      setLoading(false);
    }
  }, [clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const closeConfirm = () => {
    setPending(null);
    confirmErrors.clear();
  };

  const confirm = async () => {
    if (!pending) return;
    setConfirming(true);
    confirmErrors.clear();
    try {
      if (pending.kind === "remove") {
        await removeTraegerMember(pending.member.accountId);
        if (pending.member.email === ownEmail) {
          // Wer sich selbst entfernt, hat keinen Zugang mehr.
          await signOut({ redirect: false });
          window.location.assign(traegerPath("/traeger/login"));
          return;
        }
        success("Die Person ist aus dem Träger-Büro entfernt.");
      } else {
        await revokeTraegerInvitation(pending.invitation.id);
        success("Die Einladung ist zurückgenommen.");
      }
      setPending(null);
      await load();
    } catch (err) {
      logger.error("traeger_account_change_failed", {
        kind: pending.kind,
        error: err instanceof Error ? err.message : String(err),
      });
      void confirmErrors.show(err, {
        object:
          pending.kind === "remove" ? "das Entfernen" : "das Zurücknehmen",
      });
    } finally {
      setConfirming(false);
    }
  };

  const memberColumns: DataTableColumn<TraegerMember>[] = [
    {
      key: "email",
      header: "E-Mail",
      render: (member) => (
        <span className="font-medium wrap-anywhere text-gray-900">
          {member.email}
          {member.email === ownEmail ? (
            <span className="ml-1 font-normal text-gray-500">(Sie)</span>
          ) : null}
        </span>
      ),
      sortValue: (member) => member.email,
      stacked: "title",
    },
    {
      key: "roles",
      header: "Rolle",
      render: (member) => roleNames(member.roles),
      stacked: "field",
    },
    {
      key: "actions",
      header: "",
      render: (member) => (
        <OverflowMenu
          ariaLabel={`Aktionen für ${member.email}`}
          triggerSize="sm"
          items={[
            { label: "Rollen ändern", onClick: () => setEditing(member) },
            {
              label: "Aus dem Träger-Büro entfernen",
              destructive: true,
              onClick: () => setPending({ kind: "remove", member }),
            },
          ]}
        />
      ),
      align: "right",
      stacked: "meta",
    },
  ];

  const invitationColumns: DataTableColumn<TraegerInvitation>[] = [
    {
      key: "email",
      header: "E-Mail",
      render: (invitation) => (
        <span className="font-medium wrap-anywhere text-gray-900">
          {invitation.email}
        </span>
      ),
      sortValue: (invitation) => invitation.email,
      stacked: "title",
    },
    {
      key: "roles",
      header: "Rolle",
      render: (invitation) => roleNames(invitation.roles),
      stacked: "field",
    },
    {
      key: "expires",
      header: "Link gilt bis",
      render: (invitation) =>
        `${formatDate(invitation.expiresAt)}, ${formatTime(invitation.expiresAt)} Uhr`,
      stacked: "field",
    },
    {
      key: "actions",
      header: "",
      render: (invitation) => (
        <OverflowMenu
          ariaLabel={`Aktionen für die Einladung an ${invitation.email}`}
          triggerSize="sm"
          items={[
            {
              label: "Einladung zurücknehmen",
              destructive: true,
              onClick: () => setPending({ kind: "revoke", invitation }),
            },
          ]}
        />
      ),
      align: "right",
      stacked: "meta",
    },
  ];

  const memberCount = buero.members.length;
  const invitationCount = buero.invitations.length;

  return (
    <TenantPage
      title="Personen"
      info="Wer im Träger-Büro arbeitet und welche Rolle die Person hat."
      stats={
        memberCount > 0 ? (
          <TenantPageStats
            items={[
              {
                value: memberCount,
                label: memberCount === 1 ? "Person" : "Personen",
              },
              ...(invitationCount > 0
                ? [
                    {
                      value: invitationCount,
                      label:
                        invitationCount === 1
                          ? "offene Einladung"
                          : "offene Einladungen",
                    },
                  ]
                : []),
            ]}
          />
        ) : null
      }
      statsLoading={loading}
      actions={
        <Button
          type="button"
          variant="primary"
          size="md"
          onClick={() => setInviting(true)}
        >
          <UserPlus className="mr-2 h-4 w-4" aria-hidden="true" />
          Person einladen
        </Button>
      }
      error={bueroLoad.error}
      overlays={
        <>
          <InviteTraegerAccountModal
            isOpen={inviting}
            onClose={() => setInviting(false)}
            onInvited={load}
          />
          <ChangeTraegerRolesModal
            member={editing}
            onClose={() => setEditing(null)}
            onChanged={load}
          />
          <ConfirmDeleteModal
            isOpen={pending !== null}
            onClose={closeConfirm}
            onConfirm={() => void confirm()}
            title={
              pending?.kind === "revoke"
                ? "Einladung zurücknehmen?"
                : "Person entfernen?"
            }
            description={
              pending?.kind === "revoke" ? (
                <>
                  Der Link an{" "}
                  <span className="font-medium wrap-anywhere">
                    {pending.invitation.email}
                  </span>{" "}
                  funktioniert danach nicht mehr.
                </>
              ) : (
                <>
                  <span className="font-medium wrap-anywhere">
                    {pending?.kind === "remove" ? pending.member.email : ""}
                  </span>{" "}
                  kann sich danach nicht mehr im Träger-Portal anmelden. Das
                  moto-Konto der Person bleibt bestehen.
                </>
              )
            }
            confirmLabel={
              pending?.kind === "revoke" ? "Zurücknehmen" : "Entfernen"
            }
            loadingLabel={
              pending?.kind === "revoke"
                ? "Wird zurückgenommen …"
                : "Wird entfernt …"
            }
            gate={{
              mode: "twoStep",
              firstStepLabel:
                pending?.kind === "revoke"
                  ? "Ja, zurücknehmen"
                  : "Ja, entfernen",
            }}
            loading={confirming}
            error={confirmErrors.error}
          />
        </>
      }
    >
      <DataTable
        columns={memberColumns}
        rows={[...buero.members]}
        getRowKey={(member) => member.accountId}
        rowHasInteractiveControls
        isLoading={loading}
        loadingRowCount={3}
        defaultSortKey="email"
        stackedOnMobile
        emptyState={
          <EmptyState
            icon={<Users className="h-12 w-12" aria-hidden="true" />}
            title="Noch keine Personen im Träger-Büro."
          />
        }
      />
      {buero.invitations.length > 0 ? (
        <SectionCard
          title="Offene Einladungen"
          description="Diese Personen haben eine E-Mail bekommen, die Einladung aber noch nicht angenommen."
        >
          <DataTable
            columns={invitationColumns}
            rows={[...buero.invitations]}
            getRowKey={(invitation) => invitation.id}
            rowHasInteractiveControls
            defaultSortKey="email"
            stackedOnMobile
          />
        </SectionCard>
      ) : null}
    </TenantPage>
  );
}

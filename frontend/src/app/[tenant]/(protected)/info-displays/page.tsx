"use client";

import { useMemo, useState } from "react";
import { useSession } from "next-auth/react";
import { QRCodeSVG } from "qrcode.react";
import { Copy, Check } from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";

import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { DataTable, DataTableStatusBadge } from "~/components/ui/data-table";
import type { DataTableColumn } from "~/components/ui/data-table";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { FormModal } from "~/components/ui/form-modal";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { Alert } from "~/components/ui/alert";
import { DisplayModeGuard } from "~/components/tenant/display-mode-guard";
import { useRequirePermission } from "~/lib/hooks/use-require-permission";
import { hasPermission, isAdmin } from "~/lib/auth-utils";
import { useSWRAuth } from "~/lib/swr";
import { useTenant } from "~/lib/tenant-context";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import type { InfoDisplay } from "~/lib/display-helpers";
import { buildDisplayUrl } from "~/lib/display-helpers";
import {
  listDisplays,
  createDisplay,
  updateDisplay,
  regenerateDisplayToken,
  deleteDisplay,
} from "~/lib/display-api";

const logger = createLogger({ component: "InfoDisplaysPage" });

interface TokenModalState {
  displayName: string;
  url: string;
}

// display.enabled is opt-in (default off, issue #1325 follow-up). Navigation
// already hides the sidebar entry; DisplayModeGuard 404s a direct URL visit
// on a tenant that hasn't enabled the feature.
export default function InfoDisplaysPage() {
  return (
    <DisplayModeGuard>
      <InfoDisplaysPageContent />
    </DisplayModeGuard>
  );
}

function InfoDisplaysPageContent() {
  // Viewing needs display:read OR display:manage (matching the backend list
  // route); mutating actions are additionally gated on display:manage below.
  const { isLoading: permissionLoading } = useRequirePermission([
    "display:read",
    "display:manage",
  ]);
  const { data: session } = useSession();
  const canManage =
    isAdmin(session) || hasPermission(session, "display:manage");
  const { tenantSlug } = useTenant();

  const {
    data: displays,
    error: displaysError,
    isLoading,
    mutate,
  } = useSWRAuth<InfoDisplay[]>("info-displays", listDisplays);

  const loadError = displaysError
    ? "Fehler beim Laden der Info-Displays. Bitte versuche es später erneut."
    : "";

  const [searchQuery, setSearchQuery] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const [createOpen, setCreateOpen] = useState(false);
  const [renameTarget, setRenameTarget] = useState<InfoDisplay | null>(null);
  const [nameInput, setNameInput] = useState("");
  const [deleteTarget, setDeleteTarget] = useState<InfoDisplay | null>(null);
  const [deleteError, setDeleteError] = useState("");
  const [tokenModal, setTokenModal] = useState<TokenModalState | null>(null);

  const filtered = useMemo(() => {
    const query = searchQuery.trim().toLowerCase();
    if (!query) return displays ?? [];
    return (displays ?? []).filter((d) => d.name.toLowerCase().includes(query));
  }, [displays, searchQuery]);

  const run = async <T,>(
    action: () => Promise<T>,
    failEvent: string,
  ): Promise<{ ok: true; value: T } | { ok: false }> => {
    setBusy(true);
    setError("");
    try {
      const value = await action();
      await mutate();
      return { ok: true, value };
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      logger.error(failEvent, { error: message });
      setError(message);
      return { ok: false };
    } finally {
      setBusy(false);
    }
  };

  const handleCreate = async () => {
    const result = await run(
      () => createDisplay(nameInput),
      "display_create_failed",
    );
    if (!result.ok) return;
    const { display, token } = result.value;
    setCreateOpen(false);
    setNameInput("");
    setTokenModal({
      displayName: display.name,
      url: buildDisplayUrl(token, tenantSlug),
    });
  };

  const handleRename = async () => {
    if (!renameTarget) return;
    const result = await run(
      () => updateDisplay(renameTarget.id, { name: nameInput }),
      "display_rename_failed",
    );
    if (!result.ok) return;
    setRenameTarget(null);
    setNameInput("");
  };

  const handleToggleActive = (display: InfoDisplay) =>
    run(async () => {
      await updateDisplay(display.id, { isActive: !display.isActive });
    }, "display_toggle_failed");

  const handleRegenerate = async (display: InfoDisplay) => {
    const result = await run(
      () => regenerateDisplayToken(display.id),
      "display_regenerate_failed",
    );
    if (!result.ok) return;
    setTokenModal({
      displayName: display.name,
      url: buildDisplayUrl(result.value, tenantSlug),
    });
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setBusy(true);
    setDeleteError("");
    try {
      await deleteDisplay(deleteTarget.id);
      await mutate();
      setDeleteTarget(null);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      logger.error("display_delete_failed", { error: message });
      setDeleteError(message);
    } finally {
      setBusy(false);
    }
  };

  const columns: DataTableColumn<InfoDisplay>[] = [
    {
      key: "name",
      header: "Name",
      render: (row) => (
        <span className="font-medium text-gray-900">{row.name}</span>
      ),
      sortValue: (row) => row.name,
    },
    {
      key: "status",
      header: "Status",
      render: (row) => <DataTableStatusBadge active={row.isActive} />,
      sortValue: (row) => (row.isActive ? 0 : 1),
    },
    {
      key: "created",
      header: "Erstellt",
      render: (row) => (
        <span className="text-gray-500">{formatDate(row.createdAt)}</span>
      ),
      sortValue: (row) => row.createdAt,
    },
    // Mutating actions require display:manage; read-only viewers get no menu.
    ...(canManage
      ? [
          {
            key: "actions",
            header: "",
            align: "right",
            render: (row) => (
              <OverflowMenu
                items={[
                  {
                    label: "Umbenennen",
                    onClick: () => {
                      setNameInput(row.name);
                      setRenameTarget(row);
                    },
                  },
                  {
                    label: "Neuen Link erstellen",
                    onClick: () => void handleRegenerate(row),
                  },
                  {
                    label: row.isActive ? "Deaktivieren" : "Aktivieren",
                    onClick: () => void handleToggleActive(row),
                  },
                  {
                    label: "Löschen",
                    destructive: true,
                    onClick: () => {
                      setDeleteError("");
                      setDeleteTarget(row);
                    },
                  },
                ]}
              />
            ),
          } satisfies DataTableColumn<InfoDisplay>,
        ]
      : []),
  ];

  return (
    <div className="space-y-6">
      <PageHeaderWithSearch
        title="Info-Displays"
        badge={{
          icon: <MotoConceptIcon concept="infoDisplays" size={18} />,
          count: displays?.length ?? 0,
        }}
        search={{
          value: searchQuery,
          onChange: setSearchQuery,
          placeholder: "Display suchen …",
        }}
        actionButton={
          canManage ? (
            <Button
              type="button"
              size="md"
              onClick={() => {
                setNameInput("");
                setCreateOpen(true);
              }}
            >
              Neues Display
            </Button>
          ) : undefined
        }
      />

      {(error || loadError) && (
        <Alert type="error" message={error || loadError} />
      )}

      <DataTable
        columns={columns}
        rows={filtered}
        getRowKey={(row) => row.id}
        isLoading={permissionLoading || isLoading}
        defaultSortKey="name"
        emptyState={
          loadError ? (
            // A failed load must never masquerade as "no displays yet".
            <div className="py-12 text-center text-gray-500">
              <p className="font-medium">
                Displays konnten nicht geladen werden
              </p>
              <p className="mt-1 text-sm">
                Bitte lade die Seite neu oder versuche es später erneut.
              </p>
            </div>
          ) : (
            <div className="py-12 text-center text-gray-500">
              <p className="font-medium">Noch keine Info-Displays</p>
              <p className="mt-1 text-sm">
                Erstelle ein Display und öffne den Link im Browser des
                Fernsehers oder Smartboards.
              </p>
            </div>
          )
        }
      />

      {/* Create / rename modal */}
      <FormModal
        isOpen={createOpen || renameTarget !== null}
        onClose={() => {
          setCreateOpen(false);
          setRenameTarget(null);
        }}
        title={renameTarget ? "Display umbenennen" : "Neues Info-Display"}
        size="sm"
        footer={
          <div className="flex justify-end gap-3">
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={() => {
                setCreateOpen(false);
                setRenameTarget(null);
              }}
            >
              Abbrechen
            </Button>
            <Button
              type="button"
              size="md"
              disabled={busy || nameInput.trim() === ""}
              onClick={() =>
                void (renameTarget ? handleRename() : handleCreate())
              }
            >
              {renameTarget ? "Speichern" : "Erstellen"}
            </Button>
          </div>
        }
      >
        <div className="space-y-2">
          <label
            htmlFor="display-name"
            className="block text-sm font-medium text-gray-700"
          >
            Name des Displays
          </label>
          <Input
            id="display-name"
            value={nameInput}
            onChange={(e) => setNameInput(e.target.value)}
            placeholder="z. B. Eingangsbereich"
            maxLength={100}
          />
          {!renameTarget && (
            <p className="text-sm text-gray-500">
              Nach dem Erstellen wird der Link zum Dashboard genau einmal
              angezeigt.
            </p>
          )}
        </div>
      </FormModal>

      {/* One-time token modal */}
      {tokenModal && (
        <TokenModal state={tokenModal} onClose={() => setTokenModal(null)} />
      )}

      {/* Delete confirmation */}
      {deleteTarget && (
        <ConfirmDeleteModal
          isOpen
          title="Display löschen"
          description={
            <>
              Das Display <strong>{deleteTarget.name}</strong> wird gelöscht und
              der zugehörige Link sofort ungültig. Der Bildschirm zeigt danach
              nur noch einen Hinweis.
            </>
          }
          gate={{ mode: "twoStep" }}
          onConfirm={handleDelete}
          onClose={() => setDeleteTarget(null)}
          loading={busy}
          error={deleteError}
        />
      )}
    </div>
  );
}

function TokenModal({
  state,
  onClose,
}: {
  readonly state: TokenModalState;
  readonly onClose: () => void;
}) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(state.url);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Clipboard access can fail (permissions); the URL stays selectable.
    }
  };

  return (
    <FormModal
      isOpen
      onClose={onClose}
      title={`Link für „${state.displayName}"`}
      size="md"
      footer={
        <div className="flex justify-end">
          <Button type="button" size="md" onClick={onClose}>
            Fertig
          </Button>
        </div>
      }
    >
      <div className="space-y-5">
        <Alert
          type="warning"
          message="Dieser Link wird nur einmal angezeigt. Kopiere ihn jetzt oder scanne den QR-Code am Zielgerät. Bei Verlust kannst du jederzeit einen neuen Link erstellen."
        />
        <div className="flex items-center gap-2">
          <code className="min-w-0 flex-1 truncate rounded-md border border-gray-200 bg-gray-50 px-3 py-2 text-sm text-gray-700">
            {state.url}
          </code>
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={() => void handleCopy()}
          >
            {copied ? (
              <Check className="text-moto-green h-4 w-4" />
            ) : (
              <Copy className="h-4 w-4" />
            )}
            <span className="ml-2">{copied ? "Kopiert" : "Kopieren"}</span>
          </Button>
        </div>
        <div className="flex justify-center rounded-2xl border border-gray-200 bg-white p-6">
          <QRCodeSVG value={state.url} size={192} />
        </div>
        <p className="text-sm text-gray-500">
          Öffne den Link im Browser des Fernsehers oder Smartboards. Das
          Dashboard aktualisiert sich automatisch.
        </p>
      </div>
    </FormModal>
  );
}

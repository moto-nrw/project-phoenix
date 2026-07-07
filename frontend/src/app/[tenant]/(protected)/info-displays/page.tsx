"use client";

import { useMemo, useState } from "react";
import { QRCodeSVG } from "qrcode.react";
import { MonitorPlay, Copy, Check } from "lucide-react";

import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { DataTable, DataTableStatusBadge } from "~/components/ui/data-table";
import type { DataTableColumn } from "~/components/ui/data-table";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { FormModal } from "~/components/ui/form-modal";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { Alert } from "~/components/ui/alert";
import { Loading } from "~/components/ui/loading";
import { useRequirePermission } from "~/lib/hooks/use-require-permission";
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

export default function InfoDisplaysPage() {
  const { isReady, isLoading: permissionLoading } =
    useRequirePermission("display:manage");
  const { tenantSlug } = useTenant();

  const {
    data: displays,
    isLoading,
    mutate,
  } = useSWRAuth<InfoDisplay[]>("info-displays", listDisplays);

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

  const run = async (action: () => Promise<void>, failEvent: string) => {
    setBusy(true);
    setError("");
    try {
      await action();
      await mutate();
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      logger.error(failEvent, { error: message });
      setError(message);
    } finally {
      setBusy(false);
    }
  };

  const handleCreate = () =>
    run(async () => {
      const { display, token } = await createDisplay(nameInput);
      setCreateOpen(false);
      setNameInput("");
      setTokenModal({
        displayName: display.name,
        url: buildDisplayUrl(token, tenantSlug),
      });
    }, "display_create_failed");

  const handleRename = () =>
    run(async () => {
      if (!renameTarget) return;
      await updateDisplay(renameTarget.id, { name: nameInput });
      setRenameTarget(null);
      setNameInput("");
    }, "display_rename_failed");

  const handleToggleActive = (display: InfoDisplay) =>
    run(async () => {
      await updateDisplay(display.id, { isActive: !display.isActive });
    }, "display_toggle_failed");

  const handleRegenerate = (display: InfoDisplay) =>
    run(async () => {
      const token = await regenerateDisplayToken(display.id);
      setTokenModal({
        displayName: display.name,
        url: buildDisplayUrl(token, tenantSlug),
      });
    }, "display_regenerate_failed");

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
    },
  ];

  if (permissionLoading || !isReady) {
    return <Loading />;
  }

  return (
    <div className="space-y-6">
      <PageHeaderWithSearch
        title="Info-Displays"
        badge={{
          icon: <MonitorPlay className="h-4 w-4" />,
          count: displays?.length ?? 0,
        }}
        search={{
          value: searchQuery,
          onChange: setSearchQuery,
          placeholder: "Display suchen …",
        }}
        actionButton={
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
        }
      />

      {error && <Alert type="error" message={error} />}

      <DataTable
        columns={columns}
        rows={filtered}
        getRowKey={(row) => row.id}
        isLoading={isLoading}
        defaultSortKey="name"
        emptyState={
          <div className="py-12 text-center text-gray-500">
            <p className="font-medium">Noch keine Info-Displays</p>
            <p className="mt-1 text-sm">
              Erstelle ein Display und öffne den Link im Browser des Fernsehers
              oder Smartboards.
            </p>
          </div>
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
              <Check className="h-4 w-4 text-[#83CD2D]" />
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

"use client";

import { useEffect, useMemo, useState, useCallback } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { DatabasePageLayout } from "~/components/database/database-page-layout";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import type {
  FilterConfig,
  ActiveFilter,
} from "~/components/ui/page-header/types";
import { getDbOperationMessage } from "@/lib/use-notification";
import { createCrudService } from "@/lib/database/service-factory";
import { rolesConfig } from "@/lib/database/configs/roles.config";
import type { Role } from "@/lib/auth-helpers";
import {
  RoleCreateModal,
  RoleDetailModal,
  RoleEditModal,
} from "@/components/roles";
import { RolePermissionManagementModal } from "@/components/auth";
import { useToast } from "~/contexts/ToastContext";
import { useIsMobile } from "~/hooks/useIsMobile";

export default function RolesPage() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [roles, setRoles] = useState<Role[]>([]);
  const [searchTerm, setSearchTerm] = useState("");
  const isMobile = useIsMobile();

  // Modals
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [createLoading, setCreateLoading] = useState(false);
  const [showDetailModal, setShowDetailModal] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);
  const [selectedRole, setSelectedRole] = useState<Role | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [showPermissionModal, setShowPermissionModal] = useState(false);

  const { success: toastSuccess } = useToast();

  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  const service = useMemo(() => createCrudService(rolesConfig), []);

  const fetchRoles = useCallback(async () => {
    try {
      setLoading(true);
      const data = await service.getList({ page: 1, pageSize: 500 });
      const arr = Array.isArray(data.data) ? data.data : [];
      setRoles(arr);
      setError(null);
    } catch (err) {
      console.error("Error fetching roles:", err);
      setError(
        "Fehler beim Laden der Rollen. Bitte versuchen Sie es später erneut.",
      );
      setRoles([]);
    } finally {
      setLoading(false);
    }
  }, [service]);

  useEffect(() => {
    fetchRoles().catch(() => {
      // Error already handled in fetchRoles
    });
  }, [fetchRoles]);

  const filters: FilterConfig[] = useMemo(() => [], []);
  const activeFilters: ActiveFilter[] = useMemo(
    () =>
      searchTerm
        ? [
            {
              id: "search",
              label: `"${searchTerm}"`,
              onRemove: () => setSearchTerm(""),
            },
          ]
        : [],
    [searchTerm],
  );

  const filteredRoles = useMemo(() => {
    let arr = [...roles];
    if (searchTerm) {
      const q = searchTerm.toLowerCase();
      arr = arr.filter(
        (r) =>
          r.name.toLowerCase().includes(q) ||
          (r.description?.toLowerCase().includes(q) ?? false),
      );
    }
    arr.sort((a, b) => a.name.localeCompare(b.name, "de"));
    return arr;
  }, [roles, searchTerm]);

  const handleSelectRole = async (role: Role) => {
    setSelectedRole(role);
    setShowDetailModal(true);
    try {
      setDetailLoading(true);
      const fresh = await service.getOne(role.id);
      setSelectedRole(fresh);
    } finally {
      setDetailLoading(false);
    }
  };

  const handleCreateRole = async (data: Partial<Role>) => {
    try {
      setCreateLoading(true);
      const created = await service.create(data);
      toastSuccess(
        getDbOperationMessage(
          "create",
          rolesConfig.name.singular,
          created.name,
        ),
      );
      setShowCreateModal(false);
      await fetchRoles();
    } finally {
      setCreateLoading(false);
    }
  };

  const handleUpdateRole = async (data: Partial<Role>) => {
    if (!selectedRole) return;
    try {
      setDetailLoading(true);
      await service.update(selectedRole.id, data);
      toastSuccess(
        getDbOperationMessage(
          "update",
          rolesConfig.name.singular,
          selectedRole.name,
        ),
      );
      const refreshed = await service.getOne(selectedRole.id);
      setSelectedRole(refreshed);
      setShowEditModal(false);
      setShowDetailModal(true);
      await fetchRoles();
    } finally {
      setDetailLoading(false);
    }
  };

  const handleDeleteRole = async () => {
    if (!selectedRole) return;
    try {
      setDetailLoading(true);
      await service.delete(selectedRole.id);
      toastSuccess(
        getDbOperationMessage(
          "delete",
          rolesConfig.name.singular,
          selectedRole.name,
        ),
      );
      setShowDetailModal(false);
      setSelectedRole(null);
      await fetchRoles();
    } finally {
      setDetailLoading(false);
    }
  };

  const handleEditClick = () => {
    setShowDetailModal(false);
    setShowEditModal(true);
  };

  return (
    <DatabasePageLayout loading={loading} sessionLoading={status === "loading"}>
      <div className="mb-4">
        <PageHeaderWithSearch
          title={isMobile ? "Rollen" : ""}
          badge={{
            icon: (
              <svg
                className="h-5 w-5 text-gray-600"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"
                />
              </svg>
            ),
            count: filteredRoles.length,
            label: "Rollen",
          }}
          search={{
            value: searchTerm,
            onChange: setSearchTerm,
            placeholder: "Rollen suchen...",
          }}
          filters={filters}
          activeFilters={activeFilters}
          onClearAllFilters={() => {
            setSearchTerm("");
          }}
          actionButton={
            !isMobile && (
              <button
                onClick={() => setShowCreateModal(true)}
                className="group relative flex h-10 w-10 items-center justify-center rounded-full bg-gradient-to-br from-purple-500 to-purple-600 text-white shadow-lg transition-all duration-300 hover:scale-110 hover:shadow-xl active:scale-95"
                style={{
                  background:
                    "linear-gradient(135deg, rgb(168, 85, 247) 0%, rgb(147, 51, 234) 100%)",
                  willChange: "transform, opacity",
                  WebkitTransform: "translateZ(0)",
                  transform: "translateZ(0)",
                }}
                aria-label="Rolle erstellen"
              >
                <div className="pointer-events-none absolute inset-[2px] rounded-full bg-gradient-to-br from-white/20 to-white/0 opacity-0 transition-opacity duration-300 group-hover:opacity-100"></div>
                <svg
                  className="relative h-5 w-5 transition-transform duration-300 group-active:rotate-90"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  strokeWidth={2.5}
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M12 4.5v15m7.5-7.5h-15"
                  />
                </svg>
                <div className="pointer-events-none absolute inset-0 scale-0 rounded-full bg-white/20 opacity-0 transition-transform duration-500 group-hover:scale-100 group-hover:opacity-100"></div>
              </button>
            )
          }
        />
      </div>

      <button
        onClick={() => setShowCreateModal(true)}
        className="group pointer-events-auto fixed right-4 bottom-24 z-40 flex h-14 w-14 translate-y-0 items-center justify-center rounded-full bg-gradient-to-br from-purple-500 to-purple-600 text-white opacity-100 shadow-[0_8px_30px_rgb(0,0,0,0.12)] transition-all duration-300 ease-out hover:shadow-[0_8px_40px_rgba(109,40,217,0.3)] active:scale-95 md:hidden"
        style={{
          background:
            "linear-gradient(135deg, rgb(168, 85, 247) 0%, rgb(147, 51, 234) 100%)",
          willChange: "transform, opacity",
          WebkitTransform: "translateZ(0)",
          transform: "translateZ(0)",
        }}
        aria-label="Rolle erstellen"
      >
        <div className="pointer-events-none absolute inset-[2px] rounded-full bg-gradient-to-br from-white/20 to-white/0 opacity-0 transition-opacity duration-300 group-hover:opacity-100"></div>
        <svg
          className="pointer-events-none relative h-6 w-6 transition-transform duration-300 group-active:rotate-90"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          strokeWidth={2.5}
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            d="M12 4.5v15m7.5-7.5h-15"
          />
        </svg>
        <div className="pointer-events-none absolute inset-0 scale-0 rounded-full bg-white/20 opacity-0 transition-transform duration-500 group-hover:scale-100 group-hover:opacity-100"></div>
      </button>

      {error && (
        <div className="mb-6 rounded-lg border border-red-200 bg-red-50 p-4">
          <p className="text-sm text-red-800">{error}</p>
        </div>
      )}

      {filteredRoles.length === 0 ? (
        <div className="flex min-h-[300px] items-center justify-center">
          <div className="text-center">
            <svg
              className="mx-auto h-12 w-12 text-gray-400"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={1.5}
                d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"
              />
            </svg>
            <h3 className="mt-4 text-lg font-medium text-gray-900">
              {searchTerm ? "Keine Rollen gefunden" : "Keine Rollen vorhanden"}
            </h3>
            <p className="mt-2 text-sm text-gray-600">
              {searchTerm
                ? "Versuchen Sie einen anderen Suchbegriff."
                : "Es wurden noch keine Rollen erstellt."}
            </p>
          </div>
        </div>
      ) : (
        <div className="space-y-3">
          {filteredRoles.map((role, index) => {
            const handleClick = () => void handleSelectRole(role);
            return (
              <button
                type="button"
                key={role.id}
                onClick={handleClick}
                className="group relative w-full cursor-pointer overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 text-left shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-500 active:scale-[0.99] md:hover:-translate-y-1 md:hover:scale-[1.01] md:hover:border-purple-300/60 md:hover:bg-white md:hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)]"
                style={{
                  animationName: "fadeInUp",
                  animationDuration: "0.5s",
                  animationTimingFunction: "ease-out",
                  animationFillMode: "forwards",
                  animationDelay: `${index * 0.03}s`,
                  opacity: 0,
                }}
              >
                <div className="pointer-events-none absolute inset-0 rounded-3xl bg-gradient-to-br from-purple-50/80 to-indigo-100/80 opacity-[0.03]"></div>
                <div className="pointer-events-none absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
                <div className="pointer-events-none absolute inset-0 rounded-3xl ring-1 ring-white/20 transition-all duration-300 md:group-hover:ring-purple-300/60"></div>

                <div className="relative flex items-center gap-4 p-5">
                  <div className="flex-shrink-0">
                    <div className="flex h-12 w-12 items-center justify-center rounded-full bg-gradient-to-br from-purple-500 to-purple-600 font-semibold text-white shadow-md transition-transform duration-300 md:group-hover:scale-110">
                      {role.name?.charAt(0)?.toUpperCase() ?? "R"}
                    </div>
                  </div>
                  <div className="min-w-0 flex-1">
                    <h3 className="text-lg font-semibold text-gray-900 transition-colors duration-300 md:group-hover:text-purple-600">
                      {role.name}
                    </h3>
                    {role.description && (
                      <p className="mt-0.5 line-clamp-1 text-sm text-gray-600">
                        {role.description}
                      </p>
                    )}
                    {typeof role.permissions?.length === "number" && (
                      <div className="mt-1 flex items-center gap-2">
                        <span className="inline-flex items-center rounded-full bg-purple-100 px-2 py-1 text-xs font-medium text-purple-700">
                          {role.permissions.length} Berechtigungen
                        </span>
                      </div>
                    )}
                  </div>
                  <div className="flex-shrink-0">
                    <svg
                      className="h-6 w-6 text-gray-400 transition-all duration-300 md:group-hover:translate-x-1 md:group-hover:text-purple-600"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M9 5l7 7-7 7"
                      />
                    </svg>
                  </div>
                </div>

                <div className="pointer-events-none absolute inset-0 rounded-3xl bg-gradient-to-r from-transparent via-purple-100/30 to-transparent opacity-0 transition-opacity duration-300 md:group-hover:opacity-100"></div>
              </button>
            );
          })}
        </div>
      )}

      {/* Create */}
      <RoleCreateModal
        isOpen={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        onCreate={handleCreateRole}
        loading={createLoading}
      />

      {/* Detail */}
      {selectedRole && (
        <RoleDetailModal
          isOpen={showDetailModal}
          onClose={() => {
            setShowDetailModal(false);
            setSelectedRole(null);
          }}
          role={selectedRole}
          onEdit={handleEditClick}
          onDelete={() => void handleDeleteRole()}
          onManagePermissions={() => {
            setShowPermissionModal(true);
          }}
          loading={detailLoading}
        />
      )}

      {/* Edit */}
      {selectedRole && (
        <RoleEditModal
          isOpen={showEditModal}
          onClose={() => {
            setShowEditModal(false);
          }}
          role={selectedRole}
          onSave={handleUpdateRole}
          loading={detailLoading}
        />
      )}

      {/* Permission Management */}
      {selectedRole && (
        <RolePermissionManagementModal
          isOpen={showPermissionModal}
          onClose={() => setShowPermissionModal(false)}
          role={selectedRole}
          onUpdate={async () => {
            await fetchRoles();
            // Refresh selectedRole to update detail modal with new permissions
            const refreshed = await service.getOne(selectedRole.id);
            setSelectedRole(refreshed);
          }}
        />
      )}

      {/* Success toasts handled globally */}
    </DatabasePageLayout>
  );
}

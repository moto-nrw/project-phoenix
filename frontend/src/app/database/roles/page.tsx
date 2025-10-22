"use client";

import { useEffect, useMemo, useState, useCallback } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import type { FilterConfig, ActiveFilter } from "~/components/ui/page-header/types";
import { getDbOperationMessage } from "@/lib/use-notification";
import { createCrudService } from "@/lib/database/service-factory";
import { rolesConfig } from "@/lib/database/configs/roles.config";
import type { Role } from "@/lib/auth-helpers";
import { RoleCreateModal, RoleDetailModal, RoleEditModal } from "@/components/roles";
import { RolePermissionManagementModal } from "@/components/auth";
import { useToast } from "~/contexts/ToastContext";

import { Loading } from "~/components/ui/loading";
export default function RolesPage() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [roles, setRoles] = useState<Role[]>([]);
  const [searchTerm, setSearchTerm] = useState("");
  const [isMobile, setIsMobile] = useState(false);
  const [isFabVisible, setIsFabVisible] = useState(true);
  const [lastScrollY, setLastScrollY] = useState(0);

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

  useEffect(() => {
    const checkMobile = () => setIsMobile(window.innerWidth < 768);
    checkMobile();
    window.addEventListener('resize', checkMobile);
    return () => window.removeEventListener('resize', checkMobile);
  }, []);

  useEffect(() => {
    const handleScroll = () => {
      const current = window.scrollY;
      if (current > lastScrollY && current > 100) setIsFabVisible(false);
      else setIsFabVisible(true);
      setLastScrollY(current);
    };
    window.addEventListener("scroll", handleScroll, { passive: true });
    return () => window.removeEventListener("scroll", handleScroll);
  }, [lastScrollY]);

  const fetchRoles = useCallback(async () => {
    try {
      setLoading(true);
      const data = await service.getList({ page: 1, pageSize: 500 });
      const arr = Array.isArray(data.data) ? data.data : [];
      setRoles(arr);
      setError(null);
    } catch (err) {
      console.error("Error fetching roles:", err);
      setError("Fehler beim Laden der Rollen. Bitte versuchen Sie es später erneut.");
      setRoles([]);
    } finally { setLoading(false); }
  }, [service]);

  useEffect(() => { void fetchRoles(); }, [fetchRoles]);

  const filters: FilterConfig[] = useMemo(() => [], []);
  const activeFilters: ActiveFilter[] = useMemo(() => (
    searchTerm ? [{ id: 'search', label: `"${searchTerm}"`, onRemove: () => setSearchTerm("") }] : []
  ), [searchTerm]);

  const filteredRoles = useMemo(() => {
    let arr = [...roles];
    if (searchTerm) {
      const q = searchTerm.toLowerCase();
      arr = arr.filter(r =>
        r.name.toLowerCase().includes(q) ||
        (r.description?.toLowerCase().includes(q) ?? false)
      );
    }
    arr.sort((a, b) => a.name.localeCompare(b.name, 'de'));
    return arr;
  }, [roles, searchTerm]);

  const handleSelectRole = async (role: Role) => {
    setSelectedRole(role);
    setShowDetailModal(true);
    try {
      setDetailLoading(true);
      const fresh = await service.getOne(role.id);
      setSelectedRole(fresh);
    } finally { setDetailLoading(false); }
  };

  const handleCreateRole = async (data: Partial<Role>) => {
    try {
      setCreateLoading(true);
      const created = await service.create(data);
      toastSuccess(getDbOperationMessage('create', rolesConfig.name.singular, created.name));
      setShowCreateModal(false);
      await fetchRoles();
    } finally { setCreateLoading(false); }
  };

  const handleUpdateRole = async (data: Partial<Role>) => {
    if (!selectedRole) return;
    try {
      setDetailLoading(true);
      await service.update(selectedRole.id, data);
      toastSuccess(getDbOperationMessage('update', rolesConfig.name.singular, selectedRole.name));
      const refreshed = await service.getOne(selectedRole.id);
      setSelectedRole(refreshed);
      setShowEditModal(false);
      setShowDetailModal(true);
      await fetchRoles();
    } finally { setDetailLoading(false); }
  };

  const handleDeleteRole = async () => {
    if (!selectedRole) return;
    try {
      setDetailLoading(true);
      await service.delete(selectedRole.id);
      toastSuccess(getDbOperationMessage('delete', rolesConfig.name.singular, selectedRole.name));
      setShowDetailModal(false);
      setSelectedRole(null);
      await fetchRoles();
    } finally { setDetailLoading(false); }
  };

  const handleEditClick = () => { setShowDetailModal(false); setShowEditModal(true); };

  if (status === "loading" || loading) {
    return (
      <ResponsiveLayout>
        <Loading fullPage={false} />
      </ResponsiveLayout>
    );
  }

  return (
    <ResponsiveLayout>
      <div className="w-full">
        {isMobile && (
          <button onClick={() => (window.location.href = '/database')} className="flex items-center gap-2 text-gray-600 hover:text-gray-900 mb-3 transition-colors duration-200 relative z-10" aria-label="Zurück zur Datenverwaltung">
            <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" /></svg>
            <span className="text-sm font-medium">Zurück</span>
          </button>
        )}

        <div className="mb-4">
          <PageHeaderWithSearch
            title={isMobile ? "Rollen" : ""}
            badge={{
              icon: (
                <svg className="h-5 w-5 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
                </svg>
              ),
              count: filteredRoles.length,
              label: "Rollen"
            }}
            search={{ value: searchTerm, onChange: setSearchTerm, placeholder: "Rollen suchen..." }}
            filters={filters}
            activeFilters={activeFilters}
            onClearAllFilters={() => { setSearchTerm(""); }}
            actionButton={!isMobile && (
              <button
                onClick={() => setShowCreateModal(true)}
                className="relative w-10 h-10 bg-gradient-to-br from-purple-500 to-purple-600 text-white rounded-full shadow-lg hover:shadow-xl transition-all duration-300 flex items-center justify-center group hover:scale-110 active:scale-95"
                aria-label="Rolle erstellen"
              >
                <div className="absolute inset-[2px] rounded-full bg-gradient-to-br from-white/20 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300 pointer-events-none"></div>
                <svg className="relative h-5 w-5 transition-transform duration-300 group-active:rotate-90" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}><path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" /></svg>
                <div className="absolute inset-0 rounded-full bg-white/20 scale-0 group-hover:scale-100 transition-transform duration-500 opacity-0 group-hover:opacity-100 pointer-events-none"></div>
              </button>
            )}
          />
        </div>

        <button
          onClick={() => setShowCreateModal(true)}
          className={`md:hidden fixed right-4 bottom-24 z-40 w-14 h-14 bg-gradient-to-br from-purple-500 to-purple-600 text-white rounded-full shadow-[0_8px_30px_rgb(0,0,0,0.12)] hover:shadow-[0_8px_40px_rgba(109,40,217,0.3)] flex items-center justify-center group active:scale-95 transition-all duration-300 ease-out ${isFabVisible ? 'translate-y-0 opacity-100 pointer-events-auto' : 'translate-y-32 opacity-0 pointer-events-none'}`}
          aria-label="Rolle erstellen"
        >
          <div className="absolute inset-[2px] rounded-full bg-gradient-to-br from-white/20 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300 pointer-events-none"></div>
          <svg className="relative h-6 w-6 transition-transform duration-300 group-active:rotate-90 pointer-events-none" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}><path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" /></svg>
          <div className="absolute inset-0 rounded-full bg-white/20 scale-0 group-hover:scale-100 transition-transform duration-500 opacity-0 group-hover:opacity-100 pointer-events-none"></div>
        </button>

        {error && (
          <div className="mb-6 rounded-lg bg-red-50 border border-red-200 p-4">
            <p className="text-sm text-red-800">{error}</p>
          </div>
        )}

        {filteredRoles.length === 0 ? (
          <div className="flex min-h-[300px] items-center justify-center">
            <div className="text-center">
              <svg className="mx-auto h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" /></svg>
              <h3 className="mt-4 text-lg font-medium text-gray-900">{searchTerm ? 'Keine Rollen gefunden' : 'Keine Rollen vorhanden'}</h3>
              <p className="mt-2 text-sm text-gray-600">{searchTerm ? 'Versuchen Sie einen anderen Suchbegriff.' : 'Es wurden noch keine Rollen erstellt.'}</p>
            </div>
          </div>
        ) : (
          <div className="space-y-3">
            {filteredRoles.map((role, index) => (
              <div
                key={role.id}
                onClick={() => void handleSelectRole(role)}
                className="group cursor-pointer relative overflow-hidden rounded-3xl bg-white/90 backdrop-blur-md border border-gray-100/50 shadow-[0_8px_30px_rgb(0,0,0,0.12)] transition-all duration-500 md:hover:scale-[1.01] md:hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)] md:hover:bg-white md:hover:-translate-y-1 active:scale-[0.99] md:hover:border-purple-300/60"
                style={{ animationName: 'fadeInUp', animationDuration: '0.5s', animationTimingFunction: 'ease-out', animationFillMode: 'forwards', animationDelay: `${index * 0.03}s`, opacity: 0 }}
              >
                <div className="pointer-events-none absolute inset-0 bg-gradient-to-br from-purple-50/80 to-indigo-100/80 opacity-[0.03] rounded-3xl"></div>
                <div className="pointer-events-none absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
                <div className="pointer-events-none absolute inset-0 rounded-3xl ring-1 ring-white/20 md:group-hover:ring-purple-300/60 transition-all duration-300"></div>

                <div className="relative flex items-center gap-4 p-5">
                  <div className="flex-shrink-0">
                    <div className="h-12 w-12 rounded-full bg-gradient-to-br from-purple-500 to-purple-600 flex items-center justify-center text-white font-semibold shadow-md md:group-hover:scale-110 transition-transform duration-300">
                      {role.name?.charAt(0)?.toUpperCase() ?? 'R'}
                    </div>
                  </div>
                  <div className="flex-1 min-w-0">
                    <h3 className="text-lg font-semibold text-gray-900 md:group-hover:text-purple-600 transition-colors duration-300">{role.name}</h3>
                    {role.description && (
                      <p className="text-sm text-gray-600 mt-0.5 line-clamp-1">{role.description}</p>
                    )}
                    {typeof role.permissions?.length === 'number' && (
                      <div className="flex items-center gap-2 mt-1">
                        <span className="inline-flex items-center px-2 py-1 rounded-full text-xs font-medium bg-purple-100 text-purple-700">{role.permissions.length} Berechtigungen</span>
                      </div>
                    )}
                  </div>
                  <div className="flex-shrink-0">
                    <svg className="h-6 w-6 text-gray-400 md:group-hover:text-purple-600 md:group-hover:translate-x-1 transition-all duration-300" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" /></svg>
                  </div>
                </div>

                <div className="pointer-events-none absolute inset-0 rounded-3xl opacity-0 md:group-hover:opacity-100 transition-opacity duration-300 bg-gradient-to-r from-transparent via-purple-100/30 to-transparent"></div>
              </div>
            ))}
            <style jsx>{`
              @keyframes fadeInUp { from { opacity:0; transform: translateY(20px);} to {opacity:1; transform: translateY(0);} }
            `}</style>
          </div>
        )}
      </div>

      {/* Create */}
      <RoleCreateModal isOpen={showCreateModal} onClose={() => setShowCreateModal(false)} onCreate={handleCreateRole} loading={createLoading} />

      {/* Detail */}
      {selectedRole && (
        <RoleDetailModal
          isOpen={showDetailModal}
          onClose={() => { setShowDetailModal(false); setSelectedRole(null); }}
          role={selectedRole}
          onEdit={handleEditClick}
          onDelete={() => void handleDeleteRole()}
          onManagePermissions={() => { setShowPermissionModal(true); }}
          loading={detailLoading}
        />
      )}

      {/* Edit */}
      {selectedRole && (
        <RoleEditModal
          isOpen={showEditModal}
          onClose={() => { setShowEditModal(false); }}
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
          onUpdate={() => void fetchRoles()}
        />
      )}

      {/* Success toasts handled globally */}
    </ResponsiveLayout>
  );
}

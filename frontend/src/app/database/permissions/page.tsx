"use client";

import { useEffect, useMemo, useState, useCallback } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import type { ActiveFilter, FilterConfig } from "~/components/ui/page-header/types";
import { SimpleAlert } from "@/components/simple/SimpleAlert";
import { getDbOperationMessage } from "@/lib/use-notification";
import { createCrudService } from "@/lib/database/service-factory";
import { permissionsConfig } from "@/lib/database/configs/permissions.config";
import type { Permission } from "@/lib/auth-helpers";
import { PermissionCreateModal, PermissionDetailModal, PermissionEditModal } from "@/components/permissions";

export default function PermissionsPage() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [permissions, setPermissions] = useState<Permission[]>([]);
  const [searchTerm, setSearchTerm] = useState("");
  const [isMobile, setIsMobile] = useState(false);
  const [isFabVisible, setIsFabVisible] = useState(true);
  const [lastScrollY, setLastScrollY] = useState(0);

  // Modals
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [createLoading, setCreateLoading] = useState(false);
  const [showDetailModal, setShowDetailModal] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);
  const [selectedPermission, setSelectedPermission] = useState<Permission | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);

  const [showSuccessAlert, setShowSuccessAlert] = useState(false);
  const [successMessage, setSuccessMessage] = useState("");

  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  const service = useMemo(() => createCrudService(permissionsConfig), []);

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

  const fetchPermissions = useCallback(async () => {
    try {
      setLoading(true);
      const data = await service.getList({ page: 1, pageSize: 500 });
      const arr = Array.isArray(data.data) ? data.data : [];
      setPermissions(arr);
      setError(null);
    } catch (err) {
      console.error("Error fetching permissions:", err);
      setError("Fehler beim Laden der Berechtigungen. Bitte versuchen Sie es später erneut.");
      setPermissions([]);
    } finally { setLoading(false); }
  }, [service]);

  useEffect(() => { void fetchPermissions(); }, [fetchPermissions]);

  // Lokalisierung für Ressource/Aktion und typische englische Bezeichnungen
  const resourceLabels: Record<string, string> = {
    users: 'Benutzer', roles: 'Rollen', permissions: 'Berechtigungen', activities: 'Aktivitäten',
    rooms: 'Räume', groups: 'Gruppen', visits: 'Besuche', schedules: 'Zeitpläne', config: 'Konfiguration',
    feedback: 'Feedback', iot: 'Geräte', system: 'System', admin: 'Administration',
  };
  const actionLabels: Record<string, string> = {
    create: 'Erstellen', read: 'Ansehen', update: 'Bearbeiten', delete: 'Löschen', list: 'Auflisten',
    manage: 'Verwalten', assign: 'Zuweisen', enroll: 'Anmelden', '*': 'Alle',
  };
  const toDisplay = (p: Permission) => `${resourceLabels[p.resource] ?? p.resource}: ${actionLabels[p.action] ?? p.action}`;

  // Anzeigename + Beschreibung exakt wie in den Daten anzeigen; bei fehlendem Anzeigenamen auf Ressource:Aktion ausweichen
  const displayTitle = (p: Permission) => p.name?.trim() ? p.name : toDisplay(p);

  const filters: FilterConfig[] = useMemo(() => [], []);
  const activeFilters: ActiveFilter[] = useMemo(() => (
    searchTerm ? [{ id: 'search', label: `"${searchTerm}"`, onRemove: () => setSearchTerm("") }] : []
  ), [searchTerm]);

  const filteredPermissions = useMemo(() => {
    let arr = [...permissions];
    if (searchTerm) {
      const q = searchTerm.toLowerCase();
      arr = arr.filter(p =>
        p.name.toLowerCase().includes(q) ||
        (p.description?.toLowerCase().includes(q) ?? false) ||
        p.resource.toLowerCase().includes(q) ||
        p.action.toLowerCase().includes(q)
      );
    }
    arr.sort((a, b) => {
      const r = a.resource.localeCompare(b.resource, 'de');
      if (r !== 0) return r;
      const a2 = a.action.localeCompare(b.action, 'de');
      if (a2 !== 0) return a2;
      return (a.name || '').localeCompare(b.name || '', 'de');
    });
    return arr;
  }, [permissions, searchTerm]);

  const handleSelectPermission = async (perm: Permission) => {
    setSelectedPermission(perm);
    setShowDetailModal(true);
    try {
      setDetailLoading(true);
      const fresh = await service.getOne(perm.id);
      setSelectedPermission(fresh);
    } finally { setDetailLoading(false); }
  };

  const handleCreatePermission = async (data: Partial<Permission>) => {
    try {
      setCreateLoading(true);
      const created = await service.create(data);
      const display = `${created.resource}: ${created.action}`;
      setSuccessMessage(getDbOperationMessage('create', permissionsConfig.name.singular, display));
      setShowSuccessAlert(true);
      setShowCreateModal(false);
      await fetchPermissions();
    } finally { setCreateLoading(false); }
  };

  const handleUpdatePermission = async (data: Partial<Permission>) => {
    if (!selectedPermission) return;
    try {
      setDetailLoading(true);
      await service.update(selectedPermission.id, data);
      const display = `${selectedPermission.resource}: ${selectedPermission.action}`;
      setSuccessMessage(getDbOperationMessage('update', permissionsConfig.name.singular, display));
      setShowSuccessAlert(true);
      const refreshed = await service.getOne(selectedPermission.id);
      setSelectedPermission(refreshed);
      setShowEditModal(false);
      setShowDetailModal(true);
      await fetchPermissions();
    } finally { setDetailLoading(false); }
  };

  const handleDeletePermission = async () => {
    if (!selectedPermission) return;
    try {
      setDetailLoading(true);
      await service.delete(selectedPermission.id);
      const display = `${selectedPermission.resource}: ${selectedPermission.action}`;
      setSuccessMessage(getDbOperationMessage('delete', permissionsConfig.name.singular, display));
      setShowSuccessAlert(true);
      setShowDetailModal(false);
      setSelectedPermission(null);
      await fetchPermissions();
    } finally { setDetailLoading(false); }
  };

  const handleEditClick = () => { setShowDetailModal(false); setShowEditModal(true); };

  if (status === "loading" || loading) {
    return (
      <ResponsiveLayout>
        <div className="flex min-h-[50vh] items-center justify-center">
          <div className="flex flex-col items-center gap-4">
            <div className="h-12 w-12 animate-spin rounded-full border-2 border-gray-200 border-t-indigo-600"></div>
            <p className="text-gray-600">Berechtigungen werden geladen...</p>
          </div>
        </div>
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

        <div className="relative z-30 mb-4">
          <PageHeaderWithSearch
            title={isMobile ? "Berechtigungen" : ""}
            badge={{
              icon: (
                <svg className="h-5 w-5 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                </svg>
              ),
              count: filteredPermissions.length,
              label: "Berechtigungen"
            }}
            search={{ value: searchTerm, onChange: setSearchTerm, placeholder: "Berechtigungen suchen..." }}
            filters={filters}
            activeFilters={activeFilters}
            onClearAllFilters={() => { setSearchTerm(""); }}
            actionButton={!isMobile && (
              <button
                onClick={() => setShowCreateModal(true)}
                className="w-10 h-10 bg-gradient-to-br from-pink-500 to-rose-600 text-white rounded-full shadow-lg hover:shadow-xl transition-all duration-300 flex items-center justify-center group hover:scale-110 active:scale-95"
                aria-label="Berechtigung erstellen"
              >
                <div className="absolute inset-[2px] rounded-full bg-gradient-to-br from-white/20 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300"></div>
                <svg className="relative h-5 w-5 transition-transform duration-300 group-active:rotate-90" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}><path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" /></svg>
                <div className="absolute inset-0 rounded-full bg-white/20 scale-0 group-hover:scale-100 transition-transform duration-500 opacity-0 group-hover:opacity-100"></div>
              </button>
            )}
          />
        </div>

        <button
          onClick={() => setShowCreateModal(true)}
          className={`md:hidden fixed right-4 bottom-24 z-40 w-14 h-14 bg-gradient-to-br from-pink-500 to-rose-600 text-white rounded-full shadow-[0_8px_30px_rgb(0,0,0,0.12)] hover:shadow-[0_8px_40px_rgba(244,114,182,0.3)] flex items-center justify-center group active:scale-95 transition-all duration-300 ease-out ${isFabVisible ? 'translate-y-0 opacity-100 pointer-events-auto' : 'translate-y-32 opacity-0 pointer-events-none'}`}
          aria-label="Berechtigung erstellen"
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

        {filteredPermissions.length === 0 ? (
          <div className="flex min-h-[300px] items-center justify-center">
            <div className="text-center">
              <svg className="mx-auto h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M15 7a2 2 0 012 2v1a2 2 0 11-4 0V9a2 2 0 012-2m-6 6h3l3 3 3-3 3 3-7 7-5-5v-2a2 2 0 012-2" /></svg>
              <h3 className="mt-4 text-lg font-medium text-gray-900">{searchTerm ? 'Keine Berechtigungen gefunden' : 'Keine Berechtigungen vorhanden'}</h3>
              <p className="mt-2 text-sm text-gray-600">{searchTerm ? 'Versuchen Sie einen anderen Suchbegriff.' : 'Es wurden noch keine Berechtigungen erstellt.'}</p>
            </div>
          </div>
        ) : (
          <div className="space-y-3">
            {filteredPermissions.map((perm, index) => (
              <div
                key={perm.id}
                onClick={() => void handleSelectPermission(perm)}
                className="group cursor-pointer relative overflow-hidden rounded-3xl bg-white/90 backdrop-blur-md border border-gray-100/50 shadow-[0_8px_30px_rgb(0,0,0,0.12)] transition-all duration-500 md:hover:scale-[1.01] md:hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)] md:hover:bg-white md:hover:-translate-y-1 active:scale-[0.99] md:hover:border-indigo-300/60"
                style={{ animationName: 'fadeInUp', animationDuration: '0.5s', animationTimingFunction: 'ease-out', animationFillMode: 'forwards', animationDelay: `${index * 0.03}s`, opacity: 0 }}
              >
                <div className="pointer-events-none absolute inset-0 bg-gradient-to-br from-pink-50/80 to-rose-100/80 opacity-[0.03] rounded-3xl"></div>
                <div className="pointer-events-none absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
                <div className="pointer-events-none absolute inset-0 rounded-3xl ring-1 ring-white/20 md:group-hover:ring-pink-300/60 transition-all duration-300"></div>

                <div className="relative flex items-center gap-4 p-5">
                  <div className="flex-shrink-0">
                    <div className="h-12 w-12 rounded-full bg-gradient-to-br from-pink-500 to-rose-600 flex items-center justify-center text-white font-semibold shadow-md md:group-hover:scale-110 transition-transform duration-300">
                      {perm.resource?.charAt(0)?.toUpperCase() ?? 'P'}
                    </div>
                  </div>
                  <div className="flex-1 min-w-0">
                    <h3 className="text-lg font-semibold text-gray-900 md:group-hover:text-pink-600 transition-colors duration-300">{displayTitle(perm)}</h3>
                    {perm.description && (
                      <p className="text-sm text-gray-600 mt-0.5 line-clamp-1">{perm.description}</p>
                    )}
                  </div>
                  <div className="flex-shrink-0">
                    <svg className="h-6 w-6 text-gray-400 md:group-hover:text-indigo-600 md:group-hover:translate-x-1 transition-all duration-300" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" /></svg>
                  </div>
                </div>

                <div className="pointer-events-none absolute inset-0 rounded-3xl opacity-0 md:group-hover:opacity-100 transition-opacity duration-300 bg-gradient-to-r from-transparent via-pink-100/30 to-transparent"></div>
              </div>
            ))}
            <style jsx>{`
              @keyframes fadeInUp { from { opacity:0; transform: translateY(20px);} to {opacity:1; transform: translateY(0);} }
            `}</style>
          </div>
        )}
      </div>

      {/* Create */}
      <PermissionCreateModal isOpen={showCreateModal} onClose={() => setShowCreateModal(false)} onCreate={handleCreatePermission} loading={createLoading} />

      {/* Detail */}
      {selectedPermission && (
        <PermissionDetailModal
          isOpen={showDetailModal}
          onClose={() => { setShowDetailModal(false); setSelectedPermission(null); }}
          permission={selectedPermission}
          onEdit={handleEditClick}
          onDelete={() => void handleDeletePermission()}
          loading={detailLoading}
        />
      )}

      {/* Edit */}
      {selectedPermission && (
        <PermissionEditModal
          isOpen={showEditModal}
          onClose={() => { setShowEditModal(false); }}
          permission={selectedPermission}
          onSave={handleUpdatePermission}
          loading={detailLoading}
        />
      )}

      {showSuccessAlert && (
        <SimpleAlert type="success" message={successMessage} autoClose duration={3000} onClose={() => setShowSuccessAlert(false)} />
      )}
    </ResponsiveLayout>
  );
}

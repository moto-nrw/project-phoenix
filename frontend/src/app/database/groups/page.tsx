"use client";

import { useEffect, useMemo, useState, useCallback } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import type { FilterConfig, ActiveFilter } from "~/components/ui/page-header/types";
import { SimpleAlert } from "@/components/simple/SimpleAlert";
import { getDbOperationMessage } from "@/lib/use-notification";
import { createCrudService } from "@/lib/database/service-factory";
import { groupsConfig } from "@/lib/database/configs/groups.config";
import type { Group } from "@/lib/group-helpers";
import { GroupCreateModal, GroupDetailModal, GroupEditModal } from "@/components/groups";

export default function GroupsPage() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [groups, setGroups] = useState<Group[]>([]);
  const [searchTerm, setSearchTerm] = useState("");
  const [roomFilter, setRoomFilter] = useState<string>("all");
  const [isMobile, setIsMobile] = useState(false);
  const [isFabVisible, setIsFabVisible] = useState(true);
  const [lastScrollY, setLastScrollY] = useState(0);

  const [showCreateModal, setShowCreateModal] = useState(false);
  const [createLoading, setCreateLoading] = useState(false);
  const [showDetailModal, setShowDetailModal] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);
  const [selectedGroup, setSelectedGroup] = useState<Group | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);

  const [showSuccessAlert, setShowSuccessAlert] = useState(false);
  const [successMessage, setSuccessMessage] = useState("");

  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  const service = useMemo(() => createCrudService(groupsConfig), []);

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

  const fetchGroups = useCallback(async () => {
    try {
      setLoading(true);
      const data = await service.getList({ page: 1, pageSize: 500 });
      const arr = Array.isArray(data.data) ? data.data : [];
      setGroups(arr);
      setError(null);
    } catch (err) {
      console.error("Error fetching groups:", err);
      setError("Fehler beim Laden der Gruppen. Bitte versuchen Sie es später erneut.");
      setGroups([]);
    } finally {
      setLoading(false);
    }
  }, [service]);

  useEffect(() => { void fetchGroups(); }, [fetchGroups]);

  const uniqueRooms = useMemo(() => {
    const set = new Set<string>();
    groups.forEach(g => { if (g.room_name) set.add(g.room_name); });
    return Array.from(set).sort().map(r => ({ value: r, label: r }));
  }, [groups]);

  const filters: FilterConfig[] = useMemo(() => [
    {
      id: 'room',
      label: 'Raum',
      type: 'dropdown',
      value: roomFilter,
      onChange: v => setRoomFilter(v as string),
      options: [ { value: 'all', label: 'Alle Räume' }, ...uniqueRooms ],
    }
  ], [roomFilter, uniqueRooms]);

  const activeFilters: ActiveFilter[] = useMemo(() => {
    const list: ActiveFilter[] = [];
    if (searchTerm) list.push({ id: 'search', label: `"${searchTerm}"`, onRemove: () => setSearchTerm("") });
    if (roomFilter !== 'all') list.push({ id: 'room', label: roomFilter, onRemove: () => setRoomFilter('all') });
    return list;
  }, [searchTerm, roomFilter]);

  const filteredGroups = useMemo(() => {
    let arr = [...groups];
    if (searchTerm) {
      const q = searchTerm.toLowerCase();
      arr = arr.filter(g =>
        g.name.toLowerCase().includes(q) ||
        (g.room_name?.toLowerCase().includes(q) ?? false) ||
        (g.representative_name?.toLowerCase().includes(q) ?? false)
      );
    }
    if (roomFilter !== 'all') {
      arr = arr.filter(g => g.room_name === roomFilter);
    }
    arr.sort((a, b) => a.name.localeCompare(b.name, 'de'));
    return arr;
  }, [groups, searchTerm, roomFilter]);

  const handleSelectGroup = async (group: Group) => {
    setSelectedGroup(group);
    setShowDetailModal(true);
    try {
      setDetailLoading(true);
      const fresh = await service.getOne(group.id);
      setSelectedGroup(fresh);
    } finally { setDetailLoading(false); }
  };

  const handleCreateGroup = async (data: Partial<Group>) => {
    try {
      setCreateLoading(true);
      if (groupsConfig.form.transformBeforeSubmit) data = groupsConfig.form.transformBeforeSubmit(data);
      const created = await service.create(data);
      setSuccessMessage(getDbOperationMessage('create', groupsConfig.name.singular, created.name));
      setShowSuccessAlert(true);
      setShowCreateModal(false);
      await fetchGroups();
    } finally { setCreateLoading(false); }
  };

  const handleUpdateGroup = async (data: Partial<Group>) => {
    if (!selectedGroup) return;
    try {
      setDetailLoading(true);
      if (groupsConfig.form.transformBeforeSubmit) data = groupsConfig.form.transformBeforeSubmit(data);
      await service.update(selectedGroup.id, data);
      setSuccessMessage(getDbOperationMessage('update', groupsConfig.name.singular, selectedGroup.name));
      setShowSuccessAlert(true);
      const refreshed = await service.getOne(selectedGroup.id);
      setSelectedGroup(refreshed);
      setShowEditModal(false);
      setShowDetailModal(true);
      await fetchGroups();
    } finally { setDetailLoading(false); }
  };

  const handleDeleteGroup = async () => {
    if (!selectedGroup) return;
    try {
      setDetailLoading(true);
      await service.delete(selectedGroup.id);
      setSuccessMessage(getDbOperationMessage('delete', groupsConfig.name.singular, selectedGroup.name));
      setShowSuccessAlert(true);
      setShowDetailModal(false);
      setSelectedGroup(null);
      await fetchGroups();
    } finally { setDetailLoading(false); }
  };

  const handleEditClick = () => { setShowDetailModal(false); setShowEditModal(true); };

  if (status === "loading" || loading) {
    return (
      <ResponsiveLayout>
        <div className="flex min-h-[50vh] items-center justify-center">
          <div className="flex flex-col items-center gap-4">
            <div className="h-12 w-12 animate-spin rounded-full border-2 border-gray-200 border-t-[#83CD2D]"></div>
            <p className="text-gray-600">Gruppen werden geladen...</p>
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
            title={isMobile ? "Gruppen" : ""}
            badge={{
              icon: (
                <svg className="h-5 w-5 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0z" />
                </svg>
              ),
              count: filteredGroups.length,
              label: "Gruppen"
            }}
            search={{ value: searchTerm, onChange: setSearchTerm, placeholder: "Gruppen suchen..." }}
            filters={filters}
            activeFilters={activeFilters}
            onClearAllFilters={() => { setSearchTerm(""); setRoomFilter('all'); }}
            actionButton={!isMobile && (
              <button
                onClick={() => setShowCreateModal(true)}
                className="w-10 h-10 bg-gradient-to-br from-[#83CD2D] to-[#70b525] text-white rounded-full shadow-lg hover:shadow-xl transition-all duration-300 flex items-center justify-center group hover:scale-110 active:scale-95"
                aria-label="Gruppe erstellen"
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
          className={`md:hidden fixed right-4 bottom-24 z-40 w-14 h-14 bg-gradient-to-br from-[#83CD2D] to-[#70b525] text-white rounded-full shadow-[0_8px_30px_rgb(0,0,0,0.12)] hover:shadow-[0_8px_40px_rgba(112,181,37,0.3)] flex items-center justify-center group active:scale-95 transition-all duration-300 ease-out ${isFabVisible ? 'translate-y-0 opacity-100 pointer-events-auto' : 'translate-y-32 opacity-0 pointer-events-none'}`}
          aria-label="Gruppe erstellen"
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

        {filteredGroups.length === 0 ? (
          <div className="flex min-h-[300px] items-center justify-center">
            <div className="text-center">
              <svg className="mx-auto h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0z" /></svg>
              <h3 className="mt-4 text-lg font-medium text-gray-900">{searchTerm || roomFilter !== 'all' ? 'Keine Gruppen gefunden' : 'Keine Gruppen vorhanden'}</h3>
              <p className="mt-2 text-sm text-gray-600">{searchTerm || roomFilter !== 'all' ? 'Versuchen Sie andere Suchkriterien oder Filter.' : 'Es wurden noch keine Gruppen erstellt.'}</p>
            </div>
          </div>
        ) : (
          <div className="space-y-3">
            {filteredGroups.map((group, index) => (
              <div
                key={group.id}
                onClick={() => void handleSelectGroup(group)}
                className="group cursor-pointer relative overflow-hidden rounded-3xl bg-white/90 backdrop-blur-md border border-gray-100/50 shadow-[0_8px_30px_rgb(0,0,0,0.12)] transition-all duration-500 md:hover:scale-[1.01] md:hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)] md:hover:bg-white md:hover:-translate-y-1 active:scale-[0.99] md:hover:border-[#83CD2D]/50"
                style={{ animationName: 'fadeInUp', animationDuration: '0.5s', animationTimingFunction: 'ease-out', animationFillMode: 'forwards', animationDelay: `${index * 0.03}s`, opacity: 0 }}
              >
                <div className="pointer-events-none absolute inset-0 bg-gradient-to-br from-green-50/80 to-emerald-100/80 opacity-[0.03] rounded-3xl"></div>
                <div className="pointer-events-none absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
                <div className="pointer-events-none absolute inset-0 rounded-3xl ring-1 ring-white/20 md:group-hover:ring-[#83CD2D]/50 transition-all duration-300"></div>

                <div className="relative flex items-center gap-4 p-5">
                  <div className="flex-shrink-0">
                    <div className="h-12 w-12 rounded-full bg-gradient-to-br from-[#83CD2D] to-[#70b525] flex items-center justify-center text-white font-semibold shadow-md md:group-hover:scale-110 transition-transform duration-300">
                      {group.name?.charAt(0)?.toUpperCase() ?? 'G'}
                    </div>
                  </div>
                  <div className="flex-1 min-w-0">
                    <h3 className="text-lg font-semibold text-gray-900 md:group-hover:text-[#70b525] transition-colors duration-300">{group.name}</h3>
                    <div className="flex items-center gap-2 mt-1 flex-wrap">
                      {group.room_name && (
                        <span className="inline-flex items-center px-2 py-1 rounded-full text-xs font-medium bg-gray-100 text-gray-700">{group.room_name}</span>
                      )}
                    </div>
                  </div>
                  <div className="flex-shrink-0">
                    <svg className="h-6 w-6 text-gray-400 md:group-hover:text-[#70b525] md:group-hover:translate-x-1 transition-all duration-300" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" /></svg>
                  </div>
                </div>

                <div className="pointer-events-none absolute inset-0 rounded-3xl opacity-0 md:group-hover:opacity-100 transition-opacity duration-300 bg-gradient-to-r from-transparent via-green-100/30 to-transparent"></div>
              </div>
            ))}
            <style jsx>{`
              @keyframes fadeInUp { from { opacity:0; transform: translateY(20px);} to {opacity:1; transform: translateY(0);} }
            `}</style>
          </div>
        )}
      </div>

      {/* Create */}
      <GroupCreateModal isOpen={showCreateModal} onClose={() => setShowCreateModal(false)} onCreate={handleCreateGroup} loading={createLoading} />

      {/* Detail */}
      {selectedGroup && (
        <GroupDetailModal
          isOpen={showDetailModal}
          onClose={() => { setShowDetailModal(false); setSelectedGroup(null); }}
          group={selectedGroup}
          onEdit={handleEditClick}
          onDelete={() => void handleDeleteGroup()}
          loading={detailLoading}
        />
      )}

      {/* Edit */}
      {selectedGroup && (
        <GroupEditModal
          isOpen={showEditModal}
          onClose={() => { setShowEditModal(false); }}
          group={selectedGroup}
          onSave={handleUpdateGroup}
          loading={detailLoading}
        />
      )}

      {showSuccessAlert && (
        <SimpleAlert type="success" message={successMessage} autoClose duration={3000} onClose={() => setShowSuccessAlert(false)} />
      )}
    </ResponsiveLayout>
  );
}

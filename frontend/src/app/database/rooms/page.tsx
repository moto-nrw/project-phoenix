"use client";

import { useState, useEffect, useMemo, useCallback } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import type { FilterConfig, ActiveFilter } from "~/components/ui/page-header/types";
import { getDbOperationMessage } from "@/lib/use-notification";
import { createCrudService } from "@/lib/database/service-factory";
import { roomsConfig } from "@/lib/database/configs/rooms.config";
import type { Room } from "@/lib/room-helpers";
import { RoomCreateModal, RoomDetailModal, RoomEditModal } from "@/components/rooms";
import { useToast } from "~/contexts/ToastContext";

import { Loading } from "~/components/ui/loading";
export default function RoomsPage() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [rooms, setRooms] = useState<Room[]>([]);
  const [searchTerm, setSearchTerm] = useState("");
  const [categoryFilter, setCategoryFilter] = useState<string>("all");
  const [isMobile, setIsMobile] = useState(false);
  const [isFabVisible, setIsFabVisible] = useState(true);
  const [lastScrollY, setLastScrollY] = useState(0);

  // Modals
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [createLoading, setCreateLoading] = useState(false);

  const [showDetailModal, setShowDetailModal] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);
  const [selectedRoom, setSelectedRoom] = useState<Room | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);

  const { success: toastSuccess } = useToast();

  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  const service = useMemo(() => createCrudService(roomsConfig), []);

  // Mobile detection
  useEffect(() => {
    const checkMobile = () => setIsMobile(window.innerWidth < 768);
    checkMobile();
    window.addEventListener('resize', checkMobile);
    return () => window.removeEventListener('resize', checkMobile);
  }, []);

  // FAB visibility on scroll
  useEffect(() => {
    const handleScroll = () => {
      const current = window.scrollY;
      if (current > lastScrollY && current > 100) {
        setIsFabVisible(false);
      } else {
        setIsFabVisible(true);
      }
      setLastScrollY(current);
    };
    window.addEventListener("scroll", handleScroll, { passive: true });
    return () => window.removeEventListener("scroll", handleScroll);
  }, [lastScrollY]);

  // Fetch rooms
  const fetchRooms = useCallback(async () => {
    try {
      setLoading(true);
      const data = await service.getList({ page: 1, pageSize: 500 });
      const arr = Array.isArray(data.data) ? data.data : [];
      setRooms(arr);
      setError(null);
    } catch (err) {
      console.error("Error fetching rooms:", err);
      setError("Fehler beim Laden der Räume. Bitte versuchen Sie es später erneut.");
      setRooms([]);
    } finally {
      setLoading(false);
    }
  }, [service]);

  useEffect(() => { void fetchRooms(); }, [fetchRooms]);

  // Unique categories from current data
  const uniqueCategories = useMemo(() => {
    const set = new Set<string>();
    rooms.forEach(r => { if (r.category) set.add(r.category); });
    return Array.from(set).sort().map(c => ({ value: c, label: c }));
  }, [rooms]);

  // Filters config
  const filters: FilterConfig[] = useMemo(() => [
    {
      id: 'category',
      label: 'Kategorie',
      type: 'dropdown',
      value: categoryFilter,
      onChange: v => setCategoryFilter(v as string),
      options: [
        { value: 'all', label: 'Alle Kategorien' },
        ...uniqueCategories,
      ],
    },
  ], [categoryFilter, uniqueCategories]);

  const activeFilters: ActiveFilter[] = useMemo(() => {
    const list: ActiveFilter[] = [];
    if (searchTerm) list.push({ id: 'search', label: `"${searchTerm}"`, onRemove: () => setSearchTerm("") });
    if (categoryFilter !== 'all') list.push({ id: 'category', label: categoryFilter, onRemove: () => setCategoryFilter('all') });
    return list;
  }, [searchTerm, categoryFilter]);

  // Derived list
  const filteredRooms = useMemo(() => {
    let arr = [...rooms];
    if (searchTerm) {
      const q = searchTerm.toLowerCase();
      arr = arr.filter(r =>
        r.name.toLowerCase().includes(q) ||
        (r.building?.toLowerCase().includes(q) ?? false) ||
        (r.category?.toLowerCase().includes(q) ?? false)
      );
    }
    if (categoryFilter !== 'all') {
      arr = arr.filter(r => r.category === categoryFilter);
    }
    // Sort by name
    arr.sort((a, b) => a.name.localeCompare(b.name, 'de'));
    return arr;
  }, [rooms, searchTerm, categoryFilter]);

  // Select room => open detail and refresh details
  const handleSelectRoom = async (room: Room) => {
    setSelectedRoom(room);
    setShowDetailModal(true);
    try {
      setDetailLoading(true);
      const fresh = await service.getOne(room.id);
      setSelectedRoom(fresh);
    } finally {
      setDetailLoading(false);
    }
  };

  // Create room
  const handleCreateRoom = async (data: Partial<Room>) => {
    try {
      setCreateLoading(true);
      const created = await service.create(data);
      toastSuccess(getDbOperationMessage('create', roomsConfig.name.singular, created.name));
      setShowCreateModal(false);
      await fetchRooms();
    } finally {
      setCreateLoading(false);
    }
  };

  // Update room
  const handleUpdateRoom = async (data: Partial<Room>) => {
    if (!selectedRoom) return;
    try {
      setDetailLoading(true);
      await service.update(selectedRoom.id, data);
      const name = selectedRoom.name;
      toastSuccess(getDbOperationMessage('update', roomsConfig.name.singular, name));
      const refreshed = await service.getOne(selectedRoom.id);
      setSelectedRoom(refreshed);
      setShowEditModal(false);
      setShowDetailModal(true);
      await fetchRooms();
    } catch (e) {
      console.error("Error updating room", e);
      throw e;
    } finally {
      setDetailLoading(false);
    }
  };

  // Delete room
  const handleDeleteRoom = async () => {
    if (!selectedRoom) return;
    try {
      setDetailLoading(true);
      await service.delete(selectedRoom.id);
      toastSuccess(getDbOperationMessage('delete', roomsConfig.name.singular, selectedRoom.name));
      setShowDetailModal(false);
      setSelectedRoom(null);
      await fetchRooms();
    } finally {
      setDetailLoading(false);
    }
  };

  const handleEditClick = () => {
    setShowDetailModal(false);
    setShowEditModal(true);
  };

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
        {/* Mobile Back Button */}
        {isMobile && (
          <button
            onClick={() => (window.location.href = '/database')}
            className="flex items-center gap-2 text-gray-600 hover:text-gray-900 mb-3 transition-colors duration-200 relative z-10"
            aria-label="Zurück zur Datenverwaltung"
          >
            <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
            </svg>
            <span className="text-sm font-medium">Zurück</span>
          </button>
        )}

        {/* Header */}
        <div className="mb-4">
          <PageHeaderWithSearch
            title={isMobile ? "Räume" : ""}
            badge={{
              icon: (
                <svg className="h-5 w-5 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
                </svg>
              ),
              count: filteredRooms.length,
              label: "Räume",
            }}
            search={{ value: searchTerm, onChange: setSearchTerm, placeholder: "Räume suchen..." }}
            filters={filters}
            activeFilters={activeFilters}
            onClearAllFilters={() => { setSearchTerm(""); setCategoryFilter('all'); }}
            actionButton={!isMobile && (
              <button
                onClick={() => setShowCreateModal(true)}
                className="relative w-10 h-10 bg-gradient-to-br from-indigo-500 to-indigo-600 text-white rounded-full shadow-lg hover:shadow-xl transition-all duration-300 flex items-center justify-center group hover:scale-110 active:scale-95"
                aria-label="Raum erstellen"
              >
                <div className="absolute inset-[2px] rounded-full bg-gradient-to-br from-white/20 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300 pointer-events-none"></div>
                <svg className="relative h-5 w-5 transition-transform duration-300 group-active:rotate-90" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
                </svg>
                <div className="absolute inset-0 rounded-full bg-white/20 scale-0 group-hover:scale-100 transition-transform duration-500 opacity-0 group-hover:opacity-100 pointer-events-none"></div>
              </button>
            )}
          />
        </div>

        {/* Mobile FAB */}
        <button
          onClick={() => setShowCreateModal(true)}
          className={`md:hidden fixed right-4 bottom-24 z-40 w-14 h-14 bg-gradient-to-br from-indigo-500 to-indigo-600 text-white rounded-full shadow-[0_8px_30px_rgb(0,0,0,0.12)] hover:shadow-[0_8px_40px_rgb(79,70,229,0.3)] flex items-center justify-center group active:scale-95 transition-all duration-300 ease-out ${isFabVisible ? 'translate-y-0 opacity-100 pointer-events-auto' : 'translate-y-32 opacity-0 pointer-events-none'}`}
          aria-label="Raum erstellen"
        >
          <div className="absolute inset-[2px] rounded-full bg-gradient-to-br from-white/20 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300 pointer-events-none"></div>
          <svg className="relative h-6 w-6 transition-transform duration-300 group-active:rotate-90 pointer-events-none" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
          </svg>
          <div className="absolute inset-0 rounded-full bg-white/20 scale-0 group-hover:scale-100 transition-transform duration-500 opacity-0 group-hover:opacity-100 pointer-events-none"></div>
        </button>

        {/* Error */}
        {error && (
          <div className="mb-6 rounded-lg bg-red-50 border border-red-200 p-4">
            <p className="text-sm text-red-800">{error}</p>
          </div>
        )}

        {/* List */}
        {filteredRooms.length === 0 ? (
          <div className="flex min-h-[300px] items-center justify-center">
            <div className="text-center">
              <svg className="mx-auto h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
              </svg>
              <h3 className="mt-4 text-lg font-medium text-gray-900">{searchTerm || categoryFilter !== 'all' ? 'Keine Räume gefunden' : 'Keine Räume vorhanden'}</h3>
              <p className="mt-2 text-sm text-gray-600">{searchTerm || categoryFilter !== 'all' ? 'Versuchen Sie andere Suchkriterien oder Filter.' : 'Es wurden noch keine Räume erstellt.'}</p>
            </div>
          </div>
        ) : (
          <div className="space-y-3">
            {filteredRooms.map((room, index) => {
              const initial = room.name?.charAt(0)?.toUpperCase() ?? 'R';

              return (
                <div
                  key={room.id}
                  onClick={() => void handleSelectRoom(room)}
                  className="group cursor-pointer relative overflow-hidden rounded-3xl bg-white/90 backdrop-blur-md border border-gray-100/50 shadow-[0_8px_30px_rgb(0,0,0,0.12)] transition-all duration-500 md:hover:scale-[1.01] md:hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)] md:hover:bg-white md:hover:-translate-y-1 active:scale-[0.99] md:hover:border-indigo-200/50"
                  style={{
                    animationName: 'fadeInUp',
                    animationDuration: '0.5s',
                    animationTimingFunction: 'ease-out',
                    animationFillMode: 'forwards',
                    animationDelay: `${index * 0.03}s`,
                    opacity: 0,
                  }}
                >
                  <div className="pointer-events-none absolute inset-0 bg-gradient-to-br from-indigo-50/80 to-blue-100/80 opacity-[0.03] rounded-3xl"></div>
                  <div className="pointer-events-none absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
                  <div className="pointer-events-none absolute inset-0 rounded-3xl ring-1 ring-white/20 md:group-hover:ring-indigo-200/60 transition-all duration-300"></div>

                  <div className="relative flex items-center gap-4 p-5">
                    <div className="flex-shrink-0">
                      <div className="h-12 w-12 rounded-full bg-gradient-to-br from-indigo-500 to-indigo-600 flex items-center justify-center text-white font-semibold shadow-md md:group-hover:scale-110 transition-transform duration-300">
                        {initial}
                      </div>
                    </div>

                    <div className="flex-1 min-w-0">
                      <h3 className="text-lg font-semibold text-gray-900 md:group-hover:text-indigo-600 transition-colors duration-300">
                        {room.name}
                      </h3>
                      <div className="flex items-center gap-2 mt-1 flex-wrap">
                        {room.building && (
                          <span className="inline-flex items-center px-2 py-1 rounded-full text-xs font-medium bg-gray-100 text-gray-700">
                            {room.building} • Etage {room.floor}
                          </span>
                        )}
                        {!room.building && (
                          <span className="inline-flex items-center px-2 py-1 rounded-full text-xs font-medium bg-gray-100 text-gray-700">
                            Etage {room.floor}
                          </span>
                        )}
                        {room.capacity ? (
                          <span className="inline-flex items-center px-2 py-1 rounded-full text-xs font-medium bg-blue-100 text-blue-700">
                            {room.capacity} Plätze
                          </span>
                        ) : null}
                      </div>
                    </div>

                    <div className="flex-shrink-0">
                      <svg className="h-6 w-6 text-gray-400 md:group-hover:text-indigo-600 md:group-hover:translate-x-1 transition-all duration-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                      </svg>
                    </div>
                  </div>

                  <div className="pointer-events-none absolute inset-0 rounded-3xl opacity-0 md:group-hover:opacity-100 transition-opacity duration-300 bg-gradient-to-r from-transparent via-indigo-100/30 to-transparent"></div>
                </div>
              );
            })}

            <style jsx>{`
              @keyframes fadeInUp {
                from { opacity: 0; transform: translateY(20px); }
                to { opacity: 1; transform: translateY(0); }
              }
            `}</style>
          </div>
        )}
      </div>

      {/* Create Modal */}
      <RoomCreateModal
        isOpen={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        onCreate={handleCreateRoom}
        loading={createLoading}
      />

      {/* Detail Modal */}
      {selectedRoom && (
        <RoomDetailModal
          isOpen={showDetailModal}
          onClose={() => { setShowDetailModal(false); setSelectedRoom(null); }}
          room={selectedRoom}
          onEdit={handleEditClick}
          onDelete={() => void handleDeleteRoom()}
          loading={detailLoading}
        />
      )}

      {/* Edit Modal */}
      {selectedRoom && (
        <RoomEditModal
          isOpen={showEditModal}
          onClose={() => { setShowEditModal(false); }}
          room={selectedRoom}
          onSave={handleUpdateRoom}
          loading={detailLoading}
        />
      )}

      {/* Success toasts are handled globally */}
    </ResponsiveLayout>
  );
}

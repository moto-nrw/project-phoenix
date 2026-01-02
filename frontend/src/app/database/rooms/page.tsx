"use client";

import { useState, useEffect, useMemo, useCallback } from "react";
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
import { roomsConfig } from "@/lib/database/configs/rooms.config";
import type { Room } from "@/lib/room-helpers";
import {
  RoomCreateModal,
  RoomDetailModal,
  RoomEditModal,
} from "@/components/rooms";
import { useToast } from "~/contexts/ToastContext";
import { useIsMobile } from "~/hooks/useIsMobile";

export default function RoomsPage() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [rooms, setRooms] = useState<Room[]>([]);
  const [searchTerm, setSearchTerm] = useState("");
  const [categoryFilter, setCategoryFilter] = useState<string>("all");
  const isMobile = useIsMobile();

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
      setError(
        "Fehler beim Laden der Räume. Bitte versuchen Sie es später erneut.",
      );
      setRooms([]);
    } finally {
      setLoading(false);
    }
  }, [service]);

  useEffect(() => {
    fetchRooms().catch(() => {
      // Error already handled in fetchRooms
    });
  }, [fetchRooms]);

  // Unique categories from current data
  const uniqueCategories = useMemo(() => {
    const set = new Set<string>();
    rooms.forEach((r) => {
      if (r.category) set.add(r.category);
    });
    return Array.from(set)
      .sort((a, b) => a.localeCompare(b, "de"))
      .map((c) => ({ value: c, label: c }));
  }, [rooms]);

  // Filters config
  const filters: FilterConfig[] = useMemo(
    () => [
      {
        id: "category",
        label: "Kategorie",
        type: "dropdown",
        value: categoryFilter,
        onChange: (v) => setCategoryFilter(v as string),
        options: [
          { value: "all", label: "Alle Kategorien" },
          ...uniqueCategories,
        ],
      },
    ],
    [categoryFilter, uniqueCategories],
  );

  const activeFilters: ActiveFilter[] = useMemo(() => {
    const list: ActiveFilter[] = [];
    if (searchTerm)
      list.push({
        id: "search",
        label: `"${searchTerm}"`,
        onRemove: () => setSearchTerm(""),
      });
    if (categoryFilter !== "all")
      list.push({
        id: "category",
        label: categoryFilter,
        onRemove: () => setCategoryFilter("all"),
      });
    return list;
  }, [searchTerm, categoryFilter]);

  // Derived list
  const filteredRooms = useMemo(() => {
    let arr = [...rooms];
    if (searchTerm) {
      const q = searchTerm.toLowerCase();
      arr = arr.filter(
        (r) =>
          r.name.toLowerCase().includes(q) ||
          (r.building?.toLowerCase().includes(q) ?? false) ||
          (r.category?.toLowerCase().includes(q) ?? false),
      );
    }
    if (categoryFilter !== "all") {
      arr = arr.filter((r) => r.category === categoryFilter);
    }
    // Sort by name
    arr.sort((a, b) => a.name.localeCompare(b.name, "de"));
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
      // Apply transform to ensure floor is number and color has default
      if (roomsConfig.form.transformBeforeSubmit) {
        data = roomsConfig.form.transformBeforeSubmit(data);
      }
      const created = await service.create(data);
      toastSuccess(
        getDbOperationMessage(
          "create",
          roomsConfig.name.singular,
          created.name,
        ),
      );
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
      // Apply transform to ensure floor is number and color has default
      if (roomsConfig.form.transformBeforeSubmit) {
        data = roomsConfig.form.transformBeforeSubmit(data);
      }
      await service.update(selectedRoom.id, data);
      const name = selectedRoom.name;
      toastSuccess(
        getDbOperationMessage("update", roomsConfig.name.singular, name),
      );
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
      toastSuccess(
        getDbOperationMessage(
          "delete",
          roomsConfig.name.singular,
          selectedRoom.name,
        ),
      );
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

  return (
    <DatabasePageLayout loading={loading} sessionLoading={status === "loading"}>
      <div className="mb-4">
        <PageHeaderWithSearch
          title={isMobile ? "Räume" : ""}
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
                  d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4"
                />
              </svg>
            ),
            count: filteredRooms.length,
            label: "Räume",
          }}
          search={{
            value: searchTerm,
            onChange: setSearchTerm,
            placeholder: "Räume suchen...",
          }}
          filters={filters}
          activeFilters={activeFilters}
          onClearAllFilters={() => {
            setSearchTerm("");
            setCategoryFilter("all");
          }}
          actionButton={
            !isMobile && (
              <button
                onClick={() => setShowCreateModal(true)}
                className="group relative flex h-10 w-10 items-center justify-center rounded-full bg-gradient-to-br from-indigo-500 to-indigo-600 text-white shadow-lg transition-all duration-300 hover:scale-110 hover:shadow-xl active:scale-95"
                style={{
                  background:
                    "linear-gradient(135deg, rgb(99, 102, 241) 0%, rgb(79, 70, 229) 100%)",
                  willChange: "transform, opacity",
                  WebkitTransform: "translateZ(0)",
                  transform: "translateZ(0)",
                }}
                aria-label="Raum erstellen"
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

      {/* Mobile FAB */}
      <button
        onClick={() => setShowCreateModal(true)}
        className="group pointer-events-auto fixed right-4 bottom-24 z-40 flex h-14 w-14 translate-y-0 items-center justify-center rounded-full bg-gradient-to-br from-indigo-500 to-indigo-600 text-white opacity-100 shadow-[0_8px_30px_rgb(0,0,0,0.12)] transition-all duration-300 ease-out hover:shadow-[0_8px_40px_rgb(79,70,229,0.3)] active:scale-95 md:hidden"
        style={{
          background:
            "linear-gradient(135deg, rgb(99, 102, 241) 0%, rgb(79, 70, 229) 100%)",
          willChange: "transform, opacity",
          WebkitTransform: "translateZ(0)",
          transform: "translateZ(0)",
        }}
        aria-label="Raum erstellen"
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

      {/* Error */}
      {error && (
        <div className="mb-6 rounded-lg border border-red-200 bg-red-50 p-4">
          <p className="text-sm text-red-800">{error}</p>
        </div>
      )}

      {/* List */}
      {filteredRooms.length === 0 ? (
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
                d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4"
              />
            </svg>
            <h3 className="mt-4 text-lg font-medium text-gray-900">
              {searchTerm || categoryFilter !== "all"
                ? "Keine Räume gefunden"
                : "Keine Räume vorhanden"}
            </h3>
            <p className="mt-2 text-sm text-gray-600">
              {searchTerm || categoryFilter !== "all"
                ? "Versuchen Sie andere Suchkriterien oder Filter."
                : "Es wurden noch keine Räume erstellt."}
            </p>
          </div>
        </div>
      ) : (
        <div className="space-y-3">
          {filteredRooms.map((room, index) => {
            const initial = room.name?.charAt(0)?.toUpperCase() ?? "R";
            const handleClick = () => void handleSelectRoom(room);

            return (
              <button
                type="button"
                key={room.id}
                onClick={handleClick}
                className="group relative w-full cursor-pointer overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 text-left shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-500 active:scale-[0.99] md:hover:-translate-y-1 md:hover:scale-[1.01] md:hover:border-indigo-200/50 md:hover:bg-white md:hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)]"
                style={{
                  animationName: "fadeInUp",
                  animationDuration: "0.5s",
                  animationTimingFunction: "ease-out",
                  animationFillMode: "forwards",
                  animationDelay: `${index * 0.03}s`,
                  opacity: 0,
                }}
              >
                <div className="pointer-events-none absolute inset-0 rounded-3xl bg-gradient-to-br from-indigo-50/80 to-blue-100/80 opacity-[0.03]"></div>
                <div className="pointer-events-none absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
                <div className="pointer-events-none absolute inset-0 rounded-3xl ring-1 ring-white/20 transition-all duration-300 md:group-hover:ring-indigo-200/60"></div>

                <div className="relative flex items-center gap-4 p-5">
                  <div className="flex-shrink-0">
                    <div className="flex h-12 w-12 items-center justify-center rounded-full bg-gradient-to-br from-indigo-500 to-indigo-600 font-semibold text-white shadow-md transition-transform duration-300 md:group-hover:scale-110">
                      {initial}
                    </div>
                  </div>

                  <div className="min-w-0 flex-1">
                    <h3 className="text-lg font-semibold text-gray-900 transition-colors duration-300 md:group-hover:text-indigo-600">
                      {room.name}
                    </h3>
                    <div className="mt-1 flex flex-wrap items-center gap-2">
                      {room.building && room.floor !== undefined && (
                        <span className="inline-flex items-center rounded-full bg-gray-100 px-2 py-1 text-xs font-medium text-gray-700">
                          {room.building} • Etage {room.floor}
                        </span>
                      )}
                      {room.building && room.floor === undefined && (
                        <span className="inline-flex items-center rounded-full bg-gray-100 px-2 py-1 text-xs font-medium text-gray-700">
                          {room.building}
                        </span>
                      )}
                      {!room.building && room.floor !== undefined && (
                        <span className="inline-flex items-center rounded-full bg-gray-100 px-2 py-1 text-xs font-medium text-gray-700">
                          Etage {room.floor}
                        </span>
                      )}
                      {room.capacity ? (
                        <span className="inline-flex items-center rounded-full bg-blue-100 px-2 py-1 text-xs font-medium text-blue-700">
                          {room.capacity} Plätze
                        </span>
                      ) : null}
                    </div>
                  </div>

                  <div className="flex-shrink-0">
                    <svg
                      className="h-6 w-6 text-gray-400 transition-all duration-300 md:group-hover:translate-x-1 md:group-hover:text-indigo-600"
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

                <div className="pointer-events-none absolute inset-0 rounded-3xl bg-gradient-to-r from-transparent via-indigo-100/30 to-transparent opacity-0 transition-opacity duration-300 md:group-hover:opacity-100"></div>
              </button>
            );
          })}
        </div>
      )}

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
          onClose={() => {
            setShowDetailModal(false);
            setSelectedRoom(null);
          }}
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
          onClose={() => {
            setShowEditModal(false);
          }}
          room={selectedRoom}
          onSave={handleUpdateRoom}
          loading={detailLoading}
        />
      )}

      {/* Success toasts are handled globally */}
    </DatabasePageLayout>
  );
}

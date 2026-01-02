"use client";

import { useEffect, useMemo, useState, useCallback } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import type {
  FilterConfig,
  ActiveFilter,
} from "~/components/ui/page-header/types";
import { getDbOperationMessage } from "@/lib/use-notification";
import { createCrudService } from "@/lib/database/service-factory";
import { groupsConfig } from "@/lib/database/configs/groups.config";
import type { Group } from "@/lib/group-helpers";
import {
  GroupCreateModal,
  GroupDetailModal,
  GroupEditModal,
} from "@/components/groups";
import { useToast } from "~/contexts/ToastContext";
import { useIsMobile } from "~/hooks/useIsMobile";
import { MobileBackButton } from "~/components/ui/mobile-back-button";

import { Loading } from "~/components/ui/loading";

export default function GroupsPage() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [groups, setGroups] = useState<Group[]>([]);
  const [searchTerm, setSearchTerm] = useState("");
  const [roomFilter, setRoomFilter] = useState<string>("all");
  const isMobile = useIsMobile();

  const [showCreateModal, setShowCreateModal] = useState(false);
  const [createLoading, setCreateLoading] = useState(false);
  const [showDetailModal, setShowDetailModal] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);
  const [selectedGroup, setSelectedGroup] = useState<Group | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);

  const { success: toastSuccess } = useToast();

  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  const service = useMemo(() => createCrudService(groupsConfig), []);

  const fetchGroups = useCallback(async () => {
    try {
      setLoading(true);
      const data = await service.getList({ page: 1, pageSize: 500 });
      const arr = Array.isArray(data.data) ? data.data : [];
      setGroups(arr);
      setError(null);
    } catch (err) {
      console.error("Error fetching groups:", err);
      setError(
        "Fehler beim Laden der Gruppen. Bitte versuchen Sie es später erneut.",
      );
      setGroups([]);
    } finally {
      setLoading(false);
    }
  }, [service]);

  useEffect(() => {
    void fetchGroups();
  }, [fetchGroups]);

  const uniqueRooms = useMemo(() => {
    const set = new Set<string>();
    groups.forEach((g) => {
      if (g.room_name) set.add(g.room_name);
    });
    return Array.from(set)
      .sort((a, b) => a.localeCompare(b, "de"))
      .map((r) => ({ value: r, label: r }));
  }, [groups]);

  const filters: FilterConfig[] = useMemo(
    () => [
      {
        id: "room",
        label: "Raum",
        type: "dropdown",
        value: roomFilter,
        onChange: (v) => setRoomFilter(v as string),
        options: [{ value: "all", label: "Alle Räume" }, ...uniqueRooms],
      },
    ],
    [roomFilter, uniqueRooms],
  );

  const activeFilters: ActiveFilter[] = useMemo(() => {
    const list: ActiveFilter[] = [];
    if (searchTerm)
      list.push({
        id: "search",
        label: `"${searchTerm}"`,
        onRemove: () => setSearchTerm(""),
      });
    if (roomFilter !== "all")
      list.push({
        id: "room",
        label: roomFilter,
        onRemove: () => setRoomFilter("all"),
      });
    return list;
  }, [searchTerm, roomFilter]);

  const filteredGroups = useMemo(() => {
    let arr = [...groups];
    if (searchTerm) {
      const q = searchTerm.toLowerCase();
      arr = arr.filter(
        (g) =>
          g.name.toLowerCase().includes(q) ||
          (g.room_name?.toLowerCase().includes(q) ?? false) ||
          (g.representative_name?.toLowerCase().includes(q) ?? false),
      );
    }
    if (roomFilter !== "all") {
      arr = arr.filter((g) => g.room_name === roomFilter);
    }
    arr.sort((a, b) => a.name.localeCompare(b.name, "de"));
    return arr;
  }, [groups, searchTerm, roomFilter]);

  const handleSelectGroup = async (group: Group) => {
    setSelectedGroup(group);
    setShowDetailModal(true);
    try {
      setDetailLoading(true);
      const fresh = await service.getOne(group.id);
      setSelectedGroup(fresh);
    } finally {
      setDetailLoading(false);
    }
  };

  const handleCreateGroup = async (data: Partial<Group>) => {
    try {
      setCreateLoading(true);
      if (groupsConfig.form.transformBeforeSubmit)
        data = groupsConfig.form.transformBeforeSubmit(data);
      const created = await service.create(data);
      toastSuccess(
        getDbOperationMessage(
          "create",
          groupsConfig.name.singular,
          created.name,
        ),
      );
      setShowCreateModal(false);
      await fetchGroups();
    } finally {
      setCreateLoading(false);
    }
  };

  const handleUpdateGroup = async (data: Partial<Group>) => {
    if (!selectedGroup) return;
    try {
      setDetailLoading(true);
      if (groupsConfig.form.transformBeforeSubmit)
        data = groupsConfig.form.transformBeforeSubmit(data);
      await service.update(selectedGroup.id, data);
      toastSuccess(
        getDbOperationMessage(
          "update",
          groupsConfig.name.singular,
          selectedGroup.name,
        ),
      );
      const refreshed = await service.getOne(selectedGroup.id);
      setSelectedGroup(refreshed);
      setShowEditModal(false);
      setShowDetailModal(true);
      await fetchGroups();
    } finally {
      setDetailLoading(false);
    }
  };

  const handleDeleteGroup = async () => {
    if (!selectedGroup) return;
    try {
      setDetailLoading(true);
      await service.delete(selectedGroup.id);
      toastSuccess(
        getDbOperationMessage(
          "delete",
          groupsConfig.name.singular,
          selectedGroup.name,
        ),
      );
      setShowDetailModal(false);
      setSelectedGroup(null);
      await fetchGroups();
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
        <MobileBackButton />

        <div className="mb-4">
          <PageHeaderWithSearch
            title={isMobile ? "Gruppen" : ""}
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
                    d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0z"
                  />
                </svg>
              ),
              count: filteredGroups.length,
              label: "Gruppen",
            }}
            search={{
              value: searchTerm,
              onChange: setSearchTerm,
              placeholder: "Gruppen suchen...",
            }}
            filters={filters}
            activeFilters={activeFilters}
            onClearAllFilters={() => {
              setSearchTerm("");
              setRoomFilter("all");
            }}
            actionButton={
              !isMobile && (
                <button
                  onClick={() => setShowCreateModal(true)}
                  className="group relative flex h-10 w-10 items-center justify-center rounded-full bg-gradient-to-br from-[#83CD2D] to-[#70b525] text-white shadow-lg transition-all duration-300 hover:scale-110 hover:shadow-xl active:scale-95"
                  style={{
                    background:
                      "linear-gradient(135deg, rgb(131, 205, 45) 0%, rgb(112, 181, 37) 100%)",
                    willChange: "transform, opacity",
                    WebkitTransform: "translateZ(0)",
                    transform: "translateZ(0)",
                  }}
                  aria-label="Gruppe erstellen"
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
          className="group pointer-events-auto fixed right-4 bottom-24 z-40 flex h-14 w-14 translate-y-0 items-center justify-center rounded-full bg-gradient-to-br from-[#83CD2D] to-[#70b525] text-white opacity-100 shadow-[0_8px_30px_rgb(0,0,0,0.12)] transition-all duration-300 ease-out hover:shadow-[0_8px_40px_rgba(112,181,37,0.3)] active:scale-95 md:hidden"
          style={{
            background:
              "linear-gradient(135deg, rgb(131, 205, 45) 0%, rgb(112, 181, 37) 100%)",
            willChange: "transform, opacity",
            WebkitTransform: "translateZ(0)",
            transform: "translateZ(0)",
          }}
          aria-label="Gruppe erstellen"
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

        {filteredGroups.length === 0 ? (
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
                  d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0z"
                />
              </svg>
              <h3 className="mt-4 text-lg font-medium text-gray-900">
                {searchTerm || roomFilter !== "all"
                  ? "Keine Gruppen gefunden"
                  : "Keine Gruppen vorhanden"}
              </h3>
              <p className="mt-2 text-sm text-gray-600">
                {searchTerm || roomFilter !== "all"
                  ? "Versuchen Sie andere Suchkriterien oder Filter."
                  : "Es wurden noch keine Gruppen erstellt."}
              </p>
            </div>
          </div>
        ) : (
          <div className="space-y-3">
            {filteredGroups.map((group, index) => {
              const handleClick = () => void handleSelectGroup(group);
              return (
                <button
                  type="button"
                  key={group.id}
                  onClick={handleClick}
                  className="group relative w-full cursor-pointer overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 text-left shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-500 active:scale-[0.99] md:hover:-translate-y-1 md:hover:scale-[1.01] md:hover:border-[#83CD2D]/50 md:hover:bg-white md:hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)]"
                  style={{
                    animationName: "fadeInUp",
                    animationDuration: "0.5s",
                    animationTimingFunction: "ease-out",
                    animationFillMode: "forwards",
                    animationDelay: `${index * 0.03}s`,
                    opacity: 0,
                  }}
                >
                  <div className="pointer-events-none absolute inset-0 rounded-3xl bg-gradient-to-br from-green-50/80 to-emerald-100/80 opacity-[0.03]"></div>
                  <div className="pointer-events-none absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
                  <div className="pointer-events-none absolute inset-0 rounded-3xl ring-1 ring-white/20 transition-all duration-300 md:group-hover:ring-[#83CD2D]/50"></div>

                  <div className="relative flex items-center gap-4 p-5">
                    <div className="flex-shrink-0">
                      <div className="flex h-12 w-12 items-center justify-center rounded-full bg-gradient-to-br from-[#83CD2D] to-[#70b525] font-semibold text-white shadow-md transition-transform duration-300 md:group-hover:scale-110">
                        {group.name?.charAt(0)?.toUpperCase() ?? "G"}
                      </div>
                    </div>
                    <div className="min-w-0 flex-1">
                      <h3 className="text-lg font-semibold text-gray-900 transition-colors duration-300 md:group-hover:text-[#70b525]">
                        {group.name}
                      </h3>
                      <div className="mt-1 flex flex-wrap items-center gap-2">
                        {group.room_name && (
                          <span className="inline-flex items-center rounded-full bg-gray-100 px-2 py-1 text-xs font-medium text-gray-700">
                            {group.room_name}
                          </span>
                        )}
                      </div>
                    </div>
                    <div className="flex-shrink-0">
                      <svg
                        className="h-6 w-6 text-gray-400 transition-all duration-300 md:group-hover:translate-x-1 md:group-hover:text-[#70b525]"
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

                  <div className="pointer-events-none absolute inset-0 rounded-3xl bg-gradient-to-r from-transparent via-green-100/30 to-transparent opacity-0 transition-opacity duration-300 md:group-hover:opacity-100"></div>
                </button>
              );
            })}
          </div>
        )}
      </div>

      {/* Create */}
      <GroupCreateModal
        isOpen={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        onCreate={handleCreateGroup}
        loading={createLoading}
      />

      {/* Detail */}
      {selectedGroup && (
        <GroupDetailModal
          isOpen={showDetailModal}
          onClose={() => {
            setShowDetailModal(false);
            setSelectedGroup(null);
          }}
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
          onClose={() => {
            setShowEditModal(false);
          }}
          group={selectedGroup}
          onSave={handleUpdateGroup}
          loading={detailLoading}
        />
      )}

      {/* Success toasts handled globally */}
    </ResponsiveLayout>
  );
}

"use client";

import { useState } from "react";
import { DatabasePage } from "@/components/ui/database/database-page";
import { groupsConfig } from "@/lib/database/configs/groups.config";
import { GroupStudentEnrollmentModal } from "@/components/groups";
import type { Group } from "@/lib/group-helpers";

export default function GroupsPage() {
  const [studentModalOpen, setStudentModalOpen] = useState(false);
  const [selectedGroup, setSelectedGroup] = useState<Group | null>(null);
  const [refreshKey, setRefreshKey] = useState(0);

  // Create a modified config with our custom action handler
  const modifiedConfig = {
    ...groupsConfig,
    detail: {
      ...groupsConfig.detail,
      actions: {
        ...groupsConfig.detail.actions,
        custom: [
          {
            label: 'Schüler verwalten',
            onClick: (group: Group) => {
              setSelectedGroup(group);
              setStudentModalOpen(true);
            },
            color: 'bg-blue-600 text-white hover:bg-blue-700',
          },
        ],
      },
    },
  };

  const handleStudentUpdate = () => {
    // Trigger a refresh of the groups list
    setRefreshKey(prev => prev + 1);
  };

  return (
    <>
      <DatabasePage key={refreshKey} config={modifiedConfig} />
      
      {selectedGroup && (
        <GroupStudentEnrollmentModal
          isOpen={studentModalOpen}
          onClose={() => {
            setStudentModalOpen(false);
            setSelectedGroup(null);
          }}
          group={selectedGroup}
          onUpdate={handleStudentUpdate}
        />
      )}
    </>
  );
}

"use client";

import { useState } from "react";
import { DatabasePage } from "@/components/ui/database/database-page";
import { teachersConfig } from "@/lib/database/configs/teachers.config";
import { TeacherRoleManagementModal, TeacherPermissionManagementModal } from "@/components/teachers";
import type { Teacher } from "@/lib/teacher-api";

export default function TeachersPage() {
  const [roleModalOpen, setRoleModalOpen] = useState(false);
  const [permissionModalOpen, setPermissionModalOpen] = useState(false);
  const [selectedTeacher, setSelectedTeacher] = useState<Teacher | null>(null);
  const [refreshKey, setRefreshKey] = useState(0);

  // Create a modified config with our custom action handlers
  const modifiedConfig = {
    ...teachersConfig,
    detail: {
      ...teachersConfig.detail,
      actions: {
        ...teachersConfig.detail.actions,
        custom: [
          {
            label: 'Rollen verwalten',
            onClick: (teacher: Teacher) => {
              console.log('Rollen verwalten clicked - teacher:', teacher);
              setSelectedTeacher(teacher);
              setRoleModalOpen(true);
            },
            color: 'bg-purple-600 text-white hover:bg-purple-700',
          },
          {
            label: 'Berechtigungen verwalten',
            onClick: (teacher: Teacher) => {
              console.log('Berechtigungen verwalten clicked - teacher:', teacher);
              setSelectedTeacher(teacher);
              setPermissionModalOpen(true);
            },
            color: 'bg-indigo-600 text-white hover:bg-indigo-700',
          },
        ],
      },
    },
  };

  const handleUpdate = () => {
    // Trigger a refresh of the teachers list
    setRefreshKey(prev => prev + 1);
  };

  return (
    <>
      <DatabasePage key={refreshKey} config={modifiedConfig} />
      
      {selectedTeacher && (
        <>
          <TeacherRoleManagementModal
            isOpen={roleModalOpen}
            onClose={() => {
              setRoleModalOpen(false);
              setSelectedTeacher(null);
            }}
            teacher={selectedTeacher}
            onUpdate={handleUpdate}
          />
          
          <TeacherPermissionManagementModal
            isOpen={permissionModalOpen}
            onClose={() => {
              setPermissionModalOpen(false);
              setSelectedTeacher(null);
            }}
            teacher={selectedTeacher}
            onUpdate={handleUpdate}
          />
        </>
      )}
    </>
  );
}
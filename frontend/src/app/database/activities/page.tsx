"use client";

import { useState } from "react";
import { DatabasePage } from "@/components/ui/database/database-page";
import { activitiesConfig } from "@/lib/database/configs/activities.config";
import { StudentEnrollmentModal } from "@/components/activities/student-enrollment-modal";
import type { Activity } from "@/lib/activity-helpers";

export default function ActivitiesPage() {
  const [studentModalOpen, setStudentModalOpen] = useState(false);
  const [selectedActivity, setSelectedActivity] = useState<Activity | null>(null);
  const [refreshKey, setRefreshKey] = useState(0);

  // Create a modified config with our custom action handler
  const modifiedConfig = {
    ...activitiesConfig,
    detail: {
      ...activitiesConfig.detail,
      actions: {
        ...activitiesConfig.detail.actions,
        custom: [
          {
            label: 'Schüler verwalten',
            onClick: (activity: Activity) => {
              setSelectedActivity(activity);
              setStudentModalOpen(true);
            },
            color: 'bg-blue-600 text-white hover:bg-blue-700',
          },
          {
            label: 'Zeiten verwalten',
            onClick: (activity: Activity) => {
              window.location.href = `/database/activities/${activity.id}/times`;
            },
            color: 'bg-green-600 text-white hover:bg-green-700',
          },
        ],
      },
    },
  };

  const handleStudentUpdate = () => {
    // Trigger a refresh of the activities list
    setRefreshKey(prev => prev + 1);
  };

  return (
    <>
      <DatabasePage key={refreshKey} config={modifiedConfig} />
      
      {selectedActivity && (
        <StudentEnrollmentModal
          isOpen={studentModalOpen}
          onClose={() => {
            setStudentModalOpen(false);
            setSelectedActivity(null);
          }}
          activity={selectedActivity}
          onUpdate={handleStudentUpdate}
        />
      )}
    </>
  );
}
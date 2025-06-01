"use client";

import { useState, useEffect } from "react";
import { FormModal, Notification } from "~/components/ui";
import { useNotification } from "~/lib/use-notification";
import * as activityService from "~/lib/activity-api";
import type { Activity, ActivityStudent } from "~/lib/activity-helpers";

// Type for available students returned by the API
type AvailableStudent = {
  id: string;
  name: string;
  school_class: string;
};

interface StudentEnrollmentModalProps {
  isOpen: boolean;
  onClose: () => void;
  activity: Activity;
  onUpdate: () => void;
}

export function StudentEnrollmentModal({
  isOpen,
  onClose,
  activity,
  onUpdate,
}: StudentEnrollmentModalProps) {
  const { notification, showSuccess, showError, showWarning } = useNotification();
  const [enrolledStudents, setEnrolledStudents] = useState<ActivityStudent[]>([]);
  const [availableStudents, setAvailableStudents] = useState<AvailableStudent[]>([]);
  const [selectedStudents, setSelectedStudents] = useState<string[]>([]);
  const [searchTerm, setSearchTerm] = useState("");
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [activeTab, setActiveTab] = useState<"enrolled" | "available">("enrolled");

  // Fetch enrolled students
  const fetchEnrolledStudents = async () => {
    try {
      const students = await activityService.getEnrolledStudents(activity.id);
      setEnrolledStudents(students);
    } catch (error) {
      console.error("Error fetching enrolled students:", error);
      showError("Fehler beim Laden der eingeschriebenen Schüler");
    }
  };

  // Fetch available students
  const fetchAvailableStudents = async () => {
    try {
      const students = await activityService.getAvailableStudents(activity.id, {
        search: searchTerm,
      });
      setAvailableStudents(students);
    } catch (error) {
      console.error("Error fetching available students:", error);
      showError("Fehler beim Laden verfügbarer Schüler");
    }
  };

  useEffect(() => {
    if (isOpen) {
      setLoading(true);
      void Promise.all([fetchEnrolledStudents(), fetchAvailableStudents()])
        .finally(() => setLoading(false));
    }
  }, [isOpen, activity.id]);

  useEffect(() => {
    if (isOpen && activeTab === "available") {
      void fetchAvailableStudents();
    }
  }, [searchTerm, activeTab]);

  const handleToggleStudent = (studentId: string) => {
    setSelectedStudents((prev) =>
      prev.includes(studentId)
        ? prev.filter((id) => id !== studentId)
        : [...prev, studentId]
    );
  };

  const handleEnrollSelected = async () => {
    if (selectedStudents.length === 0) {
      showWarning("Bitte wählen Sie mindestens einen Schüler aus");
      return;
    }

    const remainingSlots = activity.max_participant - enrolledStudents.length;
    if (selectedStudents.length > remainingSlots) {
      showError(
        `Nur ${remainingSlots} Plätze verfügbar. Sie haben ${selectedStudents.length} Schüler ausgewählt.`
      );
      return;
    }

    try {
      setSaving(true);
      const newStudentIds = [
        ...enrolledStudents.map((e) => e.student_id),
        ...selectedStudents,
      ];
      await activityService.updateGroupEnrollments(activity.id, {
        student_ids: newStudentIds,
      });
      showSuccess("Schüler erfolgreich hinzugefügt");
      setSelectedStudents([]);
      setActiveTab("enrolled");
      await fetchEnrolledStudents();
      onUpdate();
    } catch (error) {
      console.error("Error enrolling students:", error);
      showError("Fehler beim Hinzufügen der Schüler");
    } finally {
      setSaving(false);
    }
  };

  const handleUnenroll = async (studentId: string) => {
    try {
      setSaving(true);
      
      // Ensure we have a valid student ID
      if (!studentId) {
        showError("Ungültige Schüler-ID");
        return;
      }
      
      // Use the DELETE endpoint to remove individual student
      await activityService.unenrollStudent(activity.id, studentId);
      
      showSuccess("Schüler erfolgreich entfernt");
      await fetchEnrolledStudents();
      onUpdate();
    } catch (error) {
      console.error("Error unenrolling student:", error);
      // Show specific error message if available
      if (error instanceof Error) {
        showError(error.message);
      } else {
        showError("Fehler beim Entfernen des Schülers");
      }
    } finally {
      setSaving(false);
    }
  };

  const footer = activeTab === "available" && selectedStudents.length > 0 && (
    <button
      onClick={handleEnrollSelected}
      disabled={saving}
      className="rounded-lg bg-blue-600 px-6 py-2 text-white hover:bg-blue-700 disabled:opacity-50"
    >
      {saving ? "Wird gespeichert..." : `${selectedStudents.length} Schüler hinzufügen`}
    </button>
  );

  return (
    <>
      {/* Notification for success/error messages */}
      <Notification notification={notification} className="fixed top-4 right-4 z-[10000] max-w-sm" />
      
      <FormModal
        isOpen={isOpen}
        onClose={onClose}
        title={`Schüler verwalten - ${activity.name}`}
        size="xl"
        footer={footer}
      >
      <div className="space-y-4">
        {/* Stats */}
        <div className="rounded-lg bg-gray-50 p-4">
          <div className="flex justify-between items-center">
            <span className="text-sm text-gray-600">Belegung:</span>
            <span className="font-semibold">
              {enrolledStudents.length} / {activity.max_participant} Plätze
            </span>
          </div>
        </div>

        {/* Tabs */}
        <div className="flex border-b border-gray-200">
          <button
            onClick={() => setActiveTab("enrolled")}
            className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
              activeTab === "enrolled"
                ? "border-blue-500 text-blue-600"
                : "border-transparent text-gray-500 hover:text-gray-700"
            }`}
          >
            Eingeschrieben ({enrolledStudents.length})
          </button>
          <button
            onClick={() => setActiveTab("available")}
            className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
              activeTab === "available"
                ? "border-blue-500 text-blue-600"
                : "border-transparent text-gray-500 hover:text-gray-700"
            }`}
          >
            Verfügbare Schüler
          </button>
        </div>

        {/* Content */}
        {loading ? (
          <div className="text-center py-8 text-gray-500">Laden...</div>
        ) : (
          <>
            {activeTab === "enrolled" && (
              <div className="space-y-2 max-h-96 overflow-y-auto">
                {enrolledStudents.length === 0 ? (
                  <p className="text-center py-8 text-gray-500">
                    Keine Schüler eingeschrieben
                  </p>
                ) : (
                  enrolledStudents.map((enrollment) => (
                    <div
                      key={enrollment.id}
                      className="flex items-center justify-between p-3 bg-white rounded-lg border border-gray-200"
                    >
                      <div>
                        <div className="font-medium">
                          {enrollment.name ?? 'Unbekannt'}
                        </div>
                        {enrollment.school_class && (
                          <div className="text-sm text-gray-600">
                            Klasse: {enrollment.school_class}
                          </div>
                        )}
                      </div>
                      <button
                        onClick={() => void handleUnenroll(enrollment.student_id)}
                        disabled={saving}
                        className="text-red-600 hover:text-red-800 text-sm font-medium disabled:opacity-50"
                      >
                        Entfernen
                      </button>
                    </div>
                  ))
                )}
              </div>
            )}

            {activeTab === "available" && (
              <div className="space-y-4">
                {/* Search */}
                <input
                  type="text"
                  placeholder="Schüler suchen..."
                  value={searchTerm}
                  onChange={(e) => setSearchTerm(e.target.value)}
                  className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                />

                {/* Available students list */}
                <div className="space-y-2 max-h-80 overflow-y-auto border border-gray-200 rounded-lg">
                  {availableStudents.length === 0 ? (
                    <p className="text-center py-8 text-gray-500">
                      Keine verfügbaren Schüler gefunden
                    </p>
                  ) : (
                    availableStudents.map((student) => (
                      <label
                        key={student.id}
                        className="flex items-center p-3 hover:bg-gray-50 cursor-pointer"
                      >
                        <input
                          type="checkbox"
                          checked={selectedStudents.includes(student.id)}
                          onChange={() => handleToggleStudent(student.id)}
                          className="mr-3 h-4 w-4 text-blue-600 rounded focus:ring-blue-500"
                        />
                        <div className="flex-1">
                          <div className="font-medium">{student.name}</div>
                          {student.school_class && (
                            <div className="text-sm text-gray-600">
                              Klasse: {student.school_class}
                            </div>
                          )}
                        </div>
                      </label>
                    ))
                  )}
                </div>
              </div>
            )}
          </>
        )}
      </div>
      </FormModal>
    </>
  );
}
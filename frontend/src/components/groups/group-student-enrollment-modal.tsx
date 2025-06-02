"use client";

import { useState, useEffect } from "react";
import { FormModal, Notification } from "~/components/ui";
import { useNotification } from "~/lib/use-notification";
import type { Group } from "~/lib/group-helpers";

// Type for students in a group
type GroupStudent = {
  id: string;
  person_id: string;
  first_name: string;
  last_name: string;
  school_class: string;
  guardian_name?: string;
  location?: string;
  bus: boolean;
};

// Type for available students (matches Student type from student-helpers)
type AvailableStudent = {
  id: string;
  person_id?: string;
  name: string;
  first_name?: string;
  second_name?: string;
  school_class: string;
  group_id?: string;
  group_name?: string;
};

interface GroupStudentEnrollmentModalProps {
  isOpen: boolean;
  onClose: () => void;
  group: Group;
  onUpdate: () => void;
}

export function GroupStudentEnrollmentModal({
  isOpen,
  onClose,
  group,
  onUpdate,
}: GroupStudentEnrollmentModalProps) {
  const { notification, showSuccess, showError, showWarning } = useNotification();
  const [enrolledStudents, setEnrolledStudents] = useState<GroupStudent[]>([]);
  const [availableStudents, setAvailableStudents] = useState<AvailableStudent[]>([]);
  const [selectedStudents, setSelectedStudents] = useState<string[]>([]);
  const [searchTerm, setSearchTerm] = useState("");
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [activeTab, setActiveTab] = useState<"enrolled" | "available">("enrolled");

  // Fetch students in the group
  const fetchEnrolledStudents = async () => {
    try {
      const response = await fetch(`/api/groups/${group.id}/students`);
      if (!response.ok) {
        throw new Error('Failed to fetch group students');
      }
      const result = await response.json() as { data?: GroupStudent[]; status?: string; message?: string } | GroupStudent[];
      
      // Debug logging
      console.log("Group students response:", result);
      
      // Handle both wrapped and unwrapped responses
      let data: GroupStudent[] = [];
      if (Array.isArray(result)) {
        data = result;
      } else if (result && typeof result === 'object' && 'data' in result) {
        data = Array.isArray(result.data) ? result.data : [];
      }
      
      console.log("Parsed enrolled students:", data);
      setEnrolledStudents(data);
    } catch (error) {
      console.error("Error fetching enrolled students:", error);
      showError("Fehler beim Laden der Gruppenschüler");
    }
  };

  // Fetch available students (those not in any group)
  const fetchAvailableStudents = async () => {
    try {
      // Fetch all students with optional search
      const params = new URLSearchParams();
      if (searchTerm) {
        params.append('search', searchTerm);
      }
      
      const response = await fetch(`/api/students?${params.toString()}`);
      if (!response.ok) {
        throw new Error('Failed to fetch students');
      }
      const result = await response.json() as { data?: AvailableStudent[]; pagination?: unknown } | AvailableStudent[];
      
      console.log("Raw students API response:", result);
      
      // Handle the wrapped response structure
      let allStudents: AvailableStudent[] = [];
      if (Array.isArray(result)) {
        allStudents = result;
      } else if (result && typeof result === 'object' && 'data' in result) {
        // The response is wrapped, check if data contains the students array
        const data = result.data as any;
        console.log("Data property content:", data);
        
        if (Array.isArray(data)) {
          allStudents = data;
        } else if (data && typeof data === 'object' && 'data' in data && Array.isArray(data.data)) {
          // Double wrapped - paginated response inside wrapped response
          allStudents = data.data;
        }
      }
      
      console.log("All students fetched:", allStudents);
      console.log("Current group ID:", group.id);
      
      // Show all students except those already in this specific group
      // Students in other groups can be moved to this group
      const availableStudents = allStudents.filter((student) => {
        // Convert both IDs to strings for comparison
        const studentGroupId = student.group_id ? String(student.group_id) : null;
        const currentGroupId = String(group.id);
        
        // Student is available if they're not in this group
        // (they can be in no group or a different group)
        const isNotInThisGroup = studentGroupId !== currentGroupId;
        
        console.log(`Student ${student.first_name || student.name}: group_id=${studentGroupId}, current_group=${currentGroupId}, available=${isNotInThisGroup}`);
        return isNotInThisGroup;
      });
      
      console.log("Available students after filtering:", availableStudents);
      setAvailableStudents(availableStudents);
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
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isOpen, group.id]);

  useEffect(() => {
    if (isOpen && activeTab === "available") {
      void fetchAvailableStudents();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [searchTerm, activeTab, isOpen]);

  const handleToggleStudent = (studentId: string) => {
    setSelectedStudents((prev) =>
      prev.includes(studentId)
        ? prev.filter((id) => id !== studentId)
        : [...prev, studentId]
    );
  };

  const handleAssignSelected = async () => {
    if (selectedStudents.length === 0) {
      showWarning("Bitte wählen Sie mindestens einen Schüler aus");
      return;
    }

    try {
      setSaving(true);
      
      // Assign each selected student to the group
      const assignPromises = selectedStudents.map(async (studentId) => {
        const response = await fetch(`/api/students/${studentId}`, {
          method: 'PATCH',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({
            group_id: parseInt(group.id),
          }),
        });
        
        if (!response.ok) {
          throw new Error(`Failed to assign student ${studentId}`);
        }
      });
      
      await Promise.all(assignPromises);
      
      showSuccess("Schüler erfolgreich zur Gruppe hinzugefügt");
      setSelectedStudents([]);
      setActiveTab("enrolled");
      await fetchEnrolledStudents();
      onUpdate();
    } catch (error) {
      console.error("Error assigning students:", error);
      showError("Fehler beim Hinzufügen der Schüler zur Gruppe");
    } finally {
      setSaving(false);
    }
  };

  const handleRemoveFromGroup = async (studentId: string) => {
    try {
      setSaving(true);
      
      // Remove student from group by setting group_id to null
      const response = await fetch(`/api/students/${studentId}`, {
        method: 'PATCH',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          group_id: null,
        }),
      });
      
      if (!response.ok) {
        throw new Error('Failed to remove student from group');
      }
      
      showSuccess("Schüler erfolgreich aus der Gruppe entfernt");
      await fetchEnrolledStudents();
      onUpdate();
    } catch (error) {
      console.error("Error removing student from group:", error);
      showError("Fehler beim Entfernen des Schülers aus der Gruppe");
    } finally {
      setSaving(false);
    }
  };

  const footer = activeTab === "available" && selectedStudents.length > 0 && (
    <button
      onClick={handleAssignSelected}
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
        title={`Schüler verwalten - ${group.name}`}
        size="xl"
        footer={footer}
      >
        <div className="space-y-4">
          {/* Stats */}
          <div className="rounded-lg bg-gray-50 p-4">
            <div className="flex justify-between items-center">
              <span className="text-sm text-gray-600">Gruppengröße:</span>
              <span className="font-semibold">
                {Array.isArray(enrolledStudents) ? enrolledStudents.length : 0} Schüler
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
              In der Gruppe ({Array.isArray(enrolledStudents) ? enrolledStudents.length : 0})
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
                  {!Array.isArray(enrolledStudents) || enrolledStudents.length === 0 ? (
                    <p className="text-center py-8 text-gray-500">
                      Keine Schüler in dieser Gruppe
                    </p>
                  ) : (
                    enrolledStudents.map((student) => (
                      <div
                        key={student.id}
                        className="flex items-center justify-between p-3 bg-white rounded-lg border border-gray-200"
                      >
                        <div>
                          <div className="font-medium">
                            {student.first_name} {student.last_name}
                          </div>
                          <div className="text-sm text-gray-600">
                            Klasse: {student.school_class}
                          </div>
                        </div>
                        <button
                          onClick={() => void handleRemoveFromGroup(student.id)}
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
                            <div className="font-medium">
                              {student.name || `${student.first_name || ''} ${student.second_name || ''}`}
                            </div>
                            <div className="text-sm text-gray-600">
                              Klasse: {student.school_class}
                              {student.group_name && student.group_id && String(student.group_id) !== String(group.id) && (
                                <span className="ml-2 text-orange-600">
                                  • Bereits in Gruppe: {student.group_name}
                                </span>
                              )}
                            </div>
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
"use client";

import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { useState, useEffect } from "react";
import { teacherService, type Teacher } from "@/lib/teacher-api";
import { DatabaseListPage, SelectFilter, CreateFormModal, DetailFormModal, Notification } from "@/components/ui";
import { TeacherListItem, TeacherForm, TeacherDetailView } from "@/components/teachers";
import { useNotification, getDbOperationMessage } from "@/lib/use-notification";

export default function TeachersPage() {
  const { notification, showSuccess } = useNotification();
  const [teachers, setTeachers] = useState<Teacher[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchFilter, setSearchFilter] = useState("");
  const [roleFilter, setRoleFilter] = useState<string | null>(null);
  const [currentPage, setCurrentPage] = useState(1);
  const [pagination, setPagination] = useState<{
    current_page: number;
    page_size: number;
    total_pages: number;
    total_records: number;
  } | null>(null);
  
  // Create Modal states
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [createLoading, setCreateLoading] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const [rfidCards, setRfidCards] = useState<Array<{ id: string; label: string }>>([]);
  const [createdCredentials, setCreatedCredentials] = useState<{ email: string; password: string } | null>(null);

  // Detail Modal states
  const [showDetailModal, setShowDetailModal] = useState(false);
  const [selectedTeacher, setSelectedTeacher] = useState<Teacher | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState<string | null>(null);
  const [isEditing, setIsEditing] = useState(false);

  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  // Function to fetch RFID cards
  const fetchRfidCards = async () => {
    try {
      const response = await fetch("/api/users/rfid-cards/available");
      if (response.ok) {
        const responseData = await response.json() as { data?: Array<{ tag_id: string }> } | Array<{ tag_id: string }>;
        
        let cards: Array<{ tag_id: string }>;
        
        if (responseData && typeof responseData === 'object' && 'data' in responseData && responseData.data) {
          cards = responseData.data;
        } else if (Array.isArray(responseData)) {
          cards = responseData;
        } else {
          console.error("Unexpected RFID cards response format:", responseData);
          setRfidCards([]);
          return;
        }
        
        const transformedCards = cards.map((card, index) => ({
          id: card.tag_id, // Use the actual tag_id as the ID
          label: `RFID: ${card.tag_id}`
        }));
        
        setRfidCards(transformedCards);
      } else {
        console.error("Failed to fetch RFID cards:", response.status);
        setRfidCards([]);
      }
    } catch (err) {
      console.error("Error fetching RFID cards:", err);
      setRfidCards([]);
    }
  };

  // Function to fetch teachers with optional filters
  const fetchTeachers = async (search?: string, role?: string | null, page = 1) => {
    try {
      setLoading(true);

      // Prepare filters for API call
      const filters = {
        search: search ?? undefined,
        role: role ?? undefined,
        page: page,
        pageSize: 50
      };

      try {
        // Fetch from the real API using our teacher service
        const data = await teacherService.getTeachers(filters);

        // Ensure we got an array back
        if (!Array.isArray(data)) {
          console.error("API did not return an array:", data);
          setTeachers([]);
          setError("Unerwartetes Datenformat vom Server.");
          return;
        }

        if (data.length === 0 && !search) {
          console.log("No teachers returned from API, checking connection");
        }

        setTeachers(data);
        setError(null);
        // Note: Teacher API doesn't currently support pagination, but we'll prepare for it
        setPagination(null);
      } catch (apiErr) {
        console.error("API error when fetching teachers:", apiErr);
        setError(
          "Fehler beim Laden der Lehrerdaten. Bitte versuchen Sie es später erneut.",
        );
        setTeachers([]);
        setPagination(null);
      }
    } catch (err) {
      console.error("Error fetching teachers:", err);
      setError(
        "Fehler beim Laden der Lehrerdaten. Bitte versuchen Sie es später erneut.",
      );
      setTeachers([]);
      setPagination(null);
    } finally {
      setLoading(false);
    }
  };

  // Initial data load
  useEffect(() => {
    void fetchTeachers(undefined, undefined, currentPage);
    void fetchRfidCards();
  }, [currentPage]);

  // Handle search and role filter changes
  useEffect(() => {
    // Reset to first page when filters change
    setCurrentPage(1);
    // Debounce search to avoid too many API calls
    const timer = setTimeout(() => {
      void fetchTeachers(searchFilter, roleFilter, 1);
    }, 300);

    return () => clearTimeout(timer);
  }, [searchFilter, roleFilter]);

  if (status === "loading") {
    return <div />; // Let DatabaseListPage handle the loading state
  }

  const handleSelectTeacher = async (teacher: Teacher) => {
    setSelectedTeacher(teacher);
    setShowDetailModal(true);
    setDetailError(null);
    
    // Fetch fresh data for the selected teacher
    try {
      setDetailLoading(true);
      const freshData = await teacherService.getTeacher(teacher.id);
      setSelectedTeacher(freshData);
      
      // Fetch fresh RFID cards for editing
      await fetchRfidCards();
    } catch (err) {
      console.error("Error fetching teacher details:", err);
      setDetailError("Fehler beim Laden der Lehrerdaten.");
    } finally {
      setDetailLoading(false);
    }
  };

  // Handle teacher creation
  const handleCreateTeacher = async (teacherData: Partial<Teacher> & { password?: string }) => {
    try {
      setCreateLoading(true);
      setCreateError(null);

      // Ensure all required fields are set
      if (
        !teacherData.first_name ||
        !teacherData.last_name ||
        !teacherData.specialization
      ) {
        setCreateError("Bitte füllen Sie alle Pflichtfelder aus.");
        return;
      }

      // Create a complete teacher object with required fields
      const newTeacherData: Omit<Teacher, "id" | "name" | "created_at" | "updated_at"> & { password?: string } = {
        first_name: teacherData.first_name,
        last_name: teacherData.last_name,
        email: teacherData.email,
        specialization: teacherData.specialization,
        role: teacherData.role ?? null,
        qualifications: teacherData.qualifications ?? null,
        tag_id: teacherData.tag_id ?? null,
        staff_notes: teacherData.staff_notes ?? null,
        password: teacherData.password
      };

      // Create the teacher using the teacher service
      const newTeacher = await teacherService.createTeacher(newTeacherData);

      // If we got temporary credentials, show them to the user
      if (newTeacher.temporaryCredentials) {
        setCreatedCredentials(newTeacher.temporaryCredentials);
        showSuccess(getDbOperationMessage('create', 'Lehrer', `${newTeacher.first_name} ${newTeacher.last_name}`));
        // Don't close modal yet, show the credentials first
      } else {
        // Show success notification
        showSuccess(getDbOperationMessage('create', 'Lehrer', `${newTeacher.first_name} ${newTeacher.last_name}`));
        
        // Close modal and refresh list
        setShowCreateModal(false);
        await fetchTeachers(searchFilter, roleFilter, currentPage);
      }
    } catch (err) {
      console.error("Error creating teacher:", err);
      setCreateError(
        "Fehler beim Erstellen des Lehrers. Bitte versuchen Sie es später erneut."
      );
      throw err; // Re-throw for form to handle
    } finally {
      setCreateLoading(false);
    }
  };

  // Handle teacher update
  const handleUpdateTeacher = async (teacherData: Partial<Teacher>) => {
    if (!selectedTeacher) return;
    
    try {
      setDetailLoading(true);
      setDetailError(null);

      // Update teacher
      await teacherService.updateTeacher(
        selectedTeacher.id,
        teacherData
      );
      
      // Show success notification
      showSuccess(getDbOperationMessage('update', 'Lehrer', selectedTeacher.name));
      
      // Refresh the selected teacher data
      const refreshedTeacher = await teacherService.getTeacher(selectedTeacher.id);
      setSelectedTeacher(refreshedTeacher);
      setIsEditing(false);
      
      // Refresh the list
      await fetchTeachers(searchFilter, roleFilter, currentPage);
    } catch (err) {
      console.error("Error updating teacher:", err);
      setDetailError(
        "Fehler beim Aktualisieren des Lehrers. Bitte versuchen Sie es später erneut."
      );
      throw err;
    } finally {
      setDetailLoading(false);
    }
  };

  // Handle teacher deletion
  const handleDeleteTeacher = async () => {
    if (!selectedTeacher) return;
    
    if (window.confirm(`Sind Sie sicher, dass Sie den Lehrer "${selectedTeacher.name}" löschen möchten? Dies kann nicht rückgängig gemacht werden.`)) {
      try {
        setDetailLoading(true);
        await teacherService.deleteTeacher(selectedTeacher.id);
        
        // Show success notification
        showSuccess(getDbOperationMessage('delete', 'Lehrer', selectedTeacher.name));
        
        // Close modal and refresh list
        setShowDetailModal(false);
        setSelectedTeacher(null);
        await fetchTeachers(searchFilter, roleFilter, currentPage);
      } catch (err) {
        console.error("Error deleting teacher:", err);
        setDetailError(
          "Fehler beim Löschen des Lehrers. Bitte versuchen Sie es später erneut."
        );
      } finally {
        setDetailLoading(false);
      }
    }
  };

  // Get unique roles from loaded teachers
  const roleOptions = Array.isArray(teachers) 
    ? Array.from(
        new Set(
          teachers
            .filter(teacher => teacher.role)
            .map(teacher => teacher.role!)
        )
      ).sort().map(role => ({ value: role, label: role }))
    : [];

  return (
    <>
      {/* Notification for success/error messages */}
      <Notification notification={notification} className="fixed top-4 right-4 z-50 max-w-sm" />
      
      <DatabaseListPage
        userName={session?.user?.name ?? "Benutzer"}
        title="Lehrer auswählen"
        description="Verwalten Sie Lehrerprofile und Zuordnungen"
        listTitle="Lehrerliste"
        searchPlaceholder="Lehrer suchen..."
        searchValue={searchFilter}
        onSearchChange={setSearchFilter}
        filters={
          roleOptions.length > 0 ? (
            <div className="md:max-w-xs">
              <SelectFilter
                id="roleFilter"
                label="Rolle"
                value={roleFilter}
                onChange={setRoleFilter}
                options={roleOptions}
                placeholder="Alle Rollen"
              />
            </div>
          ) : undefined
        }
        addButton={{
          label: "Neuen Lehrer erstellen",
          onClick: () => setShowCreateModal(true)
        }}
        items={teachers}
        loading={loading}
        error={error}
        onRetry={() => fetchTeachers(searchFilter, roleFilter, currentPage)}
        itemLabel={{ singular: "Lehrer", plural: "Lehrer" }}
        emptyIcon={
          <svg className="h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
          </svg>
        }
        renderItem={(teacher: Teacher) => (
          <TeacherListItem
            key={teacher.id}
            teacher={teacher}
            onClick={handleSelectTeacher}
          />
        )}
        pagination={pagination}
        onPageChange={setCurrentPage}
      />

      {/* Create Teacher Modal */}
      <CreateFormModal
        isOpen={showCreateModal}
        onClose={() => {
          setShowCreateModal(false);
          setCreateError(null);
          setCreatedCredentials(null);
        }}
        title="Neuer Lehrer"
        size="lg"
      >
        {createError && (
          <div className="mb-4 rounded-lg bg-red-50 p-4 text-red-800">
            <p>{createError}</p>
          </div>
        )}
        
        {createdCredentials ? (
          <div className="space-y-4">
            <div className="rounded-lg border border-green-200 bg-green-50 p-6">
              <h3 className="mb-4 text-lg font-semibold text-green-800">
                Lehrer erfolgreich erstellt!
              </h3>
              <p className="mb-4 text-green-700">
                Bitte notieren Sie sich die folgenden temporären Zugangsdaten:
              </p>
              <div className="mb-4 rounded bg-white p-4 font-mono text-sm">
                <p className="mb-2">
                  <span className="font-semibold">E-Mail:</span> {createdCredentials.email}
                </p>
                <p>
                  <span className="font-semibold">Passwort:</span> {createdCredentials.password}
                </p>
              </div>
              <p className="mb-4 text-sm text-green-600">
                Der Lehrer sollte das Passwort bei der ersten Anmeldung ändern.
              </p>
            </div>
            <div className="flex justify-end">
              <button
                onClick={() => {
                  setShowCreateModal(false);
                  setCreatedCredentials(null);
                  void fetchTeachers(searchFilter, roleFilter, currentPage);
                }}
                className="rounded bg-green-600 px-4 py-2 text-white hover:bg-green-700"
              >
                Schließen
              </button>
            </div>
          </div>
        ) : (
          <TeacherForm
            initialData={{
              first_name: "",
              last_name: "",
              specialization: "",
            }}
            onSubmitAction={handleCreateTeacher}
            onCancelAction={() => setShowCreateModal(false)}
            isLoading={createLoading}
            formTitle=""
            submitLabel="Erstellen"
            rfidCards={rfidCards}
          />
        )}
      </CreateFormModal>

      {/* Detail/Edit Teacher Modal */}
      <DetailFormModal
        isOpen={showDetailModal}
        onClose={() => {
          setShowDetailModal(false);
          setSelectedTeacher(null);
          setIsEditing(false);
          setDetailError(null);
        }}
        title={isEditing ? "Lehrer bearbeiten" : "Lehrerdetails"}
        size="xl"
        loading={detailLoading}
        error={detailError}
        onRetry={() => selectedTeacher && handleSelectTeacher(selectedTeacher)}
      >
        {selectedTeacher && !isEditing && (
          <TeacherDetailView
            teacher={selectedTeacher}
            onEdit={() => setIsEditing(true)}
            onDelete={handleDeleteTeacher}
          />
        )}
        
        {selectedTeacher && isEditing && (
          <TeacherForm
            initialData={selectedTeacher}
            onSubmitAction={handleUpdateTeacher}
            onCancelAction={() => setIsEditing(false)}
            isLoading={detailLoading}
            formTitle=""
            submitLabel="Speichern"
            rfidCards={rfidCards}
          />
        )}
      </DetailFormModal>
    </>
  );
}
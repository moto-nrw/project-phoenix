"use client";

import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { useState, useEffect } from "react";
import type { Student } from "@/lib/api";
import { studentService } from "@/lib/api";
import { DatabaseListPage, SelectFilter, CreateFormModal, DetailFormModal, Notification } from "@/components/ui";
import { StudentListItem, StudentForm, StudentDetailView } from "@/components/students";
import { useNotification, getDbOperationMessage } from "@/lib/use-notification";

// Student list will be loaded from API

export default function StudentsPage() {
  const { notification, showSuccess, showError } = useNotification();
  const [students, setStudents] = useState<Student[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchFilter, setSearchFilter] = useState("");
  const [groupFilter, setGroupFilter] = useState<string | null>(null);
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

  // Detail Modal states
  const [showDetailModal, setShowDetailModal] = useState(false);
  const [selectedStudent, setSelectedStudent] = useState<Student | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState<string | null>(null);
  const [isEditing, setIsEditing] = useState(false);

  // const handleSearchInput = (value: string) => {
  //   setSearchFilter(value);
  // };

  // const handleFilterChange = (filterId: string, value: string | null) => {
  //   if (filterId === 'group') {
  //     setGroupFilter(value);
  //   }
  // };

  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  // Function to fetch students with optional filters
  const fetchStudents = async (search?: string, groupId?: string | null, page = 1) => {
    try {
      setLoading(true);

      // Prepare filters for API call
      const filters = {
        search: search ?? undefined,
        groupId: groupId ?? undefined,
        page: page,
        pageSize: 50
      };

      try {
        // Fetch from the real API using our student service
        const data = await studentService.getStudents(filters);
        console.log("Received data from studentService:", data);
        console.log("Pagination details:", data.pagination);
        
        // Ensure students is always an array
        const studentsArray = Array.isArray(data.students) ? data.students : [];
        setStudents(studentsArray);
        setPagination(data.pagination ?? null);
        setError(null);
      } catch (apiErr) {
        console.error("API error when fetching students:", apiErr);
        setError(
          "Fehler beim Laden der Schülerdaten. Bitte versuchen Sie es später erneut.",
        );
        setStudents([]);
        setPagination(null);
      }
    } catch (err) {
      console.error("Error fetching students:", err);
      setError(
        "Fehler beim Laden der Schülerdaten. Bitte versuchen Sie es später erneut.",
      );
      setStudents([]);
      setPagination(null);
    } finally {
      setLoading(false);
    }
  };

  // Initial data load
  useEffect(() => {
    void fetchStudents(undefined, undefined, currentPage);
  }, [currentPage]);

  // Handle search and group filter changes
  useEffect(() => {
    // Reset to first page when filters change
    setCurrentPage(1);
    // Debounce search to avoid too many API calls
    const timer = setTimeout(() => {
      void fetchStudents(searchFilter, groupFilter, 1);
    }, 300);

    return () => clearTimeout(timer);
  }, [searchFilter, groupFilter]);


  if (status === "loading") {
    return <div />; // Let DatabaseListPage handle the loading state
  }

  const handleSelectStudent = async (student: Student) => {
    setSelectedStudent(student);
    setShowDetailModal(true);
    setDetailError(null);
    
    // Fetch fresh data for the selected student
    try {
      setDetailLoading(true);
      const freshData = await studentService.getStudent(student.id);
      setSelectedStudent(freshData);
    } catch (err) {
      console.error("Error fetching student details:", err);
      setDetailError("Fehler beim Laden der Schülerdaten.");
    } finally {
      setDetailLoading(false);
    }
  };

  // Handle student creation
  const handleCreateStudent = async (studentData: Partial<Student>) => {
    try {
      setCreateLoading(true);
      setCreateError(null);

      // Prepare student data - the API route will handle guardian email/phone parsing
      const newStudent: Omit<Student, "id"> = {
        // Basic info (all required)
        first_name: studentData.first_name ?? '',
        second_name: studentData.second_name ?? '',
        name: `${studentData.first_name} ${studentData.second_name}`,
        
        // School info (required)
        school_class: studentData.school_class ?? '',
        group_id: studentData.group_id,
        
        // Guardian info (all required)
        name_lg: studentData.name_lg ?? '',
        contact_lg: studentData.contact_lg ?? '',
        
        // Location fields (defaults)
        current_location: "Home" as const,
        in_house: false,
        wc: false,
        school_yard: false,
        bus: studentData.bus ?? false,
        
        // Optional fields
        studentId: undefined, // Tag ID is optional, backend handles it
      };

      // Create the student
      await studentService.createStudent(newStudent);
      
      // Show success notification
      showSuccess(getDbOperationMessage('create', 'Schüler', `${newStudent.first_name} ${newStudent.second_name}`));
      
      // Close modal and refresh list
      setShowCreateModal(false);
      await fetchStudents(searchFilter, groupFilter, currentPage);
    } catch (err) {
      console.error("Error creating student:", err);
      setCreateError(
        "Fehler beim Erstellen des Schülers. Bitte versuchen Sie es später erneut."
      );
      throw err; // Re-throw for form to handle
    } finally {
      setCreateLoading(false);
    }
  };

  // Handle student update
  const handleUpdateStudent = async (studentData: Partial<Student>) => {
    if (!selectedStudent) return;
    
    try {
      setDetailLoading(true);
      setDetailError(null);

      // Update student
      await studentService.updateStudent(
        selectedStudent.id,
        studentData
      );
      
      // Show success notification
      showSuccess(getDbOperationMessage('update', 'Schüler', selectedStudent.name));
      
      // Refresh the selected student data
      const refreshedStudent = await studentService.getStudent(selectedStudent.id);
      setSelectedStudent(refreshedStudent);
      setIsEditing(false);
      
      // Refresh the list
      await fetchStudents(searchFilter, groupFilter, currentPage);
    } catch (err) {
      console.error("Error updating student:", err);
      setDetailError(
        "Fehler beim Aktualisieren des Schülers. Bitte versuchen Sie es später erneut."
      );
      throw err;
    } finally {
      setDetailLoading(false);
    }
  };

  // Handle student deletion
  const handleDeleteStudent = async () => {
    if (!selectedStudent) return;
    
    if (window.confirm("Sind Sie sicher, dass Sie diesen Schüler löschen möchten?")) {
      try {
        setDetailLoading(true);
        await studentService.deleteStudent(selectedStudent.id);
        
        // Show success notification
        showSuccess(getDbOperationMessage('delete', 'Schüler', selectedStudent.name));
        
        // Close modal and refresh list
        setShowDetailModal(false);
        setSelectedStudent(null);
        await fetchStudents(searchFilter, groupFilter, currentPage);
      } catch (err) {
        console.error("Error deleting student:", err);
        setDetailError(
          "Fehler beim Löschen des Schülers. Bitte versuchen Sie es später erneut."
        );
      } finally {
        setDetailLoading(false);
      }
    }
  };

  // Get unique groups from loaded students
  const groupOptions = Array.isArray(students) 
    ? Array.from(
        new Map(
          students
            .filter(student => student.group_id && student.group_name)
            .map(student => [student.group_id, { value: student.group_id!, label: student.group_name! }])
        ).values()
      ).sort((a, b) => a.label.localeCompare(b.label))
    : [];

  return (
    <>
      {/* Notification for success/error messages */}
      <Notification notification={notification} className="fixed top-4 right-4 z-50 max-w-sm" />
      
      <DatabaseListPage
        userName={session?.user?.name ?? "Benutzer"}
        title="Schüler auswählen"
        description="Verwalte Schülerdaten und Gruppenzuweisungen"
        listTitle="Schülerliste"
        searchPlaceholder="Schüler suchen..."
        searchValue={searchFilter}
        onSearchChange={setSearchFilter}
        filters={
          <div className="md:max-w-xs">
            <SelectFilter
              id="groupFilter"
              label="Gruppe"
              value={groupFilter}
              onChange={setGroupFilter}
              options={groupOptions}
              placeholder="Alle Gruppen"
            />
          </div>
        }
        addButton={{
          label: "Neuen Schüler erstellen",
          onClick: () => setShowCreateModal(true)
        }}
        items={students}
        loading={loading}
        error={error}
        onRetry={() => fetchStudents(searchFilter, groupFilter, currentPage)}
        itemLabel={{ singular: "Schüler", plural: "Schüler" }}
        renderItem={(student: Student) => (
          <StudentListItem
            key={student.id}
            student={student}
            onClick={handleSelectStudent}
          />
        )}
        pagination={pagination}
        onPageChange={setCurrentPage}
      />

      {/* Create Student Modal */}
      <CreateFormModal
      isOpen={showCreateModal}
      onClose={() => {
        setShowCreateModal(false);
        setCreateError(null);
      }}
      title="Neuer Schüler"
      size="lg"
    >
      {createError && (
        <div className="mb-4 rounded-lg bg-red-50 p-4 text-red-800">
          <p>{createError}</p>
        </div>
      )}
      
      <StudentForm
        initialData={{
          in_house: false,
          wc: false,
          school_yard: false,
          bus: false,
        }}
        onSubmitAction={handleCreateStudent}
        onCancelAction={() => setShowCreateModal(false)}
        isLoading={createLoading}
        formTitle=""
        submitLabel="Erstellen"
      />
    </CreateFormModal>

      {/* Detail/Edit Student Modal */}
      <DetailFormModal
        isOpen={showDetailModal}
        onClose={() => {
          setShowDetailModal(false);
          setSelectedStudent(null);
          setIsEditing(false);
          setDetailError(null);
        }}
        title={isEditing ? "Schüler bearbeiten" : "Schülerdetails"}
        size="xl"
        loading={detailLoading}
        error={detailError}
        onRetry={() => selectedStudent && handleSelectStudent(selectedStudent)}
      >
        {selectedStudent && !isEditing && (
          <StudentDetailView
            student={selectedStudent}
            onEdit={() => setIsEditing(true)}
            onDelete={handleDeleteStudent}
          />
        )}
        
        {selectedStudent && isEditing && (
          <StudentForm
            initialData={selectedStudent}
            onSubmitAction={handleUpdateStudent}
            onCancelAction={() => setIsEditing(false)}
            isLoading={detailLoading}
            formTitle=""
            submitLabel="Speichern"
          />
        )}
      </DetailFormModal>
    </>
  );
}

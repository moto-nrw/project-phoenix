"use client";

import { useSession } from "next-auth/react";
import { redirect, useRouter } from "next/navigation";
import { useState, useEffect } from "react";
import type { Student } from "@/lib/api";
import { studentService, groupService } from "@/lib/api";
import { DatabaseListPage, SelectFilter, CreateFormModal } from "@/components/ui";
import { StudentListItem, StudentForm } from "@/components/students";

// Student list will be loaded from API

export default function StudentsPage() {
  const router = useRouter();
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
  
  // Modal states
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [createLoading, setCreateLoading] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

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

  const handleSelectStudent = (student: Student) => {
    router.push(`/database/students/${student.id}`);
  };

  // Handle student creation
  const handleCreateStudent = async (studentData: Partial<Student>) => {
    try {
      setCreateLoading(true);
      setCreateError(null);

      // Prepare guardian contact fields
      let guardianEmail: string | undefined;
      let guardianPhone: string | undefined;
      
      // Parse guardian contact - check if it's an email or phone
      if (studentData.contact_lg) {
        if (studentData.contact_lg.includes('@')) {
          guardianEmail = studentData.contact_lg;
        } else {
          guardianPhone = studentData.contact_lg;
        }
      }

      // Prepare student data for the backend
      const newStudent: Omit<Student, "id"> & { 
        guardian_email?: string;
        guardian_phone?: string;
      } = {
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
        guardian_email: guardianEmail,
        guardian_phone: guardianPhone,
        
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
    </>
  );
}

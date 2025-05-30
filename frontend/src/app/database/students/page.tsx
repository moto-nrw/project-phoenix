"use client";

import { useSession } from "next-auth/react";
import { redirect, useRouter } from "next/navigation";
import { useState, useEffect } from "react";
import type { Student } from "@/lib/api";
import { studentService } from "@/lib/api";
import { DatabaseListPage, SelectFilter } from "@/components/ui";
import { StudentListItem } from "@/components/students";

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
        href: "/database/students/new"
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
  );
}

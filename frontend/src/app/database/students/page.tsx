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
  const fetchStudents = async (search?: string, groupId?: string | null) => {
    try {
      setLoading(true);

      // Prepare filters for API call
      const filters = {
        search: search ?? undefined,
        groupId: groupId ?? undefined,
      };

      try {
        // Fetch from the real API using our student service
        const data = await studentService.getStudents(filters);
        console.log("Received data from studentService:", data);
        setStudents(data);
        setError(null);
      } catch (apiErr) {
        console.error("API error when fetching students:", apiErr);
        setError(
          "Fehler beim Laden der Schülerdaten. Bitte versuchen Sie es später erneut.",
        );
        setStudents([]);
      }
    } catch (err) {
      console.error("Error fetching students:", err);
      setError(
        "Fehler beim Laden der Schülerdaten. Bitte versuchen Sie es später erneut.",
      );
      setStudents([]);
    } finally {
      setLoading(false);
    }
  };

  // Initial data load
  useEffect(() => {
    void fetchStudents();
  }, []);

  // Handle search and group filter changes
  useEffect(() => {
    // Debounce search to avoid too many API calls
    const timer = setTimeout(() => {
      void fetchStudents(searchFilter, groupFilter);
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
  const groupOptions = Array.from(
    new Map(
      students
        .filter(student => student.group_id && student.group_name)
        .map(student => [student.group_id, { value: student.group_id!, label: student.group_name! }])
    ).values()
  ).sort((a, b) => a.label.localeCompare(b.label));

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
      onRetry={() => fetchStudents()}
      itemLabel={{ singular: "Schüler", plural: "Schüler" }}
      renderItem={(student: Student) => (
        <StudentListItem
          key={student.id}
          student={student}
          onClick={handleSelectStudent}
        />
      )}
    />
  );
}

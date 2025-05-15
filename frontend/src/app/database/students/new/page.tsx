"use client";

import { useState, useEffect, Suspense } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { PageHeader } from "@/components/dashboard";
import StudentForm from "@/components/students/student-form";
import type { Student } from "@/lib/api";
import { studentService, groupService } from "@/lib/api";

function NewStudentContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const groupId = searchParams.get("groupId");

  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [groupName, setGroupName] = useState<string | null>(null);

  // Fetch group name if groupId is provided
  useEffect(() => {
    if (groupId) {
      const fetchGroupName = async () => {
        try {
          const group = await groupService.getGroup(groupId);
          setGroupName(group.name);
        } catch (err) {
          console.error("Error fetching group:", err);
        }
      };

      void fetchGroupName();
    }
  }, [groupId]);

  const handleCreateStudent = async (studentData: Partial<Student>) => {
    try {
      setLoading(true);
      setError(null);

      // Prepare student data
      const newStudent: Omit<Student, "id"> = {
        ...studentData,
        name: `${studentData.first_name} ${studentData.second_name}`,
        in_house: studentData.in_house ?? false,
        wc: studentData.wc ?? false,
        school_yard: studentData.school_yard ?? false,
        bus: studentData.bus ?? false,
        school_class: studentData.school_class ?? "",
        group_id: groupId ?? studentData.group_id, // Use groupId from URL if available
      };

      // Create student - group association now works directly via the API
      await studentService.createStudent(newStudent);

      // Navigate back to the appropriate page
      if (groupId) {
        router.push(`/database/groups/${groupId}`);
      } else {
        router.push("/database/students");
      }
    } catch (err) {
      console.error("Error creating student:", err);
      setError(
        "Fehler beim Erstellen des Schülers. Bitte versuchen Sie es später erneut.",
      );
      throw err; // Re-throw to be caught by the form component
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <PageHeader
        title={groupName ? `Neuer Schüler für ${groupName}` : "Neuer Schüler"}
        backUrl={groupId ? `/database/groups/${groupId}` : "/database/students"}
      />

      {/* Main Content */}
      <main className="mx-auto max-w-4xl p-4">
        {error && (
          <div className="mb-6 rounded-lg bg-red-50 p-4 text-red-800">
            <p>{error}</p>
          </div>
        )}

        {groupName && (
          <div className="mb-4 rounded-md border-l-4 border-blue-500 bg-blue-50 p-4 text-blue-800">
            <p className="font-medium">Hinweis</p>
            <p>
              Der neue Schüler wird automatisch der Gruppe &quot;{groupName}
              &quot; zugewiesen.
            </p>
          </div>
        )}

        <StudentForm
          initialData={{
            in_house: false,
            wc: false,
            school_yard: false,
            bus: false,
            group_id: groupId ?? "1",
          }}
          onSubmitAction={handleCreateStudent}
          onCancelAction={() => router.back()}
          isLoading={loading}
          formTitle={
            groupName
              ? `Schüler für ${groupName} erstellen`
              : "Schüler erstellen"
          }
          submitLabel="Erstellen"
        />
      </main>
    </div>
  );
}

export default function NewStudentPage() {
  return (
    <Suspense 
      fallback={
        <div className="min-h-screen bg-gray-50 flex items-center justify-center">
          <div className="text-center">
            <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-gray-900 mx-auto"></div>
            <p className="mt-4 text-gray-600">Lädt...</p>
          </div>
        </div>
      }
    >
      <NewStudentContent />
    </Suspense>
  );
}

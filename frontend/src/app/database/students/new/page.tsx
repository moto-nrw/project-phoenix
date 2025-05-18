"use client";

import { useState, useEffect, Suspense } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { PageHeader } from "@/components/dashboard";
import StudentForm from "@/components/students/student-form";
import type { Student } from "@/lib/api";
import { studentService, groupService } from "@/lib/api";

// Component that uses searchParams needs to be wrapped in Suspense
function StudentPageContent() {
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

      // Generate automatic fields
      const timestamp = Date.now();
      
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
        // Basic info
        first_name: studentData.first_name ?? '',
        second_name: studentData.second_name ?? studentData.first_name ?? '', // Backend requires last_name
        name: `${studentData.first_name} ${studentData.second_name ?? studentData.first_name}`,
        
        // School info
        school_class: studentData.school_class ?? '',
        group_id: groupId ?? studentData.group_id,
        
        // Guardian info
        name_lg: studentData.name_lg ?? '',
        contact_lg: studentData.contact_lg ?? '',
        guardian_email: guardianEmail,
        guardian_phone: guardianPhone,
        
        // Location fields
        in_house: false,
        wc: false,
        school_yard: false,
        bus: studentData.bus ?? false,
        
        // Auto-generated fields
        studentId: timestamp.toString(), // Use timestamp as a simple ID
        // No tag_id or account_id - backend will handle placeholder creation
      };

      // Create the student with a generated tag ID
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
            group_id: groupId ?? undefined,
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

// Main page component with Suspense boundary
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
      <StudentPageContent />
    </Suspense>
  );
}
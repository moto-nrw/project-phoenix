"use client";

import { Suspense, useState, useEffect } from "react";
import { useSearchParams } from "next/navigation";
import { DatabaseFormPage } from "@/components/ui";
import StudentForm from "@/components/students/student-form";
import type { Student } from "@/lib/api";
import { studentService, groupService } from "@/lib/api";

// Component that uses searchParams needs to be wrapped in Suspense
function StudentPageContent() {
  const searchParams = useSearchParams();
  const groupId = searchParams.get("groupId");
  const [groupName, setGroupName] = useState<string | null>(null);

  // Fetch group name when groupId is available
  useEffect(() => {
    if (groupId) {
      groupService.getGroup(groupId)
        .then(group => setGroupName(group.name))
        .catch(err => console.error("Error fetching group:", err));
    }
  }, [groupId]);

  return (
    <DatabaseFormPage
      config={{
        title: groupName ? `Neuer Schüler für ${groupName}` : "Neuer Schüler",
        backUrl: groupId ? `/database/groups/${groupId}` : "/database/students",
        resourceName: "Schüler",
        FormComponent: StudentForm,
        
        // Map the form data to include parsed guardian fields
        mapFormData: (studentData: Partial<Student>) => {
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
            group_id: groupId ?? studentData.group_id,
            
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

          return newStudent;
        },
        
        onCreate: async (data) => await studentService.createStudent(data as Omit<Student, "id">),
        
        successRedirectUrl: groupId ? `/database/groups/${groupId}` : "/database/students",
        
        // Customize form props based on loaded data
        formProps: {
          initialData: {
            in_house: false,
            wc: false,
            school_yard: false,
            bus: false,
            group_id: groupId ?? undefined,
          },
          formTitle: groupName ? `Schüler für ${groupName} erstellen` : "Schüler erstellen",
          submitLabel: "Erstellen",
        },
        
        // Add group assignment notice when creating from group page
        beforeForm: groupName ? (
          <div className="mb-4 rounded-md border-l-4 border-blue-500 bg-blue-50 p-4 text-blue-800">
            <p className="font-medium">Hinweis</p>
            <p>
              Der neue Schüler wird automatisch der Gruppe &quot;{groupName}&quot; zugewiesen.
            </p>
          </div>
        ) : undefined,
      }}
    />
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
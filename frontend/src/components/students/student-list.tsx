"use client";

import { useRouter } from "next/navigation";
import type { Student } from "@/lib/api";
import { isPresentLocation } from "@/lib/location-helper";

interface StudentListProps {
  students: Student[];
  onStudentClick?: (student: Student) => void;
  showDetails?: boolean;
  emptyMessage?: string;
}

export default function StudentList({
  students,
  onStudentClick,
  showDetails = true,
  emptyMessage = "Keine Schüler vorhanden.",
}: StudentListProps) {
  const router = useRouter();

  const handleStudentClick = (student: Student) => {
    if (onStudentClick) {
      onStudentClick(student);
    } else {
      router.push(`/database/students/${student.id}`);
    }
  };

  if (!students.length) {
    return (
      <div className="py-8 text-center">
        <p className="text-gray-500">{emptyMessage}</p>
      </div>
    );
  }

  return (
    <div className="space-y-2">
      {students.map((student) => {
        const isPresent = isPresentLocation(student.current_location);
        return (
          <div
            key={student.id}
            onClick={() => handleStudentClick(student)}
            className="group cursor-pointer rounded-lg border border-gray-100 bg-white p-4 shadow-sm transition-all duration-200 hover:translate-y-[-1px] hover:border-blue-200 hover:shadow-md"
          >
            <div className="flex items-center justify-between">
              <div className="flex items-center space-x-3">
                <div className="flex h-10 w-10 items-center justify-center rounded-full bg-gradient-to-r from-blue-400 to-indigo-500 font-medium text-white">
                  {(
                    student.name ??
                    (student.first_name
                      ? `${student.first_name} ${student.second_name ?? ""}`
                      : "S")
                  )
                    .slice(0, 1)
                    .toUpperCase()}
                </div>

                <div className="flex flex-col">
                  <span className="font-medium text-gray-900 transition-colors group-hover:text-blue-600">
                    {student.name ??
                      (student.first_name
                        ? `${student.first_name} ${student.second_name ?? ""}`
                        : "Unnamed Student")}
                  </span>
                  {showDetails && (
                    <span className="text-sm text-gray-500">
                      {student.school_class &&
                        `Klasse: ${student.school_class}`}
                      {student.group_name && ` | Gruppe: ${student.group_name}`}
                    </span>
                  )}
                </div>
              </div>

              <div className="flex items-center">
                <div className="mr-16 flex items-center">
                  <div
                    className={`h-2.5 w-2.5 rounded-full ${isPresent ? "bg-green-500" : "bg-gray-300"} relative transition-all duration-200 group-hover:scale-110`}
                    title={isPresent ? "Anwesend" : "Nicht anwesend"}
                  >
                    {isPresent && (
                      <span className="absolute -top-0.5 -right-0.5 flex h-3 w-3 opacity-75">
                        <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-green-400 opacity-75"></span>
                      </span>
                    )}
                  </div>
                </div>
              </div>
            </div>
          </div>
        );
      })}
    </div>
  );
}

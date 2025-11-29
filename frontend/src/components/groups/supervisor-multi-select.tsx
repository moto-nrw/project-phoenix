"use client";

import { useState, useEffect, useMemo, useRef } from "react";
import { Badge } from "@/components/ui/badge";
import type { Teacher } from "@/lib/teacher-api";
import { teacherService } from "@/lib/teacher-api";

interface SupervisorMultiSelectProps {
  selectedSupervisors: string[];
  onSelectionChange: (supervisorIds: string[]) => void;
  placeholder?: string;
  className?: string;
  onError?: (error: string) => void;
}

export function SupervisorMultiSelect({
  selectedSupervisors,
  onSelectionChange,
  placeholder = "Gruppenleitung auswählen...",
  className = "",
  onError,
}: SupervisorMultiSelectProps) {
  const [teachers, setTeachers] = useState<Teacher[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchTerm, setSearchTerm] = useState("");
  const [isOpen, setIsOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  // Fetch teachers on component mount with cleanup
  useEffect(() => {
    const controller = new AbortController();

    const fetchTeachers = async () => {
      try {
        setLoading(true);
        setError(null);
        const teachersData = await teacherService.getTeachers();

        if (!controller.signal.aborted) {
          setTeachers(teachersData);
        }
      } catch (error) {
        if (!controller.signal.aborted) {
          const errorMessage =
            error instanceof Error
              ? error.message
              : "Fehler beim Laden der Lehrer";
          setError(errorMessage);
          onError?.(errorMessage);
          console.error("Error fetching teachers:", error);
        }
      } finally {
        if (!controller.signal.aborted) {
          setLoading(false);
        }
      }
    };

    void fetchTeachers();

    return () => {
      controller.abort();
    };
  }, [onError]);

  // Filter teachers based on search term (memoized for performance)
  const filteredTeachers = useMemo(
    () =>
      teachers.filter(
        (teacher) =>
          teacher.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
          teacher.specialization
            ?.toLowerCase()
            .includes(searchTerm.toLowerCase()),
      ),
    [teachers, searchTerm],
  );

  // Get selected teachers for display (memoized)
  const selectedTeachers = useMemo(
    () =>
      teachers.filter((teacher) => selectedSupervisors.includes(teacher.id)),
    [teachers, selectedSupervisors],
  );

  const handleTeacherToggle = (teacherId: string) => {
    // Validate teacher ID exists
    if (!teacherId || !teachers.find((t) => t.id === teacherId)) {
      console.error("Invalid teacher ID:", teacherId);
      return;
    }

    const newSelection = selectedSupervisors.includes(teacherId)
      ? selectedSupervisors.filter((id) => id !== teacherId)
      : [...selectedSupervisors, teacherId];

    onSelectionChange(newSelection);
  };

  const handleRemoveTeacher = (teacherId: string) => {
    if (!teacherId) return;
    onSelectionChange(selectedSupervisors.filter((id) => id !== teacherId));
  };

  // Handle click outside to close dropdown
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (
        dropdownRef.current &&
        !dropdownRef.current.contains(event.target as Node)
      ) {
        setIsOpen(false);
      }
    };

    if (isOpen) {
      document.addEventListener("mousedown", handleClickOutside);
      return () => {
        document.removeEventListener("mousedown", handleClickOutside);
      };
    }
  }, [isOpen]);

  return (
    <div className={`relative ${className}`}>
      {/* Selected supervisors display */}
      {selectedTeachers.length > 0 && (
        <div className="mb-3 flex flex-wrap gap-2">
          {selectedTeachers.map((teacher) => (
            <div
              key={teacher.id}
              className="cursor-pointer"
              onClick={() => handleRemoveTeacher(teacher.id)}
            >
              <Badge variant="purple" size="md" className="hover:bg-purple-200">
                <span>{teacher.name}</span>
                <button
                  className="ml-1 rounded-full p-0.5 hover:bg-purple-300"
                  onClick={(e) => {
                    e.stopPropagation();
                    handleRemoveTeacher(teacher.id);
                  }}
                >
                  <svg
                    className="h-3 w-3"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M6 18L18 6M6 6l12 12"
                    />
                  </svg>
                </button>
              </Badge>
            </div>
          ))}
        </div>
      )}

      {/* Search input */}
      <div className="relative" ref={dropdownRef}>
        <input
          ref={inputRef}
          type="text"
          placeholder={placeholder}
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
          onFocus={() => setIsOpen(true)}
          className="w-full rounded-lg border border-gray-300 px-4 py-2 pr-10 transition-all duration-200 focus:ring-2 focus:ring-blue-500 focus:outline-none"
        />
        <div className="absolute top-2.5 right-3 text-gray-400">
          <svg
            className="h-5 w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
            />
          </svg>
        </div>

        {/* Dropdown */}
        {isOpen && (
          <div className="absolute z-10 mt-1 w-full rounded-lg border border-gray-300 bg-white shadow-lg">
            <div className="max-h-60 overflow-y-auto">
              {loading ? (
                <div className="p-3 text-center text-sm text-gray-500">
                  Lehrer werden geladen...
                </div>
              ) : error ? (
                <div className="p-3 text-center text-sm text-red-500">
                  Fehler: {error}
                </div>
              ) : filteredTeachers.length === 0 ? (
                <div className="p-3 text-center text-sm text-gray-500">
                  {searchTerm
                    ? "Keine Lehrer gefunden"
                    : "Keine Lehrer verfügbar"}
                </div>
              ) : (
                filteredTeachers.map((teacher) => {
                  const isSelected = selectedSupervisors.includes(teacher.id);
                  return (
                    <div
                      key={teacher.id}
                      className={`flex cursor-pointer items-center space-x-3 p-3 transition-colors hover:bg-gray-50 ${
                        isSelected ? "bg-purple-50" : ""
                      }`}
                      onMouseDown={(e) => {
                        e.preventDefault(); // Prevent blur
                        handleTeacherToggle(teacher.id);
                      }}
                    >
                      <div className="flex h-4 w-4 items-center">
                        <input
                          type="checkbox"
                          checked={isSelected}
                          onChange={() => {
                            /* Controlled by parent div */
                          }}
                          onClick={(e) => e.stopPropagation()} // Prevent double toggle
                          className="h-4 w-4 rounded border-gray-300 text-purple-600 focus:ring-purple-500"
                        />
                      </div>
                      <div className="flex-1">
                        <div className="text-sm font-medium text-gray-900">
                          {teacher.name}
                        </div>
                        {teacher.specialization && (
                          <div className="text-xs text-gray-500">
                            {teacher.specialization}
                          </div>
                        )}
                      </div>
                      {isSelected && (
                        <div className="text-purple-600">
                          <svg
                            className="h-4 w-4"
                            fill="none"
                            viewBox="0 0 24 24"
                            stroke="currentColor"
                          >
                            <path
                              strokeLinecap="round"
                              strokeLinejoin="round"
                              strokeWidth={2}
                              d="M5 13l4 4L19 7"
                            />
                          </svg>
                        </div>
                      )}
                    </div>
                  );
                })
              )}
            </div>
          </div>
        )}
      </div>

      {/* Helper text */}
      <p className="mt-1 text-xs text-gray-500">
        {selectedTeachers.length === 0
          ? "Klicken Sie hier, um Gruppenleitung auszuwählen"
          : `${selectedTeachers.length} Gruppenleiter/in${selectedTeachers.length === 1 ? "" : "nen"} ausgewählt`}
      </p>
    </div>
  );
}

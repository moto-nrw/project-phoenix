"use client";

import { useState, useEffect } from "react";
import { Badge } from "@/components/ui/badge";
import type { Teacher } from "@/lib/teacher-api";
import { teacherService } from "@/lib/teacher-api";

interface SupervisorMultiSelectProps {
  selectedSupervisors: string[];
  onSelectionChange: (supervisorIds: string[]) => void;
  placeholder?: string;
  className?: string;
}

export function SupervisorMultiSelect({
  selectedSupervisors,
  onSelectionChange,
  placeholder = "Aufsichtspersonen auswählen...",
  className = "",
}: SupervisorMultiSelectProps) {
  const [teachers, setTeachers] = useState<Teacher[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchTerm, setSearchTerm] = useState("");
  const [isOpen, setIsOpen] = useState(false);

  // Fetch teachers on component mount
  useEffect(() => {
    const fetchTeachers = async () => {
      try {
        setLoading(true);
        const teachersData = await teacherService.getTeachers();
        setTeachers(teachersData);
      } catch (error) {
        console.error("Error fetching teachers:", error);
      } finally {
        setLoading(false);
      }
    };

    void fetchTeachers();
  }, []);

  // Filter teachers based on search term
  const filteredTeachers = teachers.filter(teacher =>
    teacher.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
    teacher.specialization?.toLowerCase().includes(searchTerm.toLowerCase())
  );

  // Get selected teachers for display
  const selectedTeachers = teachers.filter(teacher => 
    selectedSupervisors.includes(teacher.id)
  );

  const handleTeacherToggle = (teacherId: string) => {
    const newSelection = selectedSupervisors.includes(teacherId)
      ? selectedSupervisors.filter(id => id !== teacherId)
      : [...selectedSupervisors, teacherId];
    
    onSelectionChange(newSelection);
  };

  const handleRemoveTeacher = (teacherId: string) => {
    onSelectionChange(selectedSupervisors.filter(id => id !== teacherId));
  };

  return (
    <div className={`relative ${className}`}>
      {/* Selected supervisors display */}
      {selectedTeachers.length > 0 && (
        <div className="mb-3 flex flex-wrap gap-2">
          {selectedTeachers.map(teacher => (
            <div
              key={teacher.id}
              className="cursor-pointer"
              onClick={() => handleRemoveTeacher(teacher.id)}
            >
              <Badge
                variant="purple"
                size="md"
                className="hover:bg-purple-200"
              >
                <span>{teacher.name}</span>
                <button
                  className="ml-1 rounded-full p-0.5 hover:bg-purple-300"
                  onClick={(e) => {
                    e.stopPropagation();
                    handleRemoveTeacher(teacher.id);
                  }}
                >
                  <svg className="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </Badge>
            </div>
          ))}
        </div>
      )}

      {/* Search input */}
      <div className="relative">
        <input
          type="text"
          placeholder={placeholder}
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
          onFocus={() => setIsOpen(true)}
          onBlur={() => setTimeout(() => setIsOpen(false), 200)}
          className="w-full rounded-lg border border-gray-300 px-4 py-2 pr-10 transition-all duration-200 focus:ring-2 focus:ring-blue-500 focus:outline-none"
        />
        <div className="absolute right-3 top-2.5 text-gray-400">
          <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
        </div>
      </div>

      {/* Dropdown */}
      {isOpen && (
        <div className="absolute z-10 mt-1 w-full rounded-lg border border-gray-300 bg-white shadow-lg">
          <div className="max-h-60 overflow-y-auto">
            {loading ? (
              <div className="p-3 text-center text-sm text-gray-500">
                Lehrer werden geladen...
              </div>
            ) : filteredTeachers.length === 0 ? (
              <div className="p-3 text-center text-sm text-gray-500">
                {searchTerm ? 'Keine Lehrer gefunden' : 'Keine Lehrer verfügbar'}
              </div>
            ) : (
              filteredTeachers.map(teacher => {
                const isSelected = selectedSupervisors.includes(teacher.id);
                return (
                  <div
                    key={teacher.id}
                    className={`flex cursor-pointer items-center space-x-3 p-3 transition-colors hover:bg-gray-50 ${
                      isSelected ? 'bg-purple-50' : ''
                    }`}
                    onClick={() => handleTeacherToggle(teacher.id)}
                  >
                    <div className="flex h-4 w-4 items-center">
                      <input
                        type="checkbox"
                        checked={isSelected}
                        onChange={() => handleTeacherToggle(teacher.id)}
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
                        <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
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

      {/* Helper text */}
      <p className="mt-1 text-xs text-gray-500">
        {selectedTeachers.length === 0 
          ? "Klicken Sie hier, um Aufsichtspersonen auszuwählen"
          : `${selectedTeachers.length} Aufsichtsperson${selectedTeachers.length === 1 ? '' : 'en'} ausgewählt`
        }
      </p>
    </div>
  );
}
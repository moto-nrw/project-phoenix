"use client";

import { ModernStatusBadge } from "./ModernStatusBadge";

interface ModernStudentProfileProps {
  student: {
    first_name: string;
    second_name: string;
    name: string;
    school_class: string;
    group_name?: string;
    current_location?: string;
  };
  index?: number;
  onBack?: () => void;
  backButtonStyle?: 'integrated' | 'floating' | 'thin-row';
  backDestination?: string;
}

export function ModernStudentProfile({ 
  student, 
  index = 0, 
  onBack,
  backButtonStyle = 'floating',
  backDestination = 'Meine Gruppe'
}: ModernStudentProfileProps) {

  // Get year from class for color indicator
  const getYear = (schoolClass: string): number => {
    const yearMatch = /^(\d)/.exec(schoolClass);
    return yearMatch?.[1] ? parseInt(yearMatch[1], 10) : 0;
  };

  const getYearColor = (year: number): string => {
    // Using neutral academic colors that don't conflict with status colors
    switch (year) {
      case 1: return "#3B82F6"; // Standard blue
      case 2: return "#10B981"; // Standard emerald
      case 3: return "#F59E0B"; // Standard amber
      case 4: return "#D946EF"; // Purple/fuchsia
      default: return "#6B7280"; // Gray for unknown
    }
  };

  const year = getYear(student.school_class);
  const yearColor = getYearColor(year);

  return (
    <>
      {/* Mobile Header (visible only on mobile) */}
      <div className="block sm:hidden bg-white/95 backdrop-blur-md border border-gray-200/50 shadow-sm rounded-2xl mb-2">
        <div className="flex items-center justify-between px-4 py-3">
          {/* Name - Now takes full width */}
          <div className="flex items-center gap-2 flex-1 min-w-0">
            <div className="flex-1 min-w-0">
              <h3 className="text-2xl font-bold text-gray-800 break-words leading-tight">
                {student.first_name}
              </h3>
              <p className="text-base font-semibold text-gray-700 break-words">
                {student.second_name}
              </p>
            </div>
          </div>
          
          {/* Right: Status Badge */}
          <div className="flex-shrink-0 ml-3">
            <span 
              className="inline-flex items-center px-3 py-1.5 rounded-xl text-xs font-bold text-white"
              style={{ 
                backgroundColor: (() => {
                  if (student.current_location === "Anwesend" || student.current_location === "In House") return "#83CD2D";
                  if (student.current_location === "Zuhause") return "#FF3130";
                  if (student.current_location === "WC") return "#5080D8";
                  if (student.current_location === "School Yard") return "#F78C10";
                  if (student.current_location === "Bus") return "#D946EF";
                  return "#6B7280";
                })()
              }}
            >
              <span className="w-1.5 h-1.5 bg-white/80 rounded-full mr-1.5"></span>
              {(() => {
                if (student.current_location === "Anwesend" || student.current_location === "In House") return "Anwesend";
                if (student.current_location === "Zuhause") return "Zuhause";
                if (student.current_location === "WC") return "WC";
                if (student.current_location === "School Yard") return "Schulhof";
                if (student.current_location === "Bus") return "Bus";
                return "Unbekannt";
              })()}
            </span>
          </div>
        </div>
      </div>
      

      {/* Desktop Header (visible only on larger screens) */}
      <div 
        className="hidden sm:block relative overflow-hidden rounded-3xl bg-white/90 backdrop-blur-md border border-gray-100/50 shadow-[0_8px_30px_rgb(0,0,0,0.12)] transition-all duration-500"
        style={{
          transform: `rotate(${(index % 3 - 1) * 0.2}deg)` // Reduced rotation for mobile
        }}
      >
        {/* Modern gradient overlay */}
        <div className="absolute inset-0 bg-gradient-to-br from-blue-50/80 to-cyan-100/80 opacity-[0.03] rounded-3xl"></div>
        {/* Subtle inner glow */}
        <div className="absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
        {/* Modern border highlight */}
        <div className="absolute inset-0 rounded-3xl ring-1 ring-white/20 transition-all duration-300"></div>
        
        <div className="relative py-4 px-3 sm:py-5 sm:px-4 lg:py-6 lg:px-5">
          {/* Desktop Profile Section */}
          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 sm:gap-6">
            {/* Student Info */}
            <div className="flex-1 min-w-0">
              {/* Name Section */}
              <div className="mb-3">
                <h1 className="text-xl sm:text-2xl lg:text-3xl font-bold text-gray-800 transition-colors duration-300 leading-tight">
                  <span>{student.first_name}</span>
                  <span className="ml-2">{student.second_name}</span>
                </h1>
              </div>
              
              {/* Class and Group Info */}
              <div className="flex flex-col sm:flex-row sm:items-center gap-2 sm:gap-4 text-gray-600 text-sm sm:text-base">
                <div className="flex items-center gap-2">
                  <svg className="h-4 w-4 sm:h-5 sm:w-5 text-gray-400 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
                  </svg>
                  <span className="font-medium">Klasse {student.school_class}</span>
                </div>
                
                {student.group_name && (
                  <div className="flex items-center gap-2">
                    <svg className="h-4 w-4 sm:h-5 sm:w-5 text-gray-400 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
                    </svg>
                    <span className="truncate">{student.group_name}</span>
                  </div>
                )}
              </div>
            </div>
            
            {/* Desktop Status Badge */}
            <div className="flex justify-center sm:justify-end">
              <ModernStatusBadge location={student.current_location} />
            </div>
          </div>

          {/* Decorative elements */}
          <div className="absolute top-4 left-4 w-5 h-5 bg-white/20 rounded-full animate-ping"></div>
          <div className="absolute bottom-4 right-4 w-3 h-3 bg-white/30 rounded-full"></div>
        </div>

        {/* Glowing border effect */}
        <div className="absolute inset-0 rounded-3xl opacity-0 transition-opacity duration-300 bg-gradient-to-r from-transparent via-blue-100/30 to-transparent"></div>
      </div>
    </>
  );
}
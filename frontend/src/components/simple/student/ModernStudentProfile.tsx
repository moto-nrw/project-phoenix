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
    current_room?: string; // New field for actual room name
  };
  index?: number;
  onBack?: () => void;
  backButtonStyle?: 'integrated' | 'floating' | 'thin-row';
  backDestination?: string;
}

export function ModernStudentProfile({ 
  student, 
  index = 0, 
  onBack: _onBack,
  backButtonStyle: _backButtonStyle = 'floating',
  backDestination: _backDestination = 'Meine Gruppe'
}: ModernStudentProfileProps) {


  return (
    <>
      {/* Mobile Header (visible only on mobile) */}
      <div className="block sm:hidden bg-white border border-gray-200 rounded-xl mb-2">
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
            <ModernStatusBadge location={student.current_location} roomName={student.current_room} />
          </div>
        </div>
      </div>
      

      {/* Desktop Header (visible only on larger screens) */}
      <div className="hidden sm:block bg-white border border-gray-200 rounded-xl">
        {/* Subtle gradient overlay */}
        <div className="absolute inset-0 bg-gradient-to-br from-gray-50/20 to-white/50 opacity-50 rounded-xl"></div>
        
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
              <ModernStatusBadge location={student.current_location} roomName={student.current_room} />
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
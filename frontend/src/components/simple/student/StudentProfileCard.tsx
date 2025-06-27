"use client";

import { StatusBadge } from "./StatusBadge";

interface StudentProfileCardProps {
  student: {
    first_name: string;
    second_name: string;
    name: string;
    school_class: string;
    group_name?: string;
    current_location?: string;
  };
  index?: number;
}

export function StudentProfileCard({ student, index = 0 }: StudentProfileCardProps) {
  // Get year from class for color indicator
  const getYear = (schoolClass: string): number => {
    const yearMatch = /^(\d)/.exec(schoolClass);
    return yearMatch?.[1] ? parseInt(yearMatch[1], 10) : 0;
  };

  const getYearColor = (year: number): string => {
    switch (year) {
      case 1: return "bg-blue-500";
      case 2: return "bg-green-500";
      case 3: return "bg-yellow-500";
      case 4: return "bg-purple-500";
      default: return "bg-gray-400";
    }
  };

  const year = getYear(student.school_class);
  const yearColor = getYearColor(year);

  // Floating animation style from ogs_groups pattern
  const floatingStyle = {
    animation: `float 8s ease-in-out infinite ${index * 0.7}s`,
    transform: `rotate(${(index % 3 - 1) * 0.5}deg)`,
  };

  return (
    <>
      <style jsx>{`
        @keyframes float {
          0%, 100% { transform: translateY(0px) rotate(${(index % 3 - 1) * 0.5}deg); }
          50% { transform: translateY(-10px) rotate(${(index % 3 - 1) * 0.5}deg); }
        }
      `}</style>
      
      <div 
        className="bg-white rounded-2xl p-6 shadow-lg hover:shadow-xl transition-all duration-300 border border-gray-100"
        style={floatingStyle}
      >
        <div className="flex items-center">
          {/* Avatar */}
          <div className="mr-6 flex h-20 w-20 items-center justify-center rounded-full bg-gradient-to-br from-teal-400 to-blue-500 text-white text-2xl font-bold shadow-lg">
            {student.first_name[0]}{student.second_name[0]}
          </div>
          
          {/* Student Info */}
          <div className="flex-1">
            <h1 className="text-2xl font-bold text-gray-900 mb-1">{student.name}</h1>
            
            <div className="flex items-center gap-3 text-gray-600 mb-3">
              <span className="font-medium">Klasse {student.school_class}</span>
              <span className={`inline-block h-3 w-3 rounded-full ${yearColor}`} title={`Jahrgang ${year}`}></span>
              {student.group_name && (
                <>
                  <span>•</span>
                  <span>Gruppe: {student.group_name}</span>
                </>
              )}
            </div>

            {/* Current Location Status */}
            <div className="flex items-center gap-2">
              <span className="text-sm font-medium text-gray-700">Standort:</span>
              <StatusBadge location={student.current_location} />
            </div>
          </div>
        </div>
      </div>
    </>
  );
}
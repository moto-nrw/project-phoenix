'use client';

import { useRouter } from 'next/navigation';
import type { Student } from '@/lib/api';

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
  emptyMessage = 'Keine Schüler vorhanden.'
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
      <div className="text-center py-8">
        <p className="text-gray-500">{emptyMessage}</p>
      </div>
    );
  }

  return (
    <div className="space-y-2">
      {students.map(student => (
        <div 
          key={student.id} 
          onClick={() => handleStudentClick(student)}
          className="group bg-white border border-gray-100 rounded-lg p-4 shadow-sm hover:shadow-md hover:border-blue-200 hover:translate-y-[-1px] transition-all duration-200 cursor-pointer"
        >
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-3">
              <div className="w-10 h-10 bg-gradient-to-r from-blue-400 to-indigo-500 rounded-full flex items-center justify-center text-white font-medium">
                {(student.name || (student.first_name ? `${student.first_name} ${student.second_name || ''}` : 'S')).slice(0, 1).toUpperCase()}
              </div>
              
              <div className="flex flex-col">
                <span className="font-medium text-gray-900 group-hover:text-blue-600 transition-colors">
                  {student.name || (student.first_name ? `${student.first_name} ${student.second_name || ''}` : 'Unnamed Student')}
                </span>
                {showDetails && (
                  <span className="text-sm text-gray-500">
                    {student.school_class && `Klasse: ${student.school_class}`}
                    {student.group_name && ` | Gruppe: ${student.group_name}`}
                  </span>
                )}
              </div>
            </div>
            
            <div className="flex items-center space-x-2">
              <div className={`h-2.5 w-2.5 rounded-full ${student.in_house ? 'bg-green-500' : 'bg-gray-300'}`} 
                   title={student.in_house ? 'Anwesend' : 'Nicht anwesend'}>
              </div>
              
              <svg 
                xmlns="http://www.w3.org/2000/svg" 
                className="h-5 w-5 text-gray-400 group-hover:text-blue-500 group-hover:translate-x-1 transition-all duration-200" 
                fill="none" 
                viewBox="0 0 24 24" 
                stroke="currentColor"
              >
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
              </svg>
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}
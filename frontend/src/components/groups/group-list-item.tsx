'use client';

import type { Group } from '@/lib/api';

interface GroupListItemProps {
  group: Group;
  onClick: () => void;
}

export default function GroupListItem({ group, onClick }: GroupListItemProps) {
  return (
    <div 
      className="group-item group bg-white border border-gray-100 rounded-lg p-4 shadow-sm hover:shadow-md hover:border-blue-200 hover:translate-y-[-1px] transition-all duration-200 cursor-pointer flex items-center justify-between"
      onClick={onClick}
    >
      <div className="flex flex-col group-hover:translate-x-1 transition-transform duration-200">
        <span className="font-semibold text-gray-900 group-hover:text-blue-600 transition-colors duration-200">
          {group.name}
        </span>
        <span className="text-sm text-gray-500">
          {group.room_name ? `Raum: ${group.room_name}` : 'Kein Raum zugewiesen'} 
          {group.student_count !== undefined ? ` | Schüler: ${group.student_count}` : ''}
        </span>
      </div>
      <svg 
        xmlns="http://www.w3.org/2000/svg" 
        className="h-5 w-5 text-gray-400 group-hover:text-blue-500 group-hover:transform group-hover:translate-x-1 transition-all duration-200" 
        fill="none" 
        viewBox="0 0 24 24" 
        stroke="currentColor"
      >
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
      </svg>
    </div>
  );
}
'use client';

import { useSession } from 'next-auth/react';
import { redirect, useRouter } from 'next/navigation';
import { useState, useEffect } from 'react';
import { DataListPage } from '@/components/dashboard';
import type { Student } from '@/lib/api';
import { studentService } from '@/lib/api';

// Mock student data for development
const mockStudents: Student[] = [
  { id: '1', name: 'Anna Müller', school_class: '1A', in_house: true },
  { id: '2', name: 'Max Schmidt', school_class: '1A', in_house: false },
  { id: '3', name: 'Sophie Weber', school_class: '2B', in_house: true },
  { id: '4', name: 'Lena Fischer', school_class: '2B', in_house: false },
  { id: '5', name: 'Noah Meyer', school_class: '3C', in_house: true },
  { id: '6', name: 'Emma Wagner', school_class: '3C', in_house: false },
  { id: '7', name: 'Luis Becker', school_class: '4D', in_house: true },
  { id: '8', name: 'Mia Hoffmann', school_class: '4D', in_house: false },
  { id: '9', name: 'Finn Schneider', school_class: '5E', in_house: true },
  { id: '10', name: 'Lara Schulz', school_class: '5E', in_house: false },
];

export default function StudentsPage() {
  const router = useRouter();
  const [students, setStudents] = useState<Student[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchFilter, setSearchFilter] = useState('');
  
  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect('/login');
    },
  });

  // Function to fetch students with optional filters
  const fetchStudents = async (search?: string) => {
    try {
      setLoading(true);
      
      // Prepare filters for API call
      const filters = {
        search: search || undefined
      };
      
      // Try to fetch from the real API
      try {
        const data = await studentService.getStudents(filters);
        setStudents(data);
        setError(null);
      } catch (apiErr) {
        console.warn('Using mock data due to API error:', apiErr);
        
        // In development, filter mock data to simulate API filtering
        let filteredMockData = mockStudents;
        if (search) {
          filteredMockData = mockStudents.filter(student => 
            student.name.toLowerCase().includes(search.toLowerCase())
          );
        }
        
        setStudents(filteredMockData);
        setError(null);
      }
    } catch (err) {
      console.error('Error fetching students:', err);
      setError('Fehler beim Laden der Schülerdaten. Bitte versuchen Sie es später erneut.');
      setStudents([]);
    } finally {
      setLoading(false);
    }
  };

  // Initial data load
  useEffect(() => {
    void fetchStudents();
  }, []);

  // Handle search filter changes
  useEffect(() => {
    // Debounce search to avoid too many API calls
    const timer = setTimeout(() => {
      void fetchStudents(searchFilter);
    }, 300);
    
    return () => clearTimeout(timer);
  }, [searchFilter]);

  if (status === 'loading' || loading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p>Loading...</p>
      </div>
    );
  }

  const handleSelectStudent = (student: Student) => {
    router.push(`/database/students/${student.id}`);
  };

  // Handle search input change - Not used directly since DataListPage handles this
  // But may be useful for future enhancements
  const handleSearchChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setSearchFilter(e.target.value);
  };

  // Custom renderer for student items
  const renderStudent = (student: Student) => (
    <>
      <div className="flex flex-col group-hover:translate-x-1 transition-transform duration-200">
        <span className="font-semibold text-gray-900 group-hover:text-blue-600 transition-colors duration-200">
          {student.name}
          {student.in_house && (
            <span className="ml-2 px-2 py-0.5 bg-green-100 text-green-800 text-xs rounded-full">
              Anwesend
            </span>
          )}
        </span>
        <span className="text-sm text-gray-500">
          Klasse: {student.school_class} 
          {student.group_name && ` | Gruppe: ${student.group_name}`}
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
    </>
  );

  // Show error if loading failed
  if (error) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center p-4">
        <div className="bg-red-50 text-red-800 p-4 rounded-lg max-w-md">
          <h2 className="font-semibold mb-2">Fehler</h2>
          <p>{error}</p>
          <button 
            onClick={() => fetchStudents()} 
            className="mt-4 px-4 py-2 bg-red-100 hover:bg-red-200 text-red-800 rounded transition-colors"
          >
            Erneut versuchen
          </button>
        </div>
      </div>
    );
  }

  return (
    <DataListPage
      title="Schülerauswahl"
      sectionTitle="Schüler auswählen"
      backUrl="/database"
      newEntityLabel="Neuen Schüler erstellen"
      newEntityUrl="/database/students/new"
      data={students}
      onSelectEntity={handleSelectStudent}
      renderEntity={renderStudent}
    />
  );
}
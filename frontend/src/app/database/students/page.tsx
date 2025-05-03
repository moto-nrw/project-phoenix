'use client';

import { useSession } from 'next-auth/react';
import { redirect, useRouter } from 'next/navigation';
import { useState, useEffect } from 'react';
import { PageHeader, SectionTitle } from '@/components/dashboard';
import type { Student } from '@/lib/api';
import { studentService } from '@/lib/api';
import { GroupSelector } from '@/components/groups';
import Link from 'next/link';

// Student list will be loaded from API

export default function StudentsPage() {
  const router = useRouter();
  const [students, setStudents] = useState<Student[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchFilter, setSearchFilter] = useState('');
  const [groupFilter, setGroupFilter] = useState<string | null>(null);
  
  // const handleSearchInput = (value: string) => {
  //   setSearchFilter(value);
  // };
  
  // const handleFilterChange = (filterId: string, value: string | null) => {
  //   if (filterId === 'group') {
  //     setGroupFilter(value);
  //   }
  // };
  
  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect('/login');
    },
  });

  // Function to fetch students with optional filters
  const fetchStudents = async (search?: string, groupId?: string | null) => {
    try {
      setLoading(true);
      
      // Prepare filters for API call
      const filters = {
        search: search ?? undefined,
        groupId: groupId ?? undefined
      };
      
      try {
        // Fetch from the real API using our student service
        const data = await studentService.getStudents(filters);
        
        if (data.length === 0 && !search && !groupId) {
          console.log('No students returned from API, checking connection');
        }
        
        setStudents(data);
        setError(null);
      } catch (apiErr) {
        console.error('API error when fetching students:', apiErr);
        setError('Fehler beim Laden der Schülerdaten. Bitte versuchen Sie es später erneut.');
        setStudents([]);
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

  // Handle search and group filter changes
  useEffect(() => {
    // Debounce search to avoid too many API calls
    const timer = setTimeout(() => {
      void fetchStudents(searchFilter, groupFilter);
    }, 300);
    
    return () => clearTimeout(timer);
  }, [searchFilter, groupFilter]);

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
          {student.group_name && student.group_id && ` | Gruppe: `}
          {student.group_name && student.group_id && (
            <a 
              href={`/database/groups/${student.group_id}`} 
              className="text-blue-600 hover:text-blue-800 hover:underline transition-colors inline-block"
              onClick={(e) => {
                e.stopPropagation(); // Prevent triggering the parent click event
              }}
            >
              {student.group_name}
            </a>
          )}
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

  // Define the filters for the student list
  /*
  const filters = [
    {
      id: 'group',
      label: 'Gruppe',
      options: [
        { label: 'Alle Gruppen', value: null },
        // The actual options will be populated by the GroupSelector component
      ],
    },
  ];
  */

  return (
    <div className="min-h-screen">
      <PageHeader 
        title="Schülerauswahl"
        backUrl="/database"
      />

      <main className="max-w-4xl mx-auto p-4">
        <div className="mb-8">
          <SectionTitle title="Schüler auswählen" />
        </div>

        {/* Search and Add Section with Filters */}
        <div className="mb-8">
          <div className="flex flex-col sm:flex-row items-center justify-between gap-4 mb-4">
            <div className="relative w-full sm:max-w-md">
              <input
                type="text"
                placeholder="Suchen..."
                value={searchFilter}
                onChange={(e) => setSearchFilter(e.target.value)}
                className="w-full px-4 py-3 rounded-lg border border-gray-300 focus:outline-none focus:ring-2 focus:ring-blue-500 pl-10 transition-all duration-200 hover:border-gray-400 focus:shadow-md"
              />
              <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                <svg 
                  xmlns="http://www.w3.org/2000/svg" 
                  className="h-5 w-5 text-gray-400" 
                  fill="none" 
                  viewBox="0 0 24 24" 
                  stroke="currentColor"
                >
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                </svg>
              </div>
            </div>
            
            <Link href="/database/students/new" className="w-full sm:w-auto">
              <button className="group w-full sm:w-auto bg-gradient-to-r from-teal-500 to-blue-600 text-white py-3 px-4 rounded-lg flex items-center gap-2 hover:from-teal-600 hover:to-blue-700 hover:scale-[1.02] hover:shadow-lg transition-all duration-200 justify-center sm:justify-start">
                <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5 transition-transform duration-200 group-hover:rotate-90" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
                </svg>
                <span>Neuen Schüler erstellen</span>
              </button>
            </Link>
          </div>
          
          {/* Filter Section */}
          <div className="flex items-center gap-2 mt-4">
            <span className="text-sm text-gray-500">Filter:</span>
            <div className="w-48">
              <GroupSelector
                value={groupFilter ?? ''}
                onChange={(value) => setGroupFilter(value === '' ? null : value)}
                includeEmpty={true}
                emptyLabel="Alle Gruppen"
                label=""
              />
            </div>
          </div>
        </div>

        {/* Student List */}
        <div className="space-y-3 w-full">
          {students.length > 0 ? (
            students.map(student => (
              <div 
                key={student.id} 
                className="group bg-white border border-gray-100 rounded-lg p-4 shadow-sm hover:shadow-md hover:border-blue-200 hover:translate-y-[-1px] transition-all duration-200 cursor-pointer flex items-center justify-between"
                onClick={() => handleSelectStudent(student)}
              >
                {renderStudent(student)}
              </div>
            ))
          ) : (
            <div className="text-center py-8">
              <p className="text-gray-500">
                {searchFilter || groupFilter ? 'Keine Ergebnisse gefunden.' : 'Keine Einträge vorhanden.'}
              </p>
            </div>
          )}
        </div>
      </main>
    </div>
  );
}
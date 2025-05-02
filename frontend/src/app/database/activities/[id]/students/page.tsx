'use client';

import { useSession } from 'next-auth/react';
import { redirect, useRouter, useParams } from 'next/navigation';
import { useState, useEffect } from 'react';
import { PageHeader, SectionTitle } from '@/components/dashboard';
import StudentList from '@/components/students/student-list';
import type { Activity } from '@/lib/activity-api';
import type { Student } from '@/lib/api';
import { activityService } from '@/lib/activity-api';
import Link from 'next/link';

export default function ActivityStudentsPage() {
  const router = useRouter();
  const params = useParams();
  const { id } = params;
  const [activity, setActivity] = useState<Activity | null>(null);
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

  // Function to fetch the activity details
  const fetchActivity = async () => {
    if (!id) return;

    try {
      setLoading(true);
      
      try {
        // Fetch activity from API
        const data = await activityService.getActivity(id as string);
        setActivity(data);
        
        // Fetch enrolled students
        const enrolledStudents = await activityService.getEnrolledStudents(id as string);
        setStudents(enrolledStudents || []); // Ensure students is always an array
        
        setError(null);
      } catch (apiErr) {
        console.error('API error when fetching activity and students:', apiErr);
        setError('Fehler beim Laden der Daten. Bitte versuchen Sie es später erneut.');
        setActivity(null);
        setStudents([]); // Ensure students is always an array
      }
    } catch (err) {
      console.error('Error fetching activity and students:', err);
      setError('Fehler beim Laden der Daten. Bitte versuchen Sie es später erneut.');
      setActivity(null);
      setStudents([]); // Ensure students is always an array
    } finally {
      setLoading(false);
    }
  };

  // Function to unenroll a student
  const handleUnenrollStudent = async (studentId: string) => {
    if (!id || !studentId) return;
    
    if (!confirm('Möchten Sie diesen Schüler wirklich von der Aktivität abmelden?')) {
      return;
    }
    
    try {
      await activityService.unenrollStudent(id as string, studentId);
      
      // Refresh student list
      await fetchActivity();
    } catch (err) {
      console.error('Error unenrolling student:', err);
      alert('Fehler beim Abmelden des Schülers. Bitte versuchen Sie es später erneut.');
    }
  };

  // Initial data load
  useEffect(() => {
    void fetchActivity();
  }, [id]);

  // Filter students based on search term
  const filteredStudents = students?.filter(student =>
    (student?.name?.toLowerCase()?.includes(searchFilter.toLowerCase()) ?? false) ||
    (student?.school_class?.toLowerCase()?.includes(searchFilter.toLowerCase()) ?? false)
  ) || [];

  if (status === 'loading' || loading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p>Loading...</p>
      </div>
    );
  }

  // Show error if loading failed
  if (error) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center p-4">
        <div className="bg-red-50 text-red-800 p-4 rounded-lg max-w-md">
          <h2 className="font-semibold mb-2">Fehler</h2>
          <p>{error}</p>
          <button 
            onClick={() => fetchActivity()} 
            className="mt-4 px-4 py-2 bg-red-100 hover:bg-red-200 text-red-800 rounded transition-colors"
          >
            Erneut versuchen
          </button>
        </div>
      </div>
    );
  }

  if (!activity) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center p-4">
        <div className="bg-orange-50 text-orange-800 p-4 rounded-lg max-w-md">
          <h2 className="font-semibold mb-2">Aktivität nicht gefunden</h2>
          <p>Die angeforderte Aktivität konnte nicht gefunden werden.</p>
          <Link href="/database/activities">
            <button className="mt-4 px-4 py-2 bg-orange-100 hover:bg-orange-200 text-orange-800 rounded transition-colors">
              Zurück zur Übersicht
            </button>
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen">
      <PageHeader 
        title={`Teilnehmer: ${activity.name}`}
        backUrl={`/database/activities/${activity.id}`}
      />

      <main className="max-w-4xl mx-auto p-4">
        <div className="mb-8">
          <SectionTitle title="Teilnehmer verwalten" />
        </div>

        {/* Search and Add Section */}
        <div className="flex flex-col sm:flex-row items-center justify-between gap-4 mb-8">
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
          
          <Link href={`/database/activities/${activity.id}/add-students`} className="w-full sm:w-auto">
            <button className="group w-full sm:w-auto bg-gradient-to-r from-teal-500 to-blue-600 text-white py-3 px-4 rounded-lg flex items-center gap-2 hover:from-teal-600 hover:to-blue-700 hover:scale-[1.02] hover:shadow-lg transition-all duration-200 justify-center sm:justify-start">
              <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5 transition-transform duration-200 group-hover:rotate-90" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
              </svg>
              <span>Schüler hinzufügen</span>
            </button>
          </Link>
        </div>

        {/* Students List */}
        <div className="bg-white border border-gray-100 rounded-lg p-6 shadow-sm">
          <div className="mb-4 flex justify-between items-center">
            <h3 className="text-lg font-semibold text-gray-800">
              Teilnehmerliste {activity.max_participant > 0 && (
                <span className="text-sm font-normal text-gray-500">
                  ({students.length} / {activity.max_participant})
                </span>
              )}
            </h3>
          </div>

          {filteredStudents.length === 0 && searchFilter ? (
            <div className="text-center py-8">
              <p className="text-gray-500">Keine Ergebnisse für "{searchFilter}"</p>
            </div>
          ) : (
            <div className="relative">
              {/* Custom wrapper to add the unenroll button to each student item */}
              <StudentList 
                students={filteredStudents}
                onStudentClick={(student) => router.push(`/database/students/${student.id}`)}
                emptyMessage="Keine Teilnehmer gefunden."
              />
              
              {/* Add unenroll buttons */}
              {filteredStudents.length > 0 && (
                <div className="absolute right-0 top-0 h-full w-full pointer-events-none">
                  {filteredStudents.map((student, index) => (
                    <div key={student.id} className="absolute right-4" style={{ top: `${index * 78 + 20}px` }}>
                      <button 
                        onClick={(e) => {
                          e.stopPropagation();
                          handleUnenrollStudent(student.id);
                        }}
                        className="p-2 text-red-500 hover:text-red-700 hover:bg-red-50 rounded transition-colors pointer-events-auto"
                        title="Schüler abmelden"
                      >
                        <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                        </svg>
                      </button>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>
      </main>
    </div>
  );
}
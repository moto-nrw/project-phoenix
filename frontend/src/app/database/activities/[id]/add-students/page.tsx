'use client';

import { useSession } from 'next-auth/react';
import { redirect, useRouter, useParams } from 'next/navigation';
import { useState, useEffect } from 'react';
import { PageHeader, SectionTitle } from '@/components/dashboard';
import type { Activity } from '@/lib/activity-api';
import type { Student } from '@/lib/api';
import { activityService } from '@/lib/activity-api';
import { studentService } from '@/lib/api';
import Link from 'next/link';

export default function AddStudentsToActivityPage() {
  const router = useRouter();
  const params = useParams();
  const { id } = params;
  const [activity, setActivity] = useState<Activity | null>(null);
  const [availableStudents, setAvailableStudents] = useState<Student[]>([]);
  const [loading, setLoading] = useState(true);
  const [enrollingStudent, setEnrollingStudent] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [searchFilter, setSearchFilter] = useState('');
  
  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect('/login');
    },
  });

  // Function to fetch data
  const fetchData = async () => {
    if (!id) return;

    try {
      setLoading(true);
      
      try {
        // Fetch activity details
        const activityData = await activityService.getActivity(id as string);
        setActivity(activityData);
        
        // Get all students
        const allStudents = await studentService.getStudents();
        
        // Filter out students already enrolled
        const enrolledStudentIds = new Set(
          (activityData.students || []).map(student => student.id)
        );
        
        // Available students are those not already enrolled
        const available = allStudents.filter(student => !enrolledStudentIds.has(student.id));
        setAvailableStudents(available);
        
        setError(null);
      } catch (apiErr) {
        console.error('API error when fetching data:', apiErr);
        setError('Fehler beim Laden der Daten. Bitte versuchen Sie es später erneut.');
        setActivity(null);
        setAvailableStudents([]);
      }
    } catch (err) {
      console.error('Error fetching data:', err);
      setError('Fehler beim Laden der Daten. Bitte versuchen Sie es später erneut.');
      setActivity(null);
      setAvailableStudents([]);
    } finally {
      setLoading(false);
    }
  };

  // Function to enroll a student
  const handleEnrollStudent = async (studentId: string) => {
    if (!id || !studentId) return;
    if (enrollingStudent) return; // Prevent duplicate enrollments
    
    try {
      setEnrollingStudent(true);
      await activityService.enrollStudent(id as string, studentId);
      
      // Remove this student from the available list
      setAvailableStudents(current => 
        current.filter(student => student.id !== studentId)
      );
    } catch (err) {
      console.error('Error enrolling student:', err);
      alert('Fehler beim Einschreiben des Schülers. Bitte versuchen Sie es später erneut.');
    } finally {
      setEnrollingStudent(false);
    }
  };

  // Initial data load
  useEffect(() => {
    void fetchData();
  }, [id]);

  // Filter students based on search term
  const filteredStudents = availableStudents.filter(student =>
    student.name.toLowerCase().includes(searchFilter.toLowerCase()) ||
    (student.school_class && student.school_class.toLowerCase().includes(searchFilter.toLowerCase()))
  );

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
            onClick={() => fetchData()} 
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

  // Check if activity is full
  const isFull = activity.participant_count !== undefined && 
                 activity.max_participant !== undefined && 
                 activity.participant_count >= activity.max_participant;

  return (
    <div className="min-h-screen">
      <PageHeader 
        title={`Schüler zu ${activity.name} hinzufügen`}
        backUrl={`/database/activities/${activity.id}/students`}
      />

      <main className="max-w-4xl mx-auto p-4">
        <div className="mb-8">
          <SectionTitle title="Schüler einschreiben" />
        </div>

        {/* Status Summary */}
        <div className="bg-white border border-gray-100 rounded-lg p-4 shadow-sm mb-6">
          <h3 className="text-lg font-medium text-gray-800">Aktivitätsstatus</h3>
          <div className="mt-2 flex flex-wrap gap-4">
            <div>
              <span className="text-sm text-gray-500">Teilnehmer:</span> 
              <span className="ml-2 font-medium">
                {activity.participant_count || 0} / {activity.max_participant}
              </span>
              {isFull && (
                <span className="ml-2 px-2 py-0.5 bg-yellow-100 text-yellow-800 text-xs rounded-full">
                  Voll
                </span>
              )}
            </div>
            <div>
              <span className="text-sm text-gray-500">Kategorie:</span>
              <span className="ml-2 font-medium">{activity.category_name || 'Keine'}</span>
            </div>
          </div>
        </div>

        {/* Search Section */}
        <div className="relative w-full mb-6">
          <input
            type="text"
            placeholder="Schüler suchen..."
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

        {/* Available Students List */}
        <div className="bg-white border border-gray-100 rounded-lg p-6 shadow-sm">
          <div className="mb-4">
            <h3 className="text-lg font-semibold text-gray-800">
              Verfügbare Schüler
            </h3>
            <p className="text-sm text-gray-500">
              Klicken Sie auf einen Schüler, um ihn zur Aktivität hinzuzufügen.
            </p>
          </div>

          {isFull && (
            <div className="mb-4 bg-yellow-50 border border-yellow-200 rounded-lg p-4">
              <div className="flex">
                <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5 text-yellow-600 mr-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                </svg>
                <div>
                  <p className="text-yellow-800 font-medium">Diese Aktivität ist bereits voll.</p>
                  <p className="text-yellow-700 text-sm">Sie können die maximale Teilnehmerzahl auf der Bearbeitungsseite erhöhen.</p>
                </div>
              </div>
            </div>
          )}

          {availableStudents.length === 0 ? (
            <div className="text-center py-8">
              <p className="text-gray-500">Keine verfügbaren Schüler gefunden.</p>
            </div>
          ) : filteredStudents.length === 0 ? (
            <div className="text-center py-8">
              <p className="text-gray-500">Keine Ergebnisse für "{searchFilter}"</p>
            </div>
          ) : (
            <div className="space-y-3">
              {filteredStudents.map((student) => (
                <div 
                  key={student.id}
                  className={`group p-4 border rounded-lg flex justify-between items-center transition-all
                    ${isFull 
                      ? 'border-gray-200 bg-gray-50 cursor-not-allowed' 
                      : 'border-gray-100 hover:border-green-200 hover:bg-green-50 cursor-pointer'
                    }`}
                  onClick={() => !isFull && handleEnrollStudent(student.id)}
                >
                  <div className="flex flex-col sm:flex-row sm:items-center gap-1 sm:gap-4">
                    <span className={`font-medium ${isFull ? 'text-gray-500' : 'text-gray-900 group-hover:text-green-700'} transition-colors`}>
                      {student.name}
                    </span>
                    {student.school_class && (
                      <span className="text-sm text-gray-500">Klasse: {student.school_class}</span>
                    )}
                    {student.in_house && (
                      <span className="px-2 py-0.5 bg-green-100 text-green-800 text-xs rounded-full inline-block">
                        Anwesend
                      </span>
                    )}
                  </div>
                  {!isFull && (
                    <svg 
                      xmlns="http://www.w3.org/2000/svg" 
                      className="h-5 w-5 text-gray-400 group-hover:text-green-600 transition-colors" 
                      fill="none" 
                      viewBox="0 0 24 24" 
                      stroke="currentColor"
                    >
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
                    </svg>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      </main>
    </div>
  );
}
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
  const [refreshing, setRefreshing] = useState(false);
  const [enrollingStudent, setEnrollingStudent] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [searchFilter, setSearchFilter] = useState('');
  
  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect('/login');
    },
  });

  // Function to fetch data
  const fetchData = async (showRefreshing = false) => {
    if (!id) return;

    try {
      if (showRefreshing) {
        setRefreshing(true);
      } else {
        setLoading(true);
      }
      
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
      setRefreshing(false);
    }
  };

  // Function to enroll a student
  const handleEnrollStudent = async (studentId: string) => {
    if (!id || !studentId) return;
    if (enrollingStudent !== null) return; // Prevent duplicate enrollments
    
    setEnrollingStudent(studentId);
    
    try {
      await activityService.enrollStudent(id as string, studentId);
      
      // Find the enrolled student for the success message
      const enrolledStudent = availableStudents.find(s => s.id === studentId);
      const studentName = enrolledStudent?.name || 'Schüler';
      
      // Remove this student from the available list
      setAvailableStudents(current => 
        current.filter(student => student.id !== studentId)
      );
      
      // Update the activity data to reflect new enrollment count
      if (activity && typeof activity.participant_count === 'number') {
        setActivity({
          ...activity,
          participant_count: activity.participant_count + 1
        });
      }
      
      // Show success toast
      const toastElement = document.createElement('div');
      toastElement.className = 'fixed bottom-4 right-4 bg-green-50 text-green-800 px-4 py-2 rounded-lg shadow-lg border border-green-200 animate-fade-in-out z-50 flex items-center';
      toastElement.innerHTML = `
        <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 mr-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
        </svg>
        <span>${studentName} erfolgreich eingeschrieben</span>
      `;
      document.body.appendChild(toastElement);
      
      // Remove the toast after 3 seconds
      setTimeout(() => {
        if (document.body.contains(toastElement)) {
          document.body.removeChild(toastElement);
        }
      }, 3000);
    } catch (err) {
      console.error('Error enrolling student:', err);
      
      // Show error toast instead of alert
      const toastElement = document.createElement('div');
      toastElement.className = 'fixed bottom-4 right-4 bg-red-50 text-red-800 px-4 py-2 rounded-lg shadow-lg border border-red-200 animate-fade-in-out z-50 flex items-center';
      toastElement.innerHTML = `
        <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 mr-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
        </svg>
        <span>Fehler beim Einschreiben des Schülers</span>
      `;
      document.body.appendChild(toastElement);
      
      // Remove the toast after 3 seconds
      setTimeout(() => {
        if (document.body.contains(toastElement)) {
          document.body.removeChild(toastElement);
        }
      }, 3000);
    } finally {
      setEnrollingStudent(null);
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
        {refreshing && (
          <div className="fixed top-4 right-4 bg-blue-50 text-blue-800 px-4 py-2 rounded-lg shadow-lg border border-blue-200 flex items-center z-50">
            <svg className="animate-spin h-4 w-4 mr-2" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
              <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
              <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            Aktualisiere Schülerliste...
          </div>
        )}
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
          <div className="mb-4 flex justify-between items-start">
            <div>
              <h3 className="text-lg font-semibold text-gray-800">
                Verfügbare Schüler
              </h3>
              <p className="text-sm text-gray-500">
                Klicken Sie auf einen Schüler, um ihn zur Aktivität hinzuzufügen.
              </p>
            </div>
            <button
              onClick={() => fetchData(true)}
              className="p-2 text-blue-500 hover:text-blue-700 hover:bg-blue-50 rounded-lg transition-colors"
              disabled={refreshing}
              title="Schülerliste aktualisieren"
            >
              <svg xmlns="http://www.w3.org/2000/svg" className={`h-5 w-5 ${refreshing ? 'animate-spin' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
            </button>
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
                    ${enrollingStudent === student.id ? 'border-green-200 bg-green-50 cursor-wait' :
                      isFull ? 'border-gray-200 bg-gray-50 cursor-not-allowed' : 
                      'border-gray-100 hover:border-green-200 hover:bg-green-50 cursor-pointer'
                    }`}
                  onClick={() => !isFull && enrollingStudent !== student.id && !enrollingStudent && handleEnrollStudent(student.id)}
                >
                  <div className="flex items-center space-x-4">
                    {/* Avatar placeholder */}
                    <div className="flex-shrink-0 h-10 w-10 rounded-full bg-gradient-to-r from-purple-400 to-indigo-500 flex items-center justify-center text-white font-medium">
                      {student.name ? student.name.charAt(0).toUpperCase() : '?'}
                    </div>
                    
                    {/* Student info */}
                    <div>
                      <span className={`font-medium ${isFull ? 'text-gray-500' : 'text-gray-900 group-hover:text-green-700'} transition-colors`}>
                        {student.name}
                      </span>
                      <div className="flex flex-wrap gap-2 mt-1">
                        {student.school_class && (
                          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-100 text-gray-800">
                            {student.school_class}
                          </span>
                        )}
                        {student.in_house && (
                          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-green-100 text-green-800">
                            <svg xmlns="http://www.w3.org/2000/svg" className="h-3 w-3 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                            </svg>
                            Anwesend
                          </span>
                        )}
                      </div>
                    </div>
                  </div>
                  
                  {/* Action button with loading state */}
                  {enrollingStudent === student.id ? (
                    <svg className="animate-spin h-5 w-5 text-green-600" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                    </svg>
                  ) : !isFull && (
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
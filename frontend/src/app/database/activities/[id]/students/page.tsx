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
  const [unenrollingStudent, setUnenrollingStudent] = useState<string | null>(null);
  const [showConfirmModal, setShowConfirmModal] = useState(false);
  const [selectedStudentId, setSelectedStudentId] = useState<string | null>(null);
  const [selectedStudentName, setSelectedStudentName] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [searchFilter, setSearchFilter] = useState('');
  const [refreshing, setRefreshing] = useState(false);
  
  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect('/login');
    },
  });

  // Function to fetch the activity details
  const fetchActivity = async (showRefreshing = false) => {
    if (!id) return;

    try {
      if (showRefreshing) {
        setRefreshing(true);
      } else {
        setLoading(true);
      }
      
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
      setRefreshing(false);
    }
  };

  // Function to open the confirmation modal
  const openUnenrollConfirmation = (studentId: string) => {
    const student = students.find(s => s.id === studentId);
    if (student) {
      setSelectedStudentId(studentId);
      setSelectedStudentName(student.name);
      setShowConfirmModal(true);
    }
  };

  // Function to unenroll a student
  const handleUnenrollStudent = async (studentId: string) => {
    if (!id || !studentId) return;
    setShowConfirmModal(false);
    
    try {
      setUnenrollingStudent(studentId);
      await activityService.unenrollStudent(id as string, studentId);
      
      // Update the student list locally - more efficient than a full refresh
      setStudents(prevStudents => prevStudents.filter(student => student.id !== studentId));
      
      // Show a short success message
      const toastElement = document.createElement('div');
      toastElement.className = 'fixed bottom-4 right-4 bg-green-50 text-green-800 px-4 py-2 rounded-lg shadow-lg border border-green-200 animate-fade-in-out';
      toastElement.textContent = 'Schüler erfolgreich abgemeldet';
      document.body.appendChild(toastElement);
      
      // Remove the toast after 3 seconds
      setTimeout(() => {
        if (document.body.contains(toastElement)) {
          document.body.removeChild(toastElement);
        }
      }, 3000);
    } catch (err) {
      console.error('Error unenrolling student:', err);
      
      // Show error toast
      const toastElement = document.createElement('div');
      toastElement.className = 'fixed bottom-4 right-4 bg-red-50 text-red-800 px-4 py-2 rounded-lg shadow-lg border border-red-200 animate-fade-in-out';
      toastElement.textContent = 'Fehler beim Abmelden des Schülers';
      document.body.appendChild(toastElement);
      
      // Remove the toast after 3 seconds
      setTimeout(() => {
        if (document.body.contains(toastElement)) {
          document.body.removeChild(toastElement);
        }
      }, 3000);
    } finally {
      setUnenrollingStudent(null);
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
      {/* Confirmation Modal */}
      {showConfirmModal && (
        <div className="fixed inset-0 bg-black bg-opacity-30 z-50 flex items-center justify-center p-4">
          <div className="bg-white rounded-lg shadow-xl max-w-md w-full p-6" onClick={e => e.stopPropagation()}>
            <div className="text-center">
              <svg className="mx-auto h-12 w-12 text-red-500" fill="none" stroke="currentColor" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"></path>
              </svg>
              <h3 className="mt-4 text-xl font-medium text-gray-900">Schüler abmelden</h3>
              <p className="mt-2 text-gray-500">
                Sind Sie sicher, dass Sie {selectedStudentName} von der Aktivität {activity.name} abmelden möchten?
              </p>
            </div>
            <div className="mt-6 flex justify-end gap-3">
              <button
                onClick={() => setShowConfirmModal(false)}
                className="px-4 py-2 bg-gray-100 text-gray-700 rounded-lg hover:bg-gray-200 transition-colors"
              >
                Abbrechen
              </button>
              <button
                onClick={() => selectedStudentId && handleUnenrollStudent(selectedStudentId)}
                className="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 transition-colors"
              >
                Schüler abmelden
              </button>
            </div>
          </div>
        </div>
      )}
      
      {/* Page Content */}
      <PageHeader 
        title={`Teilnehmer: ${activity.name}`}
        backUrl={`/database/activities/${activity.id}`}
      />

      <main className="max-w-4xl mx-auto p-4">
        {refreshing && (
          <div className="fixed top-4 right-4 bg-blue-50 text-blue-800 px-4 py-2 rounded-lg shadow-lg border border-blue-200 flex items-center">
            <svg className="animate-spin h-4 w-4 mr-2" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
              <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
              <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            Aktualisiere Daten...
          </div>
        )}
        <div className="mb-8 flex items-center justify-between">
          <SectionTitle title="Teilnehmer verwalten" />
          <button 
            onClick={() => fetchActivity(true)}
            className="p-2 text-blue-500 hover:text-blue-700 hover:bg-blue-50 rounded-lg transition-colors"
            disabled={refreshing}
            title="Aktualisieren"
          >
            <svg xmlns="http://www.w3.org/2000/svg" className={`h-5 w-5 ${refreshing ? 'animate-spin' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
          </button>
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
            
            {/* Capacity indicator */}
            <div className="hidden sm:flex items-center">
              <div className="w-32 h-2 bg-gray-200 rounded-full overflow-hidden">
                <div 
                  className={`h-full ${
                    students.length >= activity.max_participant ? 'bg-red-500' : 
                    students.length >= activity.max_participant * 0.8 ? 'bg-yellow-500' : 
                    'bg-green-500'
                  }`}
                  style={{
                    width: `${Math.min(students.length / (activity.max_participant || 1) * 100, 100)}%`
                  }}
                ></div>
              </div>
              <span className="ml-2 text-sm text-gray-600">
                {students.length >= activity.max_participant ? 'Voll' : 
                 students.length >= activity.max_participant * 0.8 ? 'Fast voll' : 
                 'Verfügbar'}
              </span>
            </div>
          </div>

          {filteredStudents.length === 0 && searchFilter ? (
            <div className="text-center py-8">
              <p className="text-gray-500">Keine Ergebnisse für "{searchFilter}"</p>
            </div>
          ) : filteredStudents.length === 0 ? (
            <div className="text-center py-12 border-2 border-dashed border-gray-200 rounded-lg">
              <svg xmlns="http://www.w3.org/2000/svg" className="h-12 w-12 mx-auto text-gray-300 mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
              </svg>
              <p className="text-gray-500 text-lg font-medium">Keine Teilnehmer eingeschrieben</p>
              <p className="text-gray-400 mt-1">Fügen Sie Schüler zu dieser Aktivität hinzu</p>
              <Link href={`/database/activities/${activity.id}/add-students`}>
                <button className="mt-4 px-4 py-2 bg-blue-50 text-blue-600 rounded-lg hover:bg-blue-100 transition-colors">
                  Schüler hinzufügen
                </button>
              </Link>
            </div>
          ) : (
            <div className="space-y-4">
              {/* Enhanced student list with more informative cards */}
              {filteredStudents.map((student) => (
                <div 
                  key={student.id}
                  className="group p-4 border border-gray-100 rounded-lg shadow-sm hover:shadow hover:border-blue-200 transition-all duration-200 bg-white"
                >
                  <div className="flex items-center justify-between">
                    <div className="flex items-center space-x-4" onClick={() => router.push(`/database/students/${student.id}`)} style={{ cursor: 'pointer' }}>
                      {/* Avatar placeholder */}
                      <div className="flex-shrink-0 h-10 w-10 rounded-full bg-gradient-to-r from-blue-400 to-indigo-500 flex items-center justify-center text-white font-medium">
                        {student.name ? student.name.charAt(0).toUpperCase() : '?'}
                      </div>
                      
                      {/* Student info */}
                      <div>
                        <h4 className="text-md font-medium text-gray-900">{student.name}</h4>
                        <div className="flex flex-wrap gap-2 mt-1">
                          {student.school_class && (
                            <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-100 text-gray-800">
                              Klasse: {student.school_class}
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
                    
                    {/* Actions */}
                    <div className="flex space-x-2">
                      <button 
                        onClick={() => router.push(`/database/students/${student.id}`)}
                        className="flex items-center justify-center h-8 w-8 text-gray-400 hover:text-blue-600 hover:bg-blue-50 rounded-full transition-all duration-200 border border-gray-200 hover:border-blue-200"
                        title="Schüler anzeigen"
                      >
                        <svg xmlns="http://www.w3.org/2000/svg" className="h-4 w-4" viewBox="0 0 20 20" fill="currentColor">
                          <path d="M10 12a2 2 0 100-4 2 2 0 000 4z" />
                          <path fillRule="evenodd" d="M.458 10C1.732 5.943 5.522 3 10 3s8.268 2.943 9.542 7c-1.274 4.057-5.064 7-9.542 7S1.732 14.057.458 10zM14 10a4 4 0 11-8 0 4 4 0 018 0z" clipRule="evenodd" />
                        </svg>
                      </button>
                      <button 
                        onClick={(e) => {
                          e.stopPropagation();
                          openUnenrollConfirmation(student.id);
                        }}
                        className="flex items-center justify-center h-8 w-8 text-gray-400 hover:text-red-600 hover:bg-red-50 rounded-full transition-all duration-200 border border-gray-200 hover:border-red-200"
                        title="Schüler abmelden"
                        disabled={unenrollingStudent === student.id}
                      >
                        {unenrollingStudent === student.id ? (
                          <svg className="animate-spin h-4 w-4 text-red-600" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                            <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                            <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                          </svg>
                        ) : (
                          <svg xmlns="http://www.w3.org/2000/svg" className="h-4 w-4" viewBox="0 0 20 20" fill="currentColor">
                            <path fillRule="evenodd" d="M9 2a1 1 0 00-.894.553L7.382 4H4a1 1 0 000 2v10a2 2 0 002 2h8a2 2 0 002-2V6a1 1 0 100-2h-3.382l-.724-1.447A1 1 0 0011 2H9zM7 8a1 1 0 012 0v6a1 1 0 11-2 0V8zm5-1a1 1 0 00-1 1v6a1 1 0 102 0V8a1 1 0 00-1-1z" clipRule="evenodd" />
                          </svg>
                        )}
                      </button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </main>
    </div>
  );
}
'use client';

import { useSession } from 'next-auth/react';
import { redirect, useRouter, useParams } from 'next/navigation';
import { useState, useEffect } from 'react';
import { PageHeader, SectionTitle } from '@/components/dashboard';
import type { Activity, ActivityCategory } from '@/lib/activity-api';
import { activityService } from '@/lib/activity-api';
import { formatActivityTimes, formatParticipantStatus } from '@/lib/activity-helpers';
import { DeleteModal } from '@/components/ui';
import Link from 'next/link';

export default function ActivityDetailsPage() {
  const router = useRouter();
  const params = useParams();
  const { id } = params;
  const [activity, setActivity] = useState<Activity | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  
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
        setError(null);
      } catch (apiErr) {
        console.error('API error when fetching activity:', apiErr);
        setError('Fehler beim Laden der Aktivitätsdaten. Bitte versuchen Sie es später erneut.');
        setActivity(null);
      }
    } catch (err) {
      console.error('Error fetching activity:', err);
      setError('Fehler beim Laden der Aktivitätsdaten. Bitte versuchen Sie es später erneut.');
      setActivity(null);
    } finally {
      setLoading(false);
    }
  };
  
  // Function to handle activity deletion
  const handleDeleteActivity = async () => {
    if (!id) return;
    
    try {
      setIsDeleting(true);
      await activityService.deleteActivity(id as string);
      router.push('/database/activities');
    } catch (err) {
      console.error('Error deleting activity:', err);
      setError('Fehler beim Löschen der Aktivität. Bitte versuchen Sie es später erneut.');
      setShowDeleteConfirm(false);
    } finally {
      setIsDeleting(false);
    }
  };

  // Initial data load
  useEffect(() => {
    void fetchActivity();
  }, [id]);

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
        title={activity.name}
        backUrl="/database/activities"
      />

      <main className="max-w-4xl mx-auto p-4">
        <div className="mb-8">
          <SectionTitle title="Aktivitätsdetails" />
        </div>

        <div className="bg-white border border-gray-100 rounded-lg p-6 shadow-sm mb-6">
          {/* General Information */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6 mb-8">
            <div>
              <h3 className="text-lg font-semibold mb-4 text-gray-800">Allgemeine Informationen</h3>
              <dl className="space-y-2">
                <div className="flex flex-col">
                  <dt className="text-sm text-gray-500">Name</dt>
                  <dd className="font-medium">{activity.name}</dd>
                </div>
                
                <div className="flex flex-col">
                  <dt className="text-sm text-gray-500">Kategorie</dt>
                  <dd className="font-medium">{activity.category_name || 'Keine Kategorie'}</dd>
                </div>
                
                <div className="flex flex-col">
                  <dt className="text-sm text-gray-500">Leitung</dt>
                  <dd className="font-medium">{activity.supervisor_name && activity.supervisor_name.trim() !== '' ? activity.supervisor_name : 'Nicht zugewiesen'}</dd>
                </div>

                <div className="flex flex-col">
                  <dt className="text-sm text-gray-500">Status</dt>
                  <dd className="font-medium flex items-center">
                    {activity.is_open_ags ? (
                      <span className="px-2 py-0.5 bg-blue-100 text-blue-800 text-xs rounded-full">
                        Offen für Anmeldungen
                      </span>
                    ) : (
                      <span className="px-2 py-0.5 bg-gray-100 text-gray-800 text-xs rounded-full">
                        Geschlossen
                      </span>
                    )}
                  </dd>
                </div>
              </dl>
            </div>

            <div>
              <h3 className="text-lg font-semibold mb-4 text-gray-800">Teilnehmer & Zeitplan</h3>
              <dl className="space-y-2">
                <div className="flex flex-col">
                  <dt className="text-sm text-gray-500">Teilnehmer</dt>
                  <dd className="font-medium">
                    {formatParticipantStatus(activity)}
                    {(activity.participant_count || 0) >= activity.max_participant && (
                      <span className="ml-2 px-2 py-0.5 bg-yellow-100 text-yellow-800 text-xs rounded-full">
                        Voll
                      </span>
                    )}
                  </dd>
                </div>
                
                <div className="flex flex-col">
                  <dt className="text-sm text-gray-500">Zeitplan</dt>
                  <dd className="font-medium">{formatActivityTimes(activity)}</dd>
                </div>
                
                <div className="flex flex-col">
                  <dt className="text-sm text-gray-500">Erstellt am</dt>
                  <dd className="font-medium">
                    {activity.created_at ? new Date(activity.created_at).toLocaleDateString('de-DE') : 'Unbekannt'}
                  </dd>
                </div>
                
                <div className="flex flex-col">
                  <dt className="text-sm text-gray-500">Aktualisiert am</dt>
                  <dd className="font-medium">
                    {activity.updated_at ? new Date(activity.updated_at).toLocaleDateString('de-DE') : 'Unbekannt'}
                  </dd>
                </div>
              </dl>
            </div>
          </div>

          {/* Action Buttons */}
          <div className="flex flex-col sm:flex-row gap-3 mt-6">
            <Link href={`/database/activities/${activity.id}/edit`}>
              <button className="w-full sm:w-auto px-4 py-2 bg-blue-500 hover:bg-blue-600 text-white rounded-lg transition-colors">
                Aktivität bearbeiten
              </button>
            </Link>
            
            <Link href={`/database/activities/${activity.id}/students`}>
              <button className="w-full sm:w-auto px-4 py-2 bg-green-500 hover:bg-green-600 text-white rounded-lg transition-colors">
                Teilnehmer verwalten ({activity.participant_count || 0})
              </button>
            </Link>
            
            <Link href={`/database/activities/${activity.id}/times`}>
              <button className="w-full sm:w-auto px-4 py-2 bg-purple-500 hover:bg-purple-600 text-white rounded-lg transition-colors">
                Zeitplan bearbeiten
              </button>
            </Link>
            
            <button 
              onClick={() => setShowDeleteConfirm(true)}
              className="w-full sm:w-auto px-4 py-2 bg-red-500 hover:bg-red-600 text-white rounded-lg transition-colors"
            >
              Aktivität löschen
            </button>
          </div>
          
          {/* Delete Confirmation Dialog */}
          <DeleteModal
            isOpen={showDeleteConfirm}
            onClose={() => setShowDeleteConfirm(false)}
            onDelete={handleDeleteActivity}
            title="Aktivität löschen"
            isDeleting={isDeleting}
          >
            <p>
              Sind Sie sicher, dass Sie die Aktivität "{activity.name}" löschen möchten?
              Dies kann nicht rückgängig gemacht werden, und alle Teilnehmerverbindungen werden entfernt.
            </p>
          </DeleteModal>
        </div>

        {/* Enrolled Students Section */}
        {activity.students && activity.students.length > 0 && (
          <div className="bg-white border border-gray-100 rounded-lg p-6 shadow-sm mb-6">
            <h3 className="text-lg font-semibold mb-4 text-gray-800">Eingeschriebene Schüler</h3>
            <div className="space-y-2">
              {activity.students.map((student) => (
                <div 
                  key={student.id}
                  className="group p-3 border border-gray-100 rounded-lg hover:border-blue-200 hover:bg-blue-50 transition-all"
                >
                  <Link href={`/database/students/${student.id}`}>
                    <div className="flex justify-between items-center">
                      <div>
                        <span className="font-medium text-gray-900 group-hover:text-blue-600 transition-colors">{student.name}</span>
                        {student.school_class && (
                          <span className="ml-2 text-sm text-gray-500">Klasse: {student.school_class}</span>
                        )}
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
                  </Link>
                </div>
              ))}
            </div>
          </div>
        )}
      </main>
    </div>
  );
}
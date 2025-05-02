'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { activityService } from '~/lib/activity-api';
import type { Activity, ActivityCategory } from '~/lib/activity-api';
import { formatActivityTimes, formatParticipantStatus } from '~/lib/activity-helpers';
import type { Student } from '~/lib/api';
import { ActivityForm } from '~/components/activities';
import { PageHeader } from '~/components/dashboard';

interface ActivityDetailPageProps {
  params: {
    id: string;
  };
}

export default function ActivityDetailPage({ params }: ActivityDetailPageProps) {
  const { id } = params;
  const [activity, setActivity] = useState<Activity | null>(null);
  const [categories, setCategories] = useState<ActivityCategory[]>([]);
  const [supervisors, setSupervisors] = useState<Array<{id: string, name: string}>>([]);
  const [enrolledStudents, setEnrolledStudents] = useState<Student[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isEditing, setIsEditing] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  
  const router = useRouter();

  useEffect(() => {
    const fetchData = async () => {
      try {
        setLoading(true);
        const [activityData, categoriesData, studentsData] = await Promise.all([
          activityService.getActivity(id),
          activityService.getCategories(),
          activityService.getEnrolledStudents(id)
        ]);
        
        setActivity(activityData);
        setCategories(categoriesData);
        setEnrolledStudents(studentsData);
        
        // TODO: Replace with actual API call to fetch supervisors
        // For now, just use the current supervisor if available
        if (activityData.supervisor_id && activityData.supervisor_name) {
          setSupervisors([
            {
              id: activityData.supervisor_id,
              name: activityData.supervisor_name
            }
          ]);
        }
        
        setError(null);
      } catch (err) {
        console.error('Error fetching activity details:', err);
        setError('Fehler beim Laden der Aktivitätsdetails. Bitte versuchen Sie es später erneut.');
      } finally {
        setLoading(false);
      }
    };

    fetchData();
  }, [id]);

  const handleUpdateActivity = async (activityData: Partial<Activity>) => {
    try {
      setIsSubmitting(true);
      
      const updatedActivity = await activityService.updateActivity(id, activityData);
      setActivity(updatedActivity);
      setIsEditing(false);
      
      // Show success message or notification
    } catch (err) {
      console.error('Error updating activity:', err);
      // Show error message
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleDeleteActivity = async () => {
    if (!window.confirm('Sind Sie sicher, dass Sie diese Aktivität löschen möchten? Diese Aktion kann nicht rückgängig gemacht werden.')) {
      return;
    }
    
    try {
      setIsSubmitting(true);
      await activityService.deleteActivity(id);
      router.push('/activities');
    } catch (err) {
      console.error('Error deleting activity:', err);
      // Show error message
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleUnenrollStudent = async (studentId: string) => {
    if (!window.confirm('Sind Sie sicher, dass Sie diesen Schüler abmelden möchten?')) {
      return;
    }
    
    try {
      await activityService.unenrollStudent(id, studentId);
      
      // Refresh enrolled students list
      const updatedStudents = await activityService.getEnrolledStudents(id);
      setEnrolledStudents(updatedStudents);
      
      // Also refresh activity to update participant count
      const updatedActivity = await activityService.getActivity(id);
      setActivity(updatedActivity);
    } catch (err) {
      console.error('Error unenrolling student:', err);
      // Show error message
    }
  };

  if (loading) {
    return (
      <div className="min-h-screen">
        <PageHeader 
          title="Aktivitätsdetails" 
          backUrl="/activities"
        />
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
          <div className="text-center py-8">
            <p className="text-gray-500">Aktivitätsdetails werden geladen...</p>
          </div>
        </div>
      </div>
    );
  }

  if (error || !activity) {
    return (
      <div className="min-h-screen">
        <PageHeader 
          title="Fehler" 
          description="Aktivitätsdetails konnten nicht geladen werden"
          backUrl="/activities"
        />
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
          <div className="bg-red-50 text-red-800 p-4 rounded-lg">
            <p>{error || 'Aktivität nicht gefunden.'}</p>
          </div>
        </div>
      </div>
    );
  }

  if (isEditing) {
    return (
      <div className="min-h-screen">
        <PageHeader 
          title={`${activity.name} bearbeiten`}
          description="Bearbeiten der Aktivitätsdetails"
          backUrl={`/activities/${id}`}
        />
        
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
          <ActivityForm
            initialData={activity}
            onSubmitAction={handleUpdateActivity}
            onCancelAction={() => setIsEditing(false)}
            isLoading={isSubmitting}
            formTitle="Aktivität bearbeiten"
            submitLabel="Speichern"
            categories={categories}
            supervisors={supervisors}
          />
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen">
      {/* Header */}
      <PageHeader 
        title={activity.name} 
        description={activity.category_name ? `Kategorie: ${activity.category_name}` : "Aktivitätsdetails"}
        backUrl="/activities"
      />
      
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <div className="flex justify-between items-center mb-6">
          <div className="flex items-center space-x-2 bg-gray-100 px-3 py-1 rounded-full">
            <div className={`h-2.5 w-2.5 rounded-full ${activity.is_open_ag ? 'bg-green-500' : 'bg-gray-400'}`}></div>
            <span className="text-sm font-medium text-gray-700">
              {activity.is_open_ag ? 'Offen für Anmeldungen' : 'Geschlossen für Anmeldungen'}
            </span>
          </div>
          
          <div className="flex space-x-2">
            <button
              onClick={() => setIsEditing(true)}
              className="px-4 py-2 bg-purple-600 text-white rounded-lg hover:bg-purple-700 transition-colors"
            >
              Bearbeiten
            </button>
            <button
              onClick={handleDeleteActivity}
              className="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 transition-colors"
            >
              Löschen
            </button>
          </div>
        </div>

        <div className="bg-white rounded-lg shadow overflow-hidden mb-8">
          <div className="p-6">
            <div className="flex justify-between items-start mb-6">
              <div>
                <h1 className="text-2xl font-bold text-gray-900">{activity.name}</h1>
                <p className="text-gray-500 mt-1">
                  {activity.category_name && `Kategorie: ${activity.category_name}`}
                </p>
              </div>
              
              <div className="flex items-center space-x-2 bg-gray-100 px-3 py-1 rounded-full">
                <div className={`h-2.5 w-2.5 rounded-full ${activity.is_open_ag ? 'bg-green-500' : 'bg-gray-400'}`}></div>
                <span className="text-sm font-medium text-gray-700">
                  {activity.is_open_ag ? 'Offen für Anmeldungen' : 'Geschlossen für Anmeldungen'}
                </span>
              </div>
            </div>
            
            <div className="grid grid-cols-1 md:grid-cols-2 gap-8 mb-8">
              <div className="bg-purple-50 p-4 rounded-lg">
                <h2 className="text-lg font-medium text-purple-800 mb-4">Details</h2>
                <dl className="space-y-2">
                  <div className="flex justify-between">
                    <dt className="text-gray-500">Leitung:</dt>
                    <dd className="font-medium text-gray-800">{activity.supervisor_name || 'Nicht zugewiesen'}</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-gray-500">Teilnehmer:</dt>
                    <dd className="font-medium text-gray-800">{formatParticipantStatus(activity)}</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-gray-500">Erstellt am:</dt>
                    <dd className="font-medium text-gray-800">
                      {new Date(activity.created_at).toLocaleDateString()}
                    </dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-gray-500">Aktualisiert am:</dt>
                    <dd className="font-medium text-gray-800">
                      {new Date(activity.updated_at).toLocaleDateString()}
                    </dd>
                  </div>
                </dl>
              </div>
              
              <div className="bg-blue-50 p-4 rounded-lg">
                <h2 className="text-lg font-medium text-blue-800 mb-4">Zeitplan</h2>
                {activity.times && activity.times.length > 0 ? (
                  <ul className="space-y-2">
                    {activity.times.map((time, index) => (
                      <li key={time.id || index} className="flex justify-between items-center p-2 bg-white rounded border border-blue-100">
                        <span className="font-medium">{time.weekday}</span>
                        <span>
                          {time.timespan?.start_time || ''} 
                          {time.timespan?.end_time ? ` - ${time.timespan.end_time}` : ''}
                        </span>
                      </li>
                    ))}
                  </ul>
                ) : (
                  <p className="text-gray-500 italic">Keine Zeiten geplant</p>
                )}
              </div>
            </div>
            
            <div className="mb-4">
              <h2 className="text-lg font-medium text-gray-800 mb-4">Teilnehmer</h2>
              {enrolledStudents.length > 0 ? (
                <div className="overflow-hidden border border-gray-200 rounded-lg">
                  <table className="min-w-full divide-y divide-gray-200">
                    <thead className="bg-gray-50">
                      <tr>
                        <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                          Name
                        </th>
                        <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                          Klasse
                        </th>
                        <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                          Gruppe
                        </th>
                        <th scope="col" className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
                          Aktionen
                        </th>
                      </tr>
                    </thead>
                    <tbody className="bg-white divide-y divide-gray-200">
                      {enrolledStudents.map(student => (
                        <tr key={student.id}>
                          <td className="px-6 py-4 whitespace-nowrap">
                            <div className="font-medium text-gray-900">{student.name}</div>
                          </td>
                          <td className="px-6 py-4 whitespace-nowrap">
                            <div className="text-gray-500">{student.school_class}</div>
                          </td>
                          <td className="px-6 py-4 whitespace-nowrap">
                            <div className="text-gray-500">{student.group_name}</div>
                          </td>
                          <td className="px-6 py-4 whitespace-nowrap text-right">
                            <button
                              onClick={() => handleUnenrollStudent(student.id)}
                              className="text-red-600 hover:text-red-900"
                            >
                              Abmelden
                            </button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : (
                <p className="text-gray-500 italic">Keine Teilnehmer angemeldet</p>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
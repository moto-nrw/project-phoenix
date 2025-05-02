'use client';

import { useSession } from 'next-auth/react';
import { redirect, useRouter, useParams } from 'next/navigation';
import { useState, useEffect, useCallback } from 'react';
import { PageHeader, SectionTitle } from '@/components/dashboard';
import type { Activity, ActivityTime } from '@/lib/activity-api';
import { activityService } from '@/lib/activity-api';
import Link from 'next/link';

export default function ActivityTimesPage() {
  const router = useRouter();
  const params = useParams();
  const { id } = params;
  const [activity, setActivity] = useState<Activity | null>(null);
  const [loading, setLoading] = useState(true);
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  
  // New time slot form
  const [weekday, setWeekday] = useState('Monday');
  const [startTime, setStartTime] = useState('');
  const [endTime, setEndTime] = useState('');
  const [addingTime, setAddingTime] = useState(false);
  const [timeSpanId, setTimeSpanId] = useState('');
  
  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect('/login');
    },
  });

  // Function to fetch the activity details
  const fetchActivity = useCallback(async () => {
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
  }, [id]);

  // Function to delete a time slot
  const handleDeleteTimeSlot = async (timeSlotId: string) => {
    if (!id || !timeSlotId) return;
    if (deleting) return; // Prevent multiple delete operations
    
    if (!confirm('Möchten Sie diesen Zeitslot wirklich löschen?')) {
      return;
    }
    
    try {
      setDeleting(true);
      await activityService.deleteTimeSlot(id as string, timeSlotId);
      
      // Refresh activity data
      await fetchActivity();
    } catch (err) {
      console.error('Error deleting time slot:', err);
      alert('Fehler beim Löschen des Zeitslots. Bitte versuchen Sie es später erneut.');
    } finally {
      setDeleting(false);
    }
  };

  // Function to add a new time slot
  const handleAddTimeSlot = async (e: React.FormEvent) => {
    e.preventDefault();
    
    if (!id || !weekday || !startTime) return;
    if (addingTime) return; // Prevent duplicate submissions
    
    try {
      setAddingTime(true);
      
      // Parse times in HH:MM format to create Date objects
      // We need to create complete timestamps for the API
      const today = new Date();
      const year = today.getFullYear();
      const month = today.getMonth();
      const day = today.getDate();
      
      // Parse hours and minutes from the time strings
      const [startHours, startMinutes] = startTime.split(':').map(Number);
      
      // Create the start time Date object
      const parsedStartTime = new Date(year, month, day, startHours, startMinutes);
      
      // Create the end time Date object if end time is provided
      let parsedEndTime = null;
      if (endTime) {
        const [endHours, endMinutes] = endTime.split(':').map(Number);
        parsedEndTime = new Date(year, month, day, endHours, endMinutes);
      }
      
      // Create a new time slot with start/end times
      const newTimeSlot = {
        weekday,
        start_time: parsedStartTime.toISOString(),
        end_time: parsedEndTime ? parsedEndTime.toISOString() : undefined,
      };
      
      await activityService.addTimeSlot(id as string, newTimeSlot);
      
      // Clear form fields
      setStartTime('');
      setEndTime('');
      setTimeSpanId(''); // Clear this field even though it's no longer visible (for backward compatibility)
      
      // Refresh activity data
      await fetchActivity();
    } catch (err) {
      console.error('Error adding time slot:', err);
      alert('Fehler beim Hinzufügen des Zeitslots. Bitte versuchen Sie es später erneut.');
    } finally {
      setAddingTime(false);
    }
  };

  // Initial data load
  useEffect(() => {
    void fetchActivity();
  }, [id, fetchActivity]);

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
        title={`Zeitplan: ${activity.name}`}
        backUrl={`/database/activities/${activity.id}`}
      />

      <main className="max-w-4xl mx-auto p-4">
        <div className="mb-8">
          <SectionTitle title="Zeitplan verwalten" />
        </div>

        {/* Current Time Slots */}
        <div className="bg-white border border-gray-100 rounded-lg p-6 shadow-sm mb-8">
          <h3 className="text-lg font-semibold text-gray-800 mb-4">Aktuelle Zeitslots</h3>
          
          {activity.times && activity.times.length > 0 ? (
            <div className="space-y-3">
              {activity.times.map((time) => (
                <div 
                  key={time.id}
                  className="flex justify-between items-center p-3 border border-gray-100 rounded-lg hover:border-blue-200 hover:bg-blue-50 transition-colors"
                >
                  <div>
                    <span className="font-medium">{time.weekday}</span>
                    {time.timespan && (
                      <span className="ml-2">
                        {time.timespan.start_time}
                        {time.timespan.end_time && ` - ${time.timespan.end_time}`}
                      </span>
                    )}
                  </div>
                  <button
                    onClick={() => handleDeleteTimeSlot(time.id)}
                    className="p-2 text-red-500 hover:text-red-700 hover:bg-red-50 rounded transition-colors"
                    disabled={deleting}
                  >
                    <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                    </svg>
                  </button>
                </div>
              ))}
            </div>
          ) : (
            <div className="text-center py-6 bg-gray-50 rounded-lg">
              <p className="text-gray-500">Keine Zeitslots gefunden.</p>
              <p className="text-sm text-gray-400 mt-1">Fügen Sie unten einen neuen Zeitslot hinzu.</p>
            </div>
          )}
        </div>

        {/* Add New Time Slot */}
        <div className="bg-white border border-gray-100 rounded-lg p-6 shadow-sm">
          <h3 className="text-lg font-semibold text-gray-800 mb-4">Neuen Zeitslot hinzufügen</h3>
          
          <form onSubmit={handleAddTimeSlot} className="space-y-4">
            {/* Weekday Selection */}
            <div>
              <label htmlFor="weekday" className="block text-sm font-medium text-gray-700 mb-1">
                Wochentag
              </label>
              <select
                id="weekday"
                value={weekday}
                onChange={(e) => setWeekday(e.target.value)}
                className="w-full px-4 py-2 rounded-lg border border-gray-300 focus:outline-none focus:ring-2 focus:ring-blue-500 transition-colors"
                required
              >
                <option value="Monday">Montag</option>
                <option value="Tuesday">Dienstag</option>
                <option value="Wednesday">Mittwoch</option>
                <option value="Thursday">Donnerstag</option>
                <option value="Friday">Freitag</option>
                <option value="Saturday">Samstag</option>
                <option value="Sunday">Sonntag</option>
              </select>
            </div>
            
            {/* Time Selection */}
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label htmlFor="startTime" className="block text-sm font-medium text-gray-700 mb-1">
                  Startzeit
                </label>
                <input
                  type="time"
                  id="startTime"
                  value={startTime}
                  onChange={(e) => setStartTime(e.target.value)}
                  className="w-full px-4 py-2 rounded-lg border border-gray-300 focus:outline-none focus:ring-2 focus:ring-blue-500 transition-colors"
                  required
                />
              </div>
              <div>
                <label htmlFor="endTime" className="block text-sm font-medium text-gray-700 mb-1">
                  Endzeit (optional)
                </label>
                <input
                  type="time"
                  id="endTime"
                  value={endTime}
                  onChange={(e) => setEndTime(e.target.value)}
                  className="w-full px-4 py-2 rounded-lg border border-gray-300 focus:outline-none focus:ring-2 focus:ring-blue-500 transition-colors"
                />
              </div>
            </div>
            
            {/* Start and end time inputs are used to create a timespan on the server */}
            <div>
              <p className="text-xs text-gray-500 mb-2">
                Die Zeitspanne wird automatisch auf dem Server erstellt, basierend auf den ausgewählten Zeiten.
              </p>
            </div>
            
            <div className="pt-4">
              <button
                type="submit"
                className="w-full px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors disabled:bg-blue-300"
                disabled={addingTime}
              >
                {addingTime ? 'Wird hinzugefügt...' : 'Zeitslot hinzufügen'}
              </button>
            </div>
          </form>
        </div>
      </main>
    </div>
  );
}
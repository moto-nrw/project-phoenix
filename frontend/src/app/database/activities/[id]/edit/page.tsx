'use client';

import { useSession } from 'next-auth/react';
import { redirect, useRouter, useParams } from 'next/navigation';
import { useState, useEffect } from 'react';
import { PageHeader, SectionTitle } from '@/components/dashboard';
import type { Activity, ActivityCategory } from '@/lib/activity-api';
import { activityService } from '@/lib/activity-api';
import Link from 'next/link';

export default function EditActivityPage() {
  const router = useRouter();
  const params = useParams();
  const { id } = params;
  const [activity, setActivity] = useState<Activity | null>(null);
  const [categories, setCategories] = useState<ActivityCategory[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  
  // Form state
  const [name, setName] = useState('');
  const [categoryId, setCategoryId] = useState('');
  const [maxParticipants, setMaxParticipants] = useState(10);
  const [isOpen, setIsOpen] = useState(true);
  
  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect('/login');
    },
  });

  // Function to fetch the activity details and categories
  const fetchData = async () => {
    if (!id) return;

    try {
      setLoading(true);
      
      try {
        // Fetch activity from API
        const activityData = await activityService.getActivity(id as string);
        setActivity(activityData);
        
        // Initialize form fields
        setName(activityData.name || '');
        setCategoryId(activityData.ag_category_id || '');
        setMaxParticipants(activityData.max_participant || 10);
        setIsOpen(activityData.is_open_ags || false);
        
        // Fetch categories
        const categoriesData = await activityService.getCategories();
        setCategories(categoriesData);
        
        setError(null);
      } catch (apiErr) {
        console.error('API error when fetching data:', apiErr);
        setError('Fehler beim Laden der Daten. Bitte versuchen Sie es später erneut.');
        setActivity(null);
      }
    } catch (err) {
      console.error('Error fetching data:', err);
      setError('Fehler beim Laden der Daten. Bitte versuchen Sie es später erneut.');
      setActivity(null);
    } finally {
      setLoading(false);
    }
  };

  // Handle form submission
  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    
    if (!id || !activity) return;
    if (saving) return; // Prevent duplicate submissions
    
    try {
      setSaving(true);
      
      // Prepare update data
      const updateData: Partial<Activity> = {
        name,
        ag_category_id: categoryId,
        max_participant: maxParticipants,
        is_open_ags: isOpen,
        // Keep supervisor, we don't allow changing it here
        supervisor_id: activity.supervisor_id
      };
      
      // Update the activity
      await activityService.updateActivity(id as string, updateData);
      
      // Redirect back to activity details
      router.push(`/database/activities/${id}`);
    } catch (err) {
      console.error('Error updating activity:', err);
      setError('Fehler beim Speichern der Aktivität. Bitte versuchen Sie es später erneut.');
    } finally {
      setSaving(false);
    }
  };

  // Initial data load
  useEffect(() => {
    void fetchData();
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

  return (
    <div className="min-h-screen">
      <PageHeader 
        title={`Aktivität bearbeiten: ${activity.name}`}
        backUrl={`/database/activities/${activity.id}`}
      />

      <main className="max-w-4xl mx-auto p-4">
        <div className="mb-8">
          <SectionTitle title="Aktivitätsdetails bearbeiten" />
        </div>

        <form onSubmit={handleSubmit} className="bg-white border border-gray-100 rounded-lg p-6 shadow-sm">
          {/* Name Input */}
          <div className="mb-6">
            <label htmlFor="name" className="block text-sm font-medium text-gray-700 mb-1">
              Name der Aktivität
            </label>
            <input
              type="text"
              id="name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="w-full px-4 py-2 rounded-lg border border-gray-300 focus:outline-none focus:ring-2 focus:ring-blue-500 transition-all duration-200"
              required
            />
          </div>

          {/* Category Dropdown */}
          <div className="mb-6">
            <label htmlFor="category" className="block text-sm font-medium text-gray-700 mb-1">
              Kategorie
            </label>
            <select
              id="category"
              value={categoryId}
              onChange={(e) => setCategoryId(e.target.value)}
              className="w-full px-4 py-2 rounded-lg border border-gray-300 focus:outline-none focus:ring-2 focus:ring-blue-500 transition-all duration-200"
              required
            >
              <option value="">Kategorie auswählen</option>
              {categories.map((category) => (
                <option key={category.id} value={category.id}>
                  {category.name}
                </option>
              ))}
            </select>
          </div>

          {/* Max Participants Input */}
          <div className="mb-6">
            <label htmlFor="maxParticipants" className="block text-sm font-medium text-gray-700 mb-1">
              Maximale Teilnehmerzahl
            </label>
            <input
              type="number"
              id="maxParticipants"
              value={maxParticipants}
              onChange={(e) => setMaxParticipants(Number(e.target.value))}
              min="1"
              max="100"
              className="w-full px-4 py-2 rounded-lg border border-gray-300 focus:outline-none focus:ring-2 focus:ring-blue-500 transition-all duration-200"
              required
            />
          </div>

          {/* Is Open Checkbox */}
          <div className="mb-6 flex items-center">
            <input
              type="checkbox"
              id="isOpen"
              checked={isOpen}
              onChange={(e) => setIsOpen(e.target.checked)}
              className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded"
            />
            <label htmlFor="isOpen" className="ml-2 block text-sm text-gray-700">
              Offen für Anmeldungen
            </label>
          </div>

          {/* Supervisor Info (Read-only) */}
          <div className="mb-6">
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Aktivitätsleitung
            </label>
            <div className="px-4 py-2 bg-gray-50 border border-gray-200 rounded-lg text-gray-700">
              {activity.supervisor_name || 'Nicht zugewiesen'}
              <p className="text-xs text-gray-500 mt-1">
                Die Aktivitätsleitung kann aktuell nicht geändert werden.
              </p>
            </div>
          </div>

          <div className="flex flex-col sm:flex-row gap-3 justify-end mt-8">
            <Link href={`/database/activities/${activity.id}`}>
              <button 
                type="button"
                className="px-4 py-2 border border-gray-300 rounded-lg text-gray-700 hover:bg-gray-50 transition-colors"
              >
                Abbrechen
              </button>
            </Link>
            <button
              type="submit"
              className="px-6 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors disabled:bg-blue-300"
              disabled={saving}
            >
              {saving ? 'Wird gespeichert...' : 'Änderungen speichern'}
            </button>
          </div>
        </form>
      </main>
    </div>
  );
}
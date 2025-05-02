'use client';

import { useSession } from 'next-auth/react';
import { redirect, useRouter } from 'next/navigation';
import { useState, useEffect } from 'react';
import { PageHeader, SectionTitle } from '@/components/dashboard';
import type { ActivityCategory } from '@/lib/activity-api';
import { activityService } from '@/lib/activity-api';
import Link from 'next/link';

export default function NewActivityPage() {
  const router = useRouter();
  const [categories, setCategories] = useState<ActivityCategory[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  
  // Form state
  const [name, setName] = useState('');
  const [categoryId, setCategoryId] = useState('');
  const [supervisorId, setSupervisorId] = useState(''); // In production, we would fetch a list of supervisors
  const [maxParticipants, setMaxParticipants] = useState(10);
  const [isOpen, setIsOpen] = useState(true);
  
  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect('/login');
    },
  });

  // Function to fetch categories
  const fetchCategories = async () => {
    try {
      setLoading(true);
      
      try {
        // Fetch categories
        const categoriesData = await activityService.getCategories();
        setCategories(categoriesData);
        
        setError(null);
      } catch (apiErr) {
        console.error('API error when fetching categories:', apiErr);
        setError('Fehler beim Laden der Kategorien. Bitte versuchen Sie es später erneut.');
      }
    } catch (err) {
      console.error('Error fetching categories:', err);
      setError('Fehler beim Laden der Kategorien. Bitte versuchen Sie es später erneut.');
    } finally {
      setLoading(false);
    }
  };

  // Handle form submission
  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    
    if (saving) return; // Prevent duplicate submissions
    
    if (!supervisorId) {
      setError('Bitte wählen Sie einen verantwortlichen Mitarbeiter aus.');
      return;
    }
    
    try {
      setSaving(true);
      
      // Prepare activity data
      const activityData = {
        name,
        ag_category_id: categoryId,
        max_participant: maxParticipants,
        is_open_ags: isOpen,
        supervisor_id: supervisorId,
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
        // In a real implementation, timeslots would be added later or as part of this form
      };
      
      // Create the activity
      const newActivity = await activityService.createActivity(activityData);
      
      // Redirect to the new activity
      router.push(`/database/activities/${newActivity.id}`);
    } catch (err) {
      console.error('Error creating activity:', err);
      setError('Fehler beim Erstellen der Aktivität. Bitte versuchen Sie es später erneut.');
    } finally {
      setSaving(false);
    }
  };

  // Initial data load
  useEffect(() => {
    void fetchCategories();
  }, []);

  if (status === 'loading' || loading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p>Loading...</p>
      </div>
    );
  }

  return (
    <div className="min-h-screen">
      <PageHeader 
        title="Neue Aktivität erstellen"
        backUrl="/database/activities"
      />

      <main className="max-w-4xl mx-auto p-4">
        <div className="mb-8">
          <SectionTitle title="Aktivitätsdetails" />
        </div>

        {error && (
          <div className="mb-6 bg-red-50 border border-red-200 rounded-lg p-4 text-red-800">
            {error}
          </div>
        )}

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
            {categories.length === 0 && (
              <div className="mt-1 text-sm text-gray-500">
                Keine Kategorien verfügbar. <Link href="/database/activities/categories" className="text-blue-600 hover:underline">Kategorie erstellen</Link>
              </div>
            )}
          </div>

          {/* Supervisor Dropdown - In a real implementation, this would be populated with actual supervisors */}
          <div className="mb-6">
            <label htmlFor="supervisor" className="block text-sm font-medium text-gray-700 mb-1">
              Verantwortlicher Mitarbeiter
            </label>
            <select
              id="supervisor"
              value={supervisorId}
              onChange={(e) => setSupervisorId(e.target.value)}
              className="w-full px-4 py-2 rounded-lg border border-gray-300 focus:outline-none focus:ring-2 focus:ring-blue-500 transition-all duration-200"
              required
            >
              <option value="">Mitarbeiter auswählen</option>
              {/* Temporary placeholder options - in production, these would come from an API */}
              <option value="1">Supervisor 1</option>
              <option value="2">Supervisor 2</option>
              <option value="3">Supervisor 3</option>
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

          <div className="flex flex-col sm:flex-row gap-3 justify-end mt-8">
            <Link href="/database/activities">
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
              {saving ? 'Wird erstellt...' : 'Aktivität erstellen'}
            </button>
          </div>

          <div className="mt-6 text-sm text-gray-500 bg-gray-50 p-4 rounded-lg">
            <p>
              Hinweis: Nach dem Erstellen können Sie Zeitslots und Teilnehmer hinzufügen.
            </p>
          </div>
        </form>
      </main>
    </div>
  );
}
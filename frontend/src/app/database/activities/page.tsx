'use client';

import { useSession } from 'next-auth/react';
import { redirect, useRouter } from 'next/navigation';
import { useState, useEffect } from 'react';
import { DataListPage } from '@/components/dashboard';
import type { Activity, ActivityCategory } from '@/lib/activity-api';
import { activityService } from '@/lib/activity-api';
import { formatActivityTimes, formatParticipantStatus } from '@/lib/activity-helpers';

export default function ActivitiesPage() {
  const router = useRouter();
  const [activities, setActivities] = useState<Activity[]>([]);
  const [categories, setCategories] = useState<ActivityCategory[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchFilter, setSearchFilter] = useState('');
  const [categoryFilter, setCategoryFilter] = useState<string | null>(null);
  
  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect('/login');
    },
  });

  // Function to fetch activities with optional filters
  const fetchActivities = async (search?: string, categoryId?: string | null) => {
    try {
      setLoading(true);
      
      // Prepare filters for API call
      const filters = {
        search: search ?? undefined,
        category_id: categoryId ?? undefined
      };
      
      try {
        // Fetch from the real API using our activity service
        const data = await activityService.getActivities(filters);
        
        if (data.length === 0 && !search && !categoryId) {
          console.log('No activities returned from API, checking connection');
        }
        
        setActivities(data);
        setError(null);
      } catch (apiErr) {
        console.error('API error when fetching activities:', apiErr);
        setError('Fehler beim Laden der Aktivitätsdaten. Bitte versuchen Sie es später erneut.');
        setActivities([]);
      }
    } catch (err) {
      console.error('Error fetching activities:', err);
      setError('Fehler beim Laden der Aktivitätsdaten. Bitte versuchen Sie es später erneut.');
      setActivities([]);
    } finally {
      setLoading(false);
    }
  };

  // Function to fetch categories
  const fetchCategories = async () => {
    try {
      const categoriesData = await activityService.getCategories();
      setCategories(categoriesData);
    } catch (err) {
      console.error('Error fetching categories:', err);
    }
  };

  // Initial data load
  useEffect(() => {
    void fetchActivities();
    void fetchCategories();
  }, []);

  // Handle search and category filter changes
  useEffect(() => {
    // Debounce search to avoid too many API calls
    const timer = setTimeout(() => {
      void fetchActivities(searchFilter, categoryFilter);
    }, 300);
    
    return () => clearTimeout(timer);
  }, [searchFilter, categoryFilter]);

  if (status === 'loading' || loading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p>Loading...</p>
      </div>
    );
  }

  const handleSelectActivity = (activity: Activity) => {
    router.push(`/database/activities/${activity.id}`);
  };

  // Custom renderer for activity items
  const renderActivity = (activity: Activity) => (
    <>
      <div className="flex flex-col group-hover:translate-x-1 transition-transform duration-200">
        <span className="font-semibold text-gray-900 group-hover:text-blue-600 transition-colors duration-200">
          {activity.name}
          {activity.is_open_ags && (
            <span className="ml-2 px-2 py-0.5 bg-blue-100 text-blue-800 text-xs rounded-full">
              Offen
            </span>
          )}
        </span>
        <div className="flex flex-wrap gap-x-4 text-sm text-gray-500">
          <span>Kategorie: {activity.category_name || 'Keine'}</span>
          <span>Teilnehmer: {formatParticipantStatus(activity)}</span>
          <span>Zeiten: {formatActivityTimes(activity)}</span>
          {activity.supervisor_name && <span>Leitung: {activity.supervisor_name}</span>}
        </div>
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
            onClick={() => fetchActivities()} 
            className="mt-4 px-4 py-2 bg-red-100 hover:bg-red-200 text-red-800 rounded transition-colors"
          >
            Erneut versuchen
          </button>
        </div>
      </div>
    );
  }

  // Create a handler for the search input in DataListPage
  const handleSearchChange = (searchTerm: string) => {
    setSearchFilter(searchTerm);
  };

  return (
    <DataListPage
      title="Aktivitätenauswahl"
      sectionTitle="Aktivität auswählen"
      backUrl="/database"
      newEntityLabel="Neue Aktivität erstellen"
      newEntityUrl="/database/activities/new"
      data={activities}
      onSelectEntityAction={handleSelectActivity}
      renderEntity={renderActivity}
      searchTerm={searchFilter}
      onSearchChange={handleSearchChange}
    />
  );
}
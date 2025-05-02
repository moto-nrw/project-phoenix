'use client';

import { useState, useEffect } from 'react';
import { activityService } from '~/lib/activity-api';
import type { Activity } from '~/lib/activity-api';
import { Card } from '~/components/ui/card';
import Link from 'next/link';

export function ActivityStats() {
  const [activities, setActivities] = useState<Activity[]>([]);
  const [stats, setStats] = useState({
    total: 0,
    openForEnrollment: 0,
    capacityUsed: 0,
    categories: 0
  });
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchData = async () => {
      try {
        setLoading(true);
        const activitiesData = await activityService.getActivities();
        const categoriesData = await activityService.getCategories();
        
        setActivities(activitiesData);
        
        // Calculate stats
        const openActivities = activitiesData.filter(a => a.is_open_ags);
        
        // Calculate capacity usage
        let totalCapacity = 0;
        let totalParticipants = 0;
        
        activitiesData.forEach(activity => {
          totalCapacity += activity.max_participant || 0;
          totalParticipants += activity.participant_count || 0;
        });
        
        const capacityPercentage = totalCapacity > 0 
          ? Math.round((totalParticipants / totalCapacity) * 100) 
          : 0;
        
        setStats({
          total: activitiesData.length,
          openForEnrollment: openActivities.length,
          capacityUsed: capacityPercentage,
          categories: categoriesData.length
        });
      } catch (err) {
        console.error('Error fetching activity stats:', err);
      } finally {
        setLoading(false);
      }
    };

    void fetchData();
  }, []);
  
  if (loading) {
    return (
      <Card className="p-6">
        <h3 className="text-lg font-semibold mb-4">Aktivitätsübersicht</h3>
        <div className="text-center text-gray-500">Daten werden geladen...</div>
      </Card>
    );
  }

  return (
    <Card className="p-6">
      <div className="flex justify-between items-center mb-4">
        <h3 className="text-lg font-semibold">Aktivitätsübersicht</h3>
        <Link 
          href="/database/activities"
          className="text-sm text-purple-600 hover:text-purple-800 font-medium"
        >
          Alle anzeigen →
        </Link>
      </div>
      
      <div className="grid grid-cols-2 gap-4">
        <div className="bg-purple-50 p-3 rounded-lg">
          <p className="text-purple-800 text-sm">Aktivitäten gesamt</p>
          <p className="text-2xl font-bold text-purple-900">{stats.total}</p>
        </div>
        
        <div className="bg-green-50 p-3 rounded-lg">
          <p className="text-green-800 text-sm">Offen für Anmeldung</p>
          <p className="text-2xl font-bold text-green-900">{stats.openForEnrollment}</p>
        </div>
        
        <div className="bg-blue-50 p-3 rounded-lg">
          <p className="text-blue-800 text-sm">Kapazität genutzt</p>
          <p className="text-2xl font-bold text-blue-900">{stats.capacityUsed}%</p>
        </div>
        
        <div className="bg-amber-50 p-3 rounded-lg">
          <p className="text-amber-800 text-sm">Kategorien</p>
          <p className="text-2xl font-bold text-amber-900">{stats.categories}</p>
        </div>
      </div>
      
      {activities.length > 0 && (
        <div className="mt-4">
          <h4 className="text-sm font-medium text-gray-700 mb-2">Neueste Aktivitäten</h4>
          <ul className="divide-y divide-gray-200">
            {activities.slice(0, 3).map(activity => (
              <li key={activity.id} className="py-2">
                <Link 
                  href={`/database/activities/${activity.id}`}
                  className="block hover:bg-gray-50 rounded-lg p-2 -mx-2"
                >
                  <div className="flex justify-between">
                    <p className="text-sm font-medium text-gray-900">{activity.name}</p>
                    <div className={`h-2 w-2 rounded-full self-center ${activity.is_open_ags ? 'bg-green-500' : 'bg-gray-400'}`}></div>
                  </div>
                  <p className="text-xs text-gray-500 mt-1">
                    {activity.category_name ?? 'Keine Kategorie'} • {activity.participant_count ?? 0} von {activity.max_participant} Teilnehmern
                  </p>
                </Link>
              </li>
            ))}
          </ul>
        </div>
      )}
    </Card>
  );
}
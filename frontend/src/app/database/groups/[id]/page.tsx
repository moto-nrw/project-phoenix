'use client';

import { useEffect, useState } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { PageHeader } from '@/components/dashboard';
import { GroupForm } from '@/components/groups';
import type { Group, Student } from '@/lib/api';
import { groupService } from '@/lib/api';

export default function GroupDetailPage() {
  const router = useRouter();
  const params = useParams();
  const groupId = params.id as string;
  
  const [group, setGroup] = useState<Group | null>(null);
  const [students, setStudents] = useState<Student[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isEditing, setIsEditing] = useState(false);

  useEffect(() => {
    const fetchGroupDetails = async () => {
      try {
        setLoading(true);
        
        // Fetch group and its students in parallel
        const [groupData, studentsData] = await Promise.all([
          groupService.getGroup(groupId),
          groupService.getGroupStudents(groupId),
        ]);
        
        setGroup(groupData);
        setStudents(studentsData);
        setError(null);
      } catch (err) {
        console.error('Error fetching group details:', err);
        setError('Fehler beim Laden der Gruppendaten. Bitte versuchen Sie es später erneut.');
        setGroup(null);
        setStudents([]);
      } finally {
        setLoading(false);
      }
    };

    if (groupId) {
      void fetchGroupDetails();
    }
  }, [groupId]);

  const handleUpdate = async (formData: Partial<Group>) => {
    try {
      setLoading(true);
      setError(null);
      
      // Update group
      const updatedGroup = await groupService.updateGroup(groupId, formData);
      setGroup(updatedGroup);
      setIsEditing(false);
    } catch (err) {
      console.error('Error updating group:', err);
      setError('Fehler beim Aktualisieren der Gruppe. Bitte versuchen Sie es später erneut.');
      throw err; // Re-throw to be caught by the form component
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async () => {
    if (window.confirm('Sind Sie sicher, dass Sie diese Gruppe löschen möchten?')) {
      try {
        setLoading(true);
        await groupService.deleteGroup(groupId);
        router.push('/database/groups');
      } catch (err) {
        console.error('Error deleting group:', err);
        // Check if the error has a specific message
        const errorMessage = err instanceof Error ? err.message : 'Unbekannter Fehler';
        
        // Handle the specific "cannot delete group with students" error case
        if (errorMessage.includes('cannot delete group with students') || 
            (errorMessage.includes('cannot delete') && errorMessage.includes('students'))) {
          setError('Die Gruppe kann nicht gelöscht werden, da sie Schüler enthält. Bitte entfernen Sie zuerst alle Schüler aus der Gruppe.');
        } else {
          setError(`Fehler beim Löschen der Gruppe: ${errorMessage}`);
        }
        
        setLoading(false);
      }
    }
  };

  if (loading && !group) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-gray-50">
        <div className="animate-pulse flex flex-col items-center">
          <div className="w-12 h-12 rounded-full bg-gradient-to-r from-teal-400 to-blue-500"></div>
          <p className="mt-4 text-gray-500">Daten werden geladen...</p>
        </div>
      </div>
    );
  }

  if (error) {
    if (!group) {
      // Full page error when no group is loaded
      return (
        <div className="flex min-h-screen flex-col items-center justify-center p-4 bg-gray-50">
          <div className="bg-red-50 text-red-800 p-6 rounded-lg max-w-md shadow-md">
            <h2 className="font-semibold text-lg mb-3">Fehler</h2>
            <p className="mb-4">{error}</p>
            <button 
              onClick={() => router.back()} 
              className="px-4 py-2 bg-red-100 hover:bg-red-200 text-red-800 rounded-lg transition-colors shadow-sm"
            >
              Zurück
            </button>
          </div>
        </div>
      );
    } else {
      // Add an error alert to the page content when group is still loaded
      // This handles the case where delete fails but we still have the group data
      
      // For important errors related to deletion constraints, keep them visible longer
      const clearTimeout = error.includes('Schüler enthält') ? 15000 : 5000;
      setTimeout(() => {
        // Auto-clear error after timeout period
        if (error) setError(null);
      }, clearTimeout);
    }
  }

  if (!group) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center p-4 bg-gray-50">
        <div className="bg-yellow-50 text-yellow-800 p-6 rounded-lg max-w-md shadow-md">
          <h2 className="font-semibold text-lg mb-3">Gruppe nicht gefunden</h2>
          <p className="mb-4">Die angeforderte Gruppe konnte nicht gefunden werden.</p>
          <button 
            onClick={() => router.push('/database/groups')} 
            className="px-4 py-2 bg-yellow-100 hover:bg-yellow-200 text-yellow-800 rounded-lg transition-colors shadow-sm"
          >
            Zurück zur Übersicht
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <PageHeader 
        title={isEditing ? 'Gruppe bearbeiten' : 'Gruppendetails'}
        backUrl="/database/groups"
      />
      
      {/* Main Content */}
      <main className="max-w-4xl mx-auto p-4">
        {/* Error Alert */}
        {error && group && (
          <div className="mb-4 bg-red-100 border-l-4 border-red-500 text-red-700 p-4 rounded shadow-md" role="alert">
            <div className="flex items-start">
              <div className="flex-shrink-0">
                {/* Warning icon */}
                <svg className="h-5 w-5 text-red-500 mr-2" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor">
                  <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clipRule="evenodd" />
                </svg>
              </div>
              <div className="flex-1">
                <p className="font-bold">Aktion nicht möglich</p>
                <p className="text-sm mt-1">{error}</p>
                {error.includes('Schüler enthält') && (
                  <div className="mt-2 text-sm bg-red-50 p-2 rounded">
                    <p className="font-medium">Hinweis zur Lösung:</p>
                    <ol className="list-decimal list-inside ml-2 mt-1">
                      <li>Gehen Sie zur Schülerliste</li>
                      <li>Weisen Sie alle Schüler dieser Gruppe einer anderen Gruppe zu</li>
                      <li>Kehren Sie zurück und versuchen Sie erneut, die Gruppe zu löschen</li>
                    </ol>
                  </div>
                )}
              </div>
              <button 
                className="flex-shrink-0 ml-2 text-red-500 hover:text-red-700 transition-colors"
                onClick={() => setError(null)}
                aria-label="Schließen"
              >
                <span className="text-xl">&times;</span>
              </button>
            </div>
          </div>
        )}
        
        {isEditing ? (
          <GroupForm
            initialData={group}
            onSubmitAction={handleUpdate}
            onCancelAction={() => setIsEditing(false)}
            isLoading={loading}
            formTitle="Gruppe bearbeiten"
            submitLabel="Speichern"
          />
        ) : (
          <div className="bg-white shadow-md rounded-lg overflow-hidden">
            {/* Group card header */}
            <div className="bg-gradient-to-r from-teal-500 to-blue-600 p-6 text-white relative">
              <div className="flex items-center">
                <div className="w-20 h-20 rounded-full bg-white/30 flex items-center justify-center text-3xl font-bold mr-5">
                  {group.name[0] || 'G'}
                </div>
                <div>
                  <h1 className="text-2xl font-bold">{group.name}</h1>
                  {group.room_name && <p className="opacity-90">Raum: {group.room_name}</p>}
                  {group.representative_name && <p className="text-sm opacity-75">Vertreter: {group.representative_name}</p>}
                </div>
              </div>
            </div>
            
            {/* Content */}
            <div className="p-6">
              <div className="flex justify-between items-center mb-6">
                <h2 className="text-xl font-medium text-gray-700">Gruppendetails</h2>
                <div className="flex space-x-2">
                  <button
                    onClick={() => setIsEditing(true)}
                    className="px-4 py-2 bg-blue-50 text-blue-600 rounded-lg hover:bg-blue-100 transition-colors shadow-sm"
                  >
                    Bearbeiten
                  </button>
                  <button
                    onClick={handleDelete}
                    className="px-4 py-2 bg-red-50 text-red-600 rounded-lg hover:bg-red-100 transition-colors shadow-sm"
                  >
                    Löschen
                  </button>
                </div>
              </div>
              
              <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
                {/* Group Information */}
                <div className="space-y-4">
                  <h3 className="text-lg font-medium text-blue-800 border-b border-blue-200 pb-2">
                    Gruppendaten
                  </h3>
                  
                  <div>
                    <div className="text-sm text-gray-500">Name</div>
                    <div className="text-base">{group.name}</div>
                  </div>
                  
                  <div>
                    <div className="text-sm text-gray-500">Raum</div>
                    <div className="text-base">{group.room_name || 'Nicht zugewiesen'}</div>
                  </div>
                  
                  <div>
                    <div className="text-sm text-gray-500">Vertreter</div>
                    <div className="text-base">{group.representative_name || 'Nicht zugewiesen'}</div>
                  </div>
                  
                  <div>
                    <div className="text-sm text-gray-500">IDs</div>
                    <div className="text-xs text-gray-600 flex flex-col">
                      <span>Gruppe: {group.id}</span>
                      {group.room_id && <span>Raum: {group.room_id}</span>}
                      {group.representative_id && <span>Vertreter: {group.representative_id}</span>}
                    </div>
                  </div>
                </div>
                
                {/* Students and Supervisors */}
                <div className="space-y-8">
                  <div className="space-y-4">
                    <h3 className="text-lg font-medium text-green-800 border-b border-green-200 pb-2">
                      Schüler in dieser Gruppe
                    </h3>
                    
                    {students.length > 0 ? (
                      <div className="space-y-2">
                        {students.map(student => (
                          <div key={student.id} className="bg-green-50 p-2 rounded-lg flex justify-between items-center">
                            <span>{student.name}</span>
                            <span className="text-xs text-gray-500">{student.school_class}</span>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <p className="text-gray-500">Keine Schüler in dieser Gruppe.</p>
                    )}
                  </div>
                  
                  <div className="space-y-4">
                    <h3 className="text-lg font-medium text-purple-800 border-b border-purple-200 pb-2">
                      Aufsichtspersonen
                    </h3>
                    
                    {group.supervisors && group.supervisors.length > 0 ? (
                      <div className="space-y-2">
                        {group.supervisors.map(supervisor => (
                          <div key={supervisor.id} className="bg-purple-50 p-2 rounded-lg">
                            <span>{supervisor.name}</span>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <p className="text-gray-500">Keine Aufsichtspersonen zugewiesen.</p>
                    )}
                  </div>
                </div>
              </div>
            </div>
          </div>
        )}
      </main>
    </div>
  );
}
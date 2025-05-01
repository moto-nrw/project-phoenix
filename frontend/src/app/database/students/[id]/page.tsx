'use client';

import { useEffect, useState } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { PageHeader } from '@/components/dashboard';
import StudentForm from '@/components/students/student-form';
import type { Student } from '@/lib/api';
import { studentService } from '@/lib/api';

export default function StudentDetailPage() {
  const router = useRouter();
  const params = useParams();
  const studentId = params.id as string;
  
  const [student, setStudent] = useState<Student | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isEditing, setIsEditing] = useState(false);

  useEffect(() => {
    const fetchStudent = async () => {
      try {
        setLoading(true);
        const data = await studentService.getStudent(studentId);
        setStudent(data);
        setError(null);
      } catch (err) {
        console.error('Error fetching student:', err);
        setError('Fehler beim Laden der Schülerdaten. Bitte versuchen Sie es später erneut.');
        setStudent(null);
      } finally {
        setLoading(false);
      }
    };

    if (studentId) {
      void fetchStudent();
    }
  }, [studentId]);

  const handleUpdate = async (formData: Partial<Student>) => {
    try {
      setLoading(true);
      setError(null);
      
      // Prepare update data with custom_users_id
      const updateData: Partial<Student> = {
        ...formData,
        custom_users_id: formData.custom_users_id || student?.custom_users_id,
      };
      
      // Update student
      const updatedStudent = await studentService.updateStudent(studentId, updateData);
      setStudent(updatedStudent);
      setIsEditing(false);
    } catch (err) {
      console.error('Error updating student:', err);
      setError('Fehler beim Aktualisieren des Schülers. Bitte versuchen Sie es später erneut.');
      throw err; // Re-throw to be caught by the form component
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async () => {
    if (window.confirm('Sind Sie sicher, dass Sie diesen Schüler löschen möchten?')) {
      try {
        setLoading(true);
        await studentService.deleteStudent(studentId);
        router.push('/database/students');
      } catch (err) {
        console.error('Error deleting student:', err);
        setError('Fehler beim Löschen des Schülers. Bitte versuchen Sie es später erneut.');
        setLoading(false);
      }
    }
  };

  if (loading && !student) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-gray-50">
        <div className="animate-pulse flex flex-col items-center">
          <div className="w-12 h-12 rounded-full bg-gradient-to-r from-teal-400 to-blue-500"></div>
          <p className="mt-4 text-gray-500">Daten werden geladen...</p>
        </div>
      </div>
    );
  }

  if (error && !student) {
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
  }

  if (!student) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center p-4 bg-gray-50">
        <div className="bg-yellow-50 text-yellow-800 p-6 rounded-lg max-w-md shadow-md">
          <h2 className="font-semibold text-lg mb-3">Schüler nicht gefunden</h2>
          <p className="mb-4">Der angeforderte Schüler konnte nicht gefunden werden.</p>
          <button 
            onClick={() => router.push('/database/students')} 
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
        title={isEditing ? 'Schüler bearbeiten' : 'Schülerdetails'}
        backUrl="/database/students"
      />
      
      {/* Main Content */}
      <main className="max-w-4xl mx-auto p-4">
        {isEditing ? (
          <StudentForm
            initialData={student}
            onSubmitAction={handleUpdate}
            onCancelAction={() => setIsEditing(false)}
            isLoading={loading}
            formTitle="Schüler bearbeiten"
            submitLabel="Speichern"
          />
        ) : (
          <div className="bg-white shadow-md rounded-lg overflow-hidden">
            {/* Student card header with image placeholder */}
            <div className="bg-gradient-to-r from-teal-500 to-blue-600 p-6 text-white relative">
              <div className="flex items-center">
                <div className="w-20 h-20 rounded-full bg-white/30 flex items-center justify-center text-3xl font-bold mr-5">
                  {student.first_name?.[0] || ''}{student.second_name?.[0] || ''}
                </div>
                <div>
                  <h1 className="text-2xl font-bold">{student.name}</h1>
                  <p className="opacity-90">{student.school_class}</p>
                  {student.group_name && <p className="text-sm opacity-75">Gruppe: {student.group_name}</p>}
                </div>
              </div>
              
              {/* Status badges */}
              <div className="absolute top-6 right-6 flex flex-col space-y-2">
                {student.in_house && (
                  <span className="bg-green-400/80 text-white text-xs px-2 py-1 rounded-full">
                    Im Haus
                  </span>
                )}
                {student.wc && (
                  <span className="bg-blue-400/80 text-white text-xs px-2 py-1 rounded-full">
                    Toilette
                  </span>
                )}
                {student.school_yard && (
                  <span className="bg-yellow-400/80 text-white text-xs px-2 py-1 rounded-full">
                    Schulhof
                  </span>
                )}
                {student.bus && (
                  <span className="bg-orange-400/80 text-white text-xs px-2 py-1 rounded-full">
                    Bus
                  </span>
                )}
              </div>
            </div>
            
            {/* Content */}
            <div className="p-6">
              <div className="flex justify-between items-center mb-6">
                <h2 className="text-xl font-medium text-gray-700">Schülerdetails</h2>
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
                {/* Personal Information */}
                <div className="space-y-4">
                  <h3 className="text-lg font-medium text-blue-800 border-b border-blue-200 pb-2">
                    Persönliche Daten
                  </h3>
                  
                  <div>
                    <div className="text-sm text-gray-500">Vorname</div>
                    <div className="text-base">{student.first_name}</div>
                  </div>
                  
                  <div>
                    <div className="text-sm text-gray-500">Nachname</div>
                    <div className="text-base">{student.second_name}</div>
                  </div>
                  
                  <div>
                    <div className="text-sm text-gray-500">Klasse</div>
                    <div className="text-base">{student.school_class}</div>
                  </div>
                  
                  <div>
                    <div className="text-sm text-gray-500">Gruppe</div>
                    <div className="text-base">
                      {student.group_id && student.group_name ? (
                        <a 
                          href={`/database/groups/${student.group_id}`}
                          className="text-blue-600 hover:text-blue-800 hover:underline transition-colors"
                        >
                          {student.group_name}
                        </a>
                      ) : (
                        'Keine Gruppe zugewiesen'
                      )}
                    </div>
                  </div>
                  
                  <div>
                    <div className="text-sm text-gray-500">IDs</div>
                    <div className="text-xs text-gray-600 flex flex-col">
                      <span>Student: {student.id}</span>
                      {student.custom_users_id && <span>Benutzer: {student.custom_users_id}</span>}
                      {student.group_id && <span>Gruppe: {student.group_id}</span>}
                    </div>
                  </div>
                </div>
                
                {/* Guardian Information and Status */}
                <div className="space-y-8">
                  <div className="space-y-4">
                    <h3 className="text-lg font-medium text-purple-800 border-b border-purple-200 pb-2">
                      Erziehungsberechtigte
                    </h3>
                    
                    <div>
                      <div className="text-sm text-gray-500">Name</div>
                      <div className="text-base">{student.name_lg || 'Nicht angegeben'}</div>
                    </div>
                    
                    <div>
                      <div className="text-sm text-gray-500">Kontakt</div>
                      <div className="text-base">{student.contact_lg || 'Nicht angegeben'}</div>
                    </div>
                  </div>
                  
                  <div className="space-y-4">
                    <h3 className="text-lg font-medium text-green-800 border-b border-green-200 pb-2">
                      Status
                    </h3>
                    
                    <div className="grid grid-cols-2 gap-4">
                      <div className={`p-3 rounded-lg ${student.in_house ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-500'}`}>
                        <span className="flex items-center">
                          <span className={`mr-2 inline-block w-3 h-3 rounded-full ${student.in_house ? 'bg-green-500' : 'bg-gray-300'}`}></span>
                          Im Haus
                        </span>
                      </div>
                      
                      <div className={`p-3 rounded-lg ${student.wc ? 'bg-blue-100 text-blue-800' : 'bg-gray-100 text-gray-500'}`}>
                        <span className="flex items-center">
                          <span className={`mr-2 inline-block w-3 h-3 rounded-full ${student.wc ? 'bg-blue-500' : 'bg-gray-300'}`}></span>
                          Toilette
                        </span>
                      </div>
                      
                      <div className={`p-3 rounded-lg ${student.school_yard ? 'bg-yellow-100 text-yellow-800' : 'bg-gray-100 text-gray-500'}`}>
                        <span className="flex items-center">
                          <span className={`mr-2 inline-block w-3 h-3 rounded-full ${student.school_yard ? 'bg-yellow-500' : 'bg-gray-300'}`}></span>
                          Schulhof
                        </span>
                      </div>
                      
                      <div className={`p-3 rounded-lg ${student.bus ? 'bg-orange-100 text-orange-800' : 'bg-gray-100 text-gray-500'}`}>
                        <span className="flex items-center">
                          <span className={`mr-2 inline-block w-3 h-3 rounded-full ${student.bus ? 'bg-orange-500' : 'bg-gray-300'}`}></span>
                          Bus
                        </span>
                      </div>
                    </div>
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
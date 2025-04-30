'use client';

import { useEffect, useState } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { PageHeader } from '@/components/dashboard';
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
  const [formData, setFormData] = useState({
    first_name: '',
    second_name: '',
    school_class: '',
    name_lg: '',
    contact_lg: '',
    group_id: '',
    bus: false,
    in_house: false,
    wc: false,
    school_yard: false,
    custom_user_id: '',
  });

  useEffect(() => {
    const fetchStudent = async () => {
      try {
        setLoading(true);
        const data = await studentService.getStudent(studentId);
        setStudent(data);
        
        // Initialize form data with student data
        setFormData({
          first_name: data.first_name || '',
          second_name: data.second_name || '',
          school_class: data.school_class || '',
          name_lg: data.name_lg || '',
          contact_lg: data.contact_lg || '',
          group_id: data.group_id || '1',
          bus: data.bus || false,
          in_house: data.in_house || false,
          wc: data.wc || false,
          school_yard: data.school_yard || false,
          custom_user_id: data.custom_user_id || '',
        });
        
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

  const handleChange = (
    e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>
  ) => {
    const { name, value, type } = e.target as HTMLInputElement;
    
    if (type === 'checkbox') {
      const { checked } = e.target as HTMLInputElement;
      setFormData(prev => ({
        ...prev,
        [name]: checked,
      }));
    } else {
      setFormData(prev => ({
        ...prev,
        [name]: value,
      }));
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    
    // Validate form
    if (!formData.first_name || !formData.second_name || !formData.school_class) {
      setError('Bitte füllen Sie alle Pflichtfelder aus.');
      return;
    }
    
    try {
      setLoading(true);
      setError(null);
      
      // Prepare update data
      const updateData: Partial<Student> = {
        first_name: formData.first_name,
        second_name: formData.second_name,
        school_class: formData.school_class,
        name_lg: formData.name_lg,
        contact_lg: formData.contact_lg,
        group_id: formData.group_id,
        bus: formData.bus,
        in_house: formData.in_house,
        wc: formData.wc,
        school_yard: formData.school_yard,
        custom_user_id: formData.custom_user_id || student?.custom_user_id,
      };
      
      // Update student
      const updatedStudent = await studentService.updateStudent(studentId, updateData);
      setStudent(updatedStudent);
      setIsEditing(false);
    } catch (err) {
      console.error('Error updating student:', err);
      setError('Fehler beim Aktualisieren des Schülers. Bitte versuchen Sie es später erneut.');
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

  if (loading) {
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
            {isEditing ? (
              <form onSubmit={handleSubmit} className="space-y-6">
                <div className="bg-blue-50 p-4 rounded-lg mb-8">
                  <h2 className="text-blue-800 text-lg font-medium mb-4">Persönliche Daten</h2>
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    {/* First Name field */}
                    <div>
                      <label htmlFor="first_name" className="block text-sm font-medium text-gray-700 mb-1">
                        Vorname*
                      </label>
                      <input
                        type="text"
                        id="first_name"
                        name="first_name"
                        value={formData.first_name}
                        onChange={handleChange}
                        required
                        className="w-full px-4 py-2 rounded-lg border border-gray-300 focus:outline-none focus:ring-2 focus:ring-blue-500 transition-all duration-200"
                      />
                    </div>
                    
                    {/* Last Name field */}
                    <div>
                      <label htmlFor="second_name" className="block text-sm font-medium text-gray-700 mb-1">
                        Nachname*
                      </label>
                      <input
                        type="text"
                        id="second_name"
                        name="second_name"
                        value={formData.second_name}
                        onChange={handleChange}
                        required
                        className="w-full px-4 py-2 rounded-lg border border-gray-300 focus:outline-none focus:ring-2 focus:ring-blue-500 transition-all duration-200"
                      />
                    </div>
                    
                    {/* School Class field */}
                    <div>
                      <label htmlFor="school_class" className="block text-sm font-medium text-gray-700 mb-1">
                        Klasse*
                      </label>
                      <input
                        type="text"
                        id="school_class"
                        name="school_class"
                        value={formData.school_class}
                        onChange={handleChange}
                        required
                        className="w-full px-4 py-2 rounded-lg border border-gray-300 focus:outline-none focus:ring-2 focus:ring-blue-500 transition-all duration-200"
                      />
                    </div>
                    
                    {/* Group ID field */}
                    <div>
                      <label htmlFor="group_id" className="block text-sm font-medium text-gray-700 mb-1">
                        Gruppe ID
                      </label>
                      <input
                        type="text"
                        id="group_id"
                        name="group_id"
                        value={formData.group_id}
                        onChange={handleChange}
                        className="w-full px-4 py-2 rounded-lg border border-gray-300 focus:outline-none focus:ring-2 focus:ring-blue-500 transition-all duration-200"
                      />
                    </div>
                  </div>
                </div>
                
                <div className="bg-purple-50 p-4 rounded-lg mb-8">
                  <h2 className="text-purple-800 text-lg font-medium mb-4">Erziehungsberechtigte</h2>
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    {/* Legal Guardian Name field */}
                    <div>
                      <label htmlFor="name_lg" className="block text-sm font-medium text-gray-700 mb-1">
                        Name des Erziehungsberechtigten
                      </label>
                      <input
                        type="text"
                        id="name_lg"
                        name="name_lg"
                        value={formData.name_lg}
                        onChange={handleChange}
                        className="w-full px-4 py-2 rounded-lg border border-gray-300 focus:outline-none focus:ring-2 focus:ring-purple-500 transition-all duration-200"
                      />
                    </div>
                    
                    {/* Legal Guardian Contact field */}
                    <div>
                      <label htmlFor="contact_lg" className="block text-sm font-medium text-gray-700 mb-1">
                        Kontakt des Erziehungsberechtigten
                      </label>
                      <input
                        type="text"
                        id="contact_lg"
                        name="contact_lg"
                        value={formData.contact_lg}
                        onChange={handleChange}
                        className="w-full px-4 py-2 rounded-lg border border-gray-300 focus:outline-none focus:ring-2 focus:ring-purple-500 transition-all duration-200"
                      />
                    </div>
                  </div>
                </div>
                
                <div className="bg-green-50 p-4 rounded-lg mb-8">
                  <h2 className="text-green-800 text-lg font-medium mb-4">Status</h2>
                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                    {/* Status Checkboxes */}
                    <div className="flex items-center">
                      <input
                        type="checkbox"
                        id="in_house"
                        name="in_house"
                        checked={formData.in_house}
                        onChange={handleChange}
                        className="h-4 w-4 text-green-600 rounded border-gray-300 focus:ring-green-500"
                      />
                      <label htmlFor="in_house" className="ml-2 block text-sm text-gray-700">
                        Im Haus
                      </label>
                    </div>
                    
                    <div className="flex items-center">
                      <input
                        type="checkbox"
                        id="wc"
                        name="wc"
                        checked={formData.wc}
                        onChange={handleChange}
                        className="h-4 w-4 text-green-600 rounded border-gray-300 focus:ring-green-500"
                      />
                      <label htmlFor="wc" className="ml-2 block text-sm text-gray-700">
                        Toilette
                      </label>
                    </div>
                    
                    <div className="flex items-center">
                      <input
                        type="checkbox"
                        id="school_yard"
                        name="school_yard"
                        checked={formData.school_yard}
                        onChange={handleChange}
                        className="h-4 w-4 text-green-600 rounded border-gray-300 focus:ring-green-500"
                      />
                      <label htmlFor="school_yard" className="ml-2 block text-sm text-gray-700">
                        Schulhof
                      </label>
                    </div>
                    
                    <div className="flex items-center">
                      <input
                        type="checkbox"
                        id="bus"
                        name="bus"
                        checked={formData.bus}
                        onChange={handleChange}
                        className="h-4 w-4 text-green-600 rounded border-gray-300 focus:ring-green-500"
                      />
                      <label htmlFor="bus" className="ml-2 block text-sm text-gray-700">
                        Bus
                      </label>
                    </div>
                  </div>
                </div>
                
                {/* Form actions */}
                <div className="flex justify-end pt-4">
                  <button
                    type="button"
                    onClick={() => setIsEditing(false)}
                    className="px-4 py-2 text-gray-700 mr-2 hover:bg-gray-100 rounded-lg transition-colors shadow-sm"
                    disabled={loading}
                  >
                    Abbrechen
                  </button>
                  <button
                    type="submit"
                    className="px-6 py-2 bg-gradient-to-r from-teal-500 to-blue-600 text-white rounded-lg hover:from-teal-600 hover:to-blue-700 hover:shadow-lg transition-all duration-200"
                    disabled={loading}
                  >
                    {loading ? 'Speichern...' : 'Speichern'}
                  </button>
                </div>
              </form>
            ) : (
              <>
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
                      <div className="text-base">{student.group_name || 'Keine Gruppe zugewiesen'}</div>
                    </div>
                    
                    <div>
                      <div className="text-sm text-gray-500">IDs</div>
                      <div className="text-xs text-gray-600 flex flex-col">
                        <span>Student: {student.id}</span>
                        {student.custom_user_id && <span>Benutzer: {student.custom_user_id}</span>}
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
              </>
            )}
          </div>
        </div>
      </main>
    </div>
  );
}
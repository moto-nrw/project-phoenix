'use client';

import { useState, useEffect } from 'react';
import type { Student } from '@/lib/api';
import { GroupSelector } from '@/components/groups';

interface StudentFormProps {
  initialData?: Partial<Student>;
  onSubmitAction: (studentData: Partial<Student>) => Promise<void>;
  onCancelAction: () => void;
  isLoading: boolean;
  formTitle: string;
  submitLabel: string;
}

export default function StudentForm({
  initialData,
  onSubmitAction,
  onCancelAction,
  isLoading,
  formTitle,
  submitLabel,
}: StudentFormProps) {
  const [formData, setFormData] = useState({
    first_name: '',
    second_name: '',
    school_class: '',
    name_lg: '',
    contact_lg: '',
    group_id: '1',
    bus: false,
    in_house: false,
    wc: false,
    school_yard: false,
    custom_users_id: '',
  });
  
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (initialData) {
      setFormData({
        first_name: initialData.first_name || '',
        second_name: initialData.second_name || '',
        school_class: initialData.school_class || '',
        name_lg: initialData.name_lg || '',
        contact_lg: initialData.contact_lg || '',
        group_id: initialData.group_id || '1',
        bus: initialData.bus || false,
        in_house: initialData.in_house || false,
        wc: initialData.wc || false,
        school_yard: initialData.school_yard || false,
        custom_users_id: initialData.custom_users_id || initialData.custom_user_id || '',
      });
    }
  }, [initialData]);

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
      setError(null);
      
      // Call the provided submit function with form data
      await onSubmitAction(formData);
    } catch (err) {
      console.error('Error submitting form:', err);
      setError('Fehler beim Speichern der Schülerdaten. Bitte versuchen Sie es später erneut.');
    }
  };

  return (
    <div className="bg-white shadow-md rounded-lg overflow-hidden">
      <div className="p-6">
        <h2 className="text-xl font-bold mb-6 text-gray-800">{formTitle}</h2>
        
        {error && (
          <div className="bg-red-50 text-red-800 p-4 rounded-lg mb-6">
            <p>{error}</p>
          </div>
        )}
        
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
              
              {/* Group Selector */}
              <div>
                <GroupSelector
                  value={formData.group_id}
                  onChange={(groupId) => {
                    setFormData(prev => ({
                      ...prev,
                      group_id: groupId,
                    }));
                  }}
                  label="Gruppe"
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
              onClick={onCancelAction}
              className="px-4 py-2 text-gray-700 mr-2 hover:bg-gray-100 rounded-lg transition-colors shadow-sm"
              disabled={isLoading}
            >
              Abbrechen
            </button>
            <button
              type="submit"
              className="px-6 py-2 bg-gradient-to-r from-teal-500 to-blue-600 text-white rounded-lg hover:from-teal-600 hover:to-blue-700 hover:shadow-lg transition-all duration-200"
              disabled={isLoading}
            >
              {isLoading ? 'Wird gespeichert...' : submitLabel}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
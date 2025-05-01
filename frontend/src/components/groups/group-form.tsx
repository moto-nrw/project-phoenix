'use client';

import { useState, useEffect } from 'react';
import type { Group } from '@/lib/api';

interface GroupFormProps {
  initialData?: Partial<Group>;
  onSubmitAction: (groupData: Partial<Group>) => Promise<void>;
  onCancelAction: () => void;
  isLoading: boolean;
  formTitle: string;
  submitLabel: string;
}

export default function GroupForm({
  initialData,
  onSubmitAction,
  onCancelAction,
  isLoading,
  formTitle,
  submitLabel,
}: GroupFormProps) {
  const [formData, setFormData] = useState({
    name: '',
    room_id: '',
    representative_id: '',
  });
  
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (initialData) {
      setFormData({
        name: initialData.name || '',
        room_id: initialData.room_id || '',
        representative_id: initialData.representative_id || '',
      });
    }
  }, [initialData]);

  const handleChange = (
    e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>
  ) => {
    const { name, value } = e.target;
    setFormData(prev => ({
      ...prev,
      [name]: value,
    }));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    
    // Validate form
    if (!formData.name) {
      setError('Bitte geben Sie einen Gruppennamen ein.');
      return;
    }
    
    try {
      setError(null);
      
      // Call the provided submit function with form data
      await onSubmitAction(formData);
    } catch (err) {
      console.error('Error submitting form:', err);
      setError('Fehler beim Speichern der Gruppendaten. Bitte versuchen Sie es später erneut.');
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
            <h2 className="text-blue-800 text-lg font-medium mb-4">Gruppendaten</h2>
            <div className="grid grid-cols-1 gap-4">
              {/* Group Name field */}
              <div>
                <label htmlFor="name" className="block text-sm font-medium text-gray-700 mb-1">
                  Gruppenname*
                </label>
                <input
                  type="text"
                  id="name"
                  name="name"
                  value={formData.name}
                  onChange={handleChange}
                  required
                  className="w-full px-4 py-2 rounded-lg border border-gray-300 focus:outline-none focus:ring-2 focus:ring-blue-500 transition-all duration-200"
                />
              </div>
              
              {/* Room ID field */}
              <div>
                <label htmlFor="room_id" className="block text-sm font-medium text-gray-700 mb-1">
                  Raum ID
                </label>
                <input
                  type="text"
                  id="room_id"
                  name="room_id"
                  value={formData.room_id}
                  onChange={handleChange}
                  className="w-full px-4 py-2 rounded-lg border border-gray-300 focus:outline-none focus:ring-2 focus:ring-blue-500 transition-all duration-200"
                />
                <p className="text-xs text-gray-500 mt-1">
                  Verbindet diese Gruppe mit einem Raum
                </p>
              </div>
              
              {/* Representative ID field */}
              <div>
                <label htmlFor="representative_id" className="block text-sm font-medium text-gray-700 mb-1">
                  Vertreter ID
                </label>
                <input
                  type="text"
                  id="representative_id"
                  name="representative_id"
                  value={formData.representative_id}
                  onChange={handleChange}
                  className="w-full px-4 py-2 rounded-lg border border-gray-300 focus:outline-none focus:ring-2 focus:ring-blue-500 transition-all duration-200"
                />
                <p className="text-xs text-gray-500 mt-1">
                  Legt den Hauptverantwortlichen für diese Gruppe fest
                </p>
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
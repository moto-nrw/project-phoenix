"use client";

import { Button } from "~/components/ui/button";

interface ContactActionsProps {
  email?: string;
  phone?: string;
  studentName?: string;
}

export function ContactActions({ email, phone, studentName }: ContactActionsProps) {
  const handleEmailClick = () => {
    if (email) {
      const subject = studentName ? `Betreff: ${studentName}` : "Kontaktanfrage";
      window.location.href = `mailto:${email}?subject=${encodeURIComponent(subject)}`;
    }
  };

  const handlePhoneClick = () => {
    if (phone) {
      // Remove spaces and special characters for tel: link
      const cleanPhone = phone.replace(/\s+/g, '');
      window.location.href = `tel:${cleanPhone}`;
    }
  };

  // If no contact methods available, don't render anything
  if (!email && !phone) {
    return null;
  }

  return (
    <div className="border-t border-gray-200 pt-4">
      <h3 className="font-medium text-gray-800 mb-3 flex items-center gap-2">
        <svg className="h-5 w-5 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
        </svg>
        Kontakt aufnehmen
      </h3>
      
      <div className="flex gap-3">
        {email && (
          <Button
            variant="outline"
            className="flex items-center gap-2 hover:bg-blue-50 hover:border-blue-200 transition-colors"
            onClick={handleEmailClick}
          >
            <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
            </svg>
            E-Mail
          </Button>
        )}
        
        {phone && (
          <Button
            variant="outline"
            className="flex items-center gap-2 hover:bg-green-50 hover:border-green-200 transition-colors"
            onClick={handlePhoneClick}
          >
            <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 5a2 2 0 012-2h3.28a1 1 0 01.948.684l1.498 4.493a1 1 0 01-.502 1.21l-2.257 1.13a11.042 11.042 0 005.516 5.516l1.13-2.257a1 1 0 011.21-.502l4.493 1.498a1 1 0 01.684.949V19a2 2 0 01-2 2h-1C9.716 21 3 14.284 3 6V5z" />
            </svg>
            Anrufen
          </Button>
        )}
      </div>
    </div>
  );
}
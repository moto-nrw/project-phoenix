"use client";

interface ModernContactActionsProps {
  email?: string;
  phone?: string;
  studentName?: string;
}

export function ModernContactActions({ email, phone, studentName }: ModernContactActionsProps) {
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
    <div className="border-t border-gray-100 pt-4 sm:pt-6 mt-4 sm:mt-6">
      <div className="flex items-center gap-3 mb-3 sm:mb-4">
        <div className="h-8 w-8 rounded-lg bg-gradient-to-br from-blue-500 to-blue-600 flex items-center justify-center shadow-[0_4px_15px_rgba(59,130,246,0.3)] flex-shrink-0">
          <svg className="h-4 w-4 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
          </svg>
        </div>
        <h3 className="text-sm sm:text-base font-bold text-gray-800">Kontakt aufnehmen</h3>
      </div>
      
      {/* Mobile-First Button Layout */}
      <div className="flex flex-col gap-3">
        {email && (
          <button
            onClick={handleEmailClick}
            className="group relative flex items-center justify-center gap-3 min-h-[48px] px-4 sm:px-6 py-3 sm:py-4 rounded-2xl bg-gradient-to-br from-blue-500 to-blue-600 text-white font-semibold shadow-[0_8px_25px_rgba(59,130,246,0.4)] transition-all duration-300 hover:shadow-[0_12px_35px_rgba(59,130,246,0.5)] hover:scale-[1.02] active:scale-[0.98] overflow-hidden touch-manipulation"
          >
            {/* Background effects */}
            <div className="absolute inset-0 bg-gradient-to-br from-white/20 to-transparent rounded-2xl"></div>
            <div className="absolute inset-0 bg-gradient-to-br from-transparent to-black/10 rounded-2xl opacity-0 group-hover:opacity-100 transition-opacity duration-300"></div>
            
            <svg className="h-5 w-5 relative z-10 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
            </svg>
            <span className="relative z-10 text-sm sm:text-base">E-Mail</span>
            
            {/* Decorative ping */}
            <div className="absolute -top-1 -right-1 w-3 h-3 bg-white/30 rounded-full animate-ping"></div>
          </button>
        )}
        
        {phone && (
          <button
            onClick={handlePhoneClick}
            className="group relative flex items-center justify-center gap-3 min-h-[48px] px-4 sm:px-6 py-3 sm:py-4 rounded-2xl bg-gradient-to-br from-green-500 to-green-600 text-white font-semibold shadow-[0_8px_25px_rgba(34,197,94,0.4)] transition-all duration-300 hover:shadow-[0_12px_35px_rgba(34,197,94,0.5)] hover:scale-[1.02] active:scale-[0.98] overflow-hidden touch-manipulation"
          >
            {/* Background effects */}
            <div className="absolute inset-0 bg-gradient-to-br from-white/20 to-transparent rounded-2xl"></div>
            <div className="absolute inset-0 bg-gradient-to-br from-transparent to-black/10 rounded-2xl opacity-0 group-hover:opacity-100 transition-opacity duration-300"></div>
            
            <svg className="h-5 w-5 relative z-10 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 5a2 2 0 012-2h3.28a1 1 0 01.948.684l1.498 4.493a1 1 0 01-.502 1.21l-2.257 1.13a11.042 11.042 0 005.516 5.516l1.13-2.257a1 1 0 011.21-.502l4.493 1.498a1 1 0 01.684.949V19a2 2 0 01-2 2h-1C9.716 21 3 14.284 3 6V5z" />
            </svg>
            <span className="relative z-10 text-sm sm:text-base">Anrufen</span>
            
            {/* Decorative ping */}
            <div className="absolute -top-1 -right-1 w-3 h-3 bg-white/30 rounded-full animate-ping"></div>
          </button>
        )}
      </div>
    </div>
  );
}
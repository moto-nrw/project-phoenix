"use client";

interface ModernContactActionsProps {
  email?: string;
  phone?: string;
  studentName?: string;
}

export function ModernContactActions({
  email,
  phone,
  studentName,
}: ModernContactActionsProps) {
  const handleEmailClick = () => {
    if (email) {
      const subject = studentName
        ? `Betreff: ${studentName}`
        : "Kontaktanfrage";
      window.location.href = `mailto:${email}?subject=${encodeURIComponent(subject)}`;
    }
  };

  const handlePhoneClick = () => {
    if (phone) {
      // Remove spaces and special characters for tel: link
      const cleanPhone = phone.replace(/\s+/g, "");
      window.location.href = `tel:${cleanPhone}`;
    }
  };

  // If no contact methods available, don't render anything
  if (!email && !phone) {
    return null;
  }

  return (
    <div className="mt-4 border-t border-gray-100 pt-4">
      <p className="mb-3 text-xs text-gray-500">Kontakt aufnehmen</p>

      <div className="flex gap-2">
        {email && (
          <button
            onClick={handleEmailClick}
            className="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-all duration-200 hover:scale-105 hover:border-gray-400 hover:bg-gray-50 hover:shadow-md active:scale-100"
          >
            <svg
              className="mr-2 inline h-4 w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
              />
            </svg>
            E-Mail senden
          </button>
        )}

        {phone && (
          <button
            onClick={handlePhoneClick}
            className="rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-all duration-200 hover:scale-105 hover:bg-gray-700 hover:shadow-lg active:scale-100"
          >
            <svg
              className="mr-2 inline h-4 w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M3 5a2 2 0 012-2h3.28a1 1 0 01.948.684l1.498 4.493a1 1 0 01-.502 1.21l-2.257 1.13a11.042 11.042 0 005.516 5.516l1.13-2.257a1 1 0 011.21-.502l4.493 1.498a1 1 0 01.684.949V19a2 2 0 01-2 2h-1C9.716 21 3 14.284 3 6V5z"
              />
            </svg>
            Anrufen
          </button>
        )}
      </div>
    </div>
  );
}

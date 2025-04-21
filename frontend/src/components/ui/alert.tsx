'use client';

type AlertType = 'error' | 'success' | 'warning' | 'info';

interface AlertProps {
  type: AlertType;
  message: string;
}

export function Alert({ type, message }: AlertProps) {
  if (!message) return null;
  
  const styles = {
    error: 'bg-red-50 text-red-700 border-red-100',
    success: 'bg-green-50 text-green-700 border-green-100',
    warning: 'bg-yellow-50 text-yellow-700 border-yellow-100',
    info: 'bg-blue-50 text-blue-700 border-blue-100',
  };

  return (
    <div className={`rounded-lg p-4 text-sm shadow-sm border transition-all duration-200 hover:shadow-md hover:opacity-95 ${styles[type]}`}>
      {message}
    </div>
  );
}

'use client';

interface CardProps {
  children: React.ReactNode;
  className?: string;
  onClick?: () => void;
}

export function Card({ children, className = '', onClick }: CardProps) {
  return (
    <div 
      className={`w-full max-w-md space-y-6 rounded-xl border-0 p-8 shadow-xl bg-white/95 transition-all duration-300 hover:shadow-2xl hover:translate-y-[-2px] ${className} ${onClick ? 'cursor-pointer' : ''}`}
      onClick={onClick}
    >
      {children}
    </div>
  );
}

interface CardHeaderProps {
  title: string;
  description?: string;
}

export function CardHeader({ title, description }: CardHeaderProps) {
  return (
    <div className="text-center">
      <h1 className="text-3xl font-bold text-teal-600">{title}</h1>
      {description && (
        <p className="mt-2 text-sm text-gray-600">
          {description}
        </p>
      )}
    </div>
  );
}

interface CardContentProps {
  children: React.ReactNode;
}

export function CardContent({ children }: CardContentProps) {
  return <div className="mt-8 space-y-6">{children}</div>;
}

interface CardFooterProps {
  children: React.ReactNode;
}

export function CardFooter({ children }: CardFooterProps) {
  return <div className="mt-4 text-center text-sm">{children}</div>;
}

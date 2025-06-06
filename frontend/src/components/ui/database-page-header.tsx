export interface DatabasePageHeaderProps {
  title: string;
  description: string;
  className?: string;
}

export function DatabasePageHeader({ 
  title, 
  description, 
  className = "" 
}: DatabasePageHeaderProps) {
  return (
    <div className={`mb-4 md:mb-6 lg:mb-8 ${className}`}>
      <h1 className="text-2xl md:text-3xl font-bold text-gray-900">{title}</h1>
      <p className="mt-1 text-sm md:text-base text-gray-600">{description}</p>
    </div>
  );
}
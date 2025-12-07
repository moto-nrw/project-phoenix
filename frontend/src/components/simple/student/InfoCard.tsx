"use client";

interface InfoCardProps {
  title: string;
  children: React.ReactNode;
  borderColor?: string;
  icon?: React.ReactNode;
}

export function InfoCard({
  title,
  children,
  borderColor = "border-blue-200",
  icon,
}: InfoCardProps) {
  return (
    <div className="rounded-2xl border border-gray-100 bg-white p-6 shadow-lg transition-all duration-300 hover:shadow-xl">
      {/* Header */}
      <div
        className={`mb-4 border-b ${borderColor} flex items-center gap-3 pb-3`}
      >
        {icon && <div className="flex-shrink-0">{icon}</div>}
        <h2 className="text-xl font-bold text-gray-800">{title}</h2>
      </div>

      {/* Content */}
      <div className="space-y-4">{children}</div>
    </div>
  );
}

interface InfoItemProps {
  label: string;
  value: string | React.ReactNode;
  icon?: React.ReactNode;
}

export function InfoItem({ label, value, icon }: InfoItemProps) {
  return (
    <div className="flex items-start gap-3">
      {icon && <div className="mt-0.5 flex-shrink-0 text-gray-400">{icon}</div>}
      <div className="min-w-0 flex-1">
        <p className="mb-1 text-sm font-medium text-gray-500">{label}</p>
        <div className="font-medium break-words text-gray-900">{value}</div>
      </div>
    </div>
  );
}

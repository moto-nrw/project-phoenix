import { type ReactNode } from "react";

export interface DatabaseListSectionProps {
  title: string;
  itemCount: number;
  itemLabel?: {
    singular: string;
    plural: string;
  };
  children: ReactNode;
  className?: string;
}

export function DatabaseListSection({
  title,
  itemCount,
  itemLabel = { singular: "Eintrag", plural: "Einträge" },
  children,
  className = "",
}: DatabaseListSectionProps) {
  return (
    <div
      className={`rounded-lg border border-gray-100 bg-white shadow-md ${className}`}
    >
      <div className="p-4 md:p-6">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-lg font-semibold text-gray-900 md:text-xl">
            {title}
          </h2>
          <span className="text-sm text-gray-500">
            {itemCount}{" "}
            {itemCount === 1 ? itemLabel.singular : itemLabel.plural}
          </span>
        </div>

        <div className="space-y-2 md:space-y-3">{children}</div>
      </div>
    </div>
  );
}

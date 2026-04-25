import { MousePointer2 } from "lucide-react";

interface EmptyDetailStateProps {
  title?: string;
  description?: string;
}

export function EmptyDetailState({
  title = "Kein Eintrag ausgewählt",
  description = "Wähle einen Eintrag aus der Liste, um Details zu sehen.",
}: EmptyDetailStateProps) {
  return (
    <div className="flex h-full flex-col items-center justify-center p-12 text-center">
      <MousePointer2 className="mb-4 h-10 w-10 text-gray-300" aria-hidden />
      <h3 className="text-base font-semibold text-gray-900">{title}</h3>
      <p className="mt-1 max-w-sm text-sm text-gray-500">{description}</p>
    </div>
  );
}

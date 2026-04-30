"use client";

interface DetailLoadingSpinnerProps {
  label: string;
}

export function DetailLoadingSpinner({ label }: DetailLoadingSpinnerProps) {
  return (
    <div className="flex h-full min-h-[240px] items-center justify-center">
      <div className="flex flex-col items-center gap-4">
        <div className="h-10 w-10 animate-spin rounded-full border-2 border-gray-200 border-t-[#5080D8]" />
        <p className="text-sm text-gray-500">{label}</p>
      </div>
    </div>
  );
}

import type React from "react";

type TimetableStatTone = "neutral" | "success" | "warning" | "danger";

const TONE_PALETTE: Record<TimetableStatTone, string> = {
  neutral: "border-gray-200 bg-gray-50 text-gray-700",
  success: "border-[#83CD2D]/20 bg-[#83CD2D]/10 text-[#6BA023]",
  warning: "border-[#EAB308]/20 bg-[#EAB308]/10 text-[#92400E]",
  danger: "border-[#FF3130]/20 bg-[#FF3130]/10 text-[#CC2626]",
};

interface TimetableStatCardProps {
  icon: React.ReactNode;
  label: string;
  value: string;
  tone: TimetableStatTone;
  /**
   * "sm" (default) is the compact chip used inside the Planstatus panel —
   * visually identical to the panel's previous inline Metric. "lg" is the
   * dashboard headline KPI shown above the planner (bigger value + optional
   * sublabel).
   */
  size?: "sm" | "lg";
  sublabel?: string;
}

export function TimetableStatCard({
  icon,
  label,
  value,
  tone,
  size = "sm",
  sublabel,
}: TimetableStatCardProps) {
  if (size === "lg") {
    return (
      <div
        className={`rounded-2xl border px-4 py-3 sm:px-5 sm:py-4 ${TONE_PALETTE[tone]}`}
      >
        <div className="flex items-center gap-2 text-[11px] font-bold tracking-wide uppercase">
          {icon}
          {label}
        </div>
        <div className="mt-1.5 text-2xl font-bold sm:text-3xl">{value}</div>
        {sublabel ? (
          <div className="mt-0.5 text-xs font-medium opacity-80">
            {sublabel}
          </div>
        ) : null}
      </div>
    );
  }

  return (
    <div className={`rounded-xl border px-3 py-2 ${TONE_PALETTE[tone]}`}>
      <div className="flex items-center gap-2 text-[11px] font-bold uppercase">
        {icon}
        {label}
      </div>
      <div className="mt-1 text-lg font-bold">{value}</div>
    </div>
  );
}

// StatusIndicator component - extracted to reduce complexity in PageHeaderWithSearch
"use client";

interface StatusIndicatorProps {
  readonly color: "green" | "yellow" | "red" | "gray";
  readonly tooltip?: string;
  readonly size?: "sm" | "md";
}

/**
 * Status indicator dot with color-coded meaning
 */
export function StatusIndicator({
  color,
  tooltip,
  size = "sm",
}: StatusIndicatorProps) {
  const sizeClass = size === "sm" ? "h-2.5 w-2.5" : "h-3 w-3";

  const colorClass = getColorClass(color);

  return (
    <div
      className={`${sizeClass} flex-shrink-0 rounded-full ${colorClass}`}
      title={tooltip}
    />
  );
}

function getColorClass(color: StatusIndicatorProps["color"]): string {
  switch (color) {
    case "green":
      return "animate-pulse bg-green-500";
    case "yellow":
      return "bg-yellow-500";
    case "red":
      return "bg-red-500";
    default:
      return "bg-gray-400";
  }
}

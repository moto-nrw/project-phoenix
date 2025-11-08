import { Badge } from "./badge";

export type StatusType =
  | "present"
  | "absent"
  | "account"
  | "active"
  | "inactive";

export interface StatusBadgeProps {
  status: StatusType;
  label?: string;
  showIcon?: boolean;
}

const statusConfig: Record<
  StatusType,
  {
    variant: "green" | "gray" | "blue" | "red";
    defaultLabel: string;
    icon?: React.ReactNode;
  }
> = {
  present: {
    variant: "green",
    defaultLabel: "Anwesend",
  },
  absent: {
    variant: "gray",
    defaultLabel: "Zuhause",
  },
  account: {
    variant: "green",
    defaultLabel: "Konto",
    icon: (
      <svg className="h-3 w-3" fill="currentColor" viewBox="0 0 20 20">
        <path d="M10 9a3 3 0 100-6 3 3 0 000 6zm-7 9a7 7 0 1114 0H3z" />
      </svg>
    ),
  },
  active: {
    variant: "blue",
    defaultLabel: "Aktiv",
  },
  inactive: {
    variant: "red",
    defaultLabel: "Inaktiv",
  },
};

export function StatusBadge({
  status,
  label,
  showIcon = true,
}: StatusBadgeProps) {
  const config = statusConfig[status];
  const displayLabel = label ?? config.defaultLabel;

  return (
    <Badge variant={config.variant} icon={showIcon ? config.icon : undefined}>
      {/* Show full label on larger screens, abbreviated on mobile */}
      <span className="hidden sm:inline">{displayLabel}</span>
      <span className="sm:hidden">{displayLabel.charAt(0)}</span>
    </Badge>
  );
}

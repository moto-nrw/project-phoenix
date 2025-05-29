import Link from "next/link";
import { Badge } from "./badge";

export interface GroupBadgeProps {
  groupName: string;
  groupId: string | number;
  onClick?: (e: React.MouseEvent) => void;
}

export function GroupBadge({ groupName, groupId, onClick }: GroupBadgeProps) {
  const handleClick = (e: React.MouseEvent) => {
    e.stopPropagation(); // Prevent triggering parent click events
    onClick?.(e);
  };

  return (
    <Link
      href={`/database/groups/${groupId}`}
      onClick={handleClick}
      className="inline-flex"
    >
      <Badge
        variant="blue"
        className="hover:bg-blue-200 transition-colors duration-150"
        icon={
          <svg className="w-3 h-3" fill="currentColor" viewBox="0 0 20 20">
            <path d="M13 6a3 3 0 11-6 0 3 3 0 016 0zM18 8a2 2 0 11-4 0 2 2 0 014 0zM14 15a4 4 0 00-8 0v3h8v-3zM6 8a2 2 0 11-4 0 2 2 0 014 0zM16 18v-3a5.972 5.972 0 00-.75-2.906A3.005 3.005 0 0119 15v3h-3zM4.75 12.094A5.973 5.973 0 004 15v3H1v-3a3 3 0 013.75-2.906z" />
          </svg>
        }
      >
        {groupName}
      </Badge>
    </Link>
  );
}
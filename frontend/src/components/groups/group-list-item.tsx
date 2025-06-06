import type { Group } from "@/lib/api";
import { 
  DatabaseListItem, 
  Badge,
  Link
} from "@/components/ui";

interface GroupListItemProps {
  group: Group;
  onClick: () => void;
}

export function GroupListItem({ group, onClick }: GroupListItemProps) {
  const badges = [];

  // Add room badge as a clickable link
  if (group.room_name && group.room_id) {
    badges.push(
      <Link 
        key="room"
        href={`/rooms/${group.room_id}`}
        onClick={(e) => e.stopPropagation()} // Prevent triggering group click
      >
        <Badge variant="purple" icon={
          <svg className="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
          </svg>
        }>
          {group.room_name}
        </Badge>
      </Link>
    );
  } else if (!group.room_name) {
    badges.push(
      <Badge key="no-room" variant="gray">
        Kein Raum
      </Badge>
    );
  }

  // Add student count badge
  if (group.student_count !== undefined) {
    badges.push(
      <Badge key="students" variant="blue" icon={
        <svg className="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
        </svg>
      }>
        {group.student_count} {group.student_count === 1 ? 'Schüler' : 'Schüler'}
      </Badge>
    );
  }

  // Group icon
  const groupIcon = (
    <div className="flex h-10 w-10 md:h-12 md:w-12 flex-shrink-0 items-center justify-center rounded-full bg-gradient-to-r from-purple-400 to-pink-400 text-white">
      <svg className="h-5 w-5 md:h-6 md:w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
      </svg>
    </div>
  );

  return (
    <DatabaseListItem
      id={group.id}
      onClick={onClick}
      title={group.name}
      badges={badges}
      leftIcon={groupIcon}
    />
  );
}

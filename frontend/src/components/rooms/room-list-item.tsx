import type { Room } from "@/lib/api";
import { DatabaseListItem, Badge } from "@/components/ui";

interface RoomListItemProps {
  room: Room;
  onClick: () => void;
}

export function RoomListItem({ room, onClick }: RoomListItemProps) {
  const badges = [];

  // Add category badge
  badges.push(
    <Badge
      key="category"
      variant="indigo"
      icon={
        <svg
          className="h-3 w-3"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.994 1.994 0 013 12V7a4 4 0 014-4z"
          />
        </svg>
      }
    >
      {room.category}
    </Badge>,
  );

  // Add building and floor badge
  if (room.building) {
    badges.push(
      <Badge
        key="location"
        variant="gray"
        icon={
          <svg
            className="h-3 w-3"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4"
            />
          </svg>
        }
      >
        {room.building} - Etage {room.floor}
      </Badge>,
    );
  }

  // Add capacity badge
  badges.push(
    <Badge
      key="capacity"
      variant="blue"
      icon={
        <svg
          className="h-3 w-3"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z"
          />
        </svg>
      }
    >
      {room.capacity} Plätze
    </Badge>,
  );

  // Room color dot
  const colorIndicator = room.color
    ? {
        type: "dot" as const,
        value: room.color,
      }
    : undefined;

  // Subtitle with status and group info
  let subtitle = null;
  if (room.isOccupied) {
    subtitle = (
      <div className="flex flex-wrap items-center gap-2">
        <Badge variant="red">Belegt</Badge>
        {room.groupName && (
          <span className="text-xs text-gray-600">
            Gruppe: {room.groupName}
          </span>
        )}
        {room.activityName && (
          <span className="text-xs text-gray-500">• {room.activityName}</span>
        )}
      </div>
    );
  } else {
    subtitle = <Badge variant="green">Frei</Badge>;
  }

  // Room icon based on category
  const getRoomIcon = () => {
    const iconClass = "h-5 w-5 md:h-6 md:w-6";
    switch (room.category.toLowerCase()) {
      case "klassenraum":
        return (
          <svg
            className={iconClass}
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253"
            />
          </svg>
        );
      case "fachraum":
        return (
          <svg
            className={iconClass}
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M19.428 15.428a2 2 0 00-1.022-.547l-2.387-.477a6 6 0 00-3.86.517l-.318.158a6 6 0 01-3.86.517L6.05 15.21a2 2 0 00-1.806.547M8 4h8l-1 1v5.172a2 2 0 00.586 1.414l5 5c1.26 1.26.367 3.414-1.415 3.414H4.828c-1.782 0-2.674-2.154-1.414-3.414l5-5A2 2 0 009 10.172V5L8 4z"
            />
          </svg>
        );
      case "turnhalle":
        return (
          <svg
            className={iconClass}
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6"
            />
          </svg>
        );
      default:
        return (
          <svg
            className={iconClass}
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4"
            />
          </svg>
        );
    }
  };

  const roomIcon = (
    <div
      className={`flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-full md:h-12 md:w-12 ${
        room.isOccupied
          ? "bg-gradient-to-r from-red-400 to-orange-400"
          : "bg-gradient-to-r from-green-400 to-teal-400"
      } text-white`}
    >
      {getRoomIcon()}
    </div>
  );

  return (
    <DatabaseListItem
      id={room.id}
      onClick={onClick}
      title={room.name}
      badges={badges}
      leftIcon={roomIcon}
      subtitle={subtitle}
      indicator={colorIndicator}
    />
  );
}

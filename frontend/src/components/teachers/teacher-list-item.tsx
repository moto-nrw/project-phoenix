import { type Teacher } from "@/lib/teacher-api";
import { 
  DatabaseListItem, 
  Badge,
  StatusBadge 
} from "@/components/ui";

export interface TeacherListItemProps {
  teacher: Teacher;
  onClick: (teacher: Teacher) => void;
}

export function TeacherListItem({ teacher, onClick }: TeacherListItemProps) {
  const badges = [];

  // Add specialization badge
  if (teacher.specialization) {
    badges.push(
      <Badge key="specialization" variant="gray">
        {teacher.specialization}
      </Badge>
    );
  }

  // Add role badge
  if (teacher.role) {
    badges.push(
      <Badge key="role" variant="blue">
        {teacher.role}
      </Badge>
    );
  }

  // Add account status badge
  if (teacher.email) {
    badges.push(
      <StatusBadge key="account" status="account" showIcon={true} />
    );
  }

  // Teacher avatar
  const avatar = (
    <div className="flex h-10 w-10 md:h-12 md:w-12 flex-shrink-0 items-center justify-center rounded-full bg-gradient-to-r from-blue-400 to-indigo-500 font-medium text-white text-sm md:text-base">
      {teacher.name.charAt(0).toUpperCase()}
    </div>
  );

  return (
    <DatabaseListItem
      id={teacher.id}
      onClick={() => onClick(teacher)}
      title={teacher.name}
      badges={badges}
      leftIcon={avatar}
      minHeight="md"
    />
  );
}
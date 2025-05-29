import { type Student } from "@/lib/api";
import { formatStudentName } from "@/lib/student-helpers";
import { 
  DatabaseListItem, 
  ClassBadge, 
  GroupBadge, 
  StatusBadge 
} from "@/components/ui";

export interface StudentListItemProps {
  student: Student;
  onClick: (student: Student) => void;
}

export function StudentListItem({ student, onClick }: StudentListItemProps) {
  const badges = [];

  // Add class badge
  if (student.school_class) {
    badges.push(
      <ClassBadge key="class" className={student.school_class} />
    );
  }

  // Add group badge
  if (student.group_name && student.group_id) {
    badges.push(
      <GroupBadge 
        key="group"
        groupName={student.group_name} 
        groupId={student.group_id}
        asLink={false} // Prevent nested links
      />
    );
  }

  return (
    <DatabaseListItem
      id={student.id}
      onClick={() => onClick(student)}
      title={formatStudentName(student)}
      badges={badges}
      indicator={student.in_house ? {
        type: "dot",
        value: "bg-green-500"
      } : undefined}
      subtitle={student.in_house && (
        <StatusBadge status="present" />
      )}
    />
  );
}
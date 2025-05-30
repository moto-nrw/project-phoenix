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

  // Student avatar with gradient background based on school class
  const getGradientColors = (schoolClass: string) => {
    // Use different gradients based on class level
    if (schoolClass.startsWith('1') || schoolClass.startsWith('2')) {
      return 'from-green-400 to-teal-500'; // Lower grades
    } else if (schoolClass.startsWith('3') || schoolClass.startsWith('4')) {
      return 'from-purple-400 to-pink-500'; // Middle grades
    } else if (schoolClass.startsWith('5') || schoolClass.startsWith('6')) {
      return 'from-orange-400 to-red-500'; // Upper grades
    }
    return 'from-blue-400 to-indigo-500'; // Default
  };

  const avatar = (
    <div className={`flex h-10 w-10 md:h-12 md:w-12 flex-shrink-0 items-center justify-center rounded-full bg-gradient-to-r ${getGradientColors(student.school_class || '')} font-medium text-white text-sm md:text-base`}>
      {formatStudentName(student).charAt(0).toUpperCase()}
    </div>
  );

  return (
    <DatabaseListItem
      id={student.id}
      onClick={() => onClick(student)}
      title={formatStudentName(student)}
      badges={badges}
      leftIcon={avatar}
      indicator={student.in_house ? {
        type: "dot",
        value: "bg-green-500"
      } : undefined}
      subtitle={student.in_house && (
        <StatusBadge status="present" />
      )}
      minHeight="md"
    />
  );
}
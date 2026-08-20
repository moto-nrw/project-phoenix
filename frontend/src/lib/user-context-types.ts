export interface EducationalGroup {
  id: string;
  name: string;
  roomId?: string;
  room?: {
    id: string;
    name: string;
  };
  viaSubstitution?: boolean;
}

export interface SupervisedGroup {
  id: string;
  name: string;
  roomId?: string;
  room?: {
    id: string;
    name: string;
  };
}

interface Staff {
  id: string;
  personId: string;
}

export interface UserContextResponse {
  educationalGroups: EducationalGroup[];
  supervisedGroups: SupervisedGroup[];
  currentStaff: Staff | null;
  educationalGroupIds: string[];
  educationalGroupRoomNames: string[];
  supervisedRoomNames: string[];
  incomplete: boolean;
  unavailableSections: string[];
}

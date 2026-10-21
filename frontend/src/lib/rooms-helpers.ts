// Room type definitions

export interface BackendRoom {
  id: number;
  name: string;
  building?: string;
  floor: number;
  capacity: number;
  category: string;
  color: string;
  created_at?: string;
  updated_at?: string;
}

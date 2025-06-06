"use client";

import { DatabasePage } from "@/components/ui/database/database-page";
import { roomsConfig } from "@/lib/database/configs/rooms.config";

export default function RoomsPage() {
  return <DatabasePage config={roomsConfig} />;
}

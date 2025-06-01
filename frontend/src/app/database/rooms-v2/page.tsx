"use client";

import { DatabasePage } from "@/components/ui/database";
import { roomsConfig } from "@/lib/database";

export default function RoomsV2Page() {
  return <DatabasePage config={roomsConfig} />;
}
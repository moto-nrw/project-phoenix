import { NextResponse } from "next/server";
import { createGetHandler } from "@/lib/route-wrapper";

export const GET = createGetHandler(async (request, token, params) => {
  // Return an empty array for now
  // TODO: Implement proper RFID card fetching from backend
  return NextResponse.json([]);
});
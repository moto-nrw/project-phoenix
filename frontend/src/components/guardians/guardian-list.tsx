"use client";

import { GuardianWithRelationship, getGuardianFullName, getRelationshipTypeLabel } from "@/lib/guardian-helpers";
import ModernContactActions from "@/components/simple/student";
import { Trash2, Edit, UserCheck, Phone, AlertCircle } from "lucide-react";

interface GuardianListProps {
  guardians: GuardianWithRelationship[];
  onEdit?: (guardian: GuardianWithRelationship) => void;
  onDelete?: (guardianId: string) => void;
  readOnly?: boolean;
  showRelationship?: boolean;
}

export default function GuardianList({
  guardians,
  onEdit,
  onDelete,
  readOnly = false,
  showRelationship = true,
}: GuardianListProps) {
  if (guardians.length === 0) {
    return (
      <div className="text-center py-8 text-gray-500">
        <AlertCircle className="mx-auto h-12 w-12 text-gray-400 mb-2" />
        <p>Keine Erziehungsberechtigten zugewiesen</p>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {guardians.map((guardian) => (
        <div
          key={guardian.id}
          className={`rounded-lg border p-4 ${
            guardian.isPrimary ? "border-purple-300 bg-purple-50" : "border-gray-200 bg-white"
          }`}
        >
          {/* Header with name and actions */}
          <div className="flex items-start justify-between mb-3">
            <div>
              <h4 className="font-semibold text-lg flex items-center gap-2">
                {getGuardianFullName(guardian)}
                {guardian.isPrimary && (
                  <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-purple-100 text-purple-800">
                    <UserCheck className="h-3 w-3" />
                    Primär
                  </span>
                )}
              </h4>
              {showRelationship && (
                <p className="text-sm text-gray-600">
                  {getRelationshipTypeLabel(guardian.relationshipType)}
                </p>
              )}
            </div>

            {!readOnly && (
              <div className="flex gap-2">
                {onEdit && (
                  <button
                    onClick={() => onEdit(guardian)}
                    className="p-2 text-blue-600 hover:bg-blue-50 rounded-lg transition-colors"
                    title="Bearbeiten"
                  >
                    <Edit className="h-4 w-4" />
                  </button>
                )}
                {onDelete && (
                  <button
                    onClick={() => onDelete(guardian.id)}
                    className="p-2 text-red-600 hover:bg-red-50 rounded-lg transition-colors"
                    title="Entfernen"
                  >
                    <Trash2 className="h-4 w-4" />
                  </button>
                )}
              </div>
            )}
          </div>

          {/* Contact Information */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            <InfoItem
              label="E-Mail"
              value={guardian.email || "Nicht angegeben"}
              icon={<Phone className="h-4 w-4" />}
            />
            <InfoItem
              label="Telefon"
              value={guardian.phone || "Nicht angegeben"}
              icon={<Phone className="h-4 w-4" />}
            />
            <InfoItem
              label="Mobiltelefon"
              value={guardian.mobilePhone || "Nicht angegeben"}
              icon={<Phone className="h-4 w-4" />}
            />
            <InfoItem
              label="Bevorzugter Kontakt"
              value={
                guardian.preferredContactMethod === "email"
                  ? "E-Mail"
                  : guardian.preferredContactMethod === "phone"
                  ? "Telefon"
                  : "Mobiltelefon"
              }
            />
          </div>

          {/* Additional Information */}
          {showRelationship && (
            <div className="mt-3 pt-3 border-t border-gray-200 grid grid-cols-2 md:grid-cols-4 gap-2 text-sm">
              {guardian.isEmergencyContact && (
                <span className="inline-flex items-center gap-1 px-2 py-1 rounded-md bg-red-50 text-red-700">
                  <AlertCircle className="h-3 w-3" />
                  Notfallkontakt
                </span>
              )}
              {guardian.canPickup && (
                <span className="inline-flex items-center px-2 py-1 rounded-md bg-green-50 text-green-700">
                  Abholberechtigt
                </span>
              )}
              {guardian.emergencyPriority > 1 && (
                <span className="inline-flex items-center px-2 py-1 rounded-md bg-yellow-50 text-yellow-700">
                  Priorität: {guardian.emergencyPriority}
                </span>
              )}
            </div>
          )}

          {guardian.pickupNotes && (
            <div className="mt-3 p-2 bg-gray-50 rounded-md">
              <p className="text-sm text-gray-700">
                <span className="font-medium">Abholhinweise:</span> {guardian.pickupNotes}
              </p>
            </div>
          )}

          {/* Contact Actions */}
          <div className="mt-3">
            <ModernContactActions
              email={guardian.email}
              phone={guardian.phone || guardian.mobilePhone}
              studentName={getGuardianFullName(guardian)}
            />
          </div>
        </div>
      ))}
    </div>
  );
}

// Helper component for displaying information items
function InfoItem({
  label,
  value,
  icon,
}: {
  label: string;
  value: string;
  icon?: React.ReactNode;
}) {
  return (
    <div>
      <div className="flex items-center gap-1 text-xs text-gray-500 mb-1">
        {icon}
        <span>{label}</span>
      </div>
      <p className="text-sm font-medium text-gray-900">{value}</p>
    </div>
  );
}

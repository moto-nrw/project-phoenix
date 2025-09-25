interface ModernStatusBadgeProps {
  location?: string;
  roomName?: string;
}

export function ModernStatusBadge({ location, roomName }: ModernStatusBadgeProps) {
  // Status details using exact colors from ogs_groups, myroom, and search badges
  const getStatusDetails = () => {
    if (location === "Anwesend" || location === "In House" || location?.startsWith("Anwesend")) {
      // If we have a specific room name, use it
      const label = roomName ?? (() => {
        if (location?.startsWith("Anwesend - ")) {
          // Extract activity/room name from "Anwesend - Aktivität" or "Anwesend - Room Name" format
          return location.substring(11);
        }
        if (location?.startsWith("Anwesend in ")) {
          // Extract room name from "Anwesend in Room Name" format
          return location.substring(12);
        }
        return "Anwesend";
      })();
      
      return { 
        label, 
        bgColor: "#83CD2D", // Gruppenraum green
        shadow: "0 8px 25px rgba(131, 205, 45, 0.4)",
        badgeColor: "text-white backdrop-blur-sm"
      };
    } else if (location === "Zuhause") {
      return { 
        label: "Zuhause", 
        bgColor: "#FF3130", // Zuhause red
        shadow: "0 8px 25px rgba(255, 49, 48, 0.4)",
        badgeColor: "text-white backdrop-blur-sm"
      };
    } else if (location === "WC") {
      return { 
        label: "WC", 
        bgColor: "#5080D8", // Room blue
        shadow: "0 8px 25px rgba(80, 128, 216, 0.4)",
        badgeColor: "text-white backdrop-blur-sm"
      };
    } else if (location === "School Yard") {
      return { 
        label: "Schulhof", 
        bgColor: "#F78C10", // Schulhof orange
        shadow: "0 8px 25px rgba(247, 140, 16, 0.4)",
        badgeColor: "text-white backdrop-blur-sm"
      };
    } else if (location === "Unterwegs") {
      return { 
        label: "Unterwegs", 
        bgColor: "#D946EF", // Unterwegs purple/fuchsia
        shadow: "0 8px 25px rgba(217, 70, 239, 0.4)",
        badgeColor: "text-white backdrop-blur-sm"
      };
    } else if (location === "Bus") {
      return { 
        label: "Bus", 
        bgColor: "#D946EF", // Bus purple/fuchsia (same as Unterwegs)
        shadow: "0 8px 25px rgba(217, 70, 239, 0.4)",
        badgeColor: "text-white backdrop-blur-sm"
      };
    }
    return { 
      label: "Unbekannt", 
      bgColor: "#6B7280",
      shadow: "0 8px 25px rgba(107, 114, 128, 0.4)",
      badgeColor: "text-white backdrop-blur-sm"
    };
  };

  const status = getStatusDetails();

  return (
    <span
      className={`inline-flex items-center px-3 py-1.5 rounded-full text-xs font-bold ${status.badgeColor} transition-all duration-300`}
      style={{
        backgroundColor: status.bgColor
      }}
    >
      <span className="w-2 h-2 bg-white/80 rounded-full mr-2 animate-pulse"></span>
      {status.label}
    </span>
  );
}
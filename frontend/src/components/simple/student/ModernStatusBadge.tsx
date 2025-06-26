interface ModernStatusBadgeProps {
  location?: string;
}

export function ModernStatusBadge({ location }: ModernStatusBadgeProps) {
  // Status details using exact colors from ogs_groups, myroom, and search badges
  const getStatusDetails = () => {
    if (location === "Anwesend" || location === "In House") {
      return { 
        label: "Anwesend", 
        bgColor: "#83CD2D", // Gruppenraum green
        shadow: "0 8px 25px rgba(131, 205, 45, 0.4)",
        badgeColor: "text-white backdrop-blur-sm"
      };
    } else if (location === "Abwesend") {
      return { 
        label: "Abwesend", 
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
    } else if (location === "Bus") {
      return { 
        label: "Bus", 
        bgColor: "#D946EF", // Unterwegs purple/fuchsia
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
      className={`inline-flex items-center px-5 py-3 rounded-2xl text-base font-bold ${status.badgeColor} transition-all duration-300 hover:scale-105 shadow-lg`}
      style={{ 
        backgroundColor: status.bgColor,
        boxShadow: status.shadow
      }}
    >
      <span className="w-3 h-3 bg-white/80 rounded-full mr-3 animate-pulse"></span>
      {status.label}
    </span>
  );
}
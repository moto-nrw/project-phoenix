interface StatusBadgeProps {
  location?: string;
}

export function StatusBadge({ location }: StatusBadgeProps) {
  // Determine status details based on location
  const getStatusDetails = () => {
    if (location === "Anwesend" || location === "In House") {
      return { 
        label: "Anwesend", 
        bgColor: "bg-green-500", 
        textColor: "text-green-800", 
        bgLight: "bg-green-100" 
      };
    } else if (location === "Abwesend") {
      return { 
        label: "Abwesend", 
        bgColor: "bg-orange-500", 
        textColor: "text-orange-800", 
        bgLight: "bg-orange-100" 
      };
    } else if (location === "WC") {
      return { 
        label: "WC", 
        bgColor: "bg-blue-500", 
        textColor: "text-blue-800", 
        bgLight: "bg-blue-100" 
      };
    } else if (location === "School Yard") {
      return { 
        label: "Schulhof", 
        bgColor: "bg-yellow-500", 
        textColor: "text-yellow-800", 
        bgLight: "bg-yellow-100" 
      };
    } else if (location === "Bus") {
      return { 
        label: "Bus", 
        bgColor: "bg-purple-500", 
        textColor: "text-purple-800", 
        bgLight: "bg-purple-100" 
      };
    }
    return { 
      label: "Unbekannt", 
      bgColor: "bg-gray-500", 
      textColor: "text-gray-800", 
      bgLight: "bg-gray-100" 
    };
  };

  const status = getStatusDetails();

  return (
    <div className={`inline-flex items-center rounded-full px-3 py-1 ${status.bgLight} ${status.textColor} font-medium text-sm`}>
      <span className={`mr-1.5 inline-block h-2 w-2 rounded-full ${status.bgColor}`}></span>
      {status.label}
    </div>
  );
}
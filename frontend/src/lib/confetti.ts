const COLORS = ["#FF3130", "#F78C10", "#83DC2D", "#5080D8"];
const LOGO_SELECTOR = ".mb-8.flex.justify-center img";

export function launchConfetti(): void {
  const confettiContainer = document.createElement("div");
  confettiContainer.style.position = "fixed";
  confettiContainer.style.width = "100%";
  confettiContainer.style.height = "100%";
  confettiContainer.style.top = "0";
  confettiContainer.style.left = "0";
  confettiContainer.style.pointerEvents = "none";
  confettiContainer.style.zIndex = "9999";
  document.body.appendChild(confettiContainer);

  const logoElement = document.querySelector(LOGO_SELECTOR);
  const logoRect = logoElement?.getBoundingClientRect();

  const startX = logoRect
    ? logoRect.left + logoRect.width / 2
    : window.innerWidth / 2;
  const startY = logoRect
    ? logoRect.top + logoRect.height / 2
    : window.innerHeight / 2;

  for (let i = 0; i < 100; i++) {
    const delay = i < 50 ? 0 : Math.random() * 100;

    setTimeout(() => {
      if (typeof document === "undefined" || !confettiContainer.isConnected)
        return;

      const confetti = document.createElement("div");
      const color = COLORS[Math.floor(Math.random() * COLORS.length)];

      confetti.style.position = "absolute";
      confetti.style.width = `${Math.random() * 8 + 3}px`;
      confetti.style.height = `${Math.random() * 4 + 3}px`;
      confetti.style.backgroundColor = color ?? "#FF3130";
      confetti.style.borderRadius = Math.random() > 0.5 ? "50%" : "0";
      confetti.style.opacity = "0.8";

      confetti.style.left = `${startX}px`;
      confetti.style.top = `${startY}px`;

      let angle = 0;
      const quadrant = Math.floor(Math.random() * 4);
      switch (quadrant) {
        case 0:
          angle = (Math.random() * Math.PI) / 2;
          break;
        case 1:
          angle = Math.PI / 2 + (Math.random() * Math.PI) / 2;
          break;
        case 2:
          angle = Math.PI + (Math.random() * Math.PI) / 2;
          break;
        case 3:
          angle = (3 * Math.PI) / 2 + (Math.random() * Math.PI) / 2;
          break;
      }

      const distance = 150 + Math.random() * 200;
      const endX = Math.cos(angle) * distance;
      const endY = Math.sin(angle) * distance;

      const midDistance = distance * 0.6;
      const midX = Math.cos(angle) * midDistance;
      const midY = Math.sin(angle) * midDistance;

      confettiContainer.appendChild(confetti);

      const animation = confetti.animate(
        [
          {
            transform: "translate(-50%, -50%) rotate(0deg)",
            opacity: 0.8,
          },
          {
            transform: `translate(${midX}px, ${midY}px) rotate(${Math.random() * 360}deg)`,
            opacity: 0.6,
          },
          {
            transform: `translate(${endX}px, ${endY}px) rotate(${Math.random() * 720}deg)`,
            opacity: 0,
          },
        ],
        {
          duration: Math.random() * 1500 + 1500,
          easing: "cubic-bezier(0.25, 0.46, 0.45, 0.94)",
        },
      );

      animation.onfinish = () => {
        confetti.remove();
        if (confettiContainer.children.length === 0) {
          confettiContainer.remove();
        }
      };
    }, delay);
  }
}

export function clearConfetti(): void {
  const existingConfetti = document.querySelector(
    'div[style*="z-index: 9999"]',
  );
  if (existingConfetti) {
    existingConfetti.remove();
  }
}

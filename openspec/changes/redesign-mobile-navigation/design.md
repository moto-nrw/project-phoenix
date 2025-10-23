# Mobile Navigation Redesign - Design Document

## Architecture Overview

This redesign focuses on the mobile navigation layer of the frontend, specifically targeting the bottom tab bar, top navigation bar, and overflow menu modal. Changes are isolated to the UI layer with no backend or API modifications required.

```
┌─────────────────────────────────────────┐
│  iOS-Style Top Navigation Bar           │ ← New: 56px, centered title
│  (Back | Title | Actions)               │    Remove: gradient line
├─────────────────────────────────────────┤
│                                         │
│  Page Content (Unchanged)               │
│  - /database, /settings, etc.           │
│  - Existing modals and cards            │
│                                         │
├─────────────────────────────────────────┤
│  Floating "Mehr" Modal (Redesigned)     │ ← New: iOS bottom sheet
│  ┌─────────────────────────────────┐   │    28px radius, margins
│  │ Grid of navigation options      │   │
│  └─────────────────────────────────┘   │
├─────────────────────────────────────────┤
│  Floating Bottom Tab Bar (Redesigned)   │ ← New: Glassmorphic pill
│  ┌─────────────────────────────────┐   │    16px margins, 28px radius
│  │ 🏠  👥  🏢  👤  ⋯               │   │
│  └─────────────────────────────────┘   │
└─────────────────────────────────────────┘
```

## Design Principles

### 0. Navigation Parity with Desktop Sidebar

**Rationale:** Mobile users need access to the same functionality as desktop users. No features should be hidden or inaccessible on mobile.

**Desktop Sidebar Navigation Items (from `sidebar.tsx`):**

1. **Home** (Dashboard) - `requiresAdmin`
2. **Meine Gruppe** (OGS Groups) - `requiresGroups`
3. **Mitarbeiter** (Staff) - `alwaysShow`
4. **Mein Raum** (My Room) - `requiresActiveSupervision`
5. **Kindersuche** (Student Search) - `requiresSupervision`
6. **Räume** (Rooms) - `alwaysShow`
7. **Aktivitäten** (Activities) - `alwaysShow`
8. **Vertretungen** (Substitutions) - `requiresAdmin`
9. **Datenverwaltung** (Database) - `requiresAdmin`
10. **Einstellungen** (Settings) - `alwaysShow`

**Mobile Implementation Strategy:**
- **Bottom Tab Bar:** Show 3-5 most frequently used items (Home, Meine Gruppe, Mein Raum, Mitarbeiter, Mehr)
- **"Mehr" Modal:** Show remaining items in clean grid (Kindersuche, Räume, Aktivitäten, Vertretungen, Datenverwaltung, Einstellungen)
- **Permission filtering:** Same logic as desktop (no changes to permission system)
- **Dynamic layout:** Adjust based on user role (admins vs supervisors vs staff)

**Icon Migration:**
- **Current:** Inline SVG paths (e.g., `<path d="M10 20v-6h4v6h5v-8h3L12 3 2 12h3v8z"/>`)
- **New:** Font Awesome icons (e.g., `<FontAwesomeIcon icon={faHome} />`)
- **NO emojis:** User explicitly requested no emojis in UI

### 1. iOS Human Interface Guidelines Compliance

**Rationale:** Users expect mobile apps to follow platform conventions. iOS/Android users have muscle memory for bottom tab bars and top navigation patterns.

**Key Standards:**
- Bottom tab bar: 3-5 items, tinted active state, 49pt height
- Top navigation: Back (left), Title (centered), Actions (right)
- Touch targets: Minimum 44×44pt
- Glassmorphism: Translucent backgrounds with backdrop blur

**Trade-off:** Requires different styling for mobile vs desktop, but provides better UX on each platform.

### 2. Ultra-Minimalist Design Language (Instagram/Twitter 2025)

**Rationale:** Modern apps prioritize content over decoration. Instagram, Twitter, and Uber demonstrate that the best UI is invisible - users shouldn't think about navigation, they should just use it.

**Core Principles:**
- **Content is king:** Navigation should never compete with content
- **Reduce cognitive load:** Fewer visual elements = faster task completion
- **Icon-based:** Universal, language-independent
- **White background:** Clean, fast, battery-efficient
- **Minimal decoration:** No glassmorphism, no shadows, no gradients (except identity)

**Implementation:**
```tsx
// Ultra-minimal bottom nav (Instagram pattern)
bg-white                           // Clean white background
border-t border-gray-200           // Single top border only
// NO backdrop-blur, NO shadows, NO pills

// Active state (minimal)
text-[#5080D8]                     // Color change only
// OR use filled icon variant (faHomeSolid vs faHome)
```

**Why This Works:**
- ✅ Faster rendering (no blur effects)
- ✅ Better battery life (simpler rendering)
- ✅ Universal browser support (no fallbacks needed)
- ✅ Familiar to users (Instagram/Twitter pattern)
- ✅ Content-first approach reduces distractions

### 3. Component Library Standards

**Rationale:** Use established, well-tested component libraries for consistency and maintainability.

**shadcn/UI Components:**
- **Sheet:** For "Mehr" modal (bottom sheet pattern)
- **Button:** For navigation items and actions
- **Separator:** For visual dividers
- **Badge:** For counts or status indicators

**Font Awesome Icons:**
```tsx
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import {
  faHome,
  faUserGroup,
  faBuilding,
  faUserTie,
  faSearch,
  faDoorOpen,
  faClipboardList,
  faRotate,
  faDatabase,
  faCog
} from '@fortawesome/free-solid-svg-icons';

// Usage
<FontAwesomeIcon icon={faHome} className="w-5 h-5" />
```

**Icon Mapping (Desktop Sidebar → Font Awesome):**
| Item | Current SVG | Font Awesome Icon |
|------|-------------|-------------------|
| Home | `M10 20v-6h4v6...` | `faHome` |
| Meine Gruppe | `M17 20h5v-2a3...` | `faUserGroup` |
| Mitarbeiter | `M10 6H5a2 2...` | `faUserTie` |
| Mein Raum | `M19 21V5a2 2...` | `faDoorOpen` |
| Kindersuche | `M21 21l-6-6m2-5...` | `faSearch` |
| Räume | `M19 21V5a2 2...` | `faBuilding` |
| Aktivitäten | `M9 5H7a2 2...` | `faClipboardList` |
| Vertretungen | `M4 4v5h.582m15.356...` | `faRotate` |
| Datenverwaltung | `M4 7v10c0 2.21...` | `faDatabase` |
| Einstellungen | `M10.325 4.317c.426...` | `faCog` |

**Installation:**
```bash
npm install @fortawesome/fontawesome-svg-core @fortawesome/free-solid-svg-icons @fortawesome/react-fontawesome
npx shadcn@latest init
npx shadcn@latest add sheet button separator badge
```

### 4. Ultra-Minimalist Design Tokens

**Rationale:** Instagram/Twitter/Uber prove that less is more. Clean, simple, fast.

**Design Tokens:**
```tsx
// Backgrounds (Ultra-clean)
bg-white                        // Navigation bar
bg-white                        // Modals
// NO backdrop-blur, NO transparency, NO gradients

// Borders (Minimal separation)
border-t border-gray-200        // Top border only (1px)
border-b border-gray-100        // Bottom border only (1px)
// NO decorative borders, NO gradient borders

// Active States (Icon-based)
text-[#5080D8]                  // Primary color for active
text-gray-500                   // Inactive state
// Use Font Awesome filled vs outline variants

// Typography
text-xs                         // Small labels (11-12px)
font-medium                     // Medium weight
// Clean, readable, minimal

// Spacing (Generous touch targets)
py-3 px-4                       // Navigation items
min-h-[44px]                    // Touch target minimum
```

**What's REMOVED (Decorative Elements):**
- ❌ Glassmorphism (backdrop-blur)
- ❌ Floating pill containers
- ❌ Decorative shadows
- ❌ Decorative multi-color gradients
- ❌ Gradient border effects
- ❌ Gradient accent lines
- ❌ Complex shadow stacking
- ❌ Rounded pill shapes

**What's KEPT (Identity Elements ONLY):**
- ✅ Logo text gradient (brand identity)
- ✅ Avatar gradient (user identity)
- ✅ Simple 1px borders (functional separation)

**Philosophy:**
*"The best navigation is invisible. Users shouldn't admire your nav bar - they should use it without thinking."* - Instagram Design Team

## Component Architecture

### Bottom Tab Bar Component

**File:** `frontend/src/components/dashboard/mobile-bottom-nav.tsx`

**Current Structure:**
```tsx
<nav> (fixed, edge-to-edge)
  ├─ Gradient backdrop (decorative - REMOVE)
  ├─ Gradient accent line (decorative - REMOVE)
  ├─ Main nav items (4 items)
  └─ "Mehr" button
```

**New Structure (Instagram/Twitter Pattern):**
```tsx
<nav className="fixed bottom-0 left-0 right-0 bg-white border-t border-gray-200">
  {/* NO floating container, NO glassmorphism */}
  <div className="flex items-center justify-around px-2 py-2">
    {/* Icon + tiny label for each item */}
    <NavItem icon={faHome} label="Home" active={true} />
    <NavItem icon={faUserGroup} label="Gruppe" />
    <NavItem icon={faDoorOpen} label="Raum" />
    <NavItem icon={faUserTie} label="Team" />
    <NavItem icon={faEllipsis} label="Mehr" />
  </div>
  {/* Safe area padding */}
  <div className="h-safe-area-inset-bottom" />
</nav>
```

**Key Changes:**
- Remove: Lines 284-296 (gradient border - decorative)
- Remove: Line 448 (gradient accent line - decorative)
- Remove: Line 445 (gradient backdrop - decorative)
- **Simplify container:** Edge-to-edge white background, single top border
- Update: Animation duration 500ms → 200ms
- **Keep**: Lines 135-150 (auto-hide scroll behavior)
- Update: Slide-out animation (simple translate-y)
- **Migrate icons:** Replace inline SVG with Font Awesome (outline/filled variants)
- **Minimal active state:** Color change or icon variant (NO backgrounds, NO scaling)
- **Ensure parity:** All 10 desktop sidebar items accessible (4-5 in tab bar, rest in modal)

**Preserved Logic:**
- Permission-based item filtering (identical to desktop sidebar)
- Active route detection
- Session-based role checking
- Overflow menu state management
- Dynamic item visibility based on supervision/admin status

**Visual Changes:**
```tsx
// OLD (Decorative)
<nav className="fixed bottom-0 bg-gradient-to-t from-white/95 backdrop-blur-xl">
  <div className="h-0.5 bg-gradient-to-r from-[#5080d8]/30 via-gray-200 to-[#83cd2d]/30" />
  <div className="rounded-t-3xl mx-4 mb-4 bg-white/85 backdrop-blur-2xl shadow-2xl">
    {/* Content */}
  </div>
</nav>

// NEW (Minimal - Instagram pattern)
<nav className="fixed bottom-0 left-0 right-0 bg-white border-t border-gray-200">
  <div className="flex items-center justify-around px-2 py-2">
    {/* Content */}
  </div>
</nav>
```

#### Auto-Hide Scroll Behavior (KEEP)

**Rationale:** User specifically requested to keep this modern pattern. Maximizes content viewing area while maintaining easy access to navigation.

**Current Implementation (lines 135-150):**
```tsx
const [isVisible, setIsVisible] = useState(true);
const [lastScrollY, setLastScrollY] = useState(0);

useEffect(() => {
  const handleScroll = () => {
    const currentScrollY = window.scrollY;

    // Hide when scrolling DOWN past threshold
    if (currentScrollY > lastScrollY && currentScrollY > 100) {
      setIsVisible(false);
    }
    // Show when scrolling UP
    else if (currentScrollY < lastScrollY) {
      setIsVisible(true);
    }

    setLastScrollY(currentScrollY);
  };

  window.addEventListener("scroll", handleScroll, { passive: true });
  return () => window.removeEventListener("scroll", handleScroll);
}, [lastScrollY]);
```

**Animation (Simple Slide):**
```tsx
// Edge-to-edge design - simple translate-y-full
className={`
  fixed bottom-0 left-0 right-0 bg-white border-t border-gray-200
  transition-transform duration-300 ease-in-out
  ${isVisible ? 'translate-y-0' : 'translate-y-full'}
`}
```

**Why This Works:**
- ✅ Simple, clean animation (no complex calculations)
- ✅ Smooth 300ms duration (not jarring)
- ✅ Immediate reappear when scrolling up
- ✅ 100px threshold prevents accidental triggers
- ✅ Common pattern in Instagram, Twitter, Medium, Uber
- ✅ Better performance (simple transform)

### Top Navigation Bar Component

**File:** `frontend/src/components/dashboard/header.tsx`

**Current Structure (Mobile):**
```tsx
<header>
  ├─ Gradient top line (decorative - REMOVE)
  ├─ Logo + Brand + Breadcrumb
  └─ Actions + Avatar
```

**New Structure (Instagram/WhatsApp Pattern):**
```tsx
<header className="lg:hidden sticky top-0 bg-white border-b border-gray-100 z-50">
  <div className="h-14 px-4 flex items-center justify-between">
    {/* LEFT: Simple back button */}
    <button className="p-2 -ml-2 hover:bg-gray-50 rounded-lg min-h-[44px]">
      <FontAwesomeIcon icon={faChevronLeft} className="w-5 h-5 text-gray-700" />
    </button>

    {/* CENTER: Page title (absolute positioning) */}
    <h1 className="absolute left-1/2 -translate-x-1/2 text-[17px] font-semibold text-gray-900">
      {pageTitle}
    </h1>

    {/* RIGHT: Avatar with gradient (identity element - KEEP) */}
    <div className="w-8 h-8 rounded-full bg-gradient-to-br from-[#5080D8] to-[#83CD2D]">
      {initials}
    </div>
  </div>
</header>
```

**Key Changes:**
- Remove: Line 147 (gradient top line)
- Remove: Breadcrumb on mobile
- Add: Back button component (LEFT aligned)
- Update: Title positioning (absolute center)
- Update: Height 64px → 56px
- Conditional: Show back button only when needed

**Preserved Elements:**
- Logo gradient (identity)
- Avatar gradient (identity)
- Session expiry warnings
- Profile menu dropdown

### "Mehr" Modal Component

**File:** `frontend/src/components/dashboard/mobile-bottom-nav.tsx` (lines 278-436)

**Current Structure:**
```tsx
<div> (edge-to-edge, slides from bottom)
  ├─ Gradient border overlay (REMOVE)
  ├─ Header + Close button
  ├─ "Aktivität erstellen" button
  └─ Grid of navigation items
</div>
```

**New Structure (Minimal Bottom Sheet - Twitter/Uber Pattern):**
```tsx
<Sheet open={isOpen} onOpenChange={setIsOpen}>
  <SheetContent
    side="bottom"
    className="bg-white rounded-t-2xl border-t border-gray-200"
  >
    {/* Simple header */}
    <SheetHeader className="px-6 py-4 border-b border-gray-100">
      <SheetTitle className="text-[17px] font-semibold">
        Weitere Optionen
      </SheetTitle>
    </SheetHeader>

    {/* Clean list (NOT grid - simpler, more intuitive) */}
    <div className="divide-y divide-gray-100">
      {items.map(item => (
        <Link className="flex items-center gap-4 px-6 py-4 hover:bg-gray-50 active:bg-gray-100">
          <FontAwesomeIcon icon={item.icon} className="w-5 h-5 text-gray-600" />
          <span className="text-base font-medium">{item.label}</span>
        </Link>
      ))}
    </div>
  </SheetContent>
</Sheet>
```

**Key Changes:**
- Remove: Lines 284-296 (gradient border completely)
- **Migrate to shadcn/UI Sheet:** Replace custom modal implementation
- **Simplify layout:** Use list instead of grid (more intuitive, less cognitive load)
- **Clean white background:** NO glassmorphism, NO decorative effects
- **Minimal styling:** Simple rounded-t-2xl (16px), single border
- Update: Animation duration 500ms → 200ms (Sheet handles this)
- **Clean list items:** Icon + label, hover state only
- Remove: "Aktivität erstellen" special button (move to regular nav item if needed)

**Preserved Logic:**
- Permission filtering for menu items
- Close on backdrop click (Sheet handles)
- Item click handling
- Navigation to all pages

**Visual Simplification:**
```tsx
// OLD (Heavy styling)
<div className="rounded-3xl bg-white/95 backdrop-blur-2xl border-2 shadow-2xl">
  <div className="grid grid-cols-2 gap-3">
    <div className="bg-gradient-to-br p-4 rounded-xl shadow-lg">...</div>
  </div>
</div>

// NEW (Minimal - Twitter pattern)
<SheetContent className="bg-white rounded-t-2xl border-t border-gray-200">
  <div className="divide-y divide-gray-100">
    <Link className="flex items-center gap-4 px-6 py-4 hover:bg-gray-50">
      <FontAwesomeIcon icon={faSearch} className="w-5 h-5 text-gray-600" />
      <span>Kindersuche</span>
    </Link>
  </div>
</SheetContent>
```

## Animation Strategy

### Current Animations (Issues)
```tsx
// Slow slide-up (500ms) - TOO SLOW
transition-all duration-500 ease-out
```

### New Animations (iOS-aligned)
```tsx
// Fast, responsive (200ms) for interactions
transition-all duration-200 ease-out

// Scroll hide/show: Slightly longer for smoothness (300ms)
transition-transform duration-300 ease-in-out

// GPU-accelerated transforms
transform: translate3d(0, 0, 0); // Hardware acceleration
will-change: transform;           // Performance hint
```

**Timing Breakdown:**
- Modal open/close: 200ms
- Tab selection: 200ms (NO scale animation)
- Hover states: 150ms
- **Scroll hide/show: 300ms** (smooth slide, not jarring)

**Performance Considerations:**
- Use `transform` instead of positional properties
- Enable hardware acceleration with `translate3d`
- Add `will-change` hints for animated properties
- Test on low-end devices (60fps target)

## Touch Target Specifications

All interactive elements must meet accessibility standards:

```tsx
// iOS Standard
min-width: 44pt (44px)
min-height: 44pt (44px)

// Android Material Design
min-width: 48dp (48px)
min-height: 48dp (48px)

// Our Implementation (conservative)
min-width: 44px
min-height: 44px
padding: 8-12px (to expand visual size)
```

**Examples:**
```tsx
// Tab bar item
<button className="min-h-[44px] min-w-[44px] py-3 px-4">

// Back button
<button className="min-h-[44px] min-w-[44px] p-2 -ml-2">

// Close button
<button className="min-h-[44px] min-w-[44px] p-2 -mr-2">
```

## Responsive Breakpoints

```tsx
// Mobile-only navigation (hide on desktop)
className="lg:hidden"

// Desktop uses existing sidebar (unchanged)
className="hidden lg:block"

// Breakpoint: 1024px (lg)
// Below: Mobile navigation
// Above: Desktop sidebar
```

**No changes to desktop navigation** - this redesign is mobile-only.

## Safe Area Handling (iOS Notch, Home Indicator)

```tsx
// Bottom tab bar - respect home indicator
<div className="pb-safe-area-inset-bottom">

// CSS implementation
padding-bottom: env(safe-area-inset-bottom);

// Fallback for older browsers
padding-bottom: 16px; /* Fallback */
padding-bottom: max(16px, env(safe-area-inset-bottom));
```

**Device Testing Required:**
- iPhone 12/13/14/15 (notch)
- iPhone 15 Pro (Dynamic Island)
- iPhone SE (no notch)
- Android (various)

## State Management

No changes to existing state management:

**Preserved State:**
- `isVisible` - Auto-hide scroll state
- `isOverflowMenuOpen` - "Mehr" modal state
- `isQuickCreateOpen` - Activity modal state
- Session state (permissions, roles)
- Supervision context (groups, rooms)

**New State (if needed):**
- `showBackButton` - Conditional back button visibility
- Navigation stack tracking (for back button)

## Migration Strategy

**Phase 1: Bottom Tab Bar**
1. Update container styling (floating, glassmorphic)
2. Remove gradient elements
3. Speed up animations
4. Test on devices

**Phase 2: Top Navigation**
1. Create mobile-specific header
2. Add back button component
3. Remove desktop elements
4. Test navigation flow

**Phase 3: "Mehr" Modal**
1. Update positioning (floating)
2. Remove gradient border
3. Redesign grid items
4. Speed up animations

**Phase 4: Polish & Testing**
1. Cross-device testing
2. Animation performance check
3. Touch target verification
4. Safe area validation

**Rollback Plan:**
If issues arise, the changes are isolated to 2 files and can be reverted via Git without affecting backend or other frontend components.

## Testing Strategy

**Visual Testing:**
- iOS Safari (iPhone 12, 14, 15)
- Android Chrome (Pixel, Samsung)
- iPad Safari (verify desktop nav still works)
- Various screen sizes (320px - 428px width)

**Functional Testing:**
- All navigation items route correctly
- Permission filtering works
- Back button navigates correctly
- "Mehr" modal opens/closes
- Active state highlights correct tab
- Touch targets are responsive

**Performance Testing:**
- Animations run at 60fps
- No jank during scroll
- Backdrop-blur performs well
- Memory usage acceptable

**Accessibility Testing:**
- Touch targets ≥44×44pt
- Color contrast ratios pass WCAG
- Focus states visible
- Screen reader compatible (future)

## Open Questions

1. **Auto-hide behavior:** ✅ **RESOLVED** - Keep and refine with floating design
   - Slides out when scrolling down (maximizes content space)
   - Slides in when scrolling up (immediate access)
   - Animation adjusted for floating layout with margins

2. **Back button logic:** When should it show?
   - **Recommendation:** Show on detail pages (students/:id, rooms/:id), hide on main pages (dashboard, ogs_groups)

3. **"Aktivität erstellen" button:** Keep in modal or move to main nav?
   - **Current:** Special button in modal
   - **Recommendation:** Keep as-is to avoid navigation logic changes

4. **Tab count:** Keep current 4 items or optimize to 3 or 5?
   - **Current:** Dynamic (4 items for most users)
   - **Recommendation:** Keep dynamic filtering, it's working well

## References

- Apple Human Interface Guidelines 2025
- [iOS Tab Bars](https://developer.apple.com/design/human-interface-guidelines/tab-bars)
- [iOS Navigation Bars](https://developer.apple.com/design/human-interface-guidelines/navigation-bars)
- Material Design Bottom Navigation
- Mobbin mobile patterns library
- Current `/database` implementation (benchmark)
- Current `/settings` implementation (benchmark)

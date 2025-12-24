# Frontend Testing Guide - Project Phoenix

**Last Updated**: 2025-12-21

A practical guide to testing your Next.js + React frontend with **real examples from your codebase**.

---

## Current State: 🚨 Critical Gap

**Frontend Code**: 25,529 lines (90+ components, 58 helpers, 15 API clients)
**Frontend Tests**: 1 file (useSSE hook only)
**Coverage**: <5%

---

## What to Test in Frontend?

### The Testing Layers

```
Frontend Testing Pyramid
        ┌──────────────┐
        │  Components  │  (User interactions, rendering)
        ├──────────────┤
        │    Hooks     │  (State management, side effects)
        ├──────────────┤
        │ API Clients  │  (Data fetching with mocks)
        ├──────────────┤
        │   Helpers    │  (Pure functions - EASIEST!)
        └──────────────┘
```

---

## 1. Testing Pure Helper Functions (START HERE!)

### Why Start with Helpers?
- ✅ **No React needed** (just plain JavaScript/TypeScript)
- ✅ **No mocking** (pure functions = predictable)
- ✅ **Fast** (milliseconds per test)
- ✅ **High confidence** (easy to understand what's broken)

### Example: Testing `date-helpers.ts`

Your file: `frontend/src/lib/date-helpers.ts`

**What functions exist?**
```typescript
// Pure functions - take input, return output, no side effects
export function formatDuration(minutes: number | null): string
export function calculateDuration(startTime: string, endTime: string | null): number | null
export function formatDate(dateString: string, includeWeekday?: boolean): string
export function formatTime(dateString: string): string
export function groupByDate<T>(items: T[], timestampKey: keyof T): Array<{date: string, entries: T[]}>
```

**Test file**: `frontend/src/lib/__tests__/date-helpers.test.ts`

```typescript
import { describe, it, expect } from 'vitest';
import {
  formatDuration,
  calculateDuration,
  formatDate,
  formatTime,
  groupByDate,
} from '../date-helpers';

describe('date-helpers', () => {
  describe('formatDuration', () => {
    // Test the "happy path" (normal usage)
    it('should format hours and minutes correctly', () => {
      expect(formatDuration(150)).toBe('2 Std. 30 Min.');
      expect(formatDuration(90)).toBe('1 Std. 30 Min.');
      expect(formatDuration(45)).toBe('45 Min.');
    });

    // Test edge cases
    it('should handle zero minutes', () => {
      expect(formatDuration(0)).toBe('0 Min.');
    });

    // Test null handling (special case in your code)
    it('should return "Aktiv" for null (ongoing session)', () => {
      expect(formatDuration(null)).toBe('Aktiv');
    });

    // Test boundary conditions
    it('should handle exactly 60 minutes', () => {
      expect(formatDuration(60)).toBe('1 Std. 0 Min.');
    });
  });

  describe('calculateDuration', () => {
    it('should calculate duration in minutes', () => {
      const start = '2024-12-21T10:00:00Z';
      const end = '2024-12-21T10:30:00Z';

      expect(calculateDuration(start, end)).toBe(30);
    });

    it('should handle hours correctly', () => {
      const start = '2024-12-21T09:00:00Z';
      const end = '2024-12-21T11:30:00Z';

      expect(calculateDuration(start, end)).toBe(150); // 2.5 hours
    });

    it('should return null if endTime not provided (ongoing)', () => {
      const start = '2024-12-21T10:00:00Z';

      expect(calculateDuration(start, null)).toBeNull();
    });
  });

  describe('formatDate', () => {
    it('should format date in German format', () => {
      const result = formatDate('2024-12-21T10:00:00Z');

      expect(result).toBe('21.12.2024'); // DD.MM.YYYY
    });

    it('should include weekday when requested', () => {
      const result = formatDate('2024-12-21T10:00:00Z', true);

      expect(result).toContain('21.12.2024');
      // Weekday should be included (Saturday in German)
    });
  });

  describe('formatTime', () => {
    it('should format time in German format', () => {
      const result = formatTime('2024-12-21T14:30:00Z');

      // Depends on timezone, but should include hours:minutes
      expect(result).toMatch(/\d{2}:\d{2}/);
    });
  });

  describe('groupByDate', () => {
    it('should group items by date', () => {
      const items = [
        { id: '1', created_at: '2024-12-21T10:00:00Z', name: 'Item 1' },
        { id: '2', created_at: '2024-12-20T14:30:00Z', name: 'Item 2' },
        { id: '3', created_at: '2024-12-21T14:00:00Z', name: 'Item 3' },
      ];

      const result = groupByDate(items, 'created_at');

      // Should have 2 groups (Dec 21 and Dec 20)
      expect(result).toHaveLength(2);

      // First group (most recent) should be Dec 21 with 2 items
      expect(result[0].date).toBe('21.12.2024');
      expect(result[0].entries).toHaveLength(2);

      // Second group should be Dec 20 with 1 item
      expect(result[1].date).toBe('20.12.2024');
      expect(result[1].entries).toHaveLength(1);
    });

    it('should handle empty array', () => {
      const result = groupByDate([], 'created_at');

      expect(result).toEqual([]);
    });
  });
});
```

**Run tests**:
```bash
cd frontend
npm run test date-helpers
```

---

### Example: Testing `location-helper.ts`

Your file: `frontend/src/lib/location-helper.ts`

**What it does**: Parses student location strings like "room-101", "Zuhause", "WC"

**Test file**: `frontend/src/lib/__tests__/location-helper.test.ts`

```typescript
import { describe, it, expect } from 'vitest';
import {
  normalizeLocation,
  isPresentLocation,
  isHomeLocation,
  isSchoolyardLocation,
  parseLocation,
} from '../location-helper';

describe('location-helper', () => {
  describe('normalizeLocation', () => {
    it('should normalize "home" to "Zuhause"', () => {
      expect(normalizeLocation('home')).toBe('Zuhause');
      expect(normalizeLocation('zuhause')).toBe('Zuhause');
    });

    it('should normalize "schoolyard" to "Schulhof"', () => {
      expect(normalizeLocation('schoolyard')).toBe('Schulhof');
      expect(normalizeLocation('schulhof')).toBe('Schulhof');
    });

    it('should normalize "wc" to "Anwesend - WC"', () => {
      expect(normalizeLocation('wc')).toBe('Anwesend - WC');
      expect(normalizeLocation('WC')).toBe('Anwesend - WC');
    });

    it('should return "Unbekannt" for empty/null', () => {
      expect(normalizeLocation()).toBe('Unbekannt');
      expect(normalizeLocation('')).toBe('Unbekannt');
      expect(normalizeLocation(null)).toBe('Unbekannt');
    });

    it('should preserve room locations', () => {
      expect(normalizeLocation('room-101')).toBe('room-101');
    });
  });

  describe('isPresentLocation', () => {
    it('should return true for room locations', () => {
      expect(isPresentLocation('room-101')).toBe(true);
      expect(isPresentLocation('room-205')).toBe(true);
    });

    it('should return true for "anwesend"', () => {
      expect(isPresentLocation('anwesend')).toBe(true);
      expect(isPresentLocation('Anwesend')).toBe(true);
    });

    it('should return false for home locations', () => {
      expect(isPresentLocation('Zuhause')).toBe(false);
      expect(isPresentLocation('home')).toBe(false);
    });

    it('should return false for unknown locations', () => {
      expect(isPresentLocation()).toBe(false);
      expect(isPresentLocation('')).toBe(false);
    });
  });

  describe('isHomeLocation', () => {
    it('should return true for "Zuhause"', () => {
      expect(isHomeLocation('Zuhause')).toBe(true);
      expect(isHomeLocation('home')).toBe(true);
    });

    it('should return false for other locations', () => {
      expect(isHomeLocation('room-101')).toBe(false);
      expect(isHomeLocation('Schulhof')).toBe(false);
    });
  });

  describe('parseLocation', () => {
    it('should parse room locations', () => {
      const result = parseLocation('room-101');

      expect(result).toEqual({
        status: expect.any(String),
        room: '101',
        details: expect.any(String),
      });
    });

    it('should parse home location', () => {
      const result = parseLocation('Zuhause');

      expect(result.status).toBeDefined();
      expect(result.room).toBeUndefined();
    });
  });
});
```

---

## 2. Testing React Components

### Component Testing Philosophy

**Test user behavior, NOT implementation details**

```typescript
// ❌ BAD - Testing implementation
expect(component.state.count).toBe(3)

// ✅ GOOD - Testing what user sees
expect(screen.getByText('3 items')).toBeInTheDocument()
```

### Example: Testing `student-list.tsx`

Your file: `frontend/src/components/students/student-list.tsx`

**What it does**: Renders a list of students with click handlers and location badges

**Test file**: `frontend/src/components/students/__tests__/student-list.test.tsx`

```typescript
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi } from 'vitest';
import StudentList from '../student-list';
import type { Student } from '@/lib/api';

describe('StudentList', () => {
  // Mock data (similar to what API returns)
  const mockStudents: Student[] = [
    {
      id: '1',
      person_id: '101',
      first_name: 'Max',
      second_name: 'Mustermann',
      school_class: '3a',
      current_location: 'room-101',
      data_retention_days: 30,
      created_at: '2024-01-01T00:00:00Z',
      updated_at: '2024-01-01T00:00:00Z',
    },
    {
      id: '2',
      person_id: '102',
      first_name: 'Anna',
      second_name: 'Schmidt',
      school_class: '3b',
      current_location: 'Zuhause',
      data_retention_days: 30,
      created_at: '2024-01-01T00:00:00Z',
      updated_at: '2024-01-01T00:00:00Z',
    },
  ] as Student[];

  it('should render empty state when no students', () => {
    render(<StudentList students={[]} />);

    // Check for empty message
    expect(screen.getByText('Keine Schüler vorhanden.')).toBeInTheDocument();
  });

  it('should render list of students', () => {
    render(<StudentList students={mockStudents} />);

    // Verify both students are displayed
    expect(screen.getByText('Max Mustermann')).toBeInTheDocument();
    expect(screen.getByText('Anna Schmidt')).toBeInTheDocument();
  });

  it('should display school class for each student', () => {
    render(<StudentList students={mockStudents} />);

    expect(screen.getByText('3a')).toBeInTheDocument();
    expect(screen.getByText('3b')).toBeInTheDocument();
  });

  it('should call onStudentClick when student is clicked', async () => {
    const onStudentClick = vi.fn();
    const user = userEvent.setup();

    render(
      <StudentList
        students={mockStudents}
        onStudentClick={onStudentClick}
      />
    );

    // Click first student
    await user.click(screen.getByText('Max Mustermann'));

    // Verify callback was called with correct student
    expect(onStudentClick).toHaveBeenCalledTimes(1);
    expect(onStudentClick).toHaveBeenCalledWith(mockStudents[0]);
  });

  it('should not call onStudentClick when no handler provided', async () => {
    const user = userEvent.setup();

    // Should not throw error when clicked
    render(<StudentList students={mockStudents} />);

    await user.click(screen.getByText('Max Mustermann'));
    // No assertion needed - just verify it doesn't crash
  });

  it('should display location badges', () => {
    render(<StudentList students={mockStudents} />);

    // LocationBadge component should render for each student
    // (Exact text depends on LocationBadge implementation)
    expect(screen.getByText(/room-101|Raum 101/i)).toBeInTheDocument();
    expect(screen.getByText(/Zuhause/i)).toBeInTheDocument();
  });

  it('should apply different styling for present vs home students', () => {
    const { container } = render(<StudentList students={mockStudents} />);

    // Verify different classes are applied based on location
    const listItems = container.querySelectorAll('.group');
    expect(listItems.length).toBeGreaterThan(0);
  });
});
```

---

### Example: Testing Form Component with State

Your file: `frontend/src/components/students/student-form.tsx`

**What it does**: Form for creating/editing students with validation

**Test file**: `frontend/src/components/students/__tests__/student-form.test.tsx`

```typescript
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi } from 'vitest';
import StudentForm from '../student-form';

describe('StudentForm', () => {
  it('should render all required fields', () => {
    render(<StudentForm onSubmit={vi.fn()} onCancel={vi.fn()} />);

    // Check for required input fields
    expect(screen.getByLabelText(/Vorname/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/Nachname/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/Klasse/i)).toBeInTheDocument();

    // Check for buttons
    expect(screen.getByRole('button', { name: /Speichern/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Abbrechen/i })).toBeInTheDocument();
  });

  it('should call onSubmit with form data when submitted', async () => {
    const onSubmit = vi.fn();
    const user = userEvent.setup();

    render(<StudentForm onSubmit={onSubmit} onCancel={vi.fn()} />);

    // Fill out form
    await user.type(screen.getByLabelText(/Vorname/i), 'Max');
    await user.type(screen.getByLabelText(/Nachname/i), 'Mustermann');
    await user.type(screen.getByLabelText(/Klasse/i), '3a');

    // Submit form
    await user.click(screen.getByRole('button', { name: /Speichern/i }));

    // Verify onSubmit was called with form data
    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          first_name: 'Max',
          second_name: 'Mustermann',
          school_class: '3a',
        })
      );
    });
  });

  it('should show validation errors for empty required fields', async () => {
    const onSubmit = vi.fn();
    const user = userEvent.setup();

    render(<StudentForm onSubmit={onSubmit} onCancel={vi.fn()} />);

    // Submit without filling form
    await user.click(screen.getByRole('button', { name: /Speichern/i }));

    // Check for validation errors
    await waitFor(() => {
      expect(screen.getByText(/Vorname.*erforderlich/i)).toBeInTheDocument();
      expect(screen.getByText(/Nachname.*erforderlich/i)).toBeInTheDocument();
    });

    // onSubmit should NOT be called
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it('should call onCancel when cancel button clicked', async () => {
    const onCancel = vi.fn();
    const user = userEvent.setup();

    render(<StudentForm onSubmit={vi.fn()} onCancel={onCancel} />);

    await user.click(screen.getByRole('button', { name: /Abbrechen/i }));

    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it('should pre-fill form when editing existing student', () => {
    const existingStudent = {
      id: '1',
      first_name: 'Anna',
      second_name: 'Schmidt',
      school_class: '4b',
    };

    render(
      <StudentForm
        student={existingStudent}
        onSubmit={vi.fn()}
        onCancel={vi.fn()}
      />
    );

    // Verify fields are pre-filled
    expect(screen.getByDisplayValue('Anna')).toBeInTheDocument();
    expect(screen.getByDisplayValue('Schmidt')).toBeInTheDocument();
    expect(screen.getByDisplayValue('4b')).toBeInTheDocument();
  });

  it('should disable submit button while submitting', async () => {
    const onSubmit = vi.fn(() => new Promise(resolve => setTimeout(resolve, 100)));
    const user = userEvent.setup();

    render(<StudentForm onSubmit={onSubmit} onCancel={vi.fn()} />);

    // Fill minimal data
    await user.type(screen.getByLabelText(/Vorname/i), 'Max');
    await user.type(screen.getByLabelText(/Nachname/i), 'Mustermann');

    const submitButton = screen.getByRole('button', { name: /Speichern/i });
    await user.click(submitButton);

    // Button should be disabled during submission
    expect(submitButton).toBeDisabled();

    // Wait for submission to complete
    await waitFor(() => {
      expect(submitButton).not.toBeDisabled();
    });
  });
});
```

---

## 3. Testing API Clients with Mock Service Worker (MSW)

### Why MSW?
- ✅ Mocks at the network level (most realistic)
- ✅ Same handlers for tests and development
- ✅ No need to mock `fetch` or axios
- ✅ Catches API contract violations

### Setup MSW

**Install**:
```bash
npm install -D msw
```

**Create mock handlers**: `frontend/src/test/mocks/handlers.ts`

```typescript
import { http, HttpResponse } from 'msw';

const BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

export const handlers = [
  // Mock GET /api/students
  http.get(`${BASE_URL}/api/students`, () => {
    return HttpResponse.json({
      status: 'success',
      data: [
        {
          id: 1,
          person_id: 101,
          first_name: 'Max',
          second_name: 'Mustermann',
          school_class: '3a',
          current_location: 'room-101',
          data_retention_days: 30,
          created_at: '2024-01-01T00:00:00Z',
          updated_at: '2024-01-01T00:00:00Z',
        },
      ],
    });
  }),

  // Mock POST /api/students
  http.post(`${BASE_URL}/api/students`, async ({ request }) => {
    const body = await request.json();

    return HttpResponse.json({
      status: 'success',
      data: {
        id: 999,
        ...body,
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      },
    });
  }),

  // Mock error response
  http.get(`${BASE_URL}/api/students/:id`, ({ params }) => {
    const { id } = params;

    if (id === '404') {
      return HttpResponse.json(
        {
          status: 'error',
          message: 'Student not found',
        },
        { status: 404 }
      );
    }

    return HttpResponse.json({
      status: 'success',
      data: {
        id: Number(id),
        first_name: 'Max',
        second_name: 'Mustermann',
      },
    });
  }),
];
```

**Setup test server**: `frontend/src/test/setup.ts`

```typescript
import '@testing-library/jest-dom';
import { setupServer } from 'msw/node';
import { handlers } from './mocks/handlers';

// Create server instance
export const server = setupServer(...handlers);

// Start server before all tests
beforeAll(() => server.listen());

// Reset handlers after each test
afterEach(() => server.resetHandlers());

// Cleanup after all tests
afterAll(() => server.close());
```

### Example: Testing API Client

Your file: `frontend/src/lib/student-api.ts`

**Test file**: `frontend/src/lib/__tests__/student-api.test.ts`

```typescript
import { describe, it, expect, beforeAll, afterEach, afterAll } from 'vitest';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { fetchStudents, createStudent, updateStudent, deleteStudent } from '../student-api';

// Setup MSW server
const server = setupServer();

beforeAll(() => server.listen());
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

describe('student-api', () => {
  describe('fetchStudents', () => {
    it('should fetch and transform student data', async () => {
      // Setup mock response
      server.use(
        http.get('http://localhost:3000/api/students', () => {
          return HttpResponse.json([
            {
              id: '1',
              first_name: 'Max',
              second_name: 'Mustermann',
              school_class: '3a',
            },
          ]);
        })
      );

      const students = await fetchStudents();

      expect(students).toHaveLength(1);
      expect(students[0]).toEqual({
        id: '1',
        firstName: 'Max',  // Transformed to camelCase
        secondName: 'Mustermann',
        schoolClass: '3a',
      });
    });

    it('should handle API errors', async () => {
      server.use(
        http.get('http://localhost:3000/api/students', () => {
          return new HttpResponse(null, { status: 500 });
        })
      );

      await expect(fetchStudents()).rejects.toThrow();
    });

    it('should pass filters as query params', async () => {
      let requestUrl = '';

      server.use(
        http.get('http://localhost:3000/api/students', ({ request }) => {
          requestUrl = request.url;
          return HttpResponse.json([]);
        })
      );

      await fetchStudents({ school_class: '3a', search: 'Max' });

      expect(requestUrl).toContain('school_class=3a');
      expect(requestUrl).toContain('search=Max');
    });
  });

  describe('createStudent', () => {
    it('should create student and return response', async () => {
      server.use(
        http.post('http://localhost:3000/api/students', async ({ request }) => {
          const body = await request.json();

          return HttpResponse.json({
            id: '999',
            ...body,
            created_at: '2024-12-21T10:00:00Z',
          });
        })
      );

      const newStudent = {
        first_name: 'Anna',
        second_name: 'Schmidt',
        school_class: '4b',
      };

      const result = await createStudent(newStudent);

      expect(result.id).toBe('999');
      expect(result.firstName).toBe('Anna');
    });
  });

  describe('updateStudent', () => {
    it('should update student', async () => {
      server.use(
        http.put('http://localhost:3000/api/students/1', async ({ request }) => {
          const body = await request.json();

          return HttpResponse.json({
            id: '1',
            ...body,
          });
        })
      );

      const result = await updateStudent('1', { school_class: '5a' });

      expect(result.schoolClass).toBe('5a');
    });
  });

  describe('deleteStudent', () => {
    it('should delete student', async () => {
      server.use(
        http.delete('http://localhost:3000/api/students/1', () => {
          return new HttpResponse(null, { status: 204 });
        })
      );

      await expect(deleteStudent('1')).resolves.not.toThrow();
    });

    it('should handle delete errors', async () => {
      server.use(
        http.delete('http://localhost:3000/api/students/1', () => {
          return HttpResponse.json(
            { status: 'error', message: 'Cannot delete' },
            { status: 400 }
          );
        })
      );

      await expect(deleteStudent('1')).rejects.toThrow();
    });
  });
});
```

---

## 4. Testing Custom Hooks

### Example: Testing a Data Fetching Hook

**Your potential hook**: `frontend/src/lib/hooks/use-students.ts`

```typescript
// Example hook (you might create this)
import { useState, useEffect } from 'react';
import { fetchStudents } from '@/lib/student-api';
import type { Student } from '@/lib/api';

export function useStudents(filters?: StudentFilters) {
  const [students, setStudents] = useState<Student[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function load() {
      try {
        setLoading(true);
        const data = await fetchStudents(filters);
        if (!cancelled) {
          setStudents(data);
          setError(null);
        }
      } catch (err) {
        if (!cancelled) {
          setError(err as Error);
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    }

    load();

    return () => {
      cancelled = true;
    };
  }, [filters]);

  return { students, loading, error };
}
```

**Test file**: `frontend/src/lib/hooks/__tests__/use-students.test.ts`

```typescript
import { renderHook, waitFor } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { useStudents } from '../use-students';
import * as studentApi from '@/lib/student-api';

// Mock the API module
vi.mock('@/lib/student-api');

describe('useStudents', () => {
  it('should fetch students on mount', async () => {
    const mockStudents = [
      { id: '1', first_name: 'Max', second_name: 'Mustermann' },
    ];

    vi.mocked(studentApi.fetchStudents).mockResolvedValue(mockStudents);

    const { result } = renderHook(() => useStudents());

    // Initially loading
    expect(result.current.loading).toBe(true);
    expect(result.current.students).toEqual([]);

    // Wait for data to load
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.students).toEqual(mockStudents);
    expect(result.current.error).toBeNull();
  });

  it('should handle fetch errors', async () => {
    const mockError = new Error('Failed to fetch');
    vi.mocked(studentApi.fetchStudents).mockRejectedValue(mockError);

    const { result } = renderHook(() => useStudents());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.error).toEqual(mockError);
    expect(result.current.students).toEqual([]);
  });

  it('should refetch when filters change', async () => {
    const mockStudents1 = [{ id: '1', school_class: '3a' }];
    const mockStudents2 = [{ id: '2', school_class: '3b' }];

    vi.mocked(studentApi.fetchStudents)
      .mockResolvedValueOnce(mockStudents1)
      .mockResolvedValueOnce(mockStudents2);

    const { result, rerender } = renderHook(
      ({ filters }) => useStudents(filters),
      { initialProps: { filters: { school_class: '3a' } } }
    );

    await waitFor(() => {
      expect(result.current.students).toEqual(mockStudents1);
    });

    // Change filters
    rerender({ filters: { school_class: '3b' } });

    await waitFor(() => {
      expect(result.current.students).toEqual(mockStudents2);
    });
  });
});
```

---

## 5. Priority Test Implementation Plan

### Week 1: Pure Functions (Easiest, Highest ROI)

**Files to test** (30-60 min each):
1. ✅ `date-helpers.ts` - 5 functions → 20-30 tests
2. ✅ `location-helper.ts` - 6 functions → 25-35 tests
3. ✅ `auth-helpers.ts` - 4 mapping functions → 15-20 tests
4. ✅ `student-privacy-helpers.ts` - Privacy logic → 10-15 tests

**Total**: ~70-100 tests, ~4-6 hours of work

### Week 2: Simple Components (Medium ROI)

**Components to test** (1-2 hours each):
1. ✅ `student-list.tsx` - List rendering → 8-12 tests
2. ✅ `location-badge.tsx` - Conditional styling → 6-10 tests
3. ✅ `activity-list.tsx` - Similar to student list → 8-12 tests
4. ✅ `teacher-list.tsx` - List with actions → 8-12 tests

**Total**: ~30-45 tests, ~6-8 hours

### Week 3: Forms & Modals (Higher Complexity)

**Components to test** (2-4 hours each):
1. ✅ `student-form.tsx` - Complex form → 15-20 tests
2. ✅ `activity-form.tsx` - Multi-step form → 15-20 tests
3. ✅ `group-create-modal.tsx` - Modal with state → 10-15 tests
4. ✅ `password-reset-modal.tsx` - Async operations → 12-18 tests

**Total**: ~50-75 tests, ~10-15 hours

### Week 4: API Clients & Integration

**API clients to test** (1-2 hours each):
1. ✅ `student-api.ts` - CRUD operations → 12-16 tests
2. ✅ `activity-api.ts` - Activity operations → 10-14 tests
3. ✅ `auth-api.ts` - Authentication → 8-12 tests
4. ✅ `api-helpers.ts` - Core API utilities → 15-20 tests

**Total**: ~45-60 tests, ~6-8 hours

---

## Running Tests

```bash
cd frontend

# Run all tests
npm run test

# Run tests in watch mode (auto-rerun on changes)
npm run test

# Run specific test file
npm run test student-list.test

# Run with coverage
npm run test:run -- --coverage

# Run with UI (interactive)
npm run test:ui
```

---

## Test File Structure

```
frontend/src/
├── lib/
│   ├── date-helpers.ts
│   ├── __tests__/
│   │   ├── date-helpers.test.ts
│   │   ├── location-helper.test.ts
│   │   └── student-api.test.ts
│   │
├── components/
│   ├── students/
│   │   ├── student-list.tsx
│   │   ├── student-form.tsx
│   │   └── __tests__/
│   │       ├── student-list.test.tsx
│   │       └── student-form.test.tsx
│   │
│   └── ui/
│       ├── modal.tsx
│       └── __tests__/
│           └── modal.test.tsx
│
└── test/
    ├── setup.ts              (Test configuration)
    └── mocks/
        └── handlers.ts       (MSW mock handlers)
```

---

## What NOT to Test

❌ **Don't test**:
- Third-party libraries (React, Next.js internals)
- Auto-generated types
- Simple pass-through props
- CSS-in-JS styles (use visual regression instead)
- Next.js API routes (test via API tests/Bruno)

✅ **DO test**:
- Business logic (calculations, transformations)
- User interactions (clicks, form submissions)
- Conditional rendering (if/else, loading states)
- Error handling
- Data transformations (backend → frontend)

---

## Common Patterns

### Testing Async Operations

```typescript
it('should load data asynchronously', async () => {
  render(<MyComponent />);

  // Wait for element to appear
  await waitFor(() => {
    expect(screen.getByText('Loaded')).toBeInTheDocument();
  });

  // Or use findBy (combines getBy + waitFor)
  expect(await screen.findByText('Loaded')).toBeInTheDocument();
});
```

### Testing User Interactions

```typescript
it('should handle button click', async () => {
  const onClick = vi.fn();
  const user = userEvent.setup();

  render(<Button onClick={onClick}>Click Me</Button>);

  await user.click(screen.getByRole('button'));

  expect(onClick).toHaveBeenCalledTimes(1);
});
```

### Testing Forms

```typescript
it('should submit form data', async () => {
  const onSubmit = vi.fn();
  const user = userEvent.setup();

  render(<MyForm onSubmit={onSubmit} />);

  await user.type(screen.getByLabelText(/name/i), 'John');
  await user.click(screen.getByRole('button', { name: /submit/i }));

  expect(onSubmit).toHaveBeenCalledWith({ name: 'John' });
});
```

---

## Summary

**Start with**: Pure helper functions (easiest, high ROI)
**Then add**: Component tests (user interactions)
**Finally**: API clients with MSW (realistic mocking)

**Goal**: 500+ tests in 4 weeks
- Week 1: 70-100 tests (helpers)
- Week 2: 30-45 tests (simple components)
- Week 3: 50-75 tests (forms/modals)
- Week 4: 45-60 tests (API clients)

**Tools needed**:
- Vitest (test runner)
- React Testing Library (component testing)
- MSW (API mocking)
- @testing-library/user-event (user interactions)

**Run tests**: `npm run test` (watch mode) or `npm run test:run` (CI mode)

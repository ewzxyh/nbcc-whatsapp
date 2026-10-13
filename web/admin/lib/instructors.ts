import type { CourseSummary, SettingsPatch } from './types'

export interface InstructorRow {
  code: string
  title: string
  instructor: string
}

export interface InstructorFormValues {
  entries: InstructorRow[]
}

// One row per course in the feed. Saved names that no longer match a course are kept as extra rows,
// so saving never drops a name silently.
export function instructorRowsFrom(
  courses: CourseSummary[],
  savedMap: Record<string, string>,
): InstructorRow[] {
  const rows: InstructorRow[] = courses.map((course) => ({
    code: course.code,
    title: course.title,
    instructor: savedMap[course.code] ?? course.instructor ?? '',
  }))
  const known = new Set(courses.map((course) => course.code))
  for (const [code, instructor] of Object.entries(savedMap)) {
    if (!known.has(code)) rows.push({ code, title: '', instructor })
  }
  return rows
}

// Drops empty names and trims. The result is the full map that PUT /api/admin/settings expects.
export function buildCourseInstructors(rows: InstructorRow[]): Record<string, string> {
  const entries: [string, string][] = []
  for (const row of rows) {
    const code = row.code.trim()
    const instructor = row.instructor.trim()
    if (code === '' || instructor === '') continue
    entries.push([code, instructor])
  }
  return Object.fromEntries(entries)
}

export function buildInstructorPatch(rows: InstructorRow[]): SettingsPatch {
  return { courseInstructors: buildCourseInstructors(rows) }
}

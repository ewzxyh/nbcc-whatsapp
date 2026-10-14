import { CAMPUS_EVENT_LABEL, courseLabel, displayTitle, formatLongDate, formatTime, isLockDownAssessment, phaseLabel, type AgendaEntry } from './agenda'
import type { AgendaItem } from './api'

// Order matters: "Final Project" must match Project before the Exam pattern sees "final".
const KIND_PATTERNS: [RegExp, string][] = [
  [/project/i, 'Project'],
  [/essay/i, 'Essay'],
  [/quiz/i, 'Quiz'],
  [/lab(?!ora)/i, 'Lab'],
  [/assign/i, 'Assignment'],
  [/exam|midterm|final/i, 'Exam'],
]

/** Assessment type read from its title, such as "Quiz 3" or "Lab01". Null when the title does not say. */
export function assessmentKind(title: string): string | null {
  const match = KIND_PATTERNS.find(([pattern]) => pattern.test(title))
  return match ? match[1] : null
}

function whenText(item: AgendaItem): string {
  const day = formatLongDate(item.date)
  if (item.allDay || !item.startAt) return `${day} (all day)`
  return `${day} at ${formatTime(item.startAt)}`
}

/** Plain-text summary of one card for copying: subject, professor, and every phase with its date and time. */
export function formatCopyText(entry: AgendaEntry): string {
  const lines: string[] = []
  if (entry.kind === 'event') {
    lines.push(`${CAMPUS_EVENT_LABEL}: ${entry.title}`)
    lines.push(`When: ${whenText(entry.item)}`)
    if (entry.item.location) lines.push(`Location: ${entry.item.location}`)
  } else {
    lines.push(`Subject: ${courseLabel(entry)}`)
    lines.push(`Assessment: ${displayTitle(entry)}`)
    if (isLockDownAssessment(entry)) lines.push('LockDown Browser required')
    if (entry.instructor) lines.push(`Professor: ${entry.instructor}`)
    for (const phase of entry.phases) {
      lines.push(`${phaseLabel(phase.phase, isLockDownAssessment(entry))}: ${whenText(phase)}`)
    }
  }
  lines.push('Times: Saint John, NB (Atlantic Time)')
  return lines.join('\n')
}

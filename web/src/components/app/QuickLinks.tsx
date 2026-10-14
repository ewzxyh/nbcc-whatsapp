import { useState } from 'react'
import { BookOpen, ClipboardList, ExternalLink, FileText, House, type LucideIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { QUICK_LINK_COURSES, type QuickLinkCourse } from '@/lib/quickLinks'

// The banner comes from NBCC eLearning, which may refuse it to visitors who are not signed in.
// A neutral block takes its place, so every card keeps the same shape.
function CourseImage({ src }: { src: string }) {
  const [failed, setFailed] = useState(false)
  if (failed) {
    return (
      <div aria-hidden="true" className="flex aspect-video w-full items-center justify-center bg-surface text-muted-foreground">
        <BookOpen className="size-10" />
      </div>
    )
  }
  return (
    <img
      src={src}
      alt=""
      width={640}
      height={360}
      loading="lazy"
      decoding="async"
      onError={() => setFailed(true)}
      className="aspect-video w-full object-cover"
    />
  )
}

function ShortcutLink({ href, label, course, icon: Icon }: { href: string; label: string; course: string; icon: LucideIcon }) {
  return (
    <Button asChild variant="outline" className="w-full justify-start">
      <a href={href} target="_blank" rel="noopener noreferrer" aria-label={`${label}, ${course} (opens in a new tab)`}>
        <Icon aria-hidden="true" />
        <span className="min-w-0 flex-1 text-left">{label}</span>
        <ExternalLink aria-hidden="true" className="size-4" />
      </a>
    </Button>
  )
}

function CourseCard({ course }: { course: QuickLinkCourse }) {
  const headingId = `quick-link-${course.id}`
  return (
    <li>
      <article aria-labelledby={headingId} className="flex h-full flex-col overflow-hidden rounded-2xl border border-input bg-card">
        <CourseImage src={course.imageUrl} />
        <div className="flex flex-1 flex-col gap-4 p-4">
          <h3 id={headingId} className="text-lg font-semibold break-words text-foreground">
            {course.name}
          </h3>
          <div className="mt-auto flex flex-col gap-2">
            <ShortcutLink href={course.homeUrl} label="Course home" course={course.name} icon={House} />
            {course.quizUrl && (
              <ShortcutLink href={course.quizUrl} label="Quiz" course={course.name} icon={ClipboardList} />
            )}
            {course.assignmentsUrl && (
              <ShortcutLink href={course.assignmentsUrl} label="Assignments" course={course.name} icon={FileText} />
            )}
          </div>
        </div>
      </article>
    </li>
  )
}

export function QuickLinks() {
  return (
    <section aria-labelledby="quick-links-heading" className="space-y-4">
      <div className="space-y-1">
        <h2 id="quick-links-heading" className="text-2xl font-semibold text-foreground">
          Course shortcuts
        </h2>
        <p className="text-base text-muted-foreground">
          Opens NBCC eLearning in a new tab. Sign in there to see your course pages, quizzes and assignments.
        </p>
      </div>
      <ul className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {QUICK_LINK_COURSES.map((course) => (
          <CourseCard key={course.id} course={course} />
        ))}
      </ul>
    </section>
  )
}

// Course shortcuts to NBCC eLearning (Brightspace). Students sign in there; this page never does.
// Links are built from each course ID, so the course, quiz and assignment pages always match.

const BASE = 'https://elearning.nbcc.ca'

export interface QuickLinkCourse {
  id: string
  name: string
  imageUrl: string
  homeUrl: string
  quizUrl: string | null
  assignmentsUrl: string | null
}

function course(
  id: string,
  name: string,
  options: { quiz?: boolean; assignments?: boolean } = {},
): QuickLinkCourse {
  return {
    id,
    name,
    imageUrl: `/course-images/${id}.jpg`,
    homeUrl: `${BASE}/d2l/home/${id}`,
    quizUrl: options.quiz === false ? null : `${BASE}/d2l/lms/quizzing/user/quizzes_list.d2l?ou=${id}`,
    assignmentsUrl:
      options.assignments === false ? null : `${BASE}/d2l/lms/dropbox/user/folders_list.d2l?ou=${id}&isprv=0`,
  }
}

export const QUICK_LINK_COURSES: QuickLinkCourse[] = [
  course('322251', 'Command Line'),
  course('322252', 'Microsoft 1'),
  course('320508', 'Linux'),
  course('321247', 'Network'),
  course('321243', 'Cloud I'),
  course('45601', 'Items of Interest', { quiz: false, assignments: false }),
  course('324186', 'Work Safely', { quiz: false, assignments: false }),
  course('12321', 'Items of Interest for Information', {
    quiz: false,
    assignments: false,
  }),
]

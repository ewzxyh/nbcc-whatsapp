// Course shortcuts to NBCC eLearning (Brightspace). Students sign in there; this page never does.
// Links are built from each course ID, so the course, quiz and assignment pages always match.

const BASE = 'https://elearning.nbcc.ca'
const IMAGE_VERSION_PLACEHOLDER = 'c0ffeec0-ffee-c0ff-eec0-ffeec0ffeec0'

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
  imageVersion: string,
  options: { quiz?: boolean; assignments?: boolean } = {},
): QuickLinkCourse {
  return {
    id,
    name,
    imageUrl: `${BASE}/d2l/api/lp/1.47/courses/${id}/image?autoSelectImage=true&versionNumber=${imageVersion}`,
    homeUrl: `${BASE}/d2l/home/${id}`,
    quizUrl: options.quiz === false ? null : `${BASE}/d2l/lms/quizzing/user/quizzes_list.d2l?ou=${id}`,
    assignmentsUrl:
      options.assignments === false ? null : `${BASE}/d2l/lms/dropbox/user/folders_list.d2l?ou=${id}&isprv=0`,
  }
}

export const QUICK_LINK_COURSES: QuickLinkCourse[] = [
  course('322251', 'Command Line', IMAGE_VERSION_PLACEHOLDER),
  course('322252', 'Microsoft 1', '755bd3d3-5fde-4d9e-be0b-acc9f5004d0d'),
  course('320508', 'Linux', '9e2f1649-07c2-4353-8858-311b335941ea'),
  course('321247', 'Network', '2861ed61-f4bc-417f-9c83-586ee45133a1'),
  course('321243', 'Cloud I', 'f91084d8-3b1c-4fe4-8299-202fffc06897'),
  course('45601', 'Items of Interest', IMAGE_VERSION_PLACEHOLDER, { quiz: false, assignments: false }),
  course('324186', 'Work Safely', 'ff47d677-36db-4bda-8199-5e9b4eafa072', { quiz: false, assignments: false }),
  course('12321', 'Items of Interest for Information', '9dcc7b47-1938-4f81-8232-f77627453b8d', {
    quiz: false,
    assignments: false,
  }),
]

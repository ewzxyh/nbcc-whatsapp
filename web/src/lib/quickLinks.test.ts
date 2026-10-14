import { describe, expect, it } from 'vitest'
import { QUICK_LINK_COURSES } from './quickLinks'

describe('quick links', () => {
  it('has unique course IDs and names', () => {
    const ids = QUICK_LINK_COURSES.map((course) => course.id)
    const names = QUICK_LINK_COURSES.map((course) => course.name)
    expect(new Set(ids).size).toBe(ids.length)
    expect(new Set(names).size).toBe(names.length)
  })

  it('points every link at the NBCC eLearning host and the same course', () => {
    for (const course of QUICK_LINK_COURSES) {
      const urls = [course.imageUrl, course.homeUrl, course.quizUrl, course.assignmentsUrl].filter(
        (url): url is string => url !== null,
      )
      for (const url of urls) {
        expect(new URL(url).origin).toBe('https://elearning.nbcc.ca')
        expect(url).toContain(course.id)
      }
      expect(course.homeUrl).toBe(`https://elearning.nbcc.ca/d2l/home/${course.id}`)
      expect(course.imageUrl).toContain(`/d2l/api/lp/1.47/courses/${course.id}/image`)
      if (course.quizUrl) expect(course.quizUrl).toContain(`ou=${course.id}`)
      if (course.assignmentsUrl) expect(course.assignmentsUrl).toContain(`ou=${course.id}&isprv=0`)
    }
  })

  it('matches the supplied quiz and assignment pages for the courses that have them', () => {
    const byName = Object.fromEntries(QUICK_LINK_COURSES.map((course) => [course.name, course]))
    expect(byName['Command Line'].quizUrl).toBe(
      'https://elearning.nbcc.ca/d2l/lms/quizzing/user/quizzes_list.d2l?ou=322251',
    )
    expect(byName['Cloud I'].assignmentsUrl).toBe(
      'https://elearning.nbcc.ca/d2l/lms/dropbox/user/folders_list.d2l?ou=321243&isprv=0',
    )
    expect(byName['Work Safely'].quizUrl).toBeNull()
  })
})

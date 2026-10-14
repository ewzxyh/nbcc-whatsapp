import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
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
      const urls = [course.homeUrl, course.quizUrl, course.assignmentsUrl].filter(
        (url): url is string => url !== null,
      )
      for (const url of urls) {
        expect(new URL(url).origin).toBe('https://elearning.nbcc.ca')
        expect(url).toContain(course.id)
      }
      expect(course.homeUrl).toBe(`https://elearning.nbcc.ca/d2l/home/${course.id}`)
      if (course.quizUrl) expect(course.quizUrl).toContain(`ou=${course.id}`)
      if (course.assignmentsUrl) expect(course.assignmentsUrl).toContain(`ou=${course.id}&isprv=0`)
    }
  })

  it('serves a bundled JPEG for every course without requiring a Brightspace session', () => {
    for (const course of QUICK_LINK_COURSES) {
      expect(course.imageUrl).toMatch(/^\/course-images\/\d+\.jpg$/)
      const image = readFileSync(fileURLToPath(new URL(`../../public${course.imageUrl}`, import.meta.url)))
      expect([...image.subarray(0, 3)]).toEqual([0xff, 0xd8, 0xff])
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

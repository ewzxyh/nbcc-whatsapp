import { readdirSync, readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { createElement } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import CalendarView from '../components/views/CalendarView'

const cssPath = fileURLToPath(new URL('../index.css', import.meta.url))
const uiDir = fileURLToPath(new URL('../components/ui/', import.meta.url))

function readTokens(): Record<string, string> {
  const css = readFileSync(cssPath, 'utf8')
  const root = css.match(/:root\s*\{([^}]*)\}/)
  if (!root) throw new Error('No :root block in index.css')
  const tokens: Record<string, string> = {}
  for (const [, name, value] of root[1].matchAll(/--([a-z-]+):\s*(#[0-9a-fA-F]{6});/g)) {
    tokens[name] = value.toLowerCase()
  }
  return tokens
}

function luminance(hex: string): number {
  const channels = [1, 3, 5].map((offset) => {
    const value = parseInt(hex.slice(offset, offset + 2), 16) / 255
    return value <= 0.03928 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4
  })
  const [r, g, b] = channels
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

export function contrastRatio(a: string, b: string): number {
  const [light, dark] = [luminance(a), luminance(b)].sort((x, y) => y - x)
  return (light + 0.05) / (dark + 0.05)
}

describe('palette contrast', () => {
  const tokens = readTokens()
  const pairs: [string, string, number][] = [
    ['foreground', 'background', 4.5],
    ['foreground', 'surface', 4.5],
    ['muted-foreground', 'background', 4.5],
    ['muted-foreground', 'surface', 4.5],
    ['muted-foreground', 'muted', 4.5],
    ['primary-foreground', 'primary', 4.5],
    ['primary-foreground', 'primary-hover', 4.5],
    ['primary', 'background', 4.5],
    ['accent-foreground', 'accent', 4.5],
    ['destructive', 'background', 4.5],
    ['destructive', 'destructive-subtle', 4.5],
    ['ring', 'background', 3],
    ['ring', 'surface', 3],
    ['input', 'background', 3],
    ['input', 'surface', 3],
  ]

  it('defines every token used in the pairs', () => {
    for (const [a, b] of pairs) {
      expect(tokens[a], `missing --${a}`).toBeDefined()
      expect(tokens[b], `missing --${b}`).toBeDefined()
    }
  })

  for (const selectedDate of ['2026-10-13', '2026-10-14']) {
    it(`makes selected ${selectedDate} distinguishable by at least 3:1`, () => {
      const html = renderToStaticMarkup(createElement(CalendarView, {
        month: '2026-10', today: '2026-10-13', selectedDate, items: [],
        onSelectDate: () => {}, onPreviousMonth: () => {}, onNextMonth: () => {}, onToday: () => {},
        filtersActive: false, onClearFilters: () => {}, onSelect: () => {},
      }))
      const selected = html.match(/<button\b[^>]*aria-pressed="true"[^>]*>/)?.[0]
      expect(selected).toBeDefined()
      const classes = selected!.match(/class="([^"]*)"/)![1].split(' ')
      const todaySelected = selectedDate === '2026-10-13'
      const selectedBackground = classes.includes('bg-primary') ? tokens.primary : tokens.accent
      const indicator = classes.includes('border-primary') && !todaySelected ? tokens.primary : selectedBackground
      expect(contrastRatio(indicator, tokens.background)).toBeGreaterThanOrEqual(3)
      const foreground = classes.includes('text-primary-foreground') ? tokens['primary-foreground'] : tokens['accent-foreground']
      expect(contrastRatio(foreground, selectedBackground)).toBeGreaterThanOrEqual(4.5)
    })
  }

  for (const [foreground, background, minimum] of pairs) {
    it(`${foreground} on ${background} is at least ${minimum}:1`, () => {
      const ratio = contrastRatio(tokens[foreground], tokens[background])
      expect(ratio).toBeGreaterThanOrEqual(minimum)
    })
  }
})

describe('no shadows in shadcn components', () => {
  it('has no shadow-* classes under src/components/ui', () => {
    const offenders = readdirSync(uiDir)
      .filter((file) => file.endsWith('.tsx'))
      .filter((file) => /\bshadow-/.test(readFileSync(`${uiDir}${file}`, 'utf8')))
    expect(offenders).toEqual([])
  })
})

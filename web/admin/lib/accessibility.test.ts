import { createElement } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import { SecretField } from '../components/secret-field'
import type { SecretAction } from './rules'

function renderSecret(action: SecretAction, error?: string) {
  return renderToStaticMarkup(createElement(SecretField, {
    id: 'test-secret', label: 'Example secret', help: 'Keep this secret private.',
    configured: true, action, value: '', error, removeTitle: 'Remove secret?',
    onActionChange: () => {}, onValueChange: () => {}, onGenerate: () => {},
  }))
}

describe('secret fields', () => {
  it('associates the help and validation error with the replacement input', () => {
    const html = renderSecret('replace', 'Enter a valid secret.')
    const input = html.match(/<input\b[^>]*id="test-secret"[^>]*>/)![0]
    expect(input).toContain('aria-describedby="test-secret-help test-secret-error"')
    expect(html).toContain('id="test-secret-help"')
    expect(html).toContain('id="test-secret-error"')
    expect(html.match(/Keep this secret private\./g)).toHaveLength(1)
  })

  for (const action of ['keep', 'replace', 'remove'] as const) {
    it(`does not submit the settings form from ${action} controls`, () => {
      const buttons = [...renderSecret(action).matchAll(/<button\b[^>]*>/g)]
      expect(buttons.length).toBeGreaterThan(0)
      for (const [button] of buttons) expect(button).toContain('type="button"')
    })
  }
})

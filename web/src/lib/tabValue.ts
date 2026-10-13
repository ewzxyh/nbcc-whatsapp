export function tabValue(kind: 'subject' | 'professor', label: string): string {
  const hex = Array.from(new TextEncoder().encode(label), (byte) => byte.toString(16).padStart(2, '0')).join('')
  return `${kind}-${hex}`
}

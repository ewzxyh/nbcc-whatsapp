import { createContext, useContext } from 'react'

/** Opening instant per assessment, from every loaded item so a hidden "Opens" phase still sets the meter window. */
export const OpeningsContext = createContext<ReadonlyMap<string, number>>(new Map())

export function useOpeningAt(activityId: string | null): number | null {
  const openings = useContext(OpeningsContext)
  return activityId ? (openings.get(activityId) ?? null) : null
}

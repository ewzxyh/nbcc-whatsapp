import { createContext, useContext, useEffect, useState } from 'react'

/** Current time in milliseconds, refreshed once a minute by the app shell so countdowns stay current. */
export const NowContext = createContext<number>(0)

export function useNow(): number {
  return useContext(NowContext)
}

export function useMinuteClock(): number {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), 60_000)
    return () => window.clearInterval(id)
  }, [])
  return now
}

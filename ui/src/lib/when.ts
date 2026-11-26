// Every date and time the proxy shows is ISO 8601, 24-hour, in the browser's
// time zone: 2026-11-24, 07:11, 2026-11-24 07:11. Never the browser's locale
// format, which shows the same moment differently to each viewer.

type When = string | number | Date

const pad = (n: number) => String(n).padStart(2, '0')

function at(t: When): Date | undefined {
  const d = t instanceof Date ? t : new Date(t)
  return isNaN(d.getTime()) ? undefined : d
}

/** 2026-11-24 */
export function isoDate(t: When): string {
  const d = at(t)
  return d ? `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}` : ''
}

/** 07:11 */
export function isoTime(t: When): string {
  const d = at(t)
  return d ? `${pad(d.getHours())}:${pad(d.getMinutes())}` : ''
}

/** 2026-11-24 07:11 */
export function isoDateTime(t: When): string {
  const d = at(t)
  return d ? `${isoDate(d)} ${isoTime(d)}` : ''
}

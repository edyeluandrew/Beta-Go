export const ugx = (amount = 0) => `UGX ${amount.toLocaleString('en-UG')}`

export const weekdayName = (day: number) =>
  ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'][day] ?? `Day ${day}`

export const prettyStatus = (status = '') => status.replaceAll('_', ' ')

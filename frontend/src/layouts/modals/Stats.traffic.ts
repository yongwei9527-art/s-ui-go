export type TrafficRecord = {
  dateTime: number
  direction: unknown
  traffic: number
}

export type TrafficBuckets = {
  uplink: Array<number | null>
  downlink: Array<number | null>
}

/**
 * Aggregates records into adjacent half-open time windows: [start, end).
 * A record on a shared boundary belongs to the bucket that starts there.
 */
export function aggregateTrafficByBucket(
  records: TrafficRecord[],
  steps: number[],
): TrafficBuckets {
  const bucketCount = Math.max(steps.length - 1, 0)
  const uplink = Array<number | null>(bucketCount).fill(null)
  const downlink = Array<number | null>(bucketCount).fill(null)

  if (bucketCount === 0) {
    return { uplink, downlink }
  }

  const start = steps[0]
  const stepSize = steps[1] - start
  if (stepSize <= 0) {
    return { uplink, downlink }
  }

  const end = steps[steps.length - 1]
  for (const record of records) {
    const timestamp = record.dateTime * 1000
    if (timestamp < start || timestamp >= end) {
      continue
    }

    const bucketIndex = Math.floor((timestamp - start) / stepSize)
    if (bucketIndex < 0 || bucketIndex >= bucketCount) {
      continue
    }

    const traffic = Number(record.traffic)
    if (!Number.isFinite(traffic)) {
      continue
    }

    const target = record.direction ? uplink : downlink
    target[bucketIndex] = (target[bucketIndex] ?? 0) + traffic
  }

  return { uplink, downlink }
}
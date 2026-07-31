package engine

import (
	"math"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
)

// millisDuration turns a wire millisecond count into a duration, saturating
// at the largest representable duration. A hostile count would otherwise
// wrap into a negative duration and read as "unset" or as a deadline
// already past.
func millisDuration(millis uint64) (duration time.Duration) {
	if millis > maxIntervalMillis {
		return time.Duration(math.MaxInt64)
	}

	return time.Duration(millis) * time.Millisecond
}

// occurrenceCount turns a signed period count into an occurrence count. A
// negative count means the reference instant precedes the schedule start,
// which materializes nothing.
func occurrenceCount(periods time.Duration) (count store.OccurrenceCount) {
	if periods < 0 {
		return 0
	}

	return store.OccurrenceCount(periods)
}

func scheduleInterval(spec protocol.ScheduleSpec) (interval time.Duration, ok bool) {
	if spec.Kind != protocol.ScheduleKindFixedInterval || spec.IntervalMillis == 0 {
		return 0, false
	}

	return millisDuration(spec.IntervalMillis), true
}

func scheduleStart(spec protocol.ScheduleSpec) (start time.Time) {
	return time.Unix(0, spec.StartUnixNano).UTC()
}

func scheduleContains(
	spec protocol.ScheduleSpec,
	occurrence time.Time,
) (contains bool) {
	start := scheduleStart(spec)

	switch spec.Kind {
	case protocol.ScheduleKindOneShot:
		return occurrence.Equal(start)
	case protocol.ScheduleKindFixedInterval:
		interval, ok := scheduleInterval(spec)
		if !ok {
			return false
		}

		if occurrence.Before(start) {
			return false
		}

		return occurrence.Sub(start)%interval == 0
	default:
		return false
	}
}

func nextOccurrence(
	spec protocol.ScheduleSpec,
	after time.Time,
) (occurrence time.Time, exists bool) {
	start := scheduleStart(spec)

	switch spec.Kind {
	case protocol.ScheduleKindOneShot:
		if start.After(after) {
			return start, true
		}

		return time.Time{}, false
	case protocol.ScheduleKindFixedInterval:
		interval, ok := scheduleInterval(spec)
		if !ok {
			return time.Time{}, false
		}

		if start.After(after) {
			return start, true
		}

		elapsed := after.Sub(start)
		periods := int64(elapsed/interval) + 1

		return start.Add(time.Duration(periods * int64(interval))), true
	default:
		return time.Time{}, false
	}
}

func missedOccurrences(
	spec protocol.ScheduleSpec,
	until time.Time,
	maxCount store.OccurrenceCount,
	maxAge time.Duration,
) (missed []time.Time, skipped store.OccurrenceCount) {
	start := scheduleStart(spec)

	switch spec.Kind {
	case protocol.ScheduleKindOneShot:
		if start.After(until) {
			return nil, 0
		}

		if maxAge > 0 && until.Sub(start) > maxAge {
			return nil, 1
		}

		return []time.Time{start}, 0
	case protocol.ScheduleKindFixedInterval:
		interval, ok := scheduleInterval(spec)
		if !ok {
			return nil, 0
		}

		if start.After(until) {
			return nil, 0
		}

		elapsed := until.Sub(start)
		periods := elapsed / interval
		total := occurrenceCount(periods) + 1

		var oldest time.Time
		if maxAge > 0 {
			oldest = until.Add(-maxAge)
		}

		collected := make([]time.Time, 0)
		occurrence := start.Add(elapsed - elapsed%interval)

		for !occurrence.Before(start) {
			if maxCount > 0 && store.OccurrenceCount(len(collected)) >= maxCount {
				break
			}

			if maxAge > 0 && occurrence.Before(oldest) {
				break
			}

			collected = append(collected, occurrence)

			occurrence = occurrence.Add(-interval)
		}

		for left, right := 0, len(collected)-1; left < right; left, right = left+1, right-1 {
			collected[left], collected[right] = collected[right], collected[left]
		}

		return collected, total - store.OccurrenceCount(len(collected))
	default:
		return nil, 0
	}
}

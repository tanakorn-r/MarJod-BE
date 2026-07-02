package timeutil

import "time"

const thailandLocationName = "Asia/Bangkok"

func ThailandLocation() *time.Location {
	loc, err := time.LoadLocation(thailandLocationName)
	if err != nil {
		return time.FixedZone("GMT+7", 7*60*60)
	}
	return loc
}

func Now() time.Time {
	return time.Now().In(ThailandLocation())
}

func InThailand(t time.Time) time.Time {
	return t.In(ThailandLocation())
}

func StartOfDay(t time.Time) time.Time {
	t = InThailand(t)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func StartOfNextDay(t time.Time) time.Time {
	return StartOfDay(t).AddDate(0, 0, 1)
}

func StartOfMonth(t time.Time) time.Time {
	t = InThailand(t)
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

func StartOfNextMonth(t time.Time) time.Time {
	return StartOfMonth(t).AddDate(0, 1, 0)
}

func StartOfWeek(t time.Time) time.Time {
	t = InThailand(t)
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return StartOfDay(t.AddDate(0, 0, -(weekday - 1)))
}

func StartOfNextWeek(t time.Time) time.Time {
	return StartOfWeek(t).AddDate(0, 0, 7)
}

func DateKey(t time.Time) string {
	return InThailand(t).Format("2006-01-02")
}

func MonthKey(t time.Time) string {
	return InThailand(t).Format("2006-01")
}

func ParseMonth(value string) (time.Time, error) {
	return time.ParseInLocation("2006-01", value, ThailandLocation())
}

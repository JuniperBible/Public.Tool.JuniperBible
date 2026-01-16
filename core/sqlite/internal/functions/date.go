package functions

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// RegisterDateTimeFunctions registers all date/time functions.
func RegisterDateTimeFunctions(r *Registry) {
	r.Register(NewScalarFunc("date", -1, dateFunc))
	r.Register(NewScalarFunc("time", -1, timeFunc))
	r.Register(NewScalarFunc("datetime", -1, datetimeFunc))
	r.Register(NewScalarFunc("julianday", -1, juliandayFunc))
	r.Register(NewScalarFunc("unixepoch", -1, unixepochFunc))
	r.Register(NewScalarFunc("strftime", -1, strftimeFunc))
	r.Register(NewScalarFunc("current_date", 0, currentDateFunc))
	r.Register(NewScalarFunc("current_time", 0, currentTimeFunc))
	r.Register(NewScalarFunc("current_timestamp", 0, currentTimestampFunc))
}

// DateTime represents a date/time value in SQLite's internal format.
type DateTime struct {
	// Julian day number (milliseconds * 86400000)
	jd int64

	// Year, Month, Day, Hour, Minute, Second
	year   int
	month  int
	day    int
	hour   int
	minute int
	second float64

	// Timezone offset in minutes
	tz int

	// Validity flags
	validJD  bool
	validYMD bool
	validHMS bool

	// Other flags
	useSubsec bool
	isError   bool
}

const (
	// Julian day for 1970-01-01 00:00:00
	unixEpochJD = 2440587.5

	// Milliseconds per day
	msPerDay = 86400000
)

// parseDateTime parses a date/time string or value.
func parseDateTime(v Value) (*DateTime, error) {
	dt := &DateTime{}

	if v.IsNull() {
		return nil, fmt.Errorf("null value")
	}

	switch v.Type() {
	case TypeInteger, TypeFloat:
		// Numeric value - could be Julian day or Unix timestamp
		f := v.AsFloat64()
		dt.setRawNumber(f)

	case TypeText:
		s := v.AsString()
		if strings.ToLower(s) == "now" {
			dt.setNow()
		} else if err := dt.parseString(s); err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("invalid date/time value")
	}

	return dt, nil
}

// setNow sets the DateTime to the current time.
func (dt *DateTime) setNow() {
	now := time.Now().UTC()
	dt.year = now.Year()
	dt.month = int(now.Month())
	dt.day = now.Day()
	dt.hour = now.Hour()
	dt.minute = now.Minute()
	dt.second = float64(now.Second()) + float64(now.Nanosecond())/1e9
	dt.validYMD = true
	dt.validHMS = true
	dt.computeJD()
}

// setRawNumber sets the DateTime from a numeric value.
func (dt *DateTime) setRawNumber(f float64) {
	// If in valid Julian day range, treat as Julian day
	if f >= 0.0 && f < 5373484.5 {
		dt.jd = int64(f*float64(msPerDay) + 0.5)
		dt.validJD = true
	} else {
		// Treat as Unix timestamp
		dt.jd = int64((f+unixEpochJD*86400.0)*1000.0 + 0.5)
		dt.validJD = true
	}
}

// parseString parses a date/time string.
func (dt *DateTime) parseString(s string) error {
	// Try YYYY-MM-DD format
	if dt.parseYMD(s) {
		return nil
	}

	// Try HH:MM:SS format
	if dt.parseHMS(s) {
		return nil
	}

	// Try as number
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		dt.setRawNumber(f)
		return nil
	}

	return fmt.Errorf("invalid date/time format: %s", s)
}

// parseYMD parses YYYY-MM-DD [HH:MM:SS] format.
func (dt *DateTime) parseYMD(s string) bool {
	// Basic YYYY-MM-DD parsing
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '-' || r == ' ' || r == 'T'
	})

	if len(parts) < 3 {
		return false
	}

	year, err := strconv.Atoi(parts[0])
	if err != nil || year < 0 || year > 9999 {
		return false
	}

	month, err := strconv.Atoi(parts[1])
	if err != nil || month < 1 || month > 12 {
		return false
	}

	day, err := strconv.Atoi(parts[2])
	if err != nil || day < 1 || day > 31 {
		return false
	}

	dt.year = year
	dt.month = month
	dt.day = day
	dt.validYMD = true

	// Check for time component
	if len(parts) > 3 {
		timePart := strings.Join(parts[3:], " ")
		if !dt.parseHMS(timePart) {
			// Try to find time after the date
			idx := strings.IndexAny(s, " T")
			if idx > 0 && idx < len(s)-1 {
				dt.parseHMS(s[idx+1:])
			}
		}
	}

	return true
}

// parseHMS parses HH:MM:SS format.
func (dt *DateTime) parseHMS(s string) bool {
	parts := strings.Split(s, ":")
	if len(parts) < 2 {
		return false
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return false
	}

	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return false
	}

	second := 0.0
	if len(parts) > 2 {
		sec, err := strconv.ParseFloat(parts[2], 64)
		if err != nil || sec < 0 || sec >= 60 {
			return false
		}
		second = sec
	}

	dt.hour = hour
	dt.minute = minute
	dt.second = second
	dt.validHMS = true

	return true
}

// computeJD computes the Julian day number from YMD and HMS.
func (dt *DateTime) computeJD() {
	if dt.validJD {
		return
	}

	year := dt.year
	month := dt.month
	day := dt.day

	if !dt.validYMD {
		year = 2000
		month = 1
		day = 1
	}

	// Meeus algorithm for Julian day calculation
	if month <= 2 {
		year--
		month += 12
	}

	a := year / 100
	b := 2 - a + a/4

	jd := int64(365.25*float64(year+4716)) +
		int64(30.6001*float64(month+1)) +
		int64(day) + int64(b) - 1524

	dt.jd = jd * msPerDay

	if dt.validHMS {
		dt.jd += int64(dt.hour)*3600000 +
			int64(dt.minute)*60000 +
			int64(dt.second*1000.0+0.5)
	}

	// Adjust for timezone
	if dt.tz != 0 {
		dt.jd -= int64(dt.tz) * 60000
	}

	dt.validJD = true
}

// computeYMD computes year, month, day from Julian day.
func (dt *DateTime) computeYMD() {
	if dt.validYMD {
		return
	}

	if !dt.validJD {
		dt.year = 2000
		dt.month = 1
		dt.day = 1
		dt.validYMD = true
		return
	}

	// Convert Julian day to calendar date (Meeus algorithm)
	z := int((dt.jd+43200000)/msPerDay) + 1
	alpha := int((float64(z) - 1867216.25) / 36524.25)
	a := z + 1 + alpha - alpha/4

	b := a + 1524
	c := int((float64(b) - 122.1) / 365.25)
	d := int(365.25 * float64(c))
	e := int(float64(b-d) / 30.6001)

	dt.day = b - d - int(30.6001*float64(e))
	if e < 14 {
		dt.month = e - 1
	} else {
		dt.month = e - 13
	}

	if dt.month > 2 {
		dt.year = c - 4716
	} else {
		dt.year = c - 4715
	}

	dt.validYMD = true
}

// computeHMS computes hour, minute, second from Julian day.
func (dt *DateTime) computeHMS() {
	if dt.validHMS {
		return
	}

	dt.computeJD()

	dayMs := int((dt.jd + 43200000) % msPerDay)
	dt.second = float64(dayMs%60000) / 1000.0
	dayMin := dayMs / 60000
	dt.minute = dayMin % 60
	dt.hour = dayMin / 60

	dt.validHMS = true
}

// applyModifier applies a modifier to the DateTime.
func (dt *DateTime) applyModifier(mod string) error {
	mod = strings.TrimSpace(strings.ToLower(mod))

	// Handle 'start of' modifiers
	if strings.HasPrefix(mod, "start of ") {
		unit := strings.TrimPrefix(mod, "start of ")
		return dt.startOf(unit)
	}

	// Handle numeric modifiers (+/- N units)
	if strings.Contains(mod, " ") {
		parts := strings.Fields(mod)
		if len(parts) >= 2 {
			amount, err := strconv.ParseFloat(parts[0], 64)
			if err == nil {
				unit := parts[1]
				if strings.HasSuffix(unit, "s") {
					unit = unit[:len(unit)-1]
				}
				return dt.add(amount, unit)
			}
		}
	}

	// Handle special modifiers
	switch mod {
	case "utc", "localtime", "auto", "subsec", "subsecond":
		// These would require more complex implementation
		return nil
	default:
		return fmt.Errorf("unknown modifier: %s", mod)
	}
}

// startOf sets the DateTime to the start of a time unit.
func (dt *DateTime) startOf(unit string) error {
	dt.computeYMD()
	dt.computeHMS()

	switch unit {
	case "day":
		dt.hour = 0
		dt.minute = 0
		dt.second = 0
		dt.validJD = false

	case "month":
		dt.day = 1
		dt.hour = 0
		dt.minute = 0
		dt.second = 0
		dt.validJD = false

	case "year":
		dt.month = 1
		dt.day = 1
		dt.hour = 0
		dt.minute = 0
		dt.second = 0
		dt.validJD = false

	default:
		return fmt.Errorf("invalid unit for 'start of': %s", unit)
	}

	return nil
}

// add adds an amount of time to the DateTime.
func (dt *DateTime) add(amount float64, unit string) error {
	dt.computeJD()

	var ms int64

	switch unit {
	case "second":
		ms = int64(amount * 1000)
	case "minute":
		ms = int64(amount * 60000)
	case "hour":
		ms = int64(amount * 3600000)
	case "day":
		ms = int64(amount * msPerDay)
	case "month":
		// Month arithmetic is special
		dt.computeYMD()
		months := int(amount)
		dt.month += months
		for dt.month > 12 {
			dt.month -= 12
			dt.year++
		}
		for dt.month < 1 {
			dt.month += 12
			dt.year--
		}
		dt.validJD = false
		return nil
	case "year":
		dt.computeYMD()
		dt.year += int(amount)
		dt.validJD = false
		return nil
	default:
		return fmt.Errorf("unknown time unit: %s", unit)
	}

	dt.jd += ms
	dt.validYMD = false
	dt.validHMS = false

	return nil
}

// formatDate formats as YYYY-MM-DD.
func (dt *DateTime) formatDate() string {
	dt.computeYMD()
	return fmt.Sprintf("%04d-%02d-%02d", dt.year, dt.month, dt.day)
}

// formatTime formats as HH:MM:SS.
func (dt *DateTime) formatTime() string {
	dt.computeHMS()
	if dt.useSubsec {
		return fmt.Sprintf("%02d:%02d:%06.3f", dt.hour, dt.minute, dt.second)
	}
	return fmt.Sprintf("%02d:%02d:%02d", dt.hour, dt.minute, int(dt.second))
}

// formatDateTime formats as YYYY-MM-DD HH:MM:SS.
func (dt *DateTime) formatDateTime() string {
	return fmt.Sprintf("%s %s", dt.formatDate(), dt.formatTime())
}

// getJulianDay returns the Julian day number.
func (dt *DateTime) getJulianDay() float64 {
	dt.computeJD()
	return float64(dt.jd) / float64(msPerDay)
}

// getUnixEpoch returns seconds since Unix epoch.
func (dt *DateTime) getUnixEpoch() float64 {
	dt.computeJD()
	jdDays := float64(dt.jd) / float64(msPerDay)
	return (jdDays - unixEpochJD) * 86400.0
}

// Date/time function implementations

func dateFunc(args []Value) (Value, error) {
	if len(args) == 0 {
		dt := &DateTime{}
		dt.setNow()
		return NewTextValue(dt.formatDate()), nil
	}

	dt, err := parseDateTime(args[0])
	if err != nil {
		return NewNullValue(), nil
	}

	// Apply modifiers
	for i := 1; i < len(args); i++ {
		if args[i].IsNull() {
			return NewNullValue(), nil
		}
		if err := dt.applyModifier(args[i].AsString()); err != nil {
			return NewNullValue(), nil
		}
	}

	return NewTextValue(dt.formatDate()), nil
}

func timeFunc(args []Value) (Value, error) {
	if len(args) == 0 {
		dt := &DateTime{}
		dt.setNow()
		return NewTextValue(dt.formatTime()), nil
	}

	dt, err := parseDateTime(args[0])
	if err != nil {
		return NewNullValue(), nil
	}

	// Apply modifiers
	for i := 1; i < len(args); i++ {
		if args[i].IsNull() {
			return NewNullValue(), nil
		}
		if err := dt.applyModifier(args[i].AsString()); err != nil {
			return NewNullValue(), nil
		}
	}

	return NewTextValue(dt.formatTime()), nil
}

func datetimeFunc(args []Value) (Value, error) {
	if len(args) == 0 {
		dt := &DateTime{}
		dt.setNow()
		return NewTextValue(dt.formatDateTime()), nil
	}

	dt, err := parseDateTime(args[0])
	if err != nil {
		return NewNullValue(), nil
	}

	// Apply modifiers
	for i := 1; i < len(args); i++ {
		if args[i].IsNull() {
			return NewNullValue(), nil
		}
		if err := dt.applyModifier(args[i].AsString()); err != nil {
			return NewNullValue(), nil
		}
	}

	return NewTextValue(dt.formatDateTime()), nil
}

func juliandayFunc(args []Value) (Value, error) {
	if len(args) == 0 {
		dt := &DateTime{}
		dt.setNow()
		return NewFloatValue(dt.getJulianDay()), nil
	}

	dt, err := parseDateTime(args[0])
	if err != nil {
		return NewNullValue(), nil
	}

	// Apply modifiers
	for i := 1; i < len(args); i++ {
		if args[i].IsNull() {
			return NewNullValue(), nil
		}
		if err := dt.applyModifier(args[i].AsString()); err != nil {
			return NewNullValue(), nil
		}
	}

	return NewFloatValue(dt.getJulianDay()), nil
}

func unixepochFunc(args []Value) (Value, error) {
	if len(args) == 0 {
		dt := &DateTime{}
		dt.setNow()
		epoch := dt.getUnixEpoch()
		if dt.useSubsec {
			return NewFloatValue(epoch), nil
		}
		return NewIntValue(int64(epoch)), nil
	}

	dt, err := parseDateTime(args[0])
	if err != nil {
		return NewNullValue(), nil
	}

	// Apply modifiers
	for i := 1; i < len(args); i++ {
		if args[i].IsNull() {
			return NewNullValue(), nil
		}
		mod := args[i].AsString()
		if strings.ToLower(mod) == "subsec" || strings.ToLower(mod) == "subsecond" {
			dt.useSubsec = true
		}
		if err := dt.applyModifier(mod); err != nil {
			return NewNullValue(), nil
		}
	}

	epoch := dt.getUnixEpoch()
	if dt.useSubsec {
		return NewFloatValue(epoch), nil
	}
	return NewIntValue(int64(epoch)), nil
}

func strftimeFunc(args []Value) (Value, error) {
	if len(args) < 1 {
		return NewNullValue(), nil
	}

	format := args[0].AsString()

	var dt *DateTime
	if len(args) == 1 {
		dt = &DateTime{}
		dt.setNow()
	} else {
		var err error
		dt, err = parseDateTime(args[1])
		if err != nil {
			return NewNullValue(), nil
		}

		// Apply modifiers
		for i := 2; i < len(args); i++ {
			if args[i].IsNull() {
				return NewNullValue(), nil
			}
			if err := dt.applyModifier(args[i].AsString()); err != nil {
				return NewNullValue(), nil
			}
		}
	}

	dt.computeYMD()
	dt.computeHMS()

	result := dt.strftime(format)
	return NewTextValue(result), nil
}

// strftime formats the DateTime according to the format string.
func (dt *DateTime) strftime(format string) string {
	var result strings.Builder

	for i := 0; i < len(format); i++ {
		if format[i] == '%' && i+1 < len(format) {
			i++
			switch format[i] {
			case 'd':
				result.WriteString(fmt.Sprintf("%02d", dt.day))
			case 'm':
				result.WriteString(fmt.Sprintf("%02d", dt.month))
			case 'Y':
				result.WriteString(fmt.Sprintf("%04d", dt.year))
			case 'H':
				result.WriteString(fmt.Sprintf("%02d", dt.hour))
			case 'M':
				result.WriteString(fmt.Sprintf("%02d", dt.minute))
			case 'S':
				result.WriteString(fmt.Sprintf("%02d", int(dt.second)))
			case 'f':
				result.WriteString(fmt.Sprintf("%06.3f", dt.second))
			case 's':
				result.WriteString(fmt.Sprintf("%d", int64(dt.getUnixEpoch())))
			case 'J':
				result.WriteString(fmt.Sprintf("%.16g", dt.getJulianDay()))
			case '%':
				result.WriteByte('%')
			default:
				result.WriteByte('%')
				result.WriteByte(format[i])
			}
		} else {
			result.WriteByte(format[i])
		}
	}

	return result.String()
}

func currentDateFunc(args []Value) (Value, error) {
	dt := &DateTime{}
	dt.setNow()
	return NewTextValue(dt.formatDate()), nil
}

func currentTimeFunc(args []Value) (Value, error) {
	dt := &DateTime{}
	dt.setNow()
	return NewTextValue(dt.formatTime()), nil
}

func currentTimestampFunc(args []Value) (Value, error) {
	dt := &DateTime{}
	dt.setNow()
	return NewTextValue(dt.formatDateTime()), nil
}

// Helper to check for leap year
func isLeapYear(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}

// Helper to get days in month
func daysInMonth(year, month int) int {
	switch month {
	case 4, 6, 9, 11:
		return 30
	case 2:
		if isLeapYear(year) {
			return 29
		}
		return 28
	default:
		return 31
	}
}

// Helper to validate date
func isValidDate(year, month, day int) bool {
	if year < 0 || year > 9999 {
		return false
	}
	if month < 1 || month > 12 {
		return false
	}
	if day < 1 || day > daysInMonth(year, month) {
		return false
	}
	return true
}

// Helper for safe float to int conversion
func safeFloatToInt(f float64) int64 {
	if f > float64(math.MaxInt64) {
		return math.MaxInt64
	}
	if f < float64(math.MinInt64) {
		return math.MinInt64
	}
	return int64(f)
}

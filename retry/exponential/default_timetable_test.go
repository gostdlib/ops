package exponential

// defaultTimeTable is a TimeTable based on the default policy.
var defaultTimeTable = TimeTable{
	MinTime: 81150000000,  // "1m21.15s"
	MaxTime: 243450000000, // "4m3.45s"
	Entries: []TimeTableEntry{
		{
			Attempt:     1,
			Interval:    0,
			MinInterval: 0,
			MaxInterval: 0,
		},
		{
			Attempt:     2,
			Interval:    100000000, // 100ms
			MinInterval: 50000000,  // 50ms
			MaxInterval: 150000000, // 150ms
		},
		{
			Attempt:     3,
			Interval:    200000000, // 200ms
			MinInterval: 100000000, // 100ms
			MaxInterval: 300000000, // 300ms
		},
		{
			Attempt:     4,
			Interval:    400000000, // 400ms
			MinInterval: 200000000, // 200ms
			MaxInterval: 600000000, // 600ms
		},
		{
			Attempt:     5,
			Interval:    800000000,  // 800ms
			MinInterval: 400000000,  // 400ms
			MaxInterval: 1200000000, // 1.2s
		},
		{
			Attempt:     6,
			Interval:    1600000000, // 1.6s
			MinInterval: 800000000,  // 800ms
			MaxInterval: 2400000000, // 2.4s
		},
		{
			Attempt:     7,
			Interval:    3200000000, // 3.2s
			MinInterval: 1600000000, // 1.6s
			MaxInterval: 4800000000, // 4.8s
		},
		{
			Attempt:     8,
			Interval:    6400000000, // 6.4s
			MinInterval: 3200000000, // 3.2s
			MaxInterval: 9600000000, // 9.6s
		},
		{
			Attempt:     9,
			Interval:    12800000000, // 12.8s
			MinInterval: 6400000000,  // 6.4s
			MaxInterval: 19200000000, // 19.2s
		},
		{
			Attempt:     10,
			Interval:    25600000000, // 25.6s
			MinInterval: 12800000000, // 12.8s
			MaxInterval: 38400000000, // 38.4s
		},
		{
			Attempt:     11,
			Interval:    51200000000, // 51.2s
			MinInterval: 25600000000, // 25.6s
			MaxInterval: 76800000000, // 76.8s
		},
		{
			Attempt:     12,
			Interval:    60000000000, // 1m0s
			MinInterval: 30000000000, // 30s
			MaxInterval: 90000000000, // 1m30s
		},
	},
}

func copyDefaultTimeTable() TimeTable {
	return defaultTimeTable
}

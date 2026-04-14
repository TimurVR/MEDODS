package task

import (
	"testing"
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
)

func TestCalculateDueDates(t *testing.T) {
	now := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	startsAt := now

	tests := []struct {
		name     string
		template taskdomain.TaskTemplate
		expected int
	}{
		{
			name: "Daily every 2 days",
			template: taskdomain.TaskTemplate{
				Type:     "daily",
				Interval: 2,
				StartsAt: &startsAt,
			},
			expected: 16,
		},
		{
			name: "Daily every 1 days",
			template: taskdomain.TaskTemplate{
				Type:     "daily",
				Interval: -1,
				StartsAt: &startsAt,
			},
			expected: 31,
		},
		{
			name: "Monthly on 31st",
			template: taskdomain.TaskTemplate{
				Type:       "monthly",
				DayOfMonth: 31,
				StartsAt:   &startsAt,
			},
			expected: 1,
		},
		{
			name: "Parity Even",
			template: taskdomain.TaskTemplate{
				Type:     "parity",
				Parity:   "even",
				StartsAt: &startsAt,
			},
			expected: 15,
		},
		{
			name: "Parity Odd",
			template: taskdomain.TaskTemplate{
				Type:     "parity",
				Parity:   "odd",
				StartsAt: &startsAt,
			},
			expected: 16,
		},
		{
			name: "Specific Days (Some beyond horizon)",
			template: taskdomain.TaskTemplate{
				Type: "specific_days",
				SpecificDays: []time.Time{
					time.Date(2026, 1, 10, 10, 0, 0, 0, time.UTC), 
					time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC), 
					time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC), 
				},
				StartsAt: &startsAt, 
			},
			expected: 3, 
		},
	}

	s := &Service{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dates := s.calculateDueDates(&tt.template)
			if len(dates) != tt.expected {
				t.Errorf("expected %d dates, got %d", tt.expected, len(dates))
			}
		})
	}
}

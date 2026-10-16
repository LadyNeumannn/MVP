package domain

import "testing"

func TestCanTransition(t *testing.T) {
	cases := []struct {
		name string
		from OrderStatus
		to   OrderStatus
		want bool
	}{
		{"created -> confirmed OK", OrderStatusCreated, OrderStatusConfirmed, true},
		{"created -> cancelled OK", OrderStatusCreated, OrderStatusCancelled, true},
		{"created -> preparing запрещено (нельзя пропускать confirmed)", OrderStatusCreated, OrderStatusPreparing, false},
		{"confirmed -> preparing OK", OrderStatusConfirmed, OrderStatusPreparing, true},
		{"preparing -> ready OK", OrderStatusPreparing, OrderStatusReady, true},
		{"ready -> delivered OK", OrderStatusReady, OrderStatusDelivered, true},
		{"ready -> cancelled запрещено (уже почти доставлен)", OrderStatusReady, OrderStatusCancelled, false},
		{"delivered -> что угодно запрещено (терминальный статус)", OrderStatusDelivered, OrderStatusCreated, false},
		{"cancelled -> что угодно запрещено (терминальный статус)", OrderStatusCancelled, OrderStatusConfirmed, false},
		{"неизвестный from", OrderStatus("unknown"), OrderStatusConfirmed, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CanTransition(tc.from, tc.to)
			if got != tc.want {
				t.Errorf("CanTransition(%q, %q) = %v, want %v", tc.from, tc.to, got, tc.want)
			}
		})
	}
}

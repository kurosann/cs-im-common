package dec

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestSqrt(t *testing.T) {
	type args struct {
		i1 decimal.Decimal
	}
	tests := []struct {
		name string
		args args
		want decimal.Decimal
	}{
		{name: "2", args: args{i1: Int(2)}, want: String("1.414213562373095")},
		{name: "5", args: args{i1: Int(5)}, want: String("2.2360679774997897")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Sqrt(tt.args.i1); !Eq(got, tt.want) {
				t.Errorf("Sqrt() = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestN(t *testing.T) {
	assert.Equal(t, N(nil), Zero)
}

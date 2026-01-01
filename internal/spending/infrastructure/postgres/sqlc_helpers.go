package postgres

import (
	"fmt"
	"math/big"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

func decimalToNumeric(value decimal.Decimal) pgtype.Numeric {
	return pgtype.Numeric{
		Int:   value.Coefficient(),
		Exp:   value.Exponent(),
		Valid: true,
	}
}

func numericToDecimal(value pgtype.Numeric) (decimal.Decimal, error) {
	if !value.Valid {
		return decimal.Decimal{}, fmt.Errorf("numeric is NULL")
	}
	if value.NaN {
		return decimal.Decimal{}, fmt.Errorf("numeric is NaN")
	}
	if value.InfinityModifier != pgtype.Finite {
		return decimal.Decimal{}, fmt.Errorf("numeric is %s", value.InfinityModifier)
	}

	intVal := value.Int
	if intVal == nil {
		intVal = big.NewInt(0)
	}

	return decimal.NewFromBigInt(intVal, value.Exp), nil
}

func timeToTimestamptz(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{
		Time:  value,
		Valid: true,
	}
}

func timestamptzToTime(value pgtype.Timestamptz) (time.Time, error) {
	if !value.Valid {
		return time.Time{}, fmt.Errorf("timestamp is NULL")
	}
	if value.InfinityModifier != pgtype.Finite {
		return time.Time{}, fmt.Errorf("timestamp is %s", value.InfinityModifier)
	}
	return value.Time, nil
}

func timestamptzToTimePtr(value pgtype.Timestamptz) (*time.Time, error) {
	if !value.Valid {
		return nil, nil
	}
	if value.InfinityModifier != pgtype.Finite {
		return nil, fmt.Errorf("timestamp is %s", value.InfinityModifier)
	}

	result := value.Time
	return &result, nil
}

func textFromString(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: true}
}

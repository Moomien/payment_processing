package decimal

import (
	"database/sql/driver"
	"fmt"
)

// Scan implements the sql.Scanner interface for database deserialization.
func (d *Decimal) Scan(value interface{}) error {
	// first try to see if the data is stored in database as a Numeric datatype
	switch v := value.(type) {
	case string:
		var err error
		*d, err = NewFromString(unquoteIfQuoted(v))
		return err

	case []byte:
		var err error
		*d, err = NewFromString(unquoteIfQuoted(string(v)))
		return err

	default:
		return fmt.Errorf("could not convert value '%+v' to any known type", value)
	}
}

// Value implements the driver.Valuer interface for database serialization.
func (d Decimal) Value() (driver.Value, error) {
	return d.String(), nil
}

func unquoteIfQuoted(value string) string {
	// If the amount is quoted, strip the quotes
	if len(value) > 2 && value[0] == '"' && value[len(value)-1] == '"' {
		return value[1 : len(value)-1]
	}

	return value
}

package bote

import (
	"context"
	"strings"
	"time"

	"github.com/maxbolgarin/abstract"
	"github.com/maxbolgarin/datetime"
	"github.com/maxbolgarin/errm"
)

// Timezones is a list of timezones by city.
type Timezones struct {
	items *abstract.SafeMap[string, string]
}

// NewTimezones creates a new Timezones. It loads timezones from the provided collection.
// It expects the collection to have a "city" and "timezone" fields in the documents.
func NewTimezones(ctx context.Context, coll *Collection) (*Timezones, error) {
	var tzRaw []timezoneRecord
	err := coll.FindAll(ctx, &tzRaw)
	if err != nil {
		return nil, err
	}

	tz := &Timezones{
		items: abstract.NewSafeMapWithSize[string, string](len(tzRaw)),
	}
	for _, item := range tzRaw {
		tz.items.Set(strings.ToLower(item.City), item.Timezone)
	}

	return tz, nil
}

// Get returns the timezone for the provided input. Input can be a city name or a time zone offset.
func (tz *Timezones) Get(input string) (*time.Location, error) {
	if len(input) == 0 {
		return nil, errm.New("empty input")
	}

	if !isNumeric(input, ' ', '+', '-', ':') {
		city := strings.ToLower(input)
		input = tz.items.Get(city)
	}

	return datetime.ParseUTCOffset(input)
}

type timezoneRecord struct {
	City     string `bson:"city"`
	Timezone string `bson:"timezone"`
}

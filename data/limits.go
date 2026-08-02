package data

import "fmt"

// ParameterBoundsError reports a request parameter outside the range accepted
// by the corresponding official Data API request builder.
type ParameterBoundsError struct {
	Parameter string
	Value     int
	Minimum   int
	Maximum   int
}

func (e *ParameterBoundsError) Error() string {
	return fmt.Sprintf(
		"data: %s must be between %d and %d, got %d",
		e.Parameter,
		e.Minimum,
		e.Maximum,
		e.Value,
	)
}

func validateBound(parameter string, value, minimum, maximum int) error {
	if value < minimum || value > maximum {
		return &ParameterBoundsError{
			Parameter: parameter,
			Value:     value,
			Minimum:   minimum,
			Maximum:   maximum,
		}
	}
	return nil
}

// validatePagination validates endpoint-specific limit and offset bounds.
// A zero limit is treated as omitted by the Go value-based API and therefore
// remains valid even on endpoints whose upstream builder requires a positive
// explicit limit.
func validatePagination(
	endpoint string,
	limit, offset, minimumLimit, maximumLimit, maximumOffset int,
) error {
	if err := validateBound(endpoint+".limit", limit, minimumLimit, maximumLimit); err != nil {
		return err
	}
	return validateBound(endpoint+".offset", offset, 0, maximumOffset)
}

func activityTimeBoundsError(p ActivityParams) error {
	if p.Start < 0 {
		return &ParameterBoundsError{Parameter: "activity.start", Value: int(p.Start), Minimum: 0}
	}
	if p.End < 0 {
		return &ParameterBoundsError{Parameter: "activity.end", Value: int(p.End), Minimum: 0}
	}
	return nil
}

func boundedLimit(limit, max int) int {
	if limit <= 0 {
		return 0
	}
	return min(limit, max)
}

func iteratorLimit(limit, defaultLimit, max int) int {
	if limit <= 0 {
		return min(defaultLimit, max)
	}
	return min(limit, max)
}

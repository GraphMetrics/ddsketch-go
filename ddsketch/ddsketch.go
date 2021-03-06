// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License 2.0.
// Copyright 2020 Datadog, Inc. for original work
// Copyright 2021 GraphMetrics for modifications

package ddsketch

import (
	"errors"
	"math"

	"github.com/graphmetrics/sketches-go/ddsketch/mapping"
	"github.com/graphmetrics/sketches-go/ddsketch/store"
)

type DDSketch struct {
	mapping.IndexMapping
	store store.Store
}

func NewDDSketch(indexMapping mapping.IndexMapping, store store.Store) *DDSketch {
	return &DDSketch{
		IndexMapping: indexMapping,
		store:        store,
	}
}

func NewDefaultDDSketch(relativeAccuracy float64) (*DDSketch, error) {
	return LogUnboundedDenseDDSketch(relativeAccuracy)
}

// Constructs an instance of DDSketch that offers constant-time insertion and whose size grows indefinitely
// to accommodate for the range of input values.
func LogUnboundedDenseDDSketch(relativeAccuracy float64) (*DDSketch, error) {
	indexMapping, err := mapping.NewLogarithmicMapping(relativeAccuracy)
	if err != nil {
		return nil, err
	}
	return NewDDSketch(indexMapping, store.NewDenseStore()), nil
}

// Constructs an instance of DDSketch that offers constant-time insertion and whose size grows until the
// maximum number of bins is reached, at which point bins with lowest indices are collapsed, which causes the
// relative accuracy guarantee to be lost on lowest quantiles if values are all positive, or the mid-range
// quantiles for values closest to zero if values include negative numbers.
func LogCollapsingLowestDenseDDSketch(relativeAccuracy float64, maxNumBins int) (*DDSketch, error) {
	indexMapping, err := mapping.NewLogarithmicMapping(relativeAccuracy)
	if err != nil {
		return nil, err
	}
	return NewDDSketch(indexMapping, store.NewCollapsingLowestDenseStore(maxNumBins)), nil
}

// Constructs an instance of DDSketch that offers constant-time insertion and whose size grows until the
// maximum number of bins is reached, at which point bins with highest indices are collapsed, which causes the
// relative accuracy guarantee to be lost on highest quantiles if values are all positive, or the lowest and
// highest quantiles if values include negative numbers.
func LogCollapsingHighestDenseDDSketch(relativeAccuracy float64, maxNumBins int) (*DDSketch, error) {
	indexMapping, err := mapping.NewLogarithmicMapping(relativeAccuracy)
	if err != nil {
		return nil, err
	}
	return NewDDSketch(indexMapping, store.NewCollapsingHighestDenseStore(maxNumBins)), nil
}

// Adds a value to the sketch.
func (s *DDSketch) Add(value float64) error {
	return s.AddWithCount(value, int32(1))
}

// Adds a value to the sketch with a int32 count.
func (s *DDSketch) AddWithCount(value float64, count int32) error {
	if value < s.MinIndexableValue() || value > s.MaxIndexableValue() {
		return errors.New("input value is outside the range that is tracked by the sketch")
	}
	if count < 0 {
		return errors.New("count cannot be negative")
	}

	s.store.AddWithCount(s.Index(value), count)
	return nil
}

// Return a (deep) copy of this sketch.
func (s *DDSketch) Copy() *DDSketch {
	return &DDSketch{
		IndexMapping: s.IndexMapping,
		store:        s.store.Copy(),
	}
}

// Return the value at the specified quantile. Return a non-nil error if the quantile is invalid
// or if the sketch is empty.
func (s *DDSketch) GetValueAtQuantile(quantile float64) (float64, error) {
	key, err := s.GetIndexAtQuantile(quantile)
	if err != nil {
		return math.NaN(), err
	}
	return s.Value(key), nil
}

// Return the index at the specified quantile. Return a non-nil error if the quantile is invalid
// or if the sketch is empty.
func (s *DDSketch) GetIndexAtQuantile(quantile float64) (int, error) {
	if quantile < 0 || quantile > 1 {
		return 0, errors.New("quantile must be between 0 and 1")
	}

	count := s.GetCount()
	if count == 0 {
		return 0, errors.New("no such element exists")
	}

	rank := quantile * float64(count-1)
	return s.store.KeyAtRank(rank), nil
}

// Return the values at the respective specified quantiles. Return a non-nil error if any of the quantiles
// is invalid or if the sketch is empty.
func (s *DDSketch) GetValuesAtQuantiles(quantiles []float64) ([]float64, error) {
	values := make([]float64, len(quantiles))
	for i, q := range quantiles {
		val, err := s.GetValueAtQuantile(q)
		if err != nil {
			return nil, err
		}
		values[i] = val
	}
	return values, nil
}

// Return the total number of values that have been added to this sketch.
func (s *DDSketch) GetCount() int32 {
	return s.store.TotalCount()
}

// Return true iff no value has been added to this sketch.
func (s *DDSketch) IsEmpty() bool {
	return s.store.IsEmpty()
}

// Return the maximum value that has been added to this sketch. Return a non-nil error if the sketch
// is empty.
func (s *DDSketch) GetMaxValue() (float64, error) {
	maxIndex, err := s.store.MaxIndex()
	if err != nil {
		return math.NaN(), err
	}
	return s.Value(maxIndex), nil
}

// Return the minimum value that has been added to this sketch. Returns a non-nil error if the sketch
// is empty.
func (s *DDSketch) GetMinValue() (float64, error) {
	minIndex, err := s.store.MinIndex()
	if err != nil {
		return math.NaN(), err
	}
	return s.Value(minIndex), nil
}

// Merges the other sketch into this one. After this operation, this sketch encodes the values that
// were added to both this and the other sketches.
func (s *DDSketch) MergeWith(other *DDSketch) error {
	if !s.IndexMapping.Equals(other.IndexMapping) {
		return errors.New("cannot merge sketches with different index mappings")
	}
	s.store.MergeWith(other.store)
	return nil
}

// Extract the bins from the store
func (s *DDSketch) Bins() <-chan store.Bin {
	return s.store.Bins()
}

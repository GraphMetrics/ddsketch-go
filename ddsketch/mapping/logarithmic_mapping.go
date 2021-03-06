// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License 2.0.
// Copyright 2020 Datadog, Inc. for original work
// Copyright 2021 GraphMetrics for modifications

package mapping

import (
	"bytes"
	"errors"
	"fmt"
	"math"
)

// An IndexMapping that is memory-optimal, that is to say that given a targeted relative accuracy, it
// requires the least number of indices to cover a given range of values. This is done by logarithmically
// mapping floating-point values to integers.
type LogarithmicMapping struct {
	relativeAccuracy      float64
	multiplier            float64
	normalizedIndexOffset float64
	minIndexableValue     float64
	maxIndexableValue     float64
}

func NewLogarithmicMapping(relativeAccuracy float64) (*LogarithmicMapping, error) {
	if relativeAccuracy <= 0 || relativeAccuracy >= 1 {
		return nil, errors.New("The relative accuracy must be between 0 and 1.")
	}
	m := &LogarithmicMapping{
		relativeAccuracy: relativeAccuracy,
		multiplier:       1 / math.Log1p(2*relativeAccuracy/(1-relativeAccuracy)),
	}
	m.minIndexableValue = m.computeMinIndexableValue()
	m.maxIndexableValue = m.computeMaxIndexableValue()
	return m, nil
}

func NewLogarithmicMappingWithGamma(gamma, indexOffset float64) (*LogarithmicMapping, error) {
	if gamma <= 1 {
		return nil, errors.New("Gamma must be greater than 1.")
	}
	m := &LogarithmicMapping{
		relativeAccuracy:      1 - 2/(1+gamma),
		multiplier:            1 / math.Log(gamma),
		normalizedIndexOffset: indexOffset,
	}
	m.minIndexableValue = m.computeMinIndexableValue()
	m.maxIndexableValue = m.computeMaxIndexableValue()
	return m, nil
}

func (m *LogarithmicMapping) Equals(other IndexMapping) bool {
	o, ok := other.(*LogarithmicMapping)
	if !ok {
		return false
	}
	tol := 1e-12
	return withinTolerance(m.multiplier, o.multiplier, tol) && withinTolerance(m.normalizedIndexOffset, o.normalizedIndexOffset, tol)
}

func (m *LogarithmicMapping) Index(value float64) int {
	index := math.Log(value)*m.multiplier + m.normalizedIndexOffset
	if index >= 0 {
		return int(index)
	} else {
		return int(index) - 1 // faster than Math.Floor
	}
}

func (m *LogarithmicMapping) Value(index int) float64 {
	return math.Exp(((float64(index) - m.normalizedIndexOffset) / m.multiplier)) * (1 + m.relativeAccuracy)
}

func (m *LogarithmicMapping) MinIndexableValue() float64 {
	return m.minIndexableValue
}

func (m *LogarithmicMapping) computeMinIndexableValue() float64 {
	return math.Max(
		math.Exp((math.MinInt16-m.normalizedIndexOffset)/m.multiplier+1), // so that index >= MinInt16
		minNormalFloat64*(1+m.relativeAccuracy)/(1-m.relativeAccuracy),
	)
}

func (m *LogarithmicMapping) MaxIndexableValue() float64 {
	return m.maxIndexableValue
}

func (m *LogarithmicMapping) computeMaxIndexableValue() float64 {
	return math.Min(
		math.Exp((math.MaxInt16-m.normalizedIndexOffset)/m.multiplier-1), // so that index <= MaxInt16
		math.Exp(expOverflow)/(1+m.relativeAccuracy),                     // so that math.Exp does not overflow
	)
}

func (m *LogarithmicMapping) RelativeAccuracy() float64 {
	return m.relativeAccuracy
}

func (m *LogarithmicMapping) string() string {
	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("relativeAccuracy: %v, multiplier: %v, normalizedIndexOffset: %v\n", m.relativeAccuracy, m.multiplier, m.normalizedIndexOffset))
	return buffer.String()
}

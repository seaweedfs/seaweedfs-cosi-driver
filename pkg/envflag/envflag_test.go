/*
Copyright 2023 Akamai Technologies, Inc.
Copyright 2024 s3gw contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package envflag_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/s3gw-tech/s3gw-cosi-driver/pkg/envflag"
)

//nolint:paralleltest
func TestString(t *testing.T) {
	const (
		DefaultValue = "Default"
		Key          = "KEY"
		Value        = "Value"
	)

	for _, tc := range []struct {
		name           string // required
		key            string
		value          string
		defaultValue   string
		expectedValue  string
		possibleValues []string
	}{
		{
			name: "simple",
		},
		{
			name:          "with default value",
			defaultValue:  DefaultValue,
			expectedValue: DefaultValue,
		},
		{
			name:          "with actual value",
			key:           Key,
			value:         Value,
			defaultValue:  DefaultValue,
			expectedValue: Value,
		},
		{
			name:           "with possible values",
			key:            Key,
			value:          Value,
			defaultValue:   DefaultValue,
			expectedValue:  Value,
			possibleValues: []string{Value, Key, DefaultValue},
		},
		{
			name:           "with possible values (not in set)",
			key:            Key,
			value:          "value",
			defaultValue:   DefaultValue,
			expectedValue:  DefaultValue,
			possibleValues: []string{Value, Key, DefaultValue},
		},
	} {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			if tc.key != "" {
				tc.key = fmt.Sprintf("TEST_%d_%s", rand.Intn(256), tc.key) // #nosec G404

				t.Setenv(tc.key, tc.value)
			}

			actual := envflag.String(tc.key, tc.defaultValue, tc.possibleValues...)
			if actual != tc.expectedValue {
				t.Errorf("expected: %s, got: %s", tc.expectedValue, actual)
			}
		})
	}
}

//nolint:paralleltest
func TestBool(t *testing.T) {
	const (
		Key          = "KEY"
		DefaultValue = true
	)

	for _, tc := range []struct {
		name           string // required
		key            string
		value          string
		defaultValue   bool
		expectedValue  bool
		possibleValues []string
	}{
		{
			name: "simple",
		},
		{
			name:          "with default value",
			defaultValue:  DefaultValue,
			expectedValue: DefaultValue,
		},
		{
			name:          "true",
			key:           Key,
			value:         "true",
			defaultValue:  DefaultValue,
			expectedValue: true,
		},
		{
			name:          "True",
			key:           Key,
			value:         "True",
			defaultValue:  DefaultValue,
			expectedValue: true,
		},
		{
			name:          "TRUE",
			key:           Key,
			value:         "TRUE",
			defaultValue:  DefaultValue,
			expectedValue: true,
		},
		{
			name:          "T",
			key:           Key,
			value:         "T",
			defaultValue:  DefaultValue,
			expectedValue: true,
		},
		{
			name:          "t",
			key:           Key,
			value:         "t",
			defaultValue:  DefaultValue,
			expectedValue: true,
		},
		{
			name:          "false",
			key:           Key,
			value:         "false",
			defaultValue:  DefaultValue,
			expectedValue: false,
		},
		{
			name:          "False",
			key:           Key,
			value:         "False",
			defaultValue:  DefaultValue,
			expectedValue: false,
		},
		{
			name:          "FALSE",
			key:           Key,
			value:         "FALSE",
			defaultValue:  DefaultValue,
			expectedValue: false,
		},
		{
			name:          "f",
			key:           Key,
			value:         "f",
			defaultValue:  DefaultValue,
			expectedValue: false,
		},
		{
			name:          "F",
			key:           Key,
			value:         "F",
			defaultValue:  DefaultValue,
			expectedValue: false,
		},
		{
			name:          "0",
			key:           Key,
			value:         "0",
			defaultValue:  DefaultValue,
			expectedValue: false,
		},
		{
			name:          "1",
			key:           Key,
			value:         "1",
			defaultValue:  DefaultValue,
			expectedValue: true,
		},
	} {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			if tc.key != "" {
				tc.key = fmt.Sprintf("TEST_%d_%s", rand.Intn(256), tc.key) // #nosec G404

				t.Setenv(tc.key, tc.value)
			}

			actual := envflag.Bool(tc.key, tc.defaultValue)
			if actual != tc.expectedValue {
				t.Errorf("expected: %t, got: %t", tc.expectedValue, actual)
			}
		})
	}
}

//nolint:paralleltest
func TestInts(t *testing.T) {
	const (
		DefaultValue = 1
		Key          = "KEY"
		Value        = 10
	)

	for _, tc := range []struct {
		name          string // required
		key           string
		value         int
		defaultValue  int
		expectedValue int
	}{
		{
			name: "simple",
		},
		{
			name:          "with default value",
			defaultValue:  DefaultValue,
			expectedValue: DefaultValue,
		},
		{
			name:          "with actual value",
			key:           Key,
			value:         Value,
			defaultValue:  DefaultValue,
			expectedValue: Value,
		},
	} {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			if tc.key != "" {
				tc.key = fmt.Sprintf("TEST_%d_%s", rand.Intn(256), tc.key) // #nosec G404

				t.Setenv(tc.key, fmt.Sprint(tc.value))
			}

			actual := envflag.Int(tc.key, tc.defaultValue)
			if actual != tc.expectedValue {
				t.Errorf("expected: %d, got: %d", tc.expectedValue, actual)
			}
		})
	}
}

/*
Copyright 2023 Akamai Technologies, Inc.
Copyright 2024 s3gw maintainers.

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

package envflag

import (
	"os"
	"strconv"
)

func String(envKey string, defaultValue string, expectedValues ...string) string {
	val, ok := os.LookupEnv(envKey)
	if !ok {
		val = defaultValue
	}

	if len(expectedValues) == 0 {
		return val
	}

	for _, ev := range expectedValues {
		if ev == val {
			return ev
		}
	}

	return defaultValue
}

func Bool(envKey string, defaultValue bool) bool {
	val, ok := os.LookupEnv(envKey)
	if !ok {
		return defaultValue
	}

	if actual, err := strconv.ParseBool(val); err == nil {
		return actual
	}

	return defaultValue
}

func Int(envKey string, defaultValue int) int {
	val, ok := os.LookupEnv(envKey)
	if !ok {
		return defaultValue
	}

	if actual, err := strconv.Atoi(val); err == nil {
		return actual
	}

	return defaultValue
}

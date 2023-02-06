// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package runtime

import "context"

// MemoryLimiter allows to set GOMEMLIMIT based on different scenarios
type MemoryLimiter interface {
	Run(ctx context.Context) error
}

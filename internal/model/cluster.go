// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package model // import "miniflux.app/v2/internal/model"

import (
	"fmt"
	"time"
)

// Cluster represents a group of related entries.
type Cluster struct {
	ID         int64      `json:"id"`
	UserID     int64      `json:"user_id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	EntryCount *int       `json:"entry_count,omitempty"`
	Entries    Entries    `json:"entries,omitempty"`
}

func (c *Cluster) String() string {
	return fmt.Sprintf("ID=%d, UserID=%d, Name=%s", c.ID, c.UserID, c.Name)
}

// Clusters represents a list of clusters.
type Clusters []*Cluster

// ClusterEntry represents the association between a cluster and an entry.
type ClusterEntry struct {
	ClusterID int64 `json:"cluster_id"`
	EntryID   int64 `json:"entry_id"`
}

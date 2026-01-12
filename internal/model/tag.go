// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package model // import "miniflux.app/v2/internal/model"

import (
	"fmt"
	"time"
)

// Tag source constants
const (
	TagSourceManual = "manual"
	TagSourceAuto   = "auto"
)

// Tag represents a user-defined tag that can be applied to entries.
type Tag struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"user_id"`
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"created_at"`
	EntryCount *int      `json:"entry_count,omitempty"`
}

func (t *Tag) String() string {
	return fmt.Sprintf("ID=%d, UserID=%d, Name=%s", t.ID, t.UserID, t.Name)
}

// Tags represents a list of tags.
type Tags []*Tag

// TagCreationRequest represents a request to create a new tag.
type TagCreationRequest struct {
	Name string `json:"name"`
}

// TagModificationRequest represents a request to modify a tag.
type TagModificationRequest struct {
	Name *string `json:"name"`
}

func (t *TagModificationRequest) Patch(tag *Tag) {
	if t.Name != nil {
		tag.Name = *t.Name
	}
}

// EntryTag represents the association between an entry and a tag.
type EntryTag struct {
	EntryID   int64     `json:"entry_id"`
	TagID     int64     `json:"tag_id"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
	TagName   string    `json:"tag_name,omitempty"`
}

// EntryTags represents a list of entry-tag associations.
type EntryTags []*EntryTag

// EntryTagRequest represents a request to add tags to an entry.
type EntryTagRequest struct {
	TagIDs []int64 `json:"tag_ids"`
}

// EntryTagByNameRequest represents a request to add tags by name to an entry.
type EntryTagByNameRequest struct {
	TagNames []string `json:"tag_names"`
	Source   string   `json:"source,omitempty"`
}

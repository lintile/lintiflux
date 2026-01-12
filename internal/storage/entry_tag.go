// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package storage // import "miniflux.app/v2/internal/storage"

import (
	"fmt"

	"github.com/lib/pq"
	"miniflux.app/v2/internal/model"
)

// AddTagToEntry adds a tag to an entry.
func (s *Storage) AddTagToEntry(userID, entryID, tagID int64, source string) error {
	// Verify entry belongs to user
	var exists bool
	err := s.db.QueryRow(`SELECT true FROM entries WHERE id=$1 AND user_id=$2`, entryID, userID).Scan(&exists)
	if err != nil {
		return fmt.Errorf(`store: entry #%d not found for user #%d: %v`, entryID, userID, err)
	}

	// Verify tag belongs to user
	err = s.db.QueryRow(`SELECT true FROM tags WHERE id=$1 AND user_id=$2`, tagID, userID).Scan(&exists)
	if err != nil {
		return fmt.Errorf(`store: tag #%d not found for user #%d: %v`, tagID, userID, err)
	}

	if source == "" {
		source = model.TagSourceManual
	}

	query := `
		INSERT INTO entry_tags (entry_id, tag_id, source)
		VALUES ($1, $2, $3)
		ON CONFLICT (entry_id, tag_id) DO UPDATE SET source = $3
	`
	_, err = s.db.Exec(query, entryID, tagID, source)
	if err != nil {
		return fmt.Errorf(`store: unable to add tag #%d to entry #%d: %v`, tagID, entryID, err)
	}

	return nil
}

// AddTagsToEntry adds multiple tags to an entry.
func (s *Storage) AddTagsToEntry(userID, entryID int64, tagIDs []int64, source string) error {
	for _, tagID := range tagIDs {
		if err := s.AddTagToEntry(userID, entryID, tagID, source); err != nil {
			return err
		}
	}
	return nil
}

// AddTagToEntryByName adds a tag to an entry by tag name, creating the tag if needed.
func (s *Storage) AddTagToEntryByName(userID, entryID int64, tagName, source string) error {
	tag, err := s.GetOrCreateTag(userID, tagName)
	if err != nil {
		return err
	}

	return s.AddTagToEntry(userID, entryID, tag.ID, source)
}

// AddTagsToEntryByName adds multiple tags to an entry by name, creating tags if needed.
func (s *Storage) AddTagsToEntryByName(userID, entryID int64, tagNames []string, source string) error {
	for _, tagName := range tagNames {
		if err := s.AddTagToEntryByName(userID, entryID, tagName, source); err != nil {
			return err
		}
	}
	return nil
}

// RemoveTagFromEntry removes a tag from an entry.
func (s *Storage) RemoveTagFromEntry(userID, entryID, tagID int64) error {
	// Verify entry belongs to user
	var exists bool
	err := s.db.QueryRow(`SELECT true FROM entries WHERE id=$1 AND user_id=$2`, entryID, userID).Scan(&exists)
	if err != nil {
		return fmt.Errorf(`store: entry #%d not found for user #%d: %v`, entryID, userID, err)
	}

	query := `DELETE FROM entry_tags WHERE entry_id=$1 AND tag_id=$2`
	_, err = s.db.Exec(query, entryID, tagID)
	if err != nil {
		return fmt.Errorf(`store: unable to remove tag #%d from entry #%d: %v`, tagID, entryID, err)
	}

	return nil
}

// RemoveAllTagsFromEntry removes all tags from an entry.
func (s *Storage) RemoveAllTagsFromEntry(userID, entryID int64) error {
	// Verify entry belongs to user
	var exists bool
	err := s.db.QueryRow(`SELECT true FROM entries WHERE id=$1 AND user_id=$2`, entryID, userID).Scan(&exists)
	if err != nil {
		return fmt.Errorf(`store: entry #%d not found for user #%d: %v`, entryID, userID, err)
	}

	query := `DELETE FROM entry_tags WHERE entry_id=$1`
	_, err = s.db.Exec(query, entryID)
	if err != nil {
		return fmt.Errorf(`store: unable to remove all tags from entry #%d: %v`, entryID, err)
	}

	return nil
}

// GetEntryTags returns all tags for an entry.
func (s *Storage) GetEntryTags(userID, entryID int64) (model.EntryTags, error) {
	query := `
		SELECT et.entry_id, et.tag_id, et.source, et.created_at, t.name
		FROM entry_tags et
		JOIN tags t ON et.tag_id = t.id
		WHERE et.entry_id = $1 AND t.user_id = $2
		ORDER BY t.name ASC
	`
	rows, err := s.db.Query(query, entryID, userID)
	if err != nil {
		return nil, fmt.Errorf(`store: unable to fetch entry tags: %v`, err)
	}
	defer rows.Close()

	entryTags := make(model.EntryTags, 0)
	for rows.Next() {
		var et model.EntryTag
		if err := rows.Scan(&et.EntryID, &et.TagID, &et.Source, &et.CreatedAt, &et.TagName); err != nil {
			return nil, fmt.Errorf(`store: unable to fetch entry tag row: %v`, err)
		}
		entryTags = append(entryTags, &et)
	}

	return entryTags, nil
}

// GetEntriesWithTag returns entry IDs that have a specific tag.
func (s *Storage) GetEntriesWithTag(userID, tagID int64) ([]int64, error) {
	query := `
		SELECT et.entry_id
		FROM entry_tags et
		JOIN entries e ON et.entry_id = e.id
		WHERE et.tag_id = $1 AND e.user_id = $2
		ORDER BY e.published_at DESC
	`
	rows, err := s.db.Query(query, tagID, userID)
	if err != nil {
		return nil, fmt.Errorf(`store: unable to fetch entries with tag: %v`, err)
	}
	defer rows.Close()

	entryIDs := make([]int64, 0)
	for rows.Next() {
		var entryID int64
		if err := rows.Scan(&entryID); err != nil {
			return nil, fmt.Errorf(`store: unable to fetch entry ID: %v`, err)
		}
		entryIDs = append(entryIDs, entryID)
	}

	return entryIDs, nil
}

// ConfirmAutoTag changes an auto-generated tag to manual (user confirmed).
func (s *Storage) ConfirmAutoTag(userID, entryID, tagID int64) error {
	// Verify entry belongs to user
	var exists bool
	err := s.db.QueryRow(`SELECT true FROM entries WHERE id=$1 AND user_id=$2`, entryID, userID).Scan(&exists)
	if err != nil {
		return fmt.Errorf(`store: entry #%d not found for user #%d: %v`, entryID, userID, err)
	}

	query := `UPDATE entry_tags SET source=$1 WHERE entry_id=$2 AND tag_id=$3`
	_, err = s.db.Exec(query, model.TagSourceManual, entryID, tagID)
	if err != nil {
		return fmt.Errorf(`store: unable to confirm tag: %v`, err)
	}

	return nil
}

// GetAutoTagsForEntry returns only auto-generated tags for an entry.
func (s *Storage) GetAutoTagsForEntry(userID, entryID int64) (model.EntryTags, error) {
	query := `
		SELECT et.entry_id, et.tag_id, et.source, et.created_at, t.name
		FROM entry_tags et
		JOIN tags t ON et.tag_id = t.id
		WHERE et.entry_id = $1 AND t.user_id = $2 AND et.source = $3
		ORDER BY t.name ASC
	`
	rows, err := s.db.Query(query, entryID, userID, model.TagSourceAuto)
	if err != nil {
		return nil, fmt.Errorf(`store: unable to fetch auto tags: %v`, err)
	}
	defer rows.Close()

	entryTags := make(model.EntryTags, 0)
	for rows.Next() {
		var et model.EntryTag
		if err := rows.Scan(&et.EntryID, &et.TagID, &et.Source, &et.CreatedAt, &et.TagName); err != nil {
			return nil, fmt.Errorf(`store: unable to fetch entry tag row: %v`, err)
		}
		entryTags = append(entryTags, &et)
	}

	return entryTags, nil
}

// RemoveAutoTagsFromEntry removes all auto-generated tags from an entry.
func (s *Storage) RemoveAutoTagsFromEntry(userID, entryID int64) error {
	// Get tag IDs for user's tags first
	query := `
		DELETE FROM entry_tags
		WHERE entry_id = $1 AND source = $2
		AND tag_id IN (SELECT id FROM tags WHERE user_id = $3)
	`
	_, err := s.db.Exec(query, entryID, model.TagSourceAuto, userID)
	if err != nil {
		return fmt.Errorf(`store: unable to remove auto tags from entry #%d: %v`, entryID, err)
	}

	return nil
}

// CountEntriesWithTag returns the number of entries with a specific tag.
func (s *Storage) CountEntriesWithTag(userID, tagID int64) (int, error) {
	var count int
	query := `
		SELECT COUNT(et.entry_id)
		FROM entry_tags et
		JOIN entries e ON et.entry_id = e.id
		WHERE et.tag_id = $1 AND e.user_id = $2
	`
	err := s.db.QueryRow(query, tagID, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf(`store: unable to count entries with tag: %v`, err)
	}

	return count, nil
}

// GetTagNamesForEntries returns tag names for multiple entries (for bulk display).
func (s *Storage) GetTagNamesForEntries(userID int64, entryIDs []int64) (map[int64][]string, error) {
	if len(entryIDs) == 0 {
		return make(map[int64][]string), nil
	}

	query := `
		SELECT et.entry_id, t.name
		FROM entry_tags et
		JOIN tags t ON et.tag_id = t.id
		WHERE t.user_id = $1 AND et.entry_id = ANY($2)
		ORDER BY et.entry_id, t.name
	`
	rows, err := s.db.Query(query, userID, pq.Array(entryIDs))
	if err != nil {
		return nil, fmt.Errorf(`store: unable to fetch tag names for entries: %v`, err)
	}
	defer rows.Close()

	result := make(map[int64][]string)
	for rows.Next() {
		var entryID int64
		var tagName string
		if err := rows.Scan(&entryID, &tagName); err != nil {
			return nil, fmt.Errorf(`store: unable to scan tag name row: %v`, err)
		}
		result[entryID] = append(result[entryID], tagName)
	}

	return result, nil
}

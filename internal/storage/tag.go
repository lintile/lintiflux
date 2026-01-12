// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package storage // import "miniflux.app/v2/internal/storage"

import (
	"database/sql"
	"errors"
	"fmt"

	"miniflux.app/v2/internal/model"
)

// TagByID returns a tag by its ID.
func (s *Storage) TagByID(userID, tagID int64) (*model.Tag, error) {
	var tag model.Tag

	query := `SELECT id, user_id, name, created_at FROM tags WHERE user_id=$1 AND id=$2`
	err := s.db.QueryRow(query, userID, tagID).Scan(&tag.ID, &tag.UserID, &tag.Name, &tag.CreatedAt)

	switch {
	case err == sql.ErrNoRows:
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf(`store: unable to fetch tag: %v`, err)
	default:
		return &tag, nil
	}
}

// TagByName returns a tag by its name for a given user.
func (s *Storage) TagByName(userID int64, name string) (*model.Tag, error) {
	var tag model.Tag

	query := `SELECT id, user_id, name, created_at FROM tags WHERE user_id=$1 AND lower(name)=lower($2)`
	err := s.db.QueryRow(query, userID, name).Scan(&tag.ID, &tag.UserID, &tag.Name, &tag.CreatedAt)

	switch {
	case err == sql.ErrNoRows:
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf(`store: unable to fetch tag by name: %v`, err)
	default:
		return &tag, nil
	}
}

// Tags returns all tags for a user.
func (s *Storage) Tags(userID int64) (model.Tags, error) {
	query := `SELECT id, user_id, name, created_at FROM tags WHERE user_id=$1 ORDER BY name ASC`
	rows, err := s.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf(`store: unable to fetch tags: %v`, err)
	}
	defer rows.Close()

	tags := make(model.Tags, 0)
	for rows.Next() {
		var tag model.Tag
		if err := rows.Scan(&tag.ID, &tag.UserID, &tag.Name, &tag.CreatedAt); err != nil {
			return nil, fmt.Errorf(`store: unable to fetch tag row: %v`, err)
		}
		tags = append(tags, &tag)
	}

	return tags, nil
}

// TagsWithCount returns all tags for a user with entry counts.
func (s *Storage) TagsWithCount(userID int64) (model.Tags, error) {
	query := `
		SELECT
			t.id,
			t.user_id,
			t.name,
			t.created_at,
			COUNT(et.entry_id) AS entry_count
		FROM tags t
		LEFT JOIN entry_tags et ON t.id = et.tag_id
		WHERE t.user_id = $1
		GROUP BY t.id, t.user_id, t.name, t.created_at
		ORDER BY t.name ASC
	`
	rows, err := s.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf(`store: unable to fetch tags with count: %v`, err)
	}
	defer rows.Close()

	tags := make(model.Tags, 0)
	for rows.Next() {
		var tag model.Tag
		var count int
		if err := rows.Scan(&tag.ID, &tag.UserID, &tag.Name, &tag.CreatedAt, &count); err != nil {
			return nil, fmt.Errorf(`store: unable to fetch tag row: %v`, err)
		}
		tag.EntryCount = &count
		tags = append(tags, &tag)
	}

	return tags, nil
}

// CreateTag creates a new tag for a user.
func (s *Storage) CreateTag(userID int64, request *model.TagCreationRequest) (*model.Tag, error) {
	var tag model.Tag

	query := `
		INSERT INTO tags (user_id, name)
		VALUES ($1, $2)
		RETURNING id, user_id, name, created_at
	`
	err := s.db.QueryRow(query, userID, request.Name).Scan(
		&tag.ID,
		&tag.UserID,
		&tag.Name,
		&tag.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf(`store: unable to create tag %q for user ID %d: %v`, request.Name, userID, err)
	}

	return &tag, nil
}

// UpdateTag updates an existing tag.
func (s *Storage) UpdateTag(tag *model.Tag) error {
	query := `UPDATE tags SET name=$1 WHERE id=$2 AND user_id=$3`
	_, err := s.db.Exec(query, tag.Name, tag.ID, tag.UserID)

	if err != nil {
		return fmt.Errorf(`store: unable to update tag: %v`, err)
	}

	return nil
}

// RemoveTag deletes a tag and all its associations with entries.
func (s *Storage) RemoveTag(userID, tagID int64) error {
	query := `DELETE FROM tags WHERE id=$1 AND user_id=$2`
	result, err := s.db.Exec(query, tagID, userID)
	if err != nil {
		return fmt.Errorf(`store: unable to remove tag: %v`, err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(`store: unable to remove tag: %v`, err)
	}

	if count == 0 {
		return errors.New(`store: no tag has been removed`)
	}

	return nil
}

// TagIDExists checks if a tag exists for a user.
func (s *Storage) TagIDExists(userID, tagID int64) bool {
	var result bool
	query := `SELECT true FROM tags WHERE user_id=$1 AND id=$2 LIMIT 1`
	s.db.QueryRow(query, userID, tagID).Scan(&result)
	return result
}

// TagNameExists checks if a tag with the given name exists for a user.
func (s *Storage) TagNameExists(userID int64, name string) bool {
	var result bool
	query := `SELECT true FROM tags WHERE user_id=$1 AND lower(name)=lower($2) LIMIT 1`
	s.db.QueryRow(query, userID, name).Scan(&result)
	return result
}

// AnotherTagExists checks if another tag exists with the same name.
func (s *Storage) AnotherTagExists(userID, tagID int64, name string) bool {
	var result bool
	query := `SELECT true FROM tags WHERE user_id=$1 AND id != $2 AND lower(name)=lower($3) LIMIT 1`
	s.db.QueryRow(query, userID, tagID, name).Scan(&result)
	return result
}

// GetOrCreateTag returns an existing tag or creates a new one.
func (s *Storage) GetOrCreateTag(userID int64, name string) (*model.Tag, error) {
	tag, err := s.TagByName(userID, name)
	if err != nil {
		return nil, err
	}

	if tag != nil {
		return tag, nil
	}

	return s.CreateTag(userID, &model.TagCreationRequest{Name: name})
}

// MergeTags merges multiple tags into one, reassigning all entries.
func (s *Storage) MergeTags(userID int64, targetTagID int64, sourceTagIDs []int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf(`store: unable to begin transaction: %v`, err)
	}

	// Reassign entries from source tags to target tag
	for _, sourceTagID := range sourceTagIDs {
		if sourceTagID == targetTagID {
			continue
		}

		// Insert entries that don't already have the target tag
		query := `
			INSERT INTO entry_tags (entry_id, tag_id, source, created_at)
			SELECT et.entry_id, $1, et.source, et.created_at
			FROM entry_tags et
			WHERE et.tag_id = $2
			ON CONFLICT (entry_id, tag_id) DO NOTHING
		`
		if _, err := tx.Exec(query, targetTagID, sourceTagID); err != nil {
			tx.Rollback()
			return fmt.Errorf(`store: unable to reassign entries: %v`, err)
		}

		// Delete the source tag (cascade will remove entry_tags)
		query = `DELETE FROM tags WHERE id = $1 AND user_id = $2`
		if _, err := tx.Exec(query, sourceTagID, userID); err != nil {
			tx.Rollback()
			return fmt.Errorf(`store: unable to delete source tag: %v`, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf(`store: unable to commit transaction: %v`, err)
	}

	return nil
}

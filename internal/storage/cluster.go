// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package storage // import "miniflux.app/v2/internal/storage"

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
	"miniflux.app/v2/internal/model"
)

// ClusterByID returns a cluster by its ID.
func (s *Storage) ClusterByID(userID, clusterID int64) (*model.Cluster, error) {
	var cluster model.Cluster
	var expiresAt sql.NullTime

	query := `SELECT id, user_id, name, created_at, expires_at FROM clusters WHERE user_id=$1 AND id=$2`
	err := s.db.QueryRow(query, userID, clusterID).Scan(
		&cluster.ID,
		&cluster.UserID,
		&cluster.Name,
		&cluster.CreatedAt,
		&expiresAt,
	)

	switch {
	case err == sql.ErrNoRows:
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf(`store: unable to fetch cluster: %v`, err)
	default:
		if expiresAt.Valid {
			cluster.ExpiresAt = &expiresAt.Time
		}
		return &cluster, nil
	}
}

// Clusters returns all non-expired clusters for a user.
func (s *Storage) Clusters(userID int64) (model.Clusters, error) {
	query := `
		SELECT c.id, c.user_id, c.name, c.created_at, c.expires_at,
		       (SELECT COUNT(*) FROM cluster_entries WHERE cluster_id = c.id) as entry_count
		FROM clusters c
		WHERE c.user_id = $1 AND (c.expires_at IS NULL OR c.expires_at > NOW())
		ORDER BY c.created_at DESC
	`
	rows, err := s.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf(`store: unable to fetch clusters: %v`, err)
	}
	defer rows.Close()

	clusters := make(model.Clusters, 0)
	for rows.Next() {
		var cluster model.Cluster
		var expiresAt sql.NullTime
		var entryCount int

		if err := rows.Scan(&cluster.ID, &cluster.UserID, &cluster.Name, &cluster.CreatedAt, &expiresAt, &entryCount); err != nil {
			return nil, fmt.Errorf(`store: unable to fetch cluster row: %v`, err)
		}

		if expiresAt.Valid {
			cluster.ExpiresAt = &expiresAt.Time
		}
		cluster.EntryCount = &entryCount
		clusters = append(clusters, &cluster)
	}

	return clusters, nil
}

// CreateCluster creates a new cluster.
func (s *Storage) CreateCluster(userID int64, name string, expiresAt *time.Time) (*model.Cluster, error) {
	var cluster model.Cluster
	var nullExpiresAt sql.NullTime

	if expiresAt != nil {
		nullExpiresAt.Time = *expiresAt
		nullExpiresAt.Valid = true
	}

	query := `
		INSERT INTO clusters (user_id, name, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, name, created_at, expires_at
	`
	var retExpiresAt sql.NullTime
	err := s.db.QueryRow(query, userID, name, nullExpiresAt).Scan(
		&cluster.ID,
		&cluster.UserID,
		&cluster.Name,
		&cluster.CreatedAt,
		&retExpiresAt,
	)

	if err != nil {
		return nil, fmt.Errorf(`store: unable to create cluster: %v`, err)
	}

	if retExpiresAt.Valid {
		cluster.ExpiresAt = &retExpiresAt.Time
	}

	return &cluster, nil
}

// AddEntryToCluster adds an entry to a cluster.
func (s *Storage) AddEntryToCluster(clusterID, entryID int64) error {
	query := `
		INSERT INTO cluster_entries (cluster_id, entry_id)
		VALUES ($1, $2)
		ON CONFLICT (cluster_id, entry_id) DO NOTHING
	`
	_, err := s.db.Exec(query, clusterID, entryID)
	if err != nil {
		return fmt.Errorf(`store: unable to add entry to cluster: %v`, err)
	}

	return nil
}

// AddEntriesToCluster adds multiple entries to a cluster.
func (s *Storage) AddEntriesToCluster(clusterID int64, entryIDs []int64) error {
	if len(entryIDs) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf(`store: unable to begin transaction: %v`, err)
	}

	stmt, err := tx.Prepare(`INSERT INTO cluster_entries (cluster_id, entry_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf(`store: unable to prepare statement: %v`, err)
	}
	defer stmt.Close()

	for _, entryID := range entryIDs {
		if _, err := stmt.Exec(clusterID, entryID); err != nil {
			tx.Rollback()
			return fmt.Errorf(`store: unable to add entry to cluster: %v`, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf(`store: unable to commit transaction: %v`, err)
	}

	return nil
}

// RemoveEntryFromCluster removes an entry from a cluster.
func (s *Storage) RemoveEntryFromCluster(clusterID, entryID int64) error {
	query := `DELETE FROM cluster_entries WHERE cluster_id = $1 AND entry_id = $2`
	_, err := s.db.Exec(query, clusterID, entryID)
	if err != nil {
		return fmt.Errorf(`store: unable to remove entry from cluster: %v`, err)
	}

	return nil
}

// GetClusterEntries returns all entries in a cluster.
func (s *Storage) GetClusterEntries(userID, clusterID int64) (model.Entries, error) {
	query := `
		SELECT
			e.id, e.user_id, e.feed_id, e.hash, e.published_at, e.title, e.url,
			e.comments_url, e.author, e.content, e.status, e.starred, e.reading_time,
			e.created_at, e.changed_at, e.share_code,
			f.title as feed_title, f.site_url as feed_site_url,
			f.icon_id, c.id as category_id, c.title as category_title
		FROM entries e
		JOIN cluster_entries ce ON e.id = ce.entry_id
		JOIN feeds f ON e.feed_id = f.id
		JOIN categories c ON f.category_id = c.id
		WHERE ce.cluster_id = $1 AND e.user_id = $2
		ORDER BY e.published_at DESC
	`
	rows, err := s.db.Query(query, clusterID, userID)
	if err != nil {
		return nil, fmt.Errorf(`store: unable to fetch cluster entries: %v`, err)
	}
	defer rows.Close()

	entries := make(model.Entries, 0)
	for rows.Next() {
		var entry model.Entry
		var iconID sql.NullInt64

		entry.Feed = &model.Feed{}
		entry.Feed.Category = &model.Category{}
		entry.Feed.Icon = &model.FeedIcon{}

		err := rows.Scan(
			&entry.ID,
			&entry.UserID,
			&entry.FeedID,
			&entry.Hash,
			&entry.Date,
			&entry.Title,
			&entry.URL,
			&entry.CommentsURL,
			&entry.Author,
			&entry.Content,
			&entry.Status,
			&entry.Starred,
			&entry.ReadingTime,
			&entry.CreatedAt,
			&entry.ChangedAt,
			&entry.ShareCode,
			&entry.Feed.Title,
			&entry.Feed.SiteURL,
			&iconID,
			&entry.Feed.Category.ID,
			&entry.Feed.Category.Title,
		)
		if err != nil {
			return nil, fmt.Errorf(`store: unable to fetch cluster entry row: %v`, err)
		}

		if iconID.Valid {
			entry.Feed.Icon.IconID = iconID.Int64
		}

		entries = append(entries, &entry)
	}

	return entries, nil
}

// GetClusterWithEntries returns a cluster with all its entries.
func (s *Storage) GetClusterWithEntries(userID, clusterID int64) (*model.Cluster, error) {
	cluster, err := s.ClusterByID(userID, clusterID)
	if err != nil {
		return nil, err
	}

	if cluster == nil {
		return nil, nil
	}

	entries, err := s.GetClusterEntries(userID, clusterID)
	if err != nil {
		return nil, err
	}

	cluster.Entries = entries
	count := len(entries)
	cluster.EntryCount = &count

	return cluster, nil
}

// RemoveCluster removes a cluster and all its entry associations.
func (s *Storage) RemoveCluster(userID, clusterID int64) error {
	query := `DELETE FROM clusters WHERE id = $1 AND user_id = $2`
	result, err := s.db.Exec(query, clusterID, userID)
	if err != nil {
		return fmt.Errorf(`store: unable to remove cluster: %v`, err)
	}

	count, _ := result.RowsAffected()
	if count == 0 {
		return fmt.Errorf(`store: cluster not found`)
	}

	return nil
}

// RemoveExpiredClusters removes all expired clusters.
func (s *Storage) RemoveExpiredClusters() (int64, error) {
	query := `DELETE FROM clusters WHERE expires_at IS NOT NULL AND expires_at < NOW()`
	result, err := s.db.Exec(query)
	if err != nil {
		return 0, fmt.Errorf(`store: unable to remove expired clusters: %v`, err)
	}

	return result.RowsAffected()
}

// RemoveAllClusters removes all clusters for a user.
func (s *Storage) RemoveAllClusters(userID int64) error {
	query := `DELETE FROM clusters WHERE user_id = $1`
	_, err := s.db.Exec(query, userID)
	if err != nil {
		return fmt.Errorf(`store: unable to remove all clusters: %v`, err)
	}

	return nil
}

// GetEntriesForClustering returns recent entries that can be clustered.
func (s *Storage) GetEntriesForClustering(userID int64, limit int, maxAgeDays int) (model.Entries, error) {
	query := `
		SELECT
			e.id, e.user_id, e.feed_id, e.title, e.url, e.published_at, e.content,
			f.title as feed_title
		FROM entries e
		JOIN feeds f ON e.feed_id = f.id
		WHERE e.user_id = $1
		  AND e.status != 'removed'
		  AND e.published_at > NOW() - INTERVAL '1 day' * $2
		ORDER BY e.published_at DESC
		LIMIT $3
	`
	rows, err := s.db.Query(query, userID, maxAgeDays, limit)
	if err != nil {
		return nil, fmt.Errorf(`store: unable to fetch entries for clustering: %v`, err)
	}
	defer rows.Close()

	entries := make(model.Entries, 0)
	for rows.Next() {
		var entry model.Entry
		entry.Feed = &model.Feed{}

		err := rows.Scan(
			&entry.ID,
			&entry.UserID,
			&entry.FeedID,
			&entry.Title,
			&entry.URL,
			&entry.Date,
			&entry.Content,
			&entry.Feed.Title,
		)
		if err != nil {
			return nil, fmt.Errorf(`store: unable to fetch entry row for clustering: %v`, err)
		}

		entries = append(entries, &entry)
	}

	return entries, nil
}

// GetEntryClusters returns all clusters that contain a specific entry.
func (s *Storage) GetEntryClusters(userID, entryID int64) (model.Clusters, error) {
	query := `
		SELECT c.id, c.user_id, c.name, c.created_at, c.expires_at
		FROM clusters c
		JOIN cluster_entries ce ON c.id = ce.cluster_id
		WHERE ce.entry_id = $1 AND c.user_id = $2
		  AND (c.expires_at IS NULL OR c.expires_at > NOW())
		ORDER BY c.created_at DESC
	`
	rows, err := s.db.Query(query, entryID, userID)
	if err != nil {
		return nil, fmt.Errorf(`store: unable to fetch entry clusters: %v`, err)
	}
	defer rows.Close()

	clusters := make(model.Clusters, 0)
	for rows.Next() {
		var cluster model.Cluster
		var expiresAt sql.NullTime

		if err := rows.Scan(&cluster.ID, &cluster.UserID, &cluster.Name, &cluster.CreatedAt, &expiresAt); err != nil {
			return nil, fmt.Errorf(`store: unable to fetch cluster row: %v`, err)
		}

		if expiresAt.Valid {
			cluster.ExpiresAt = &expiresAt.Time
		}
		clusters = append(clusters, &cluster)
	}

	return clusters, nil
}

// UpdateEntrySummary updates the summary for an entry.
func (s *Storage) UpdateEntrySummary(entryID int64, summary string) error {
	query := `UPDATE entries SET summary = $1, summarized_at = NOW() WHERE id = $2`
	_, err := s.db.Exec(query, summary, entryID)
	if err != nil {
		return fmt.Errorf(`store: unable to update entry summary: %v`, err)
	}

	return nil
}

// UpdateEntryEmbedding updates the embedding for an entry.
func (s *Storage) UpdateEntryEmbedding(entryID int64, embedding []byte) error {
	query := `UPDATE entries SET embedding = $1 WHERE id = $2`
	_, err := s.db.Exec(query, embedding, entryID)
	if err != nil {
		return fmt.Errorf(`store: unable to update entry embedding: %v`, err)
	}

	return nil
}

// GetEntriesWithoutSummary returns entries that don't have a summary yet.
func (s *Storage) GetEntriesWithoutSummary(userID int64, feedIDs []int64, limit int) (model.Entries, error) {
	var query string
	var rows *sql.Rows
	var err error

	if len(feedIDs) > 0 {
		query = `
			SELECT e.id, e.user_id, e.feed_id, e.title, e.url, e.content, e.published_at
			FROM entries e
			WHERE e.user_id = $1
			  AND e.feed_id = ANY($2)
			  AND e.status != 'removed'
			  AND e.summary IS NULL
			ORDER BY e.published_at DESC
			LIMIT $3
		`
		rows, err = s.db.Query(query, userID, pq.Array(feedIDs), limit)
	} else {
		query = `
			SELECT e.id, e.user_id, e.feed_id, e.title, e.url, e.content, e.published_at
			FROM entries e
			WHERE e.user_id = $1
			  AND e.status != 'removed'
			  AND e.summary IS NULL
			ORDER BY e.published_at DESC
			LIMIT $2
		`
		rows, err = s.db.Query(query, userID, limit)
	}

	if err != nil {
		return nil, fmt.Errorf(`store: unable to fetch entries without summary: %v`, err)
	}
	defer rows.Close()

	entries := make(model.Entries, 0)
	for rows.Next() {
		var entry model.Entry
		err := rows.Scan(
			&entry.ID,
			&entry.UserID,
			&entry.FeedID,
			&entry.Title,
			&entry.URL,
			&entry.Content,
			&entry.Date,
		)
		if err != nil {
			return nil, fmt.Errorf(`store: unable to fetch entry row: %v`, err)
		}
		entries = append(entries, &entry)
	}

	return entries, nil
}

// GetEntriesWithoutEmbedding returns entries that don't have an embedding yet.
func (s *Storage) GetEntriesWithoutEmbedding(userID int64, limit int, maxAgeDays int) (model.Entries, error) {
	query := `
		SELECT e.id, e.user_id, e.feed_id, e.title, e.url, e.content, e.published_at
		FROM entries e
		WHERE e.user_id = $1
		  AND e.status != 'removed'
		  AND e.embedding IS NULL
		  AND e.published_at > NOW() - INTERVAL '1 day' * $2
		ORDER BY e.published_at DESC
		LIMIT $3
	`
	rows, err := s.db.Query(query, userID, maxAgeDays, limit)
	if err != nil {
		return nil, fmt.Errorf(`store: unable to fetch entries without embedding: %v`, err)
	}
	defer rows.Close()

	entries := make(model.Entries, 0)
	for rows.Next() {
		var entry model.Entry
		err := rows.Scan(
			&entry.ID,
			&entry.UserID,
			&entry.FeedID,
			&entry.Title,
			&entry.URL,
			&entry.Content,
			&entry.Date,
		)
		if err != nil {
			return nil, fmt.Errorf(`store: unable to fetch entry row: %v`, err)
		}
		entries = append(entries, &entry)
	}

	return entries, nil
}

// MarkFullTextFetched marks an entry as having full text fetched.
func (s *Storage) MarkFullTextFetched(entryID int64) error {
	query := `UPDATE entries SET full_text_fetched_at = NOW() WHERE id = $1`
	_, err := s.db.Exec(query, entryID)
	if err != nil {
		return fmt.Errorf(`store: unable to mark full text fetched: %v`, err)
	}

	return nil
}

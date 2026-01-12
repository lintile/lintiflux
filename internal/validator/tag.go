// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package validator // import "miniflux.app/v2/internal/validator"

import (
	"miniflux.app/v2/internal/locale"
	"miniflux.app/v2/internal/model"
	"miniflux.app/v2/internal/storage"
)

// ValidateTagCreation validates tag creation.
func ValidateTagCreation(store *storage.Storage, userID int64, request *model.TagCreationRequest) *locale.LocalizedError {
	if request.Name == "" {
		return locale.NewLocalizedError("error.tag_name_required")
	}

	if len(request.Name) > 255 {
		return locale.NewLocalizedError("error.tag_name_too_long")
	}

	if store.TagNameExists(userID, request.Name) {
		return locale.NewLocalizedError("error.tag_already_exists")
	}

	return nil
}

// ValidateTagModification validates tag modification.
func ValidateTagModification(store *storage.Storage, userID, tagID int64, request *model.TagModificationRequest) *locale.LocalizedError {
	if request.Name != nil {
		if *request.Name == "" {
			return locale.NewLocalizedError("error.tag_name_required")
		}

		if len(*request.Name) > 255 {
			return locale.NewLocalizedError("error.tag_name_too_long")
		}

		if store.AnotherTagExists(userID, tagID, *request.Name) {
			return locale.NewLocalizedError("error.tag_already_exists")
		}
	}

	return nil
}

// ValidateEntryTagRequest validates a request to add tags to an entry.
func ValidateEntryTagRequest(request *model.EntryTagRequest) *locale.LocalizedError {
	if len(request.TagIDs) == 0 {
		return locale.NewLocalizedError("error.tag_ids_required")
	}

	return nil
}

// ValidateEntryTagByNameRequest validates a request to add tags by name to an entry.
func ValidateEntryTagByNameRequest(request *model.EntryTagByNameRequest) *locale.LocalizedError {
	if len(request.TagNames) == 0 {
		return locale.NewLocalizedError("error.tag_names_required")
	}

	for _, name := range request.TagNames {
		if name == "" {
			return locale.NewLocalizedError("error.tag_name_required")
		}
		if len(name) > 255 {
			return locale.NewLocalizedError("error.tag_name_too_long")
		}
	}

	if request.Source != "" && request.Source != model.TagSourceManual && request.Source != model.TagSourceAuto {
		return locale.NewLocalizedError("error.invalid_tag_source")
	}

	return nil
}

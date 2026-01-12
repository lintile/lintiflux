// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package api // import "miniflux.app/v2/internal/api"

import (
	json_parser "encoding/json"
	"net/http"

	"miniflux.app/v2/internal/http/request"
	"miniflux.app/v2/internal/http/response/json"
	"miniflux.app/v2/internal/model"
	"miniflux.app/v2/internal/validator"
)

func (h *handler) getTags(w http.ResponseWriter, r *http.Request) {
	userID := request.UserID(r)
	includeCounts := request.QueryStringParam(r, "counts", "false")

	var tags model.Tags
	var err error

	if includeCounts == "true" {
		tags, err = h.store.TagsWithCount(userID)
	} else {
		tags, err = h.store.Tags(userID)
	}

	if err != nil {
		json.ServerError(w, r, err)
		return
	}

	json.OK(w, r, tags)
}

func (h *handler) createTag(w http.ResponseWriter, r *http.Request) {
	userID := request.UserID(r)

	var tagCreationRequest model.TagCreationRequest
	if err := json_parser.NewDecoder(r.Body).Decode(&tagCreationRequest); err != nil {
		json.BadRequest(w, r, err)
		return
	}

	if validationErr := validator.ValidateTagCreation(h.store, userID, &tagCreationRequest); validationErr != nil {
		json.BadRequest(w, r, validationErr.Error())
		return
	}

	tag, err := h.store.CreateTag(userID, &tagCreationRequest)
	if err != nil {
		json.ServerError(w, r, err)
		return
	}

	json.Created(w, r, tag)
}

func (h *handler) updateTag(w http.ResponseWriter, r *http.Request) {
	userID := request.UserID(r)
	tagID := request.RouteInt64Param(r, "tagID")

	tag, err := h.store.TagByID(userID, tagID)
	if err != nil {
		json.ServerError(w, r, err)
		return
	}

	if tag == nil {
		json.NotFound(w, r)
		return
	}

	var tagModificationRequest model.TagModificationRequest
	if err := json_parser.NewDecoder(r.Body).Decode(&tagModificationRequest); err != nil {
		json.BadRequest(w, r, err)
		return
	}

	if validationErr := validator.ValidateTagModification(h.store, userID, tag.ID, &tagModificationRequest); validationErr != nil {
		json.BadRequest(w, r, validationErr.Error())
		return
	}

	tagModificationRequest.Patch(tag)

	if err := h.store.UpdateTag(tag); err != nil {
		json.ServerError(w, r, err)
		return
	}

	json.OK(w, r, tag)
}

func (h *handler) removeTag(w http.ResponseWriter, r *http.Request) {
	userID := request.UserID(r)
	tagID := request.RouteInt64Param(r, "tagID")

	if !h.store.TagIDExists(userID, tagID) {
		json.NotFound(w, r)
		return
	}

	if err := h.store.RemoveTag(userID, tagID); err != nil {
		json.ServerError(w, r, err)
		return
	}

	json.NoContent(w, r)
}

func (h *handler) getEntryTags(w http.ResponseWriter, r *http.Request) {
	userID := request.UserID(r)
	entryID := request.RouteInt64Param(r, "entryID")

	entryTags, err := h.store.GetEntryTags(userID, entryID)
	if err != nil {
		json.ServerError(w, r, err)
		return
	}

	json.OK(w, r, entryTags)
}

func (h *handler) addTagsToEntry(w http.ResponseWriter, r *http.Request) {
	userID := request.UserID(r)
	entryID := request.RouteInt64Param(r, "entryID")

	var tagRequest model.EntryTagByNameRequest
	if err := json_parser.NewDecoder(r.Body).Decode(&tagRequest); err != nil {
		json.BadRequest(w, r, err)
		return
	}

	if validationErr := validator.ValidateEntryTagByNameRequest(&tagRequest); validationErr != nil {
		json.BadRequest(w, r, validationErr.Error())
		return
	}

	source := tagRequest.Source
	if source == "" {
		source = model.TagSourceManual
	}

	if err := h.store.AddTagsToEntryByName(userID, entryID, tagRequest.TagNames, source); err != nil {
		json.ServerError(w, r, err)
		return
	}

	entryTags, err := h.store.GetEntryTags(userID, entryID)
	if err != nil {
		json.ServerError(w, r, err)
		return
	}

	json.Created(w, r, entryTags)
}

func (h *handler) removeTagFromEntry(w http.ResponseWriter, r *http.Request) {
	userID := request.UserID(r)
	entryID := request.RouteInt64Param(r, "entryID")
	tagID := request.RouteInt64Param(r, "tagID")

	if err := h.store.RemoveTagFromEntry(userID, entryID, tagID); err != nil {
		json.ServerError(w, r, err)
		return
	}

	json.NoContent(w, r)
}

func (h *handler) getEntriesByTag(w http.ResponseWriter, r *http.Request) {
	userID := request.UserID(r)
	tagID := request.RouteInt64Param(r, "tagID")

	if !h.store.TagIDExists(userID, tagID) {
		json.NotFound(w, r)
		return
	}

	builder := h.store.NewEntryQueryBuilder(userID)
	builder.WithEntryTagID(tagID)
	builder.WithoutStatus(model.EntryStatusRemoved)
	builder.WithSorting("published_at", "DESC")
	builder.WithEnclosures()

	configureEntryQueryBuilder(builder, r)

	entries, err := builder.GetEntries()
	if err != nil {
		json.ServerError(w, r, err)
		return
	}

	count, err := builder.CountEntries()
	if err != nil {
		json.ServerError(w, r, err)
		return
	}

	json.OK(w, r, &entriesResponse{Total: count, Entries: entries})
}

func (h *handler) confirmAutoTag(w http.ResponseWriter, r *http.Request) {
	userID := request.UserID(r)
	entryID := request.RouteInt64Param(r, "entryID")
	tagID := request.RouteInt64Param(r, "tagID")

	if err := h.store.ConfirmAutoTag(userID, entryID, tagID); err != nil {
		json.ServerError(w, r, err)
		return
	}

	json.NoContent(w, r)
}

func (h *handler) dismissAutoTag(w http.ResponseWriter, r *http.Request) {
	userID := request.UserID(r)
	entryID := request.RouteInt64Param(r, "entryID")
	tagID := request.RouteInt64Param(r, "tagID")

	if err := h.store.RemoveTagFromEntry(userID, entryID, tagID); err != nil {
		json.ServerError(w, r, err)
		return
	}

	json.NoContent(w, r)
}
